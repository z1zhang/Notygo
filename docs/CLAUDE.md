# Notygo — CLAUDE.md

## 项目简介

Notygo 是自托管的 Webhook 事件路由与通知推送系统。接收外部 Webhook（如 Diun），通过映射→规则匹配→模板渲染→投递到钉钉/飞书/企微/Email/HTTP。

## 技术栈

- **后端**: Go 1.26, chi/v5, goose 迁移
- **数据库**: PostgreSQL（pgx v5 标准库接口 + 自定义 rebind 驱动，见 `internal/storage/pgdriver.go`）
- **前端**: React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui + TanStack Query + React Hook Form + Zod + sonner + marked/DOMPurify（位于 `web/`，react-i18next 双语 zh/en，编译期键校验）
- **部署**: Docker 多阶段构建（`web/` → dist 复制到镜像内 `web/dist`，Go 服务静态文件），单端口服务前后端

## 核心数据流

```
Webhook POST → Ingest(验证/限流/去重) → events 表（同事务入队 River）
  → event_process Job → applyMapping → evaluateRules → renderTemplate
  → 创建 deliveries（同事务入队 River）
  → delivery_send Job → sender.Send → 更新状态
  → River 退避重试 + stuck-sweep 周期任务兜底救援
```

## API 命名空间（单端口，路径区分数据面/控制面）

- 通知 API（数据面）：`POST /endpoint/{slug}` — webhook 接入，endpoint 级认证；**单数 /endpoint/ 刻意区别于 SPA 页面 /endpoints 与管理面 /api/v1/endpoints/，日志/转发规则一眼可辨**
- 管理 API（控制面）：`/api/v1/*`（登录有失败锁定 + 审计）+ SPA；`GET /api/v1/system` 暴露版本与非敏感运行配置（并发/限流/保留时长），供设置页展示

## 登录防爆破

- `LoginHandler` 每次成功/失败都写 `notygo_audit_logs`（action: `auth.login_success` / `auth.login_failed`，summary 含 `ip=... username=...`）
- 失败按 IP 和用户名双计数（`internal/api/login_guard.go`，内存滑动窗口），达阈值返回 429 `AUTH_001`；锁定期间的探测不再单独写审计（防审计表被刷爆）
- 用户名不存在时跑假 bcrypt 抹平时序（防用户名枚举）；ingest bearer 比较用 `subtle.ConstantTimeCompare`

## HTTP 渠道自定义请求体

- 优先级：`body_fields`（结构化树）> 默认 `{title, body, format}`；原始请求体（`body_template`）已按用户要求移除
- `body_fields` 是递归树 `BodyField{Key, ValueSource, Value, Type, Children}`（`internal/sender/http.go`）：叶子行值来源（JSON 键 `value_source`）取 `title`/`body`/`format`/`literal`（自定义），多个 key 可映射同一来源；分组行 `object`/`array` 嵌套，数组内子键名忽略（按位置）
- 组装走 `json.Marshal`（转义天然正确）；后端校验缺来源/非法来源/对象内空键重复键
- 前端 `ChannelsPage.tsx` 递归表单组件（分组行 + 叶子行缩进树）；zod v4 递归 `z.lazy` 推导会降级，resolver 需 `as Resolver<ChannelForm>` 收口

## 数据库

- **PostgreSQL**: 由 `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE` 拼装 URL 形式 DSN（`config.buildPostgresDSN`，`url.URL` 负责特殊字符转义与 IPv6 方括号）
- **连接层（pgx 原生）**：业务代码直接用 `*pgxpool.Pool`（MaxConns=50, MinConns=4, MaxConnLifetime=5m, MaxConnIdleTime=2m）；goose 只接受 `*sql.DB`，迁移经 `storage.OpenMigrationsDB`（pgx stdlib 从同一 DSN 派生）执行，业务代码不碰
- 迁移文件：`internal/storage/migrations/postgres/`（goose postgres 方言）
- **业务表命名（2026-10-26 起）**：全部加 `notygo_` 前缀（`notygo_users` 等）；`river_job` 系列是 River 库的表，**不要**加前缀也不要在 goose 里建它
- **占位符约定**：SQL 直接写 $n（pgx 原生风格），**拼接多条字符串字面量的语句必须全局连续编号**（每个字面量各自从 $1 重排是历史 bug 高发区，`test/e2e_test.go` 会抓）
- **时间**：列类型 TIMESTAMPTZ，Go 侧直接传/扫 `time.Time`（timestamptz 是绝对时间，无 MySQL DATETIME 的时区歧义）；SQL 内当前时间用 `storage.NowExpr()`（= `now()`）
- **JSONB**：字符串参数写 JSONB 列时须带显式 `?::jsonb` 转型（协议层面 text→jsonb 有歧义）；查询侧字符串比较参数带 `?::timestamptz` 同理
- **布尔**：直接传/扫 `bool`，无 boolToInt/intToBool
- **claim 查询**：调度已交给 River（`river_job` 表由 River 自带 migrator 管理，**不要**在 goose 迁移里复制其 DDL）；业务表侧的幂等抢占用 `UPDATE ... WHERE status = 期望值 RETURNING ...` 单条原子语句（MarkProcessing / MarkAttemptInProgress），重复 Job 无害
- **枚举语义列（status / run_type / format / auth_type）**：DB 存 SMALLINT 数字码 + CHECK 约束，码表注释在迁移文件头；Go 业务逻辑与 API 仍是字符串常量（真源），转换只发生在仓库层——每包 `codes.go` 提供 encode/decode（未知值报错）。`notygo_channels.type` 是开放扩展点，**保持字符串**

### 常见陷阱

1. **严禁在 Go 反引号字符串内放函数调用**（Go 会当字面文本），必须用 `"` + 字符串拼接
2. `DELETE ... LIMIT` 在 PG 中不存在，限量删除用 `WHERE id IN (SELECT id ... ORDER BY ... LIMIT n)` 子查询
3. `INSERT IGNORE` → `ON CONFLICT DO NOTHING`（靠 `RowsAffected()==0` 判定冲突跳过）
4. 迁移链已于 2026-10-22 压平为单一 `001_init.sql` 基线（数据库重建为空库后合并 002–011 增量）；只适用全新数据库，老库（goose 版本 ≥ 002）不可直接切到此版本

## 目录结构

```
cmd/server/          — main.go 入口
internal/
  api/               — HTTP handlers + middleware + router
  config/            — 环境变量配置
  delivery/          — 投递模型/仓库/发送服务（错误分类：永久/临时二分）
  queue/             — River 队列管理器/Worker/重试策略/周期任务
  channel/           — 渠道（投递目标）
  event/             — 事件模型/仓库
  ingest/            — Webhook 入口 + 限流
  mapping/           — 字段映射
  processor/         — 事件处理器（映射→规则→模板→投递）
  rules/             — 规则模型/仓库
  sender/            — 发送器：dingtalk, feishu, wecom, email, http
  endpoint/          — 接入点（Webhook 来源）
  storage/           — 数据库打开/迁移/rebind 驱动
    migrations/postgres/ — PostgreSQL 迁移
  templates/         — 模板
  secret/            — 加解密服务
  audit/             — 审计日志
  observability/     — 日志/RequestID
web/                — React 前端（目录原名 frontend/，2026-10-18 更名）
  src/app/           — router.tsx / providers.tsx / query-client.ts
  src/components/
    ui/              — shadcn/ui 基础组件（button, dialog, table, select…）
    layout/          — AppShell（侧栏+顶栏）、PageHeader、ThemeToggle、LocaleSwitch
    data-display/    — Pagination, StatusBadge, TruncatedId, JsonViewer, ResizableSheet, DetailList, TableSkeleton, CopyButton
    feedback/        — EmptyState, ErrorState, ConfirmDialog
  src/features/      — 每页一目录：auth, dashboard, endpoints, mappings, rules, templates, channels, events, deliveries, audit, playground, settings
    channels/modules/  — 渠道表单插件模块：每个 ChannelModule 含 formSchema/defaults/Fields/toConfig/fromConfig/validate（镜像后端 sender 注册表）；index.ts 组装 CHANNEL_TYPES/CHANNELS/channelSchema/defaultForm，新增渠道 = 一个模块文件 + index.ts 一行注册
  src/hooks/         — usePagination, use-lookup（选项加载 + lookupName）
  src/i18n/          — react-i18next（zh/en，标准用法）：组件直接 `useTranslation()` 拿 `t`；语言切换用 `useLocale()`（LocaleSwitch）；主文案 locales/{zh,en}.json，新增键放 locales/extra/<group>.{zh,en}.json 并在 index.tsx 注册。键编译期类型校验（zh 为键集真源，en 缺键 tsc 报错）；插值 `{name}` 单花括号（escapeValue: false，React 自己转义）；复数传 `count` + JSON 键 `_one`/`_other` 后缀；内置格式化器 `{v, number}`、`{v, datetime(dateStyle: medium)}`、`{ts, relative}`（relative 为注册的自定义 formatter，其余是 i18next v26 内置）
  src/lib/           — api-client.ts（fetch 封装 + 401 处理）、api.ts（全部端点）、types.ts（后端 DTO）、formatters.ts、utils.ts(cn)
  src/styles/        — globals.css（Tailwind 4 + shadcn 设计 token，深浅双主题）
deploy/              — docker-compose.yml
```

## 前端关键约定

- **API 契约**：类型与请求全部在 `src/lib/api.ts` / `types.ts`，页面不得直接 fetch
- **认证**：Bearer token 存 localStorage（`token`/`expiresAt`），401 全局清除并跳转 /login
- **密钥语义**：endpoints 与 channels 均返回解密明文（前端用 `originalSecretRef` 记原值、仅变更时提交，留空 = 保持不变；"configured"/"not_configured" 标记仅作解密失败/未配置时的回退，前端要过滤）；启停开关 endpoints/channels 只发 `{enabled}`
- **规则更新**：PUT /rules/{id} 校验 name，启停开关必须回传完整规则对象
- **状态枚举**：event: received/processing/completed/deduped/failed；delivery: pending/processing/delivered/retry_wait/failed/dead_letter（dead_letter 不可重试）
- **Markdown/HTML 渲染**：一律 DOMPurify.sanitize 后再 dangerouslySetInnerHTML
- **深浅主题**：next-themes，class 策略，storageKey `notygo-theme`；语言存 `notygo-locale`

## 在线演示模式（Netlify）

- `VITE_DEMO_MODE=true`（构建期 env）时，`api-client.ts` 的所有请求经动态 import 转给 `web/src/lib/demo/handler.ts`——纯前端 mock 层，正常构建不会加载该 chunk，后端零改动
- `demo/seed.ts` 种子数据（Diun + CI 场景，时间相对 now 生成）→ `demo/store.ts` 存 localStorage `notygo-demo-db`，清站点数据即重置；登录任意非空账密放行（登录页 `login.demoHint` 提示）
- `demo/preview.ts` 用 JS 复刻了后端预览管线：gjson 路径子集（`#`/`#.field`/数字下标）、multi_result_mode（first/array/count/join）、to_upper/to_lower/trim、Go 模板安全子集（字段访问 + upper/lower/trim/title/len/default + range + `{{- -}}`，缺失键输出 `<no value>`）。**改后端映射/模板行为时记得同步这里**
- `demo/handler.ts` 复刻 `router.go` 的路由：五实体 CRUD（merge 部分更新 + revision 递增 + 审计）、事件/投递详情（latest_run / attempts）、test/replay、retry 与 bulk-retry（模拟立即送达，真后端是重置为 pending）、dashboard 实时计算
- 冒烟测试：`cd web && node --experimental-strip-types --import ./scripts/register-ts-resolve.mjs scripts/demo-smoke.mts`
- 部署：仓库根 `netlify.toml`（base=web，Netlify 构建时注入 VITE_DEMO_MODE=true）；Docker 镜像构建不注入该 env，始终正常模式



| 变量 | 默认 | 说明 |
|---|---|---|
| LISTEN_ADDR | :9003 | 监听地址 |
| DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME | 127.0.0.1 / 5432 / 必填 / 必填 / notygo | PostgreSQL 连接独立变量（拼装 DSN） |
| DB_SSLMODE | disable | PostgreSQL sslmode（云托管实例一般需 require/verify-full） |
| ENCRYPTION_MASTER_KEY | 必填 | v1:base64... |
| ADMIN_INITIAL_USER | admin | 首次启动创建的管理员用户名 |
| ADMIN_INITIAL_PASSWORD | | 首次启动创建管理员用户的密码 |
| LOGIN_MAX_FAILURES | 5 | 登录失败锁定阈值（按 IP 与用户名分别计数，内存实现） |
| LOGIN_LOCKOUT_SECONDS | 900 | 登录锁定时长（秒） |
| LOG_LEVEL | info | |
| QUEUE_EVENT_CONCURRENCY | 4 | event_process 队列并发（River） |
| QUEUE_DELIVERY_CONCURRENCY | 8 | delivery_send 队列并发（River） |
| QUEUE_RETRY_BASE_DELAY | 30s | 重试退避基准间隔（指数退避） |
| QUEUE_RETRY_MAX_DELAY | 1h | 重试退避上限 |
| QUEUE_RETRY_JITTER_PERCENT | 20 | 重试退避抖动百分比 |
| RIVER_JOB_RETENTION | 24h | 已完成/取消/丢弃任务保留时长 |
| MAX_PAYLOAD_BYTES | 1048576 | Webhook Payload 最大字节数 |
| INGEST_GLOBAL_RATE_LIMIT | 100/200 | 全局限流（rate/burst） |

## 后端已修复项

- ✅ **regex 操作符**：`strings.Contains` → `regexp.Compile` + `MatchString`（256 字符上限 + 无效模式警告）
- ✅ **新增操作符**：`not_contains`、`not_exists`
- ✅ **Endpoint slug**：6 位随机 → 32 位 UUID v4（crypto/rand，无第三方依赖）
- ✅ **go.mod**：直接依赖已正确分离，不再全部标 indirect
- ✅ **Channel 密钥**：API 返回完整解密值（之前误返回加密密文）

## 已知问题与待办

- 规则评估只看映射字段，未映射的字段规则引用不到
- `notygo_channels/{id}/test` 已实现（2026-10-21）：真实测试发送，复用 `channel.BuildSenderConfig`（从 delivery worker 抽出共享），审计 action `test`
- Dashboard 统计用 4 次 API 请求拼凑，应改为专用后端端点
- Settings 页的系统配置项（保留天数、并发数等）无 API 暴露，只显示环境变量说明（诚实降级）
- `/favicon.svg` 由 SPA fallback 返回 index.html（Go 端只服务 /assets/*；与旧版行为一致）
- 本机无 docker 命令，Docker 构建未在本会话实际验证

## 常用命令

```bash
go build ./...                       # 构建检查
go test ./...                        # 运行测试（无 DSN 时 e2e 自动跳过）
TEST_DATABASE_DSN="postgres://notygo:notygo@master:5432/notygo?sslmode=disable" go test ./test/ -v   # e2e 集成测试（真库）
cd web && npx tsc -p tsconfig.app.json --noEmit  # 前端类型检查
cd web && npm run build         # 前端生产构建（含 tsc -b）
docker build -t Notygo .              # Docker 构建（web/ → web/dist）
```

## UI e2e（web/scripts/ui-e2e/）

Chrome-CDP 驱动真实页面的功能测试（endpoints/mappings/rules/templates 四脚本，endpoints 含数据面用例）。前置：Chrome 开 `--remote-debugging-port=9222`、服务跑在 :9003：

```bash
cd web && node scripts/ui-e2e/endpoints.mjs --pass <管理员密码>   # 其余参数 --base --user --cdp 可选
```

- 选择器锚定 zh 文案与 radix 结构；**ConfirmDialog 是 AlertDialog（role=alertdialog）**，编辑表单才是 role=dialog
- 失败截图存 `scripts/ui-e2e/artifacts/`——**不要把截图读进上下文**（当前模型不支持图片输入），调试用 `locator.ariaSnapshot()` 输出文本
- `npm run build` 后正在开的旧标签页要 F5；脚本自身已通过 CDP 禁用缓存免疫此问题

## 压测工具（test/load）

对运行中的服务压**完整处理管线**：默认（`-pipeline`）自动装配 4 个模板 + 4 条规则（`channel_ids` 留空，不产生投递任务）+ 3 个映射/接入点，payload 轮换取值让每条规则命中 1/4 的事件、整个规则集每次都被评估；worker 分散到 3 个接入点。压完轮询 `/api/v1/queue/stats` 量队列排空速度（真实消费吞吐）：

```bash
go run ./test/load -duration 60s -concurrency 10            # 管线模式，不限速（会撞默认全局限流 100/200 出 429）
go run ./test/load -duration 60s -concurrency 10 -rps 500   # 限 500 rps；服务端先调高 INGEST_GLOBAL_RATE_LIMIT
go run ./test/load -pipeline=false -duration 30s            # 只压接收（不建映射/规则）
go run ./test/load -rules 8 -endpoints 5                    # 自定义规则数与接入点数
```

输出：RPS、p50/p90/p99/p99.9/max 延迟、状态码分布（429 会提示限流）、队列排空耗时与消费吞吐。要加投递链路压力，给规则配上真实渠道再压（本地 127.0.0.1 渠道会被 SSRF 防护拦截，属预期行为）


## 端到端集成测试（test/e2e_test.go）

- **门控**：读 `TEST_DATABASE_DSN`，未设置即 skip——`go test ./...` 在无库机器上仍然全绿
- **隔离**：每次运行在目标库里建独立 schema（`notygo_test_<ulid>`），迁移和全部查询经 `search_path` 落在该 schema，结束 `DROP SCHEMA ... CASCADE`；dev 数据零接触
- **装配**：`newTestStack` 按 `cmd/server/main.go` 的接线在进程内拉起完整栈（pgxpool + 迁移 + River + 全部仓库 + 事件处理器 + httptest），队列用毫秒级退避参数
- **断言**：异步链路（事件处理、投递重试）用 200ms 轮询 + 超时断言，正常 1 秒内收敛
- **覆盖矩阵**：登录（成功/失败/防爆破）→ 五类配置 CRUD → ingest → 映射 → 规则匹配 → 模板渲染 → 投递创建 → 发送失败 → retry_wait → 自动重试耗尽 → failed → 手动重试 → 批量重试 → 事件重放 → 状态/渠道过滤列表 → 审计 → 仪表盘 → 健康检查
- **回归意义**：占位符编号错乱、状态码/字符串混用（BulkResetPending 曾踩）、状态机竞态、动态 WHERE 拼接这类「只有连真库才炸」的错误全被覆盖；改 SQL / 状态机 / 装配代码后先跑它

## 设计文档

`webhook_event_routing_engine_design.md` — 完整的产品与技术设计文档（中文，~1500行）

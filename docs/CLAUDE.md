# zPush — CLAUDE.md

## 项目简介

zPush 是自托管的 Webhook 事件路由与通知推送系统。接收外部 Webhook（如 Diun），通过映射→规则匹配→模板渲染→投递到钉钉/飞书/企微/Email/HTTP。

## 技术栈

- **后端**: Go 1.26, chi/v5, goose 迁移
- **数据库**: MySQL（go-sql-driver/mysql）
- **前端**: React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui + TanStack Query + React Hook Form + Zod + sonner + marked/DOMPurify（位于 `web/`，react-i18next 双语 zh/en，编译期键校验）
- **部署**: Docker 多阶段构建（`web/` → dist 复制到镜像内 `web/dist`，Go 服务静态文件），单端口服务前后端

## 核心数据流

```
Webhook POST → Ingest(验证/限流/去重) → events 表
  → EventProcessor ClaimEvent → applyMapping → evaluateRules → renderTemplate
  → 创建 deliveries → DeliveryWorker ClaimDelivery → sender.Send → 更新状态
  → Lease Recovery 定时回收超时 claim
```

## API 命名空间（单端口，路径区分数据面/控制面）

- 通知 API（数据面）：`POST /ingest/{source_slug}` — webhook 接入，source 级认证；**旧路径 `/api/v1/ingest/{slug}` 已移除（改挂顶层 `/ingest/{slug}`）**
- 管理 API（控制面）：`/api/v1/*`（登录有失败锁定 + 审计）+ SPA

## 登录防爆破

- `LoginHandler` 每次成功/失败都写 `audit_logs`（action: `auth.login_success` / `auth.login_failed`，summary 含 `ip=... username=...`）
- 失败按 IP 和用户名双计数（`internal/api/login_guard.go`，内存滑动窗口），达阈值返回 429 `AUTH_001`；锁定期间的探测不再单独写审计（防审计表被刷爆）
- 用户名不存在时跑假 bcrypt 抹平时序（防用户名枚举）；ingest bearer 比较用 `subtle.ConstantTimeCompare`

## HTTP 渠道自定义请求体

- 优先级：`body_fields`（结构化树）> 默认 `{title, body, format}`；原始请求体（`body_template`）已按用户要求移除
- `body_fields` 是递归树 `BodyField{Key, Source, Value, Type, Children}`（`internal/sender/http.go`）：叶子行值来源 `title`/`body`/`format`/`literal`（自定义），多个 key 可映射同一来源；分组行 `object`/`array` 嵌套，数组内子键名忽略（按位置）
- 组装走 `json.Marshal`（转义天然正确）；后端校验缺来源/非法来源/对象内空键重复键
- 前端 `DestinationsPage.tsx` 递归表单组件（分组行 + 叶子行缩进树）；zod v4 递归 `z.lazy` 推导会降级，resolver 需 `as Resolver<DestinationForm>` 收口

## 数据库

- **MySQL**: 由 `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME` 拼装 DSN（`config.buildMySQLDSN`，`mysql.Config.FormatDSN` 负责特殊字符转义）
- 连接池：MaxOpenConns=25, MaxIdleConns=10, ConnMaxLifetime=5m, ConnMaxIdleTime=2m
- 迁移文件：`internal/storage/migrations/mysql/`

### MySQL 踩坑要点

1. **UPDATE 子查询不能直接引用同表**，必须双层嵌套：
   ```sql
   -- ❌ MySQL 报错
   UPDATE events SET ... WHERE id = (SELECT id FROM events WHERE ...)
   -- ✅ 双层嵌套绕过
   UPDATE events SET ... WHERE id = (SELECT id FROM (SELECT id FROM events WHERE ...) AS claimable)
   ```
2. **DATETIME 列不接受 `T`/`Z` 和纳秒**，用 `storage.FormatTime()` 而非 `time.RFC3339Nano`
3. **严禁在 Go 反引号字符串内放函数调用**（Go 会当字面文本），必须用 `"` + 字符串拼接
4. MySQL 002 迁移是 no-op（001 建表已包含 rendered_format 列）

## 目录结构

```
cmd/server/          — main.go 入口
internal/
  api/               — HTTP handlers + middleware + router
  config/            — 环境变量配置
  delivery/          — 投递模型/仓库/worker/熔断器/lease recovery
  destination/       — 投递目标
  event/             — 事件模型/仓库
  ingest/            — Webhook 入口 + 限流
  mapping/           — 字段映射
  processor/         — 事件处理器（映射→规则→模板→投递）
  rules/             — 规则模型/仓库
  sender/            — 发送器：dingtalk, feishu, wecom, email, http
  source/            — Webhook 来源
  storage/           — 数据库打开/迁移/时间格式化
    migrations/mysql/   — MySQL 迁移
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
  src/features/      — 每页一目录：auth, dashboard, sources, mappings, rules, templates, destinations, events, deliveries, audit, playground, settings
    destinations/channels/  — 渠道表单插件模块：每个 ChannelModule 含 formSchema/defaults/Fields/toConfig/fromConfig/validate（镜像后端 sender 注册表）；index.ts 组装 DEST_TYPES/CHANNELS/destinationSchema/defaultForm，新增渠道 = 一个模块文件 + index.ts 一行注册
  src/hooks/         — usePagination, use-lookup（选项加载 + lookupName）
  src/i18n/          — react-i18next（zh/en，标准用法）：组件直接 `useTranslation()` 拿 `t`；语言切换用 `useLocale()`（LocaleSwitch）；主文案 locales/{zh,en}.json，新增键放 locales/extra/<group>.{zh,en}.json 并在 index.tsx 注册。键编译期类型校验（zh 为键集真源，en 缺键 tsc 报错）；插值 `{name}` 单花括号（escapeValue: false，React 自己转义）；复数传 `count` + JSON 键 `_one`/`_other` 后缀；内置格式化器 `{v, number}`、`{v, datetime(dateStyle: medium)}`、`{ts, relative}`（relative 为注册的自定义 formatter，其余是 i18next v26 内置）
  src/lib/           — api-client.ts（fetch 封装 + 401 处理）、api.ts（全部端点）、types.ts（后端 DTO）、formatters.ts、utils.ts(cn)
  src/styles/        — globals.css（Tailwind 4 + shadcn 设计 token，深浅双主题）
deploy/              — docker-compose.yml
```

## 前端关键约定

- **API 契约**：类型与请求全部在 `src/lib/api.ts` / `types.ts`，页面不得直接 fetch
- **认证**：Bearer token 存 localStorage（`token`/`expiresAt`），401 全局清除并跳转 /login
- **密钥语义**：sources 返回 `secret_ref` 为 "configured"/"not_configured" 标记（绝不回传）；destinations 返回解密明文（仅变更时提交）；启停开关 sources/destinations 只发 `{enabled}`
- **规则更新**：PUT /rules/{id} 校验 name，启停开关必须回传完整规则对象
- **状态枚举**：event: received/processing/completed/deduped/failed；delivery: pending/processing/delivered/retry_wait/failed/dead_letter（dead_letter 不可重试）
- **Markdown/HTML 渲染**：一律 DOMPurify.sanitize 后再 dangerouslySetInnerHTML
- **深浅主题**：next-themes，class 策略，storageKey `zpush-theme`；语言存 `zpush-locale`

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| LISTEN_ADDR | :9003 | 监听地址 |
| DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME | 127.0.0.1 / 3306 / 必填 / 必填 / zpush | MySQL 连接独立变量（拼装 DSN） |
| ENCRYPTION_MASTER_KEY | 必填 | v1:base64... |
| ADMIN_INITIAL_USER | admin | 首次启动创建的管理员用户名 |
| ADMIN_INITIAL_PASSWORD | | 首次启动创建管理员用户的密码 |
| LOGIN_MAX_FAILURES | 5 | 登录失败锁定阈值（按 IP 与用户名分别计数，内存实现） |
| LOGIN_LOCKOUT_SECONDS | 900 | 登录锁定时长（秒） |
| LOG_LEVEL | info | |
| EVENT_PROCESSOR_CONCURRENCY | 4 | |
| DELIVERY_WORKER_CONCURRENCY | 8 | |
| CLAIM_POLL_INTERVAL | 2s | Worker 认领轮询间隔 |
| LEASE_TIMEOUT | 30s | 认领租约超时（超时由回收器接管） |
| MAX_PAYLOAD_BYTES | 1048576 | Webhook Payload 最大字节数 |
| INGEST_GLOBAL_RATE_LIMIT | 100/200 | 全局限流（rate/burst） |

## 后端已修复项

- ✅ **regex 操作符**：`strings.Contains` → `regexp.Compile` + `MatchString`（256 字符上限 + 无效模式警告）
- ✅ **新增操作符**：`not_contains`、`not_exists`
- ✅ **Source slug**：6 位随机 → 32 位 UUID v4（crypto/rand，无第三方依赖）
- ✅ **go.mod**：直接依赖已正确分离，不再全部标 indirect
- ✅ **Destination 密钥**：API 返回完整解密值（之前误返回加密密文）

## 已知问题与待办

- 规则评估只看映射字段，未映射的字段规则引用不到
- `destinations/{id}/test` 已实现（2026-10-21）：真实测试发送，复用 `destination.BuildSenderConfig`（从 delivery worker 抽出共享），审计 action `test`
- Dashboard 统计用 4 次 API 请求拼凑，应改为专用后端端点
- Settings 页的系统配置项（保留天数、并发数等）无 API 暴露，只显示环境变量说明（诚实降级）
- `/favicon.svg` 由 SPA fallback 返回 index.html（Go 端只服务 /assets/*；与旧版行为一致）
- 本机无 docker 命令，Docker 构建未在本会话实际验证

## 常用命令

```bash
go build ./...                       # 构建检查
go test ./...                        # 运行测试
cd web && npx tsc -p tsconfig.app.json --noEmit  # 前端类型检查
cd web && npm run build         # 前端生产构建（含 tsc -b）
docker build -t zPush .              # Docker 构建（web/ → web/dist）
```

## 设计文档

`webhook_event_routing_engine_design.md` — 完整的产品与技术设计文档（中文，~1500行）

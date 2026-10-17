# zPush — CLAUDE.md

## 项目简介

zPush 是自托管的 Webhook 事件路由与通知推送系统。接收外部 Webhook（如 Diun），通过映射→规则匹配→模板渲染→投递到钉钉/飞书/企微/Email/HTTP Webhook。

## 技术栈

- **后端**: Go 1.26, chi/v5, goose 迁移
- **数据库**: MySQL（go-sql-driver/mysql）
- **前端**: React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui + TanStack Query + React Hook Form + Zod + sonner + marked/DOMPurify（位于 `frontend/`，自托管轻量 i18n，zh/en）
- **部署**: Docker 多阶段构建（`frontend/` → dist 复制到镜像内 `web/dist`，Go 服务静态文件），单端口服务前后端

## 核心数据流

```
Webhook POST → Ingest(验证/限流/去重) → events 表
  → EventProcessor ClaimEvent → applyMapping → evaluateRules → renderTemplate
  → 创建 deliveries → DeliveryWorker ClaimDelivery → sender.Send → 更新状态
  → Lease Recovery 定时回收超时 claim
```

## 数据库

- **MySQL**: `DATABASE_DSN=user:pass@tcp(host:3306)/zPush?parseTime=true`
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
  sender/            — 发送器：dingtalk, feishu, wecom, email, http_webhook
  source/            — Webhook 来源
  storage/           — 数据库打开/迁移/时间格式化
    migrations/mysql/   — MySQL 迁移
  templates/         — 模板
  secret/            — 加解密服务
  audit/             — 审计日志
  observability/     — 日志/RequestID
frontend/             — React 前端（重构自 Vue 版，旧 web/ 已删除）
  src/app/           — router.tsx / providers.tsx / query-client.ts
  src/components/
    ui/              — shadcn/ui 基础组件（button, dialog, table, select…）
    layout/          — AppShell（侧栏+顶栏）、PageHeader、ThemeToggle、LocaleSwitch
    data-display/    — Pagination, StatusBadge, TruncatedId, JsonViewer, ResizableSheet, DetailList, TableSkeleton, CopyButton
    feedback/        — EmptyState, ErrorState, ConfirmDialog
  src/features/      — 每页一目录：auth, dashboard, sources, mappings, rules, templates, destinations, events, deliveries, audit, playground, settings
  src/hooks/         — usePagination, use-lookup（选项加载 + lookupName）
  src/i18n/          — 自实现 i18n（zh/en）：主文案 locales/{zh,en}.json，新增键放 locales/extra/<group>.{zh,en}.json 并在 index.tsx 注册
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
| DATABASE_DSN | 必填 | MySQL 连接串 |
| ENCRYPTION_MASTER_KEY | 必填 | v1:base64... |
| ADMIN_INITIAL_PASSWORD | | 首次启动创建 admin |
| LOG_LEVEL | info | |
| EVENT_PROCESSOR_CONCURRENCY | 4 | |
| DELIVERY_WORKER_CONCURRENCY | 8 | |
| EVENT_RETENTION_DAYS | 30 | 保留天数（配置存在，清理逻辑未实现） |

## 后端已修复项

- ✅ **regex 操作符**：`strings.Contains` → `regexp.Compile` + `MatchString`（256 字符上限 + 无效模式警告）
- ✅ **新增操作符**：`not_contains`、`not_exists`
- ✅ **Source slug**：6 位随机 → 32 位 UUID v4（crypto/rand，无第三方依赖）
- ✅ **go.mod**：直接依赖已正确分离，不再全部标 indirect
- ✅ **Destination 密钥**：API 返回完整解密值（之前误返回加密密文）

## 已知问题与待办

- 规则评估只看映射字段，未映射的字段规则引用不到
- `destinations/{id}/test` 仍是 501 占位（前端未提供测试按钮）
- `EVENT_RETENTION_DAYS` 配置存在但没有定时清理逻辑
- Dashboard 统计用 4 次 API 请求拼凑，应改为专用后端端点
- Settings 页的系统配置项（保留天数、并发数等）无 API 暴露，只显示环境变量说明（诚实降级）
- `/favicon.svg` 由 SPA fallback 返回 index.html（Go 端只服务 /assets/*；与旧版行为一致）
- 本机无 docker 命令，Docker 构建未在本会话实际验证

## 常用命令

```bash
go build ./...                       # 构建检查
go test ./...                        # 运行测试
cd frontend && npx tsc -p tsconfig.app.json --noEmit  # 前端类型检查
cd frontend && npm run build         # 前端生产构建（含 tsc -b）
docker build -t zPush .              # Docker 构建（frontend/ → web/dist）
```

## 设计文档

`webhook_event_routing_engine_design.md` — 完整的产品与技术设计文档（中文，~1500行）

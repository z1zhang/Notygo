# zPush — CLAUDE.md

## 项目简介

zPush 是自托管的 Webhook 事件路由与通知推送系统。接收外部 Webhook（如 Diun），通过映射→规则匹配→模板渲染→投递到钉钉/飞书/企微/Email/HTTP Webhook。

## 技术栈

- **后端**: Go 1.26, chi/v5, goose 迁移
- **数据库**: MySQL（go-sql-driver/mysql）
- **前端**: Vue 3 + TypeScript + Element Plus + vue-i18n + Pinia + Vite + marked
- **部署**: Docker 多阶段构建，单端口服务前后端

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
web/src/             — 前端
  api/               — Axios 封装 + types.ts（所有后端响应类型定义）
  components/        — 共享组件：TruncatedId, StatusTag
  composables/       — 共享逻辑：useCrudPage, useOptions, useResizableDrawer
  utils/             — 共享工具：time, helpers, status, lookup
  assets/global.scss — 全局样式（表格行高、布局类、截断类）
  locales/           — zh.json / en.json 国际化
  views/             — 8 个 CRUD 视页 + Login + Dashboard + Settings
deploy/              — docker-compose.yml
```

## 前端复用层

前端已提取共享层，所有 8 个视图均使用共享模块，无独立重复代码：

| 层 | 模块 | 说明 |
|---|---|---|
| 工具 | `utils/time.ts` | `formatTime(val)` — 统一时间格式化，null 安全 |
| 工具 | `utils/helpers.ts` | `getErrorMessage(e, fallback)` — 统一错误消息提取 |
| 工具 | `utils/status.ts` | `eventStatusTagType` / `deliveryStatusTagType` / `actionTagType` |
| 工具 | `utils/lookup.ts` | `lookupName(options, id, fallback?)` — ID→名称查找 |
| 类型 | `api/types.ts` | 所有后端响应接口：Source/Mapping/Rule/Template/Destination/Event/Delivery/Audit + PaginatedResponse |
| Composable | `composables/useCrudPage.ts` | 列表/分页/删除/对话框状态管理 + i18n 删除确认 |
| Composable | `composables/useOptions.ts` | 下拉选项（sources/destinations/templates/mappings）加载 |
| Composable | `composables/useResizableDrawer.ts` | 可拖拽宽度侧栏（拖左边框调整宽度） |
| 组件 | `components/TruncatedId.vue` | ID 自适应截断 + ResizeObserver 溢出检测 + 悬浮 tooltip + 点击复制 |
| 组件 | `components/StatusTag.vue` | 标准化状态标签，kind="event"|"delivery"，自动映射 tag type |
| 全局样式 | `assets/global.scss` | 表格行高统一、`.page-actions`/`.page-filters`/`.pagination-wrap`/`.action-buttons` 布局、`.cell-ellipsis` flex 兼容截断 |

### 各视图对共享模块的使用

| 视图 | useCrudPage | useOptions | useResizableDrawer | StatusTag | TruncatedId | 主要 utils |
|------|:-:|:-:|:-:|:-:|:-:|---|
| Sources | ✅ | mappingOptions | — | — | — | helpers, lookup |
| Mappings | ✅ | — | — | — | — | helpers |
| Templates | ✅ | — | — | — | — | helpers |
| Destinations | ✅ | — | — | — | — | helpers |
| Rules | ✅ | templateOptions, destinationOptions | — | — | — | helpers, lookup |
| Events | — | sourceOptions, destinationOptions | ✅ | kind=event | ✅ | time, helpers, lookup |
| Deliveries | — | destinationOptions | ✅ | kind=delivery | ✅ | time, helpers, lookup |
| Audit | — | — | — | kind=audit | ✅ | time, helpers |
| Dashboard | — | — | — | 动态 kind | ✅ | time, helpers |

### 前端 UI 约定

- **表格行高**：`.el-table__cell { padding: 10px 0 }` + `.cell { display: flex; align-items: center; min-height: 32px }`
- **列宽策略**：固定内容（switch/tag/数字）用 `width`，可变内容（名称/ID/时间）用 `min-width`；侧栏表格全部用 `min-width`
- **文本截断**：`show-overflow-tooltip` 与 `display: flex` 不兼容，改用 `<el-tooltip>` + `.cell-ellipsis` 类
- **ID 显示**：使用 `<TruncatedId :id="row.id" />`，完整显示 + CSS 自适应截断 + 悬浮复制
- **Source slug**：表格只显示 slug，点击复制完整 webhook URL（`${origin}/api/v1/ingest/${slug}`）
- **侧栏**：`el-drawer` + `useResizableDrawer` 可拖拽宽度（初始 800px，范围 400-1200px）
- **el-descriptions 收缩**：需 `:deep(.el-descriptions__table) { table-layout: fixed; width: 100% }` 才能随 drawer 缩小
- **映射/规则编辑**：`resetForm()` 不预填空行，`fields = []` 而非 `[createEmptyField()]`
- **i18n**：删除确认按钮已国际化（`common.confirm` / `common.cancel`），复制反馈（`common.copied` / `common.copyFailed`）
- **无 monospace 覆盖**：ID/响应体等全部使用默认字体，不强制 monospace

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
- `/preview/parse`, `/preview/mapping`, `/preview/rule`, `/preview/delivery` 仍是 501 占位
- `destinations/{id}/test` 未实现
- `EVENT_RETENTION_DAYS` 配置存在但没有定时清理逻辑
- **XSS 风险**：`Deliveries.vue` 和 `Templates.vue` 使用 `v-html` 渲染 Markdown/HTML 内容，未经过 DOMPurify 消毒
- Dashboard 统计用 4 次 API 请求拼凑，应改为专用后端端点

## 常用命令

```bash
go build ./...              # 构建检查
go test ./...               # 运行测试
cd web && npx vue-tsc --noEmit  # 前端类型检查
docker build -t zPush .     # Docker 构建
```

## 设计文档

`webhook_event_routing_engine_design.md` — 完整的产品与技术设计文档（中文，~1500行）

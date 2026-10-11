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

- **MySQL**: `DATABASE_DSN=user:pass@tcp(host:3306)/zpush?parseTime=true`
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
web/src/             — 前端（views, api, locales, stores, router）
deploy/              — docker-compose.yml
```

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

## 已知问题与待办

- 规则评估只看映射字段，未映射的字段规则引用不到
- `/preview/parse`, `/preview/mapping`, `/preview/rule`, `/preview/delivery` 仍是 501 占位
- `destinations/{id}/test` 未实现
- `regex` 条件用 `strings.Contains` 模拟，需改为 `regexp`
- `EVENT_RETENTION_DAYS` 配置存在但没有定时清理逻辑
- go.mod 中直接依赖都标了 indirect，需 `go mod tidy`

## 常用命令

```bash
go build ./...              # 构建检查
go test ./...               # 运行测试
docker build -t zpush .     # Docker 构建
```

## 设计文档

`webhook_event_routing_engine_design.md` — 完整的产品与技术设计文档（中文，~1500行）

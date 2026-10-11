# zPush

自托管的 Webhook 事件路由与通知推送系统。接收外部 Webhook（如 [Diun](https://github.com/crazy-max/diun)），通过映射 → 规则匹配 → 模板渲染 → 投递到钉钉 / 飞书 / 企微 / Email / HTTP Webhook。

## ✨ 特性

- 🔄 **事件路由** — Webhook 接入 → 字段映射 → 规则匹配 → 模板渲染 → 多目标投递，完整的事件处理管线
- 📨 **多通道推送** — 内置钉钉、飞书、企业微信、Email、自定义 HTTP Webhook 五种发送器
- 🗄️ **MySQL 数据库** — MySQL 8.0，InnoDB 引擎，JSON 列类型
- 🔐 **安全设计** — 密钥加密存储、JWT 认证、请求限流、Payload 去重
- 🛡️ **可靠投递** — 基于 Lease 的任务认领、指数退避重试、熔断器、超时租约回收
- 🌐 **Web 管理界面** — Vue 3 + Element Plus，支持 i18n 国际化
- 🐳 **单容器部署** — Docker 多阶段构建，前后端合一，单端口运行

## 🏗️ 架构

```
Webhook POST → Ingest (验证 / 限流 / 去重) → events 表
  → EventProcessor ClaimEvent → applyMapping → evaluateRules → renderTemplate
  → 创建 deliveries → DeliveryWorker ClaimDelivery → sender.Send → 更新状态
  → Lease Recovery 定时回收超时 claim
```

## 🚀 快速开始

### Docker Compose（推荐）

```bash
# 克隆仓库
git clone https://github.com/z1zhang/zpush.git
cd zpush

# 生成加密主密钥（首次部署务必替换）
# 格式: v1:base64编码的32字节密钥
export ENCRYPTION_MASTER_KEY="v1:$(openssl rand -base64 32)"

# 设置管理员初始密码
export ADMIN_INITIAL_PASSWORD="your-secure-password"

# 启动
cd deploy && docker compose up -d
```

服务默认监听 `9003` 端口，访问 `http://localhost:9003` 进入管理界面

### 手动构建

```bash
# 前端
cd web && npm ci && npm run build

# 后端
go build -o zpush ./cmd/server/

# 运行
ENCRYPTION_MASTER_KEY="v1:your-base64-key" ./zpush
```

## ⚙️ 配置

所有配置通过环境变量完成：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `:9003` | 监听地址 |
| `DATABASE_DSN` | **必填** | MySQL 连接串，格式：`user:pass@tcp(host:3306)/zpush?parseTime=true` |
| `ENCRYPTION_MASTER_KEY` | **必填** | 加密主密钥，格式 `v1:base64...` |
| `ADMIN_INITIAL_PASSWORD` | — | 首次启动时创建 admin 用户的密码 |
| `LOG_LEVEL` | `info` | 日志级别 |
| `EVENT_PROCESSOR_CONCURRENCY` | `4` | 事件处理器并发数 |
| `DELIVERY_WORKER_CONCURRENCY` | `8` | 投递 Worker 并发数 |
| `MAX_PAYLOAD_BYTES` | `1048576` | Webhook Payload 最大字节数（默认 1MB） |
| `INGEST_GLOBAL_RATE_LIMIT` | `100/200` | 全局限流（rate/burst 格式） |
| `EVENT_RETENTION_DAYS` | `30` | 事件保留天数 |

### MySQL 配置

Docker Compose 已默认包含 MySQL 服务，启动即可：

```bash
DATABASE_DSN=user:password@tcp(mysql:3306)/zpush?parseTime=true
```

## 🛠️ 技术栈

**后端**

- Go 1.26
- [chi/v5](https://github.com/go-chi/chi) — HTTP 路由
- [goose/v3](https://github.com/pressly/goose) — 数据库迁移
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) — MySQL 驱动
- [tidwall/gjson](https://github.com/tidwall/gjson) — JSON 路径查询
- [sethvargo/go-retry](https://github.com/sethvargo/go-retry) — 重试策略

**前端**

- Vue 3 + TypeScript
- Element Plus — UI 组件库
- vue-i18n — 国际化
- Pinia — 状态管理
- Vite — 构建工具
- marked — Markdown 渲染

## 📁 项目结构

```
cmd/server/           — 程序入口
internal/
  api/                — HTTP handlers + middleware + router
  config/             — 环境变量配置
  delivery/           — 投递模型 / 仓库 / Worker / 熔断器 / Lease Recovery
  destination/        — 投递目标
  event/              — 事件模型 / 仓库
  ingest/             — Webhook 入口 + 限流
  mapping/            — 字段映射
  processor/          — 事件处理器（映射 → 规则 → 模板 → 投递）
  rules/              — 规则模型 / 仓库
  sender/             — 发送器：dingtalk, feishu, wecom, email, http_webhook
  source/             — Webhook 来源
  storage/            — 数据库打开 / 迁移 / 时间格式化
    migrations/mysql/   — MySQL 迁移脚本
  templates/          — 模板
  secret/             — 加解密服务
  audit/              — 审计日志
  observability/      — 日志 / RequestID
web/src/              — 前端（views, api, locales, stores, router）
deploy/               — docker-compose.yml
```

## 📋 常用命令

```bash
go build ./...                # 构建检查
go test ./...                 # 运行测试
docker build -t zpush .       # Docker 构建
```

## 📝 待办

- [ ] 规则评估支持未映射的原始字段
- [ ] Preview API 实现（`/preview/parse`, `/preview/mapping`, `/preview/rule`, `/preview/delivery`）
- [ ] 投递目标测试发送（`destinations/{id}/test`）
- [ ] `regex` 条件改用 `regexp` 包实现
- [ ] `EVENT_RETENTION_DAYS` 定时清理逻辑
- [ ] `go.mod` 依赖整理（`go mod tidy`）

## 📄 许可证

MIT License

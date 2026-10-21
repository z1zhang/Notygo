# zPush

自托管的 Webhook 事件路由与通知推送系统

接收外部 Webhook，经过**字段映射 → 规则匹配 → 模板渲染**的事件处理管线，最终投递到钉钉 / 飞书 / 企业微信 / Email / 自定义 HTTP 等渠道

## ✨ 特性

- 🔄 **完整事件管线** — Webhook 接入（验证 / 限流 / 去重）→ 字段映射 → 规则匹配 → 模板渲染 → 多目标投递，全链路可视、可回放、可重试
- 📨 **五种投递渠道** — 内置钉钉、飞书、企业微信、Email（SMTP）、自定义 HTTP（Webhook）发送器
- 🗺️ **灵活的字段映射** — JSON Path 提取任意嵌套字段，支持类型转换、默认值回退、结果转换（transform）与多结果模式
- ⚖️ **规则引擎** — 11 种操作符（`eq` / `neq` / `contains` / `not_contains` / `regex` / `gt` / `lt` / `gte` / `lte` / `exists` / `not_exists`），AND / OR 任意组合
- 📄 **模板渲染** — `{{.field}}` 占位符语法，纯文本 / Markdown / HTML 三种渲染格式，前端一律 DOMPurify 消毒后展示
- 🛡️ **可靠投递** — 基于 Lease 的任务认领、指数退避重试、超时租约自动回收，进程崩溃不丢任务
- 🔐 **安全设计** — 渠道密钥与数据源密钥 AES-256-GCM 加密落库、登录失败锁定（防爆破）、全量登录审计、双层数据面/控制面认证隔离、全局 + 单数据源限流、Payload 去重
- 🌐 **现代管理界面** — React 19 + TypeScript + Tailwind CSS 4 + shadcn/ui，深浅双主题、中英双语、实时状态徽章、JSON 查看器、沙盒（Playground）调试
- 📊 **可观测** — Dashboard 统计、事件/投递全链路追踪、审计日志、结构化日志 + RequestID
- 🐳 **单容器部署** — 前后端打包为单一镜像、单端口运行，只需一个外部 MySQL

## 🏗️ 架构

```
Webhook POST → Ingest (验证 / 限流 / 去重) → events 表
  → EventProcessor ClaimEvent → applyMapping → evaluateRules → renderTemplate
  → 创建 deliveries → DeliveryWorker ClaimDelivery → sender.Send → 更新状态
  → Lease Recovery 定时回收超时 claim
```

**数据面与控制面在单端口内通过路径命名空间分离**，便于在反向代理层对两者施加不同策略（限流、IP 白名单等）：

| 命名空间 | 端点 | 认证 |
|---|---|---|
| 通知 API（数据面） | `POST /ingest/{source_slug}` | 数据源级 Bearer token（可选） |
| 管理 API（控制面） | `/api/v1/*` | 管理员会话 token |
| 健康检查 | `GET /healthz`、`GET /readyz` | 无 |

## 🚀 快速开始

镜像已发布至 Docker Hub：[`zhangz1o/zpush`](https://hub.docker.com/r/zhangz1o/zpush)

**前置要求**：一个可访问的 MySQL 8.0+ 实例（zPush 启动时自动建表、自动迁移，无需手工导入 SQL）。

两种部署方式任选其一。部署完成后访问 `http://<主机>:9003` 进入管理界面。

### 方式一：Docker 运行

```bash
# 生成加密主密钥（v1:base64 编码的 32 字节密钥，务必自行保存，丢失后已加密的密钥无法解密）
export ENCRYPTION_MASTER_KEY="v1:$(openssl rand -base64 32)"

docker run -d \
  --name zpush \
  -p 9003:9003 \
  -e DB_HOST="your-mysql-host" \
  -e DB_USER="zpush" \
  -e DB_PASSWORD="your-mysql-password" \
  -e DB_NAME="zpush" \
  -e ENCRYPTION_MASTER_KEY="$ENCRYPTION_MASTER_KEY" \
  -e ADMIN_INITIAL_USER="admin" \
  -e ADMIN_INITIAL_PASSWORD="your-admin-password" \
  zhangz1o/zpush:latest
```

> `ADMIN_INITIAL_USER` / `ADMIN_INITIAL_PASSWORD` 仅在数据库中尚无管理员时生效（首次启动创建管理员用户），之后修改密码不经过环境变量。

### 方式二：Docker Compose 部署

创建 `docker-compose.yml`（只包含 zPush 服务，MySQL 自备）：

```yaml
services:
  zpush:
    image: zhangz1o/zpush:latest
    container_name: zpush
    restart: unless-stopped
    ports:
      - "9003:9003"
    environment:
      # 必填：MySQL 连接信息（首次启动自动建表迁移）
      DB_HOST: "your-mysql-host"
      DB_USER: "zpush"
      DB_PASSWORD: "your-mysql-password"
      DB_NAME: "zpush"
      # DB_PORT: "3306"            # 可选，默认 3306
      # 必填：加密主密钥，生成命令：openssl rand -base64 32（前面加 v1:）
      # 丢失后已加密的渠道/数据源密钥将无法解密，务必持久保存
      ENCRYPTION_MASTER_KEY: "v1:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx="
      # 仅首次启动生效：创建管理员用户的用户名与初始密码
      ADMIN_INITIAL_USER: "admin"
      ADMIN_INITIAL_PASSWORD: "change-me-please"
      # 以下均可选，默认值见下方配置表
      LOG_LEVEL: "info"
      # INGEST_GLOBAL_RATE_LIMIT: "100/200"
      # LOGIN_MAX_FAILURES: "5"
      # LOGIN_LOCKOUT_SECONDS: "900"
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9003/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

```bash
docker compose up -d

# 查看日志 / 健康状态
docker compose logs -f zpush
docker compose ps
```

### 从源码构建（可选）

```bash
git clone https://github.com/z1zhang/zpush.git
cd zpush

# 前端
cd web && npm ci && npm run build && cd ..

# 后端（Dockerfile 同样执行此流程，产出单镜像）
go build -o zpush ./cmd/server/

# 运行
DB_HOST="127.0.0.1" DB_USER="zpush" DB_PASSWORD="..." ENCRYPTION_MASTER_KEY="v1:..." ./zpush
```

仓库 `deploy/docker-compose.yml` 提供了带 MySQL 的全栈编排（本地构建镜像），适合本地开发体验。

## 🧭 首次使用流程

1. 打开 `http://<主机>:9003`，用 `admin` + 初始密码登录（登录成功/失败均写审计日志，连续失败会触发临时锁定）
2. **数据源（Source）**：创建数据源，获得 Webhook 接入 URL `http://<主机>:9003/ingest/{slug}`；可按数据源配置 Bearer 认证、限流、Payload 大小上限
3. **映射（Mapping）**：定义 JSON Path 提取规则，把原始 Payload 转成统一字段
4. **规则（Rule）**：基于映射字段做条件匹配，决定哪些事件触发投递
5. **模板（Template）**：用 `{{.field}}` 占位符编写通知内容
6. **渠道（Destination）**：配置钉钉/飞书/企微/Email/HTTP，密钥加密存储
7. 把数据源的 `/ingest/{slug}` URL 配到外部系统（如 Diun 的 webhook 通知地址），事件开始流转
8. 在**事件/投递**页面查看全链路状态，失败可单条或批量重试，也可整事件重放

**沙盒（Playground）** 支持解析、映射、模板、投递四个环节的实时预览调试，无需真实触发 Webhook。

## 📮 接入任意通知服务（HTTP 渠道）

除了内置的钉钉 / 飞书 / 企微 / Email，任何提供 HTTP 接口的服务都能通过 **HTTP 渠道**接入，例如 Telegram Bot、Bark、Discord、ntfy、企业内部网关等，无需写代码

在管理界面创建渠道时选择类型 `HTTP`：

| 配置项 | 说明 |
|---|---|
| 目标 URL | 服务的 API 地址，如 `https://api.telegram.org/bot<TOKEN>/sendMessage` |
| 请求方法 | POST / GET / PUT / PATCH |
| 自定义请求头 | 任意 Header（如 `Authorization: Bearer xxx`），默认已带 `Content-Type: application/json` |
| 密钥 | AES-256-GCM 加密落库 |
| 超时 | 默认 10000ms |

### 自定义请求体

不配置时默认发送：

```json
{ "title": "模板渲染的标题", "body": "模板渲染的内容", "format": "text" }
```

服务要求的报文格式不同时，用**自定义请求体**重新组装：添加字段并选择**值来源**——

- **模板标题 / 模板内容 / 格式**：取渲染结果，多个 key 可以映射同一个来源
- **自定义**：写死一个固定值（如 Telegram 的 `chat_id`）

字段的类型可以切成**对象**或**数组**实现嵌套（如 Discord 的 `embeds`），数组内的元素按位置排列、不需要键名。所有 JSON 转义由服务端处理，标题内容里的引号、换行不会破坏报文。

平铺示例（Telegram 风格）：

```json
{
  "url": "https://api.telegram.org/bot<TOKEN>/sendMessage",
  "method": "POST",
  "body_fields": [
    { "key": "chat_id", "source": "literal", "value": "123456" },
    { "key": "text",   "source": "body" }
  ]
}
```

嵌套示例（Discord 风格，发送时会渲染成数组套对象）：

```json
{
  "url": "https://discord.com/api/webhooks/...",
  "body_fields": [
    { "key": "username", "source": "literal", "value": "zPush" },
    { "key": "embeds", "type": "array", "children": [
      { "type": "object", "children": [
        { "key": "title",       "source": "title" },
        { "key": "description", "source": "body" }
      ] }
    ] }
  ]
}
```

> 以上 JSON 展示的是渠道配置的存储结构，直接在管理界面点选即可，不需要手写 JSON

### 安全与验证

- **SSRF 防护**：投递前解析目标主机，私网 / 环回 / 链路本地 / 云元数据地址（如 `169.254.169.254`）一律拒绝
- **测试发送**：渠道列表每行有测试按钮，用样例消息真实发送一次，返回状态码与响应体；测试与正式投递走同一套代码路径

## 🧩 开发者：新增一种投递渠道

投递渠道是插件式的：后端实现一个 `Sender` 接口并注册，前端加一组表单字段即可，管线其余部分（认领、重试、Lease 回收、状态机、测试发送）全部复用。

**1. 实现发送器** — 新建 `internal/sender/xxx.go`：

```go
type XXXSender struct{}

func NewXXXSender() *XXXSender { return &XXXSender{} }

// Send delivers a rendered message to the destination.
func (s *XXXSender) Send(ctx context.Context, destConfig DestinationConfig, msg RenderedMessage) (*SendResult, error) {
    // destConfig.Config: 渠道表单存下的 JSON 配置（自定义字段任意定义）
    // destConfig.Secret: 已解密的密钥明文（由 destination.BuildSenderConfig 统一处理）
    // msg: 渲染结果 {Title, Body, Format}
    // ...
    return &SendResult{StatusCode: 200, Body: respBody}, nil
}
```

`SendResult` 语义：`err == nil` 且 HTTP 2xx 且 `BusinessCode` 为空或 `"0"` 视为成功（`BusinessCode` 用于钉钉这类 HTTP 200 但业务码报错的 API）；`IsRetryable` 标记是否可重试。参考实现：`internal/sender/http.go`。

**2. 注册** — `cmd/server/main.go`：

```go
senderRegistry.Register("xxx", sender.NewXXXSender())
```

**3. 前端表单** — `web/src/features/destinations/channels/`：

- 新建一个渠道模块文件 `xxx.tsx`（参考 `email.tsx` 或 `httpWebhook.tsx`），导出一个 `ChannelModule`：
  - `formSchema` / `defaults`：渠道字段的 zod schema 与默认值（页面会自动并入总 schema 与表单初始值）
  - `Fields`：表单区块组件（按模块独立渲染，无需在页面里写 `type === 'xxx'` 条件）
  - `toConfig` / `fromConfig`：表单值 ↔ 后端 `destConfig.Config` JSON 的互转（编辑回填也走它）
  - `validate`：必填等 superRefine 校验
- `channels/index.ts` 的 `DEST_TYPES` 加入 `'xxx'`，`CHANNELS` 注册表加一行映射
- 文案放 `web/src/i18n/locales/`（zh / en 两份都要，键缺失会在编译期报错）

**4. 测试** — 参照 `internal/sender/http_test.go` 为配置解析与报文组装写单测。

> 渠道密钥的加解密、`Send` 超时控制、投递重试与状态流转都不需要新渠道关心——`destination.BuildSenderConfig` 负责组装配置与解密，`delivery.Worker` 负责调度。

## ⚙️ 配置

所有配置通过环境变量完成：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `:9003` | 监听地址 |
| `DB_HOST` | `127.0.0.1` | MySQL 主机地址 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | **必填** | MySQL 用户名 |
| `DB_PASSWORD` | **必填** | MySQL 密码 |
| `DB_NAME` | `zpush` | MySQL 库名 |
| `ENCRYPTION_MASTER_KEY` | **必填** | 加密主密钥，格式 `v1:base64...`（32 字节，`openssl rand -base64 32` 生成） |
| `ADMIN_INITIAL_USER` | `admin` | 首次启动时创建的管理员用户名 |
| `ADMIN_INITIAL_PASSWORD` | — | 首次启动时创建管理员用户的密码 |
| `LOG_LEVEL` | `info` | 日志级别 |
| `EVENT_PROCESSOR_CONCURRENCY` | `4` | 事件处理器并发数 |
| `DELIVERY_WORKER_CONCURRENCY` | `8` | 投递 Worker 并发数 |
| `CLAIM_POLL_INTERVAL` | `2s` | Worker 认领轮询间隔（Go duration 格式，如 `500ms`、`2s`） |
| `LEASE_TIMEOUT` | `30s` | 认领租约超时，超时未完成视为僵死由回收器接管 |
| `MAX_PAYLOAD_BYTES` | `1048576` | Webhook Payload 最大字节数（默认 1MB） |
| `INGEST_GLOBAL_RATE_LIMIT` | `100/200` | 全局限流（rate/burst 格式，每秒请求数 / 突发容量） |
| `LOGIN_MAX_FAILURES` | `5` | 登录失败锁定阈值（按 IP 与用户名分别计数） |
| `LOGIN_LOCKOUT_SECONDS` | `900` | 触发锁定后的锁定时长（秒） |

### 安全相关说明

- **密钥加密**：渠道与数据源的密钥使用 `ENCRYPTION_MASTER_KEY`（AES-256-GCM）加密后落库，API 永不回传数据源密钥；主密钥丢失 = 加密数据不可恢复，请妥善备份
- **登录防护**：登录失败按 IP 与用户名双计数，达到 `LOGIN_MAX_FAILURES` 后锁定 `LOGIN_LOCKOUT_SECONDS`；所有登录尝试（含来源 IP）写入审计日志
- **数据面认证**：数据源可配置 Bearer token，通知 API 与管理 API 认证体系完全隔离，管理端凭据泄露不影响数据面，反之亦然
- **去重**：调用方携带 `X-Delivery-Id` 请求头即可启用同数据源事件级幂等去重

## 🔌 API 概览

管理 API 统一前缀 `/api/v1`，认证方式为 `Authorization: Bearer <token>`（登录接口换取，24 小时有效）：

```
POST /api/v1/auth/login               # 登录（失败锁定 + 审计）
GET/POST/PUT/DELETE /api/v1/sources|mappings|rules|templates|destinations
POST   /api/v1/destinations/{id}/test  # 渠道测试发送
GET    /api/v1/events                 # 事件列表
POST   /api/v1/events/{id}/replay     # 事件重放
GET    /api/v1/deliveries             # 投递列表
POST   /api/v1/deliveries/{id}/retry  # 投递重试
POST   /api/v1/deliveries/bulk-retry  # 批量重试
GET    /api/v1/audit                  # 审计日志
GET    /api/v1/dashboard/stats        # Dashboard 统计
POST   /api/v1/preview/*              # 沙盒预览（parse / mapping / template / delivery）

POST /ingest/{source_slug}              # Webhook 接入端点（数据面）
GET  /healthz  /readyz                # 健康检查
```

错误响应统一为 `{"error":{"code":"DOMAIN_NNN","message":"..."}}` 结构。

## 🛠️ 技术栈

- 后端：Go + chi + MySQL
- 前端：React + TypeScript + Vite + Tailwind + shadcn/ui
- 双语界面（zh / en），Docker 一键部署

## 📁 项目结构

```
cmd/server/           — 程序入口
internal/
  api/                — HTTP handlers + middleware + router
  config/             — 环境变量配置
  delivery/           — 投递模型 / 仓库 / Worker / Lease Recovery
  destination/        — 投递目标
  event/              — 事件模型 / 仓库
  ingest/             — Webhook 入口 + 限流
  mapping/            — 字段映射
  processor/          — 事件处理器（映射 → 规则 → 模板 → 投递）
  rules/              — 规则模型 / 引擎 / 仓库
  sender/             — 发送器：dingtalk, feishu, wecom, email, http
  source/             — Webhook 数据源
  storage/            — 数据库打开 / 迁移 / 时间格式化
    migrations/mysql/ — MySQL 迁移脚本
  templates/          — 模板
  secret/             — 加解密服务
  audit/              — 审计日志
  observability/      — 日志 / RequestID
web/                  — React 前端（features 按页面分目录，i18n，shadcn/ui）
deploy/               — docker-compose.yml
```

## 🧪 常用命令

```bash
go build ./...                          # 后端构建检查
go test ./...                           # 后端测试
cd web && npx tsc -b --noEmit           # 前端类型检查
cd web && npm run build                 # 前端生产构建
docker build -t zpush .                 # 本地构建镜像
```


## 📄 许可证

MIT License

# 通用 Webhook 事件解析与消息路由引擎：产品与技术设计

> 文档用途：作为项目需求说明、总体架构设计和开发任务输入，交给其他 AI
> 或开发人员继续实现。\
> 项目定位：自托管、可配置的通用事件接入、解析、规则处理、消息模板渲染与多渠道投递平台。\
> 版本建议：V1 MVP 优先实现可靠闭环，再逐步扩展。

------------------------------------------------------------------------

## 0. 术语表

| 中文 | 英文 | 缩写 | 说明 |
|------|------|------|------|
| 事件来源 | Source | — | 一个可接收 Webhook 事件的入口配置 |
| 字段映射 | Mapping | — | 从原始 JSON 提取字段并生成统一变量的配置 |
| 规则 | Rule | — | 决定哪些事件需要投递到哪些目标的结构化条件 |
| 模板 | Template | — | 消息内容的渲染模板 |
| 投递目标 | Destination | — | 一个具体的接收方或渠道配置 |
| 原始事件 | Event | — | 一次 Webhook 请求的原始记录 |
| 事件处理记录 | EventRun | — | 一次事件处理流水线的执行结果（一个 Event 可有多次 EventRun） |
| 投递任务 | Delivery | — | 针对一个目标的独立投递任务，一条事件可创建多条 |
| 投递尝试 | DeliveryAttempt | — | 一次具体的 HTTP 发送记录 |
| 投递 | Delivery | — | 指从创建任务到最终成功/失败的完整生命周期 |
| 发送 | Send | — | 指单次 HTTP 请求动作 |
| 重放 | Replay | — | 对已有事件重新执行处理流水线 |

------------------------------------------------------------------------

## 1. 项目背景与目标

很多系统可以通过 Webhook 推送事件，但不同系统的 Payload
结构、字段名称、事件类型和消息格式都不一致。若每接入一个来源都写一套专用代码，维护成本会持续增加。

本项目希望提供一个通用引擎，让用户通过 Web 管理界面配置：

1.  从哪里接收事件；
2.  如何校验请求并保存原始 Payload；
3.  如何从不同结构的 JSON 中提取字段并转换为统一变量；
4.  哪些事件满足规则，应该发送到哪些目标；
5.  使用什么模板生成消息；
6.  如何发送、记录结果、重试失败任务和排查问题。

### 1.1 核心目标

-   接收通用 HTTP Webhook，优先支持 JSON。
-   原始 Payload 与解析结果分开保存，便于重放和排错。
-   字段映射、规则、模板、渠道配置均可通过 UI
    管理，避免把业务逻辑写死在代码中。
-   同一事件可以命中多条规则，并创建多个独立投递任务。
-   不同渠道的发送逻辑通过 Adapter 隔离。
-   使用持久化投递任务、有限重试和失败记录，避免进程重启后任务丢失。
-   支持 Docker Compose 部署，适合个人或小团队自托管。
-   第一版优先简单、可靠、易调试，不急于引入微服务和复杂消息队列。

### 1.2 非目标

V1 不应一开始就追求：

-   微服务拆分；
-   Kafka、Kubernetes 等复杂基础设施；
-   任意编程语言脚本执行；
-   完整的可视化流程编排器；
-   对外部渠道承诺严格的 exactly-once 投递；
-   一次性实现所有消息平台和高级规则。

------------------------------------------------------------------------

## 2. 核心使用场景

-   WUD（What's Up Docker）镜像更新通知转发到钉钉。
-   监控系统告警转发到钉钉、飞书、企业微信、邮件或通用 HTTP。
-   CI/CD 发布事件经过字段提取与规则过滤后通知相关群组。
-   多个业务系统将不同格式的事件接入统一入口。
-   对接入事件进行标准化、脱敏、去重、模板渲染、路由和投递追踪。

WUD 是首个端到端示例，不应成为系统的唯一数据模型或特殊硬编码路径。

------------------------------------------------------------------------

## 3. 总体架构

``` text
事件来源
  └─ HTTP Webhook
       ├─ 请求大小限制、认证、签名校验、限流
       ├─ 生成 request_id / event_id
       ├─ 保存原始 Payload 和接入元数据
       └─ 返回 202 Accepted（仅在成功持久化后）
               |
               v
         事件处理 Processor
               |
               ├─ 解析 JSON
               ├─ 执行字段映射与类型转换
               ├─ 校验必填字段
               ├─ 保存解析结果（EventRun）
               ├─ 评估规则（可命中多条）
               ├─ 渲染各规则使用的模板
               └─ 为每个目标创建独立 Delivery 任务
                         |
                         v
                  投递 Worker
                         |
              ┌──────────┼──────────┐
              v          v          v
            钉钉       飞书       通用 HTTP / Email
              |          |          |
              └──────────┼──────────┘
                         v
                 投递记录、重试、审计
                         |
                         v
          Web UI / 日志 / 指标 / 失败任务排查
```

### 3.1 设计原则

-   **模块化单体**：一个后端项目，内部按模块分层；接收 API 和 Worker
    可以在同一二进制中运行，也可按进程角色启动。
-   **持久化优先**：先写数据库，再返回接收成功；投递任务也先入库，再由
    Worker 发送。
-   **异步处理**：接入 API 不等待钉钉等外部平台完成发送。
-   **可配置优先**：字段映射、规则、模板、目标都应由配置驱动。
-   **失败隔离**：每个目标独立投递、独立重试。
-   **安全默认开启**：认证、超时、请求大小限制、敏感信息脱敏和目标地址校验不能只靠部署者记得配置。
-   **可观测性**：每个事件、规则执行和投递任务都能通过 ID 追踪。

### 3.2 双 Worker 架构

系统包含两个独立的 Worker 角色，职责和领取逻辑完全不同：

| Worker | 职责 | 领取条件 | 并发模型 |
|--------|------|----------|----------|
| **Event Processor** | 解析 → 映射 → 规则求值 → 创建 Delivery | `events.status = received` | 按事件粒度并发，每个事件处理独立 |
| **Delivery Worker** | 领取 Delivery → 发送 → 记录结果 → 重试 | `deliveries.status IN (pending, retry_wait) AND next_attempt_at <= now()` | 按目标渠道限流，同一 Destination 的 Delivery 串行或低并发 |

两个 Worker 在同一进程内以独立 goroutine pool 运行，也可通过启动参数指定角色。崩溃后通过租约超时机制回收任务。

------------------------------------------------------------------------

## 4. 推荐技术栈

### 4.1 后端

-   语言：Go。
-   HTTP：标准库 `net/http`，或选择成熟轻量框架（例如
    Chi）；不要为了简单 CRUD 过度引入框架。
-   数据库访问：选一个维护活跃的 SQL 工具或
    ORM，并保留清晰的数据访问层。
-   JSON：标准库 `encoding/json` 用于校验和反序列化。
-   字段提取：根据产品所需语法选择 JSONPath 实现。若使用
    `gjson`，必须明确它采用自身路径语法，并不等同于完整 JSONPath。
-   模板：使用受限的 Go `text/template` 或
    `html/template`；禁止执行任意系统命令和任意用户代码。
-   日志：结构化日志。
-   指标：后续增加 Prometheus 指标。

### 4.2 前端

-   Vue 3 + TypeScript。
-   UI 组件库可由实现者选择。
-   API 与后端解耦，所有配置和操作都通过后端 API 完成。
-   重要编辑页应提供预览、测试和错误提示。

### 4.3 存储与部署

-   V1：SQLite，适合单机、低到中等吞吐量的自托管场景。
-   设计数据库访问层，未来可按需要支持 PostgreSQL。
-   Docker Compose 部署，包含应用、持久化卷、健康检查。
-   初期用数据库表作为持久化任务队列，不必立刻引入 Redis、RabbitMQ 或
    Kafka。
-   如未来需要多实例、高吞吐或复杂消费语义，再评估外部队列。

### 4.4 技术选择注意事项

-   SQLite 的并发写能力有限，Worker 应控制并发和事务时长，设置合理的
    busy timeout；多实例部署或写入压力明显增长时优先评估 PostgreSQL。
-   JSONPath
    库要先确认是否支持所需语法，尤其是数组筛选、递归查找、通配符和多结果表达式。
-   不要在数据库中保存明文渠道密钥。使用应用层加密或外部 Secret
    管理，并明确备份密钥的方式。
-   模板只能访问允许暴露的字段和函数，不能开放任意代码执行能力。

------------------------------------------------------------------------

## 5. 核心领域对象与数据模型

建议至少设计以下实体。字段可按实现调整，但实体职责应保持清晰。

### 5.1 Source（事件来源）

表示一个可接收事件的来源或入口配置。

建议字段：

-   `id` — 主键，UUID/ULID
-   `name` — 显示名称
-   `slug` — 接入路径标识，**建议自动生成随机 slug（如 `src_a3k9x`），避免使用可猜测值暴露入口路径**
-   `enabled` — 是否启用
-   `auth_type` — 认证类型：`bearer`、`hmac`、`none`（仅限明确的安全场景）
-   `secret_ref` — 加密后 Secret 的存储标识（见 5.11 Secret 存储机制）
-   `payload_content_type` — V1 仅 `application/json`
-   `max_body_bytes` — 请求体大小上限
-   `rate_limit` — 速率限制配置（见 12.6 速率限制策略）
-   `mapping_id` — 关联的 Mapping 配置 ID（Source : Mapping = N : 1，多个 Source 可共享同一套映射）
-   `expected_response` — 来源方期望的响应格式（见 6.7 Webhook 来源方重试兼容）
-   `created_at`、`updated_at`

注意：

-   不同 Source 可以有不同 Token、限流、大小限制和字段映射。
-   不要把 Token、签名密钥写进日志或返回给前端。
-   公开网络入口默认应启用认证；如使用
    HMAC，要校验原始请求体签名并防止重放。

### 5.2 Mapping / Schema（字段映射）

负责从原始 JSON 中提取字段，并形成后续规则和模板统一使用的变量。

**关系**：Source : Mapping = N : 1。多个 Source 可以共享同一个 Mapping 配置。

建议字段：

-   `id` — 主键
-   `name` — 映射配置名称
-   `revision` — 配置版本号，每次修改自增
-   `fields` — 映射条目列表（JSON 数组或关联表，见下方）
-   `created_at`、`updated_at`

每条映射条目包含：

-   `source_path` — JSONPath 或明确指定的提取语法
-   `target_key` — 统一变量名，如 `container_name`
-   `value_type` — `string`、`number`、`boolean`、`object`、`array`
-   `default_value` — 字段缺失时的默认值（需区分"未配置默认值"和"默认值为 null"）
-   `required` — 是否必填
-   `transform` — 有限的内置转换：`to_upper`、`to_lower`、`trim`、`date_format(from,to)`、`enum_map({"old":"新"})`
-   `multi_result_mode` — 路径表达式返回多个结果时的策略：`first`（取首个）、`array`（接收数组）、`join(分隔符)`（拼接为字符串）、`count`（计数）；不能隐式丢弃数据
-   `description` — 字段说明

示例：

| 统一变量 | 来源路径 | 类型 | 必填 | 默认值 | multi_result_mode |
|----------|----------|------|------|--------|-------------------|
| `container_name` | `$.name` | string | 是 | 无 | — |
| `image_name` | `$.image.name` | string | 否 | 空字符串 | — |
| `current_tag` | `$.image.tag.value` | string | 否 | 无 | — |
| `new_tag` | `$.result.tag` | string | 否 | 无 | — |
| `update_available` | `$.updateAvailable` | boolean | 否 | false | — |

上表是示例配置，不代表 WUD 的所有版本都使用相同
Payload。应以实际捕获到的样本为准。

### 5.3 Rule（规则）

建议字段：

-   `id` — 主键
-   `name` — 规则名称
-   `enabled` — 是否启用
-   `priority` — 优先级，**数值越小越先执行**（遵循 Unix 惯例，0 = 最高优先级）
-   `conditions` — 结构化条件表达式（JSON）
-   `condition_mode` — `and` / `or`
-   `continue_on_match` — 命中后是否继续评估后续规则（默认 `true`，允许一条事件命中多条规则）
-   `template_id` — 关联模板
-   `destination_ids` — 关联目标列表
-   `dedupe_config` — 去重配置（见下方）
-   `revision` — 配置版本号，每次修改自增
-   `created_at`、`updated_at`

#### 去重配置结构

```json
{
  "enabled": true,
  "window": "5m",
  "key_fields": ["fields.container_name", "fields.new_tag"],
  "action": "drop"
}
```

-   `window` — 去重时间窗口
-   `key_fields` — 组成去重键的字段路径列表
-   `action` — `drop`（静默丢弃重复）| `log_only`（记录但不丢弃）

V1 条件类型：

-   字段存在 / 不存在
-   等于 / 不等于
-   大于 / 小于
-   包含 / 不包含
-   正则匹配
-   AND / OR 组合

不要用字符串拼接 SQL 的方式实现规则。条件应先解析成受控 AST 或结构化
JSON，再由规则引擎安全求值。

### 5.4 Template（消息模板）

建议字段：

-   `id` — 主键
-   `name` — 模板名称
-   `format` — `text`、`markdown`、`html`
-   `title_template` — 标题模板
-   `body_template` — 正文模板
-   `revision` — 版本号，每次修改自增
-   `created_at`、`updated_at`

能力：

-   变量插值；
-   空值和缺失字段处理；
-   少量受控条件语法；
-   预览与测试渲染；
-   模板版本管理。

模板变量来自解析结果和必要的系统元数据，例如：

-   `event.id`
-   `event.received_at`
-   `source.name`
-   `fields.container_name`
-   `fields.current_tag`
-   `fields.new_tag`

不应默认允许模板执行任意函数、访问文件系统、执行命令或发起网络请求。

### 5.5 Destination（投递目标）

表示一个具体接收方或渠道配置。

建议字段：

-   `id` — 主键
-   `name` — 目标名称
-   `type` — 渠道类型：`dingtalk`、`feishu`、`wecom`、`email`、`http_webhook`
-   `enabled` — 是否启用
-   `config` — 渠道特定配置（JSON）
-   `secret_ref` — 加密凭据的存储标识（见 5.11）
-   `timeout_ms` — 请求超时
-   `rate_limit` — 渠道级速率限制
-   `circuit_breaker` — 熔断配置（见下方）
-   `revision` — 配置版本号
-   `created_at`、`updated_at`

同一个渠道可以配置多个 Destination，例如多个钉钉机器人或不同群组。

#### 熔断配置结构

```json
{
  "failure_threshold": 5,
  "recovery_interval": "10m",
  "half_open_max_probe": 1
}
```

-   `failure_threshold` — 连续失败 N 次后打开熔断
-   `recovery_interval` — 熔断后等待多久进入半开状态
-   `half_open_max_probe` — 半开状态下允许放行的探测请求数

Destination 运行时状态（内存维护，可选持久化）：

-   `circuit_state` — `closed` | `open` | `half_open`
-   `consecutive_failures` — 连续失败计数
-   `last_failure_at` — 最近一次失败时间
-   `circuit_opened_at` — 熔断打开时间

### 5.6 Event（原始事件）

建议字段：

-   `id` — 唯一 ID，建议 ULID（有序，适合索引）
-   `source_id` — 关联来源
-   `source_event_id` — 来源方提供的事件 ID（如果存在），与 `source_id` 组合可作为去重键
-   `request_id` — HTTP 请求 ID
-   `received_at` — 接收时间
-   `content_type` — 请求 Content-Type
-   `payload_raw` — 原始请求体
-   `payload_hash` — SHA-256，用于诊断和可选去重
-   `status` — `received`、`processing`、`processed`、`failed`
-   `error_code` — 结构化错误码（见 22. 错误码表）
-   `error_summary` — 错误摘要
-   `expire_at` — 可选过期时间

要求：

-   原始 Payload 应先持久化，后续处理可异步进行。
-   为敏感字段制定脱敏与保留策略。
-   不要将完整 Payload 默认打印到普通应用日志。
-   对超大 Payload 设置上限；原始数据保存周期应可配置。

### 5.7 EventRun（事件处理记录）

**关系**：Event : EventRun = 1 : N。每次处理（包括 replay）都会创建新的 EventRun。

建议字段：

-   `id` — 主键
-   `event_id` — 关联事件
-   `run_type` — `initial`（首次处理）| `replay`（重放）
-   `mapping_id`、`mapping_revision` — 使用的映射配置及版本
-   `rule_set_snapshot` — 当时启用的规则 ID 和 revision 列表（JSON 快照）
-   `parsed_fields` — 提取后的统一字段（JSON）
-   `validation_result` — 校验结果（JSON）
-   `matched_rule_ids` — 命中的规则 ID 列表
-   `status` — `processing`、`completed`、`failed`
-   `started_at`、`finished_at`
-   `error_code`、`error_summary`

保存配置快照或版本号很重要：后续修改映射或规则时，仍能解释历史事件当时为什么命中某条规则。

### 5.8 Delivery（独立投递任务）

一条事件可以创建多条 Delivery，每个目标一条独立任务。

建议字段：

-   `id` — 主键
-   `event_id` — 关联事件
-   `event_run_id` — 关联处理记录（区分首次处理和重放产生的投递）
-   `rule_id` — 触发此投递的规则
-   `destination_id` — 投递目标
-   `idempotency_key` — 幂等键，由 `event_run_id + rule_id + destination_id` 组合生成
-   `status` — `pending`、`sending`、`success`、`retry_wait`、`failed`、`dead_letter`、`cancelled`
-   `attempt_count` — 已尝试次数
-   `max_attempts` — 最大尝试次数
-   `next_attempt_at` — 下次重试时间
-   `lease_until` — Worker 租约到期时间（见 10.1 状态机）
-   `lease_owner` — 持有租约的 Worker 标识
-   `rendered_title` — 渲染后的标题（需评估敏感数据风险，见 12.3）
-   `rendered_body` — 渲染后的正文
-   `last_error_code`、`last_error_summary`
-   `created_at`、`updated_at`

要求：

-   目标 A 成功、目标 B 失败时，只重试 B。
-   对投递任务设计幂等键，避免同一处理事务重复创建相同任务。`idempotency_key` 应设为 UNIQUE 约束。
-   `202 Accepted` 仅表示事件已经可靠接受，不表示目标渠道已经发送成功。

### 5.9 DeliveryAttempt（单次发送记录）

建议字段：

-   `id` — 主键
-   `delivery_id` — 关联投递任务
-   `attempt_no` — 第几次尝试
-   `started_at`、`finished_at`
-   `request_summary` — 脱敏后的请求摘要
-   `response_status` — HTTP 状态码
-   `response_summary` — 限制长度并脱敏的响应摘要
-   `error_code`、`error_summary`
-   `duration_ms` — 耗时

每次重试都新增记录，不覆盖之前的失败原因。

### 5.10 AuditLog（审计日志）

记录重要配置变更和管理员操作：

-   登录、登出；
-   Source、Mapping、Rule、Template、Destination
    的新增、修改、启停、删除；
-   手动重试、清理历史事件；
-   操作人、时间、对象类型、对象 ID、变更前快照、变更摘要。

### 5.11 Secret 存储机制

所有敏感凭据（Source Token、HMAC 密钥、Destination API Key 等）统一采用应用层加密存储。

**V1 方案：本地加密存储**

-   部署时通过环境变量 `ENCRYPTION_MASTER_KEY` 或文件 `--master-key-file` 提供主密钥（32 字节 AES-256-GCM）
-   数据库中 `secret_ref` 字段存储格式：`v1:<base64_nonce>:<base64_ciphertext>:<base64_tag>`
-   每个密钥独立随机 nonce，防止重放
-   支持**多主密钥共存**：`ENCRYPTION_MASTER_KEY=v1:abc123,v2:def456`，旧密钥用于解密，列表最后一个密钥用于新加密，实现密钥轮换
-   密钥轮换流程：添加新密钥到列表末尾 → 运行 `zpush rotate-secrets` 命令重新加密所有存量密钥 → 移除旧密钥
-   备份时必须同时备份主密钥，否则无法恢复凭据

**未来方案**：接入 HashiCorp Vault 或其他外部 Secret Manager。

------------------------------------------------------------------------

## 6. 原始 Payload 接收与解析设计

这是系统的核心链路，必须做到"接收"和"业务处理"解耦。

### 6.1 HTTP 接入流程

1.  校验 HTTP 方法，通常仅允许 POST。
2.  校验 `Content-Type`，V1 优先支持 `application/json`。
3.  根据 URL 中的 `source_slug` 查找 Source，校验其存在且启用。
4.  校验 Bearer Token 或 HMAC 签名。
5.  检查速率限制和请求体大小。
6.  读取原始 Body，验证 JSON 语法。
7.  生成 `request_id`、`event_id`（ULID），计算 Payload Hash。
8.  在数据库事务中插入 Event 记录（`status = received`）。
9.  事务提交成功后返回 `202 Accepted`，包含 `event_id`。
10. Event Processor 异步读取 `status = received` 的任务并处理。

任何数据库持久化失败都不能返回"已接受"的成功响应。接入端不应同步等待所有渠道发送完成。

### 6.2 字段映射流程

原始 Payload 示例：

``` json
{
  "name": "homeassistant",
  "image": {
    "name": "homeassistant/home-assistant",
    "tag": {
      "value": "2024.6.4",
      "semver": true
    }
  },
  "result": {
    "tag": "2024.6.5"
  },
  "updateAvailable": true
}
```

通过 Source 的 Mapping 配置得到统一字段：

``` json
{
  "container_name": "homeassistant",
  "image_name": "homeassistant/home-assistant",
  "current_tag": "2024.6.4",
  "new_tag": "2024.6.5",
  "update_available": true
}
```

规则和模板只依赖统一字段，不应直接耦合每个来源的原始 JSON 结构。

### 6.3 字段映射必须可配置

不要把以下映射永久写死在 Go 代码里：

``` text
container_name -> name
image_name     -> image.name
current_tag    -> image.tag.value
new_tag        -> result.tag
update_available -> updateAvailable
```

它们只能作为初始示例或可导入模板。最终应存放在数据库中，由 Web UI
编辑和版本化。

### 6.4 缺失值与类型转换

必须区分：

-   字段不存在；
-   字段存在且值为 `null`；
-   字段存在但值为空字符串；
-   字段类型错误；
-   字段值为 `false` 或 `0`。

不能简单地把所有假值都视为"字段缺失"。`required`
校验、默认值和转换逻辑应有明确顺序并编写测试。

推荐处理顺序：

1.  执行路径提取；
2.  检查字段是否存在；
3.  字段不存在 → 应用 `default_value`；
4.  字段存在但值为 `null` → 视配置决定使用 `default_value` 或保留 `null`；
5.  校验或转换类型；
6.  校验必填；
7.  保存解析结果和验证错误。

如果路径表达式返回多个结果，配置必须通过 `multi_result_mode` 明确目标变量接收数组、首个值、拼接字符串还是计数；不能隐式丢弃数据。

### 6.5 JSONPath 语法

若 UI 宣称支持 JSONPath，应选用确实实现目标 JSONPath
语法的库，并通过测试确认：

-   根路径；
-   嵌套对象；
-   数组索引；
-   数组通配符；
-   过滤表达式；
-   多结果返回；
-   不存在路径；
-   非法路径。

如果选择 `gjson`，应将 UI 中的语法明确命名为 GJSON
Path，而不要错误地宣传为完整 JSONPath。不同语法不要混用。

### 6.6 接入 API 响应

建议：

-   成功持久化：`202 Accepted`，返回 `event_id` 和
    `status: accepted`；
-   请求体过大：`413 Payload Too Large`；
-   JSON
    非法：`400 Bad Request`；
-   认证失败：`401 Unauthorized`；
-   来源禁用或不存在：按安全策略返回 `403` 或 `404`；
-   超过速率限制：`429 Too Many Requests`；
-   数据库不可用：`5xx`，不能伪装为接收成功。

示例响应：

``` json
{
  "event_id": "01JEXAMPLE...",
  "status": "accepted"
}
```

### 6.7 Webhook 来源方重试兼容

部分 Webhook 来源（如 GitHub、GitLab、WUD）在收不到预期响应时会自动重试。需要处理：

-   **来源方期望特定响应格式**：Source 配置中增加 `expected_response` 字段：

    ```json
    {
      "status_code": 200,
      "body": "{\"msg\": \"received\"}",
      "content_type": "application/json"
    }
    ```

    如果来源未配置此项，默认返回 `202 Accepted`。

-   **来源重试导致重复事件**：如果来源在请求中提供事件 ID（如 GitHub 的 `X-GitHub-Delivery` 头），应保存为 `source_event_id`，与 `source_id` 组合作为去重键。在持久化前查询是否已存在相同 `source_id + source_event_id` 的事件，若存在则返回已接受状态而不重复创建。

------------------------------------------------------------------------

## 7. 规则引擎

### 7.1 规则执行顺序

1.  读取当前启用规则；
2.  按 `priority` 升序排序（数值越小越先执行）；
3.  将解析字段传入规则求值器；
4.  记录每条规则的命中/未命中及原因；
5.  根据 `continue_on_match` 决定是否继续；
6.  对每条命中规则生成相应 Delivery。

默认建议允许一条事件命中多条规则，以便同一事件同时发送到多个群组或系统。是否"命中后停止"由规则显式控制。

### 7.2 V1 规则示例

当 `update_available == true` 时，发送到钉钉：

``` json
{
  "all": [
    {
      "field": "fields.update_available",
      "operator": "eq",
      "value": true
    }
  ]
}
```

其他示例：

-   `fields.severity` 等于 `critical`；
-   `fields.environment` 属于 `prod`；
-   `fields.message` 正则匹配 `OOM|OutOfMemory`；
-   `fields.container_name` 存在且 `fields.new_tag` 非空。

### 7.3 规则安全与性能

-   不执行用户提供的任意代码。
-   正则要限制长度、复杂度和执行时间，避免 ReDoS。
-   对规则数量、嵌套深度和求值耗时设限。
-   对规则求值错误记录明确原因，避免把错误误判为匹配成功。
-   V1 使用结构化条件；后续确有需求再评估 CEL 等表达式语言。

------------------------------------------------------------------------

## 8. 模板与消息渲染

### 8.1 模板语法

V1 使用 Go `text/template`，所有变量通过点号访问结构体字段。模板可访问的数据结构：

```go
type TemplateData struct {
    Event  EventMeta      // event.id, event.received_at, event.source_name
    Fields map[string]any // fields.container_name, fields.current_tag 等
}
```

### 8.2 模板示例：WUD 更新通知

``` text
Docker 镜像更新提醒

容器：{{ .Fields.container_name }}
镜像：{{ .Fields.image_name }}
当前版本：{{ .Fields.current_tag }}
可更新版本：{{ .Fields.new_tag }}
是否可更新：{{ .Fields.update_available }}
事件时间：{{ .Event.received_at }}
{{- if .Fields.new_tag }}

⚠️ 建议尽快更新！
{{- end }}
```

UI 中应提供变量帮助面板，列出所有可用的 `.Fields.*` 和 `.Event.*` 变量。

### 8.3 渲染要求

-   变量名稳定，且有可查看的变量列表。
-   缺失值应按配置处理，不能直接渲染成难以理解的内部对象。
-   支持标题、正文、格式、预览和测试发送。
-   对钉钉 Markdown 等渠道格式做 Adapter 层的格式适配。
-   限制输出长度，防止超长消息被拒绝。
-   防止模板注入和任意函数执行。
-   可选择在创建 Delivery
    时保存渲染结果，确保重试时内容一致；如数据敏感，则保存必要快照并制定清理策略。

------------------------------------------------------------------------

## 9. 渠道 Adapter 设计

定义统一发送接口，例如：

``` go
type Sender interface {
    Send(ctx context.Context, config DestinationConfig, message RenderedMessage) (*SendResult, error)
}

type DestinationConfig struct {
    ID        string
    Type      string
    Config    json.RawMessage // 渠道特定配置
    Timeout   time.Duration
    SecretRef string
}

type RenderedMessage struct {
    Title string
    Body  string
    Format string // text, markdown, html
}

type SendResult struct {
    StatusCode   int
    Body         string // 脱敏后
    IsRetryable  bool
    BusinessCode string // 渠道业务码，如钉钉 errcode
}
```

具体类型实现：

-   `DingTalkSender`
-   `FeishuSender`
-   `WeComSender`
-   `EmailSender`
-   `HTTPWebhookSender`
-   后续可扩展 Telegram、Slack 等。

### 9.1 Adapter 责任

-   将通用消息转换成目标平台的请求格式；
-   处理渠道特有的签名、认证和字段；
-   设置请求超时；
-   检查 HTTP 状态码；
-   检查业务响应码（例如 HTTP 200 但业务返回失败）；
-   将错误转换成统一错误结构（`error_code` + `is_retryable`）；
-   对请求、响应日志脱敏。

### 9.2 通用 HTTP Destination

这是高价值功能，但也有较高安全风险。必须：

-   只允许明确支持的协议（通常
    HTTPS，开发环境可显式开放 HTTP）；
-   防范
    SSRF：拒绝回环、私有、链路本地、保留地址和云元数据地址，除非管理员明确配置安全例外；
-   解析 DNS 后校验 IP，防止 DNS rebinding；
-   禁止或严格控制重定向，并对重定向后的目标重新校验；
-   设置连接、TLS、响应头和整体超时；
-   限制响应体大小；
-   不将 Secret
    自动转发到未经信任的地址；
-   管理界面应明确提示目标地址风险。

------------------------------------------------------------------------

## 10. 投递可靠性、重试与幂等

### 10.1 状态机

建议状态：

``` text
pending -> sending -> success
                   -> retry_wait -> sending
                   -> failed / dead_letter
```

**Worker 崩溃恢复**：

-   Delivery 在进入 `sending` 状态时设置 `lease_until = now() + lease_timeout`（建议 5 分钟）和 `lease_owner = worker_id`
-   Worker 完成发送后清除租约并更新状态
-   如果 Worker 在发送期间崩溃，其他 Worker 的回收协程定期扫描 `status = sending AND lease_until < now()` 的记录，将其重置为 `pending`（而非 `retry_wait`，因为本次发送结果未知，不应计入 attempt_count）
-   `lease_timeout` 应大于渠道最大超时时间，避免正常发送被误回收

### 10.2 重试策略

-   只对可重试错误重试，例如网络超时、连接失败、HTTP 429、部分 5xx。
-   认证失败、格式错误等通常不应盲目无限重试。
-   使用指数退避并加入随机抖动：`delay = min(base * 2^attempt + jitter, max_delay)`
    -   建议：`base = 30s`，`max_delay = 1h`，`jitter = ±20%`
-   尊重渠道返回的 `Retry-After`（如果适用）。
-   设置最大尝试次数（默认 5 次）和最长重试时间（默认 24h）。
-   达到上限后进入 `dead_letter` 状态。
-   支持管理员手动重试（重置状态为 `pending`，保留历史记录），并保留审计记录。

### 10.3 幂等与重复事件

-   接入时计算 Payload Hash，可用于诊断或可选去重，但相同 Payload
    不一定代表同一事件。
-   若来源提供事件 ID，优先使用 `source_event_id + source_id` 作为去重键。
-   为 Delivery 生成稳定幂等键：`SHA256(event_run_id + ":" + rule_id + ":" + destination_id)`，在数据库中设 UNIQUE 约束，避免同一处理事务重复创建。
-   发送方与目标渠道之间通常无法保证严格
    exactly-once；应明确采用至少一次处理并尽可能幂等的策略。
-   对同一任务重试时复用同一渲染结果。

### 10.4 数据库任务队列

V1 可用数据库表实现：

-   **Event Processor 领取**：`UPDATE events SET status = 'processing' WHERE id = (SELECT id FROM events WHERE status = 'received' ORDER BY received_at LIMIT 1)` — 单行事务领取
-   **Delivery Worker 领取**：`UPDATE deliveries SET status = 'sending', lease_until = ?, lease_owner = ? WHERE id = (SELECT id FROM deliveries WHERE status IN ('pending', 'retry_wait') AND next_attempt_at <= now() ORDER BY next_attempt_at LIMIT 1)` — 单行事务领取
-   发送网络请求期间不要长期持有数据库事务；领取后立即提交，发送完成后再开新事务更新状态
-   合理设置 Worker 并发数（Event Processor 建议 4-8，Delivery Worker 按 Destination 限流控制）
-   避免多个 Worker 重复领取同一任务（通过 `status` 条件和 `lease_owner` 保障）

### 10.5 熔断机制

当某个 Destination 持续失败时，Delivery Worker 应暂停向其发送，避免无效重试浪费资源：

1.  **Closed → Open**：连续 `failure_threshold` 次发送失败后，标记该 Destination 为 `circuit_open`，后续该 Destination 的 Delivery 跳过发送，状态保持 `pending` 不变（不消耗 attempt_count）
2.  **Open → Half-Open**：等待 `recovery_interval` 后，允许放行 `half_open_max_probe` 条 Delivery 作为探测
3.  **Half-Open → Closed**：探测成功则关闭熔断
4.  **Half-Open → Open**：探测失败则重新打开熔断，重置等待计时

熔断状态应在 UI 上明确显示，并记录到审计日志。管理员可手动重置熔断状态。

------------------------------------------------------------------------

## 11. Web 管理界面

建议主要页面：

1.  **仪表盘**：接入事件数量、处理成功率、待投递数量、失败数量、最近错误、熔断状态。
2.  **Sources**：来源列表、接入 URL、认证方式、启停、请求限制。
3.  **Payload 调试器**：粘贴/上传样本
    JSON，查看格式化原文、字段路径提取结果、缺失字段和类型错误。
4.  **Mappings**：路径、目标变量、类型、默认值、必填校验、测试提取、版本历史。
5.  **Rules**：条件构建器、优先级、命中行为、目标和模板选择、去重配置。
6.  **Templates**：变量帮助、预览、示例 Payload、测试发送、版本历史。
7.  **Destinations**：渠道配置、密钥管理、连接测试、启停、熔断状态。
8.  **Events**：事件详情、原始
    Payload（权限控制与脱敏）、解析结果、命中规则、关联投递。
9.  **Deliveries**：状态、尝试次数、响应摘要、错误、手动重试。
10. **Audit Logs**：配置变更和管理员操作。
11. **Settings**：认证、保留周期、日志级别、并发和系统配置。

### 11.1 可用性要求

-   每个编辑界面都应有校验和清晰错误提示。
-   配置保存前支持测试；映射和模板提供预览。
-   修改配置应保留版本（`revision` 自增）或审计记录。
-   删除被规则引用的模板/目标时，应阻止删除或要求明确解除引用。
-   Secret 默认只显示是否已配置，不回显完整明文。
-   关键操作（删除、批量重试、清理历史）需要确认。

------------------------------------------------------------------------

## 12. 安全要求

安全是产品基础能力，而非后续可选优化。

### 12.1 接入端安全

-   每个 Source 独立凭据；
-   支持 Bearer Token，后续支持 HMAC；
-   请求体大小限制；
-   速率限制；
-   校验 Content-Type 和 JSON；
-   防止重放（尤其是签名认证）；
-   不在 URL 查询参数中放置 Secret；
-   管理端与 Webhook 入口可配置分离。

### 12.2 管理端安全

-   V1 至少具备管理认证；不能将管理 API 裸露到公网。
-   对登录、密码存储、会话、CSRF/CORS 等按 Web 安全最佳实践实现。
-   管理员角色和普通只读角色可后续扩展。
-   重要操作记录审计日志。

### 12.3 数据安全

-   渠道凭据加密存储，密钥独立管理（见 5.11）；
-   原始 Payload 和渲染结果制定保留周期；
-   `rendered_title` / `rendered_body` 在 Delivery 表中存储敏感数据风险评估：V1 默认存储以保障重试一致性，但提供配置项控制是否保留渲染结果或设置 TTL 自动清理；
-   日志默认不记录完整 Body、Token、签名和敏感字段；
-   UI 展示 Payload 时进行权限控制和脱敏；
-   导出配置时默认排除 Secret。

### 12.4 通用 HTTP 目标安全

-   SSRF 防护必须在后端执行，不能只靠前端校验。
-   禁止访问云元数据服务、回环和非授权内网地址。
-   重新校验 DNS 解析结果和重定向目标。
-   限制协议、端口、响应体和超时。
-   对可访问的内网目标采用明确的管理员白名单策略。

### 12.5 运维安全

-   容器尽可能以非 root 运行；
-   Secret 通过环境变量、挂载文件或 Secret 管理机制注入；
-   数据库和配置文件设置最小权限；
-   定期备份数据库和加密密钥；
-   备份恢复流程应有验证步骤。

### 12.6 速率限制策略

V1 采用**进程内令牌桶算法**，适合单实例部署：

-   每个 Source 独立配置：`{ "rate": 10, "burst": 20 }` — 每秒 10 个请求，突发上限 20
-   全局入口限流：防止总体过载，配置项 `INGEST_GLOBAL_RATE_LIMIT`
-   Destination 级限流：每个渠道有独立令牌桶，防止向同一平台发送过快
-   令牌桶实现基于 `golang.org/x/time/rate`
-   多实例部署时需说明：进程内令牌桶无法跨实例协调，需外置 Redis 限流（V2+）

------------------------------------------------------------------------

## 13. 建议 API 设计

统一前缀：`/api/v1`

### 13.1 Webhook 接入

``` text
POST /api/v1/ingest/{source_slug}
```

成功后返回 `202 Accepted` 和 `event_id`。

### 13.2 配置管理

``` text
GET/POST       /api/v1/sources
GET/PUT/DELETE /api/v1/sources/{id}
POST           /api/v1/sources/{id}/test

GET/POST       /api/v1/mappings
GET/PUT/DELETE /api/v1/mappings/{id}

GET/POST       /api/v1/rules
GET/PUT/DELETE /api/v1/rules/{id}

GET/POST       /api/v1/templates
GET/PUT/DELETE /api/v1/templates/{id}

GET/POST       /api/v1/destinations
GET/PUT/DELETE /api/v1/destinations/{id}
POST           /api/v1/destinations/{id}/test
POST           /api/v1/destinations/{id}/reset-circuit
```

### 13.3 调试与运维

``` text
POST /api/v1/preview/parse       — 对样本 Payload 执行 JSONPath 提取
POST /api/v1/preview/mapping     — 对样本 Payload 执行完整 Mapping 并返回提取结果
POST /api/v1/preview/rule        — 对统一字段执行规则求值
POST /api/v1/preview/template    — 对统一字段执行模板渲染
POST /api/v1/preview/delivery    — dry-run：模拟从接收到创建 Delivery 的完整流程（不持久化）

GET  /api/v1/events
GET  /api/v1/events/{id}
POST /api/v1/events/{id}/replay  — 重放事件（创建新 EventRun）

GET  /api/v1/deliveries
GET  /api/v1/deliveries/{id}
POST /api/v1/deliveries/{id}/retry
POST /api/v1/deliveries/bulk-retry  — 批量重试（按 filter 条件）
POST /api/v1/events/bulk-delete     — 批量删除事件（按 filter 条件）

GET  /healthz
GET  /readyz
```

批量操作请求体示例：

``` json
{
  "filter": {
    "status": "dead_letter",
    "destination_id": "dest_xxx",
    "created_before": "2024-01-01T00:00:00Z"
  },
  "limit": 100
}
```

约定：

-   `healthz` 表示进程存活；
-   `readyz`
    表示数据库等关键依赖可用、服务能够接收工作。
-   对分页、排序、过滤、错误码、请求 ID 和 API 版本制定统一约定。
-   对危险操作和配置变更 API 进行认证、授权和审计。

------------------------------------------------------------------------

## 14. 部署与运行

### 14.1 Docker Compose

交付内容建议包含：

-   后端镜像；
-   前端静态资源或独立前端容器；
-   SQLite
    数据卷（如使用 SQLite）；
-   配置和加密密钥挂载；
-   健康检查；
-   日志输出到 stdout/stderr；
-   可选反向代理与 HTTPS 示例；
-   升级和备份说明。

### 14.2 运行要求

-   数据库文件放在持久化卷中，不能只存在容器可写层。
-   升级前备份数据库和密钥。
-   数据库迁移要有版本表并可追踪。
-   记录应用版本和数据库 Schema 版本。
-   优雅关机：停止领取新任务，尽量等待正在处理的任务结束，未完成任务可被后续
    Worker 恢复。
-   配置事件和投递历史的保留周期，避免磁盘持续增长。

### 14.3 配置建议

使用环境变量或配置文件设置：

-   `LISTEN_ADDR` — 监听地址和端口
-   `DATABASE_PATH` — SQLite 文件路径（或未来 `DATABASE_DSN`）
-   `ENCRYPTION_MASTER_KEY` — 加密主密钥（支持逗号分隔多密钥，见 5.11）
-   `ADMIN_INITIAL_PASSWORD` — 管理员初始化密码（仅首次启动生效）
-   `LOG_LEVEL` — 日志级别
-   `EVENT_PROCESSOR_CONCURRENCY` — Event Processor 并发数（默认 4）
-   `DELIVERY_WORKER_CONCURRENCY` — Delivery Worker 并发数（默认 8）
-   `LEASE_TIMEOUT` — Worker 租约超时（默认 5m）
-   `MAX_PAYLOAD_BYTES` — 全局 Payload 大小上限（默认 1MB）
-   `EVENT_RETENTION_DAYS` — 原始事件保留天数（默认 30）
-   `DELIVERY_ATTEMPT_RETENTION_DAYS` — DeliveryAttempt 保留天数（默认 90）
-   `INGEST_GLOBAL_RATE_LIMIT` — 全局接入限流（格式 `rate/burst`，如 `100/200`）

不要将真实 Secret 提交到 Git 仓库。

------------------------------------------------------------------------

## 15. 监控与故障排查

至少记录以下关键指标（V2/V3 可完善）：

-   接收事件数；
-   JSON 校验失败数；
-   解析失败数；
-   规则求值错误数；
-   当前待处理事件数；
-   当前待投递任务数；
-   投递成功率；
-   重试次数；
-   Dead Letter 数；
-   渠道响应耗时；
-   Worker 最后成功处理时间；
-   熔断器状态（每个 Destination）。

日志字段建议：

-   `request_id`
-   `event_id`
-   `event_run_id`
-   `delivery_id`
-   `source_id`
-   `rule_id`
-   `destination_id`
-   `attempt_no`
-   `duration_ms`
-   `error_code`

日志中不应包含完整 Token、签名密钥、完整认证头或未经脱敏的原始 Payload。

------------------------------------------------------------------------

## 16. 数据库索引策略

以下索引基于 V1 查询模式设计，应在迁移脚本中创建：

``` sql
-- 事件列表与状态查询
CREATE INDEX idx_events_source_status ON events(source_id, status, received_at);

-- Event Processor 领取任务
CREATE INDEX idx_events_status_received ON events(status, received_at)
    WHERE status = 'received';

-- 去重查询
CREATE INDEX idx_events_source_event_id ON events(source_id, source_event_id)
    WHERE source_event_id IS NOT NULL;

-- Payload Hash 诊断
CREATE INDEX idx_events_payload_hash ON events(payload_hash);

-- Delivery Worker 领取任务
CREATE INDEX idx_deliveries_status_next ON deliveries(status, next_attempt_at)
    WHERE status IN ('pending', 'retry_wait');

-- Worker 租约回收
CREATE INDEX idx_deliveries_lease ON deliveries(status, lease_until)
    WHERE status = 'sending' AND lease_until IS NOT NULL;

-- 投递尝试记录
CREATE INDEX idx_delivery_attempts_delivery ON delivery_attempts(delivery_id, attempt_no);

-- 幂等键唯一约束
CREATE UNIQUE INDEX idx_deliveries_idempotency ON deliveries(idempotency_key);
```

注意：SQLite 的部分索引（WHERE 子句）支持有限，实现时可能需要用普通索引替代。

------------------------------------------------------------------------

## 17. 数据清理策略

### 17.1 清理维度

| 数据 | 默认保留 | 清理方式 | 级联规则 |
|------|----------|----------|----------|
| Event + payload_raw | 30 天 | 定时任务 / API 触发 | 删除 Event 时级联删除关联 EventRun、Delivery、DeliveryAttempt |
| DeliveryAttempt | 90 天 | 定时任务 | 仅删除已完成 Delivery 的历史尝试记录 |
| 渲染结果 (rendered_title/body) | 与 Delivery 同生命周期 | Delivery 完成后可选清理 | 保留 Delivery 记录但清空渲染内容 |

### 17.2 清理实现

-   V1 通过 API 手动触发：`DELETE /api/v1/admin/cleanup?before=2024-01-01`
-   V2 增加内置定时清理：启动参数配置清理周期，如 `--cleanup-interval=1h`
-   清理操作记录审计日志
-   清理前统计将删除的记录数量，要求管理员确认

------------------------------------------------------------------------

## 18. Replay 语义

重放事件时需明确以下行为：

1.  **触发方式**：`POST /api/v1/events/{id}/replay`
2.  **配置版本**：使用**当前最新配置**重新处理（而非历史快照），这是最常见的运维需求（修了 Mapping 后重放旧事件）
3.  **创建新 EventRun**：`run_type = replay`，关联原 `event_id`
4.  **创建新 Delivery**：基于新 EventRun 的规则命中结果创建，拥有新的 `idempotency_key`
5.  **原投递不受影响**：之前成功的 Delivery 保持不变
6.  **审计记录**：记录 replay 操作和原因

如果需要使用历史配置版本重放，可在 API 中增加参数 `?config_revision=xxx`，V1 不实现。

------------------------------------------------------------------------

## 19. 配置变更生效机制

V1 采用**每次处理时读取最新配置**的策略：

-   Event Processor 开始处理时从数据库读取最新 Mapping（含 revision）和启用规则
-   Delivery Worker 开始发送时从数据库读取最新 Destination 配置
-   优点：简单可靠，无需配置变更通知机制
-   缺点：每次处理有一次数据库读取，V1 吞吐量下可接受

未来优化方向（V2+）：

-   进程内配置缓存 + 版本号比对（Worker 定期轮询 `max(revision)` 或收到变更通知后失效）
-   配置变更通过 channel 通知所有 Worker goroutine 刷新缓存

------------------------------------------------------------------------

## 20. 测试策略

### 20.1 单元测试

-   JSONPath 路径提取；
-   不存在字段、null、空字符串、false、0；
-   类型转换与默认值；
-   必填字段校验；
-   `multi_result_mode` 各模式；
-   规则 AND/OR 和优先级；
-   模板渲染、缺失变量和转义；
-   重试退避计算（含抖动范围校验）；
-   熔断状态转换；
-   幂等键生成与唯一约束；
-   租约超时回收逻辑；
-   渠道响应码解析；
-   Secret 加密/解密；
-   SSRF 地址校验。

### 20.2 集成测试

-   Webhook 认证失败；
-   非法 JSON；
-   请求体过大；
-   数据库写入失败；
-   Event 持久化后 Worker 处理；
-   一个事件命中多个规则；
-   同一事件创建多个 Delivery；
-   一个目标成功、另一个目标失败时只重试失败目标；
-   Worker 在发送前后崩溃的恢复行为（租约超时回收）；
-   重复请求和幂等键冲突；
-   钉钉返回 HTTP 200 但业务错误码；
-   通用 HTTP SSRF 防护；
-   Secret 不进入日志和 API 返回值；
-   熔断打开后 Delivery 跳过发送；
-   Replay 创建新 EventRun 和新 Delivery；
-   来源方重试导致重复事件时的去重。

### 20.3 端到端测试：WUD

1.  配置一个 Source（含随机 slug 和 Bearer Token）；
2.  将 WUD Webhook 指向 `/api/v1/ingest/{source_slug}`；
3.  捕获实际 Payload 样本；
4.  配置字段映射；
5.  配置 `update_available == true` 规则；
6.  配置钉钉 Destination；
7.  配置消息模板；
8.  触发一条测试事件；
9.  在 UI 中查看 Event、解析结果、规则命中和 Delivery；
10. 验证成功、失败、重试和重复事件场景。

不要假定所有 WUD 版本或事件类型的 Payload 完全一致，应以实际样本为准。

------------------------------------------------------------------------

## 21. 分阶段开发计划

### Phase 0：工程骨架

-   建立 Go 项目结构；
-   建立 Vue 3 + TypeScript 前端；
-   配置数据库迁移（含索引）；
-   Docker Compose；
-   日志、配置加载、健康检查；
-   管理端认证基础能力；
-   Secret 加密/解密机制。

**验收：** 项目可通过 Docker Compose 启动，健康检查正常，数据库持久化，密钥可加密解密。

### Phase 1：MVP 核心闭环

-   Source 与 Webhook 接入（含随机 slug、Bearer Token）；
-   请求大小限制和 JSON 校验；
-   原始 Payload 持久化；
-   可配置 Mapping（含 `multi_result_mode`）；
-   简单 AND/OR Rule（含 `priority` 和 `continue_on_match`）；
-   Template 渲染（Go `text/template`）；
-   DingTalk Adapter；
-   通用 HTTP Adapter（具备 SSRF 防护）；
-   数据库任务队列（Event Processor + Delivery Worker）；
-   投递记录和有限重试（含指数退避）；
-   Worker 租约与崩溃恢复；
-   基本 Event/Delivery 查询 API；
-   结构化错误码。

**验收：** 一条 WUD
测试事件可以接收、保存、解析、命中规则、渲染并发送到钉钉；失败时可追踪并重试；Worker 崩溃后任务可被回收。

### Phase 2：Web 管理与调试体验

-   Source、Mapping、Rule、Template、Destination CRUD；
-   Payload 调试器；
-   Mapping 预览（`preview/mapping`）；
-   Template 预览和测试发送；
-   Dry-run 端点（`preview/delivery`）；
-   Event 详情；
-   Delivery 历史和手动重试；
-   批量操作（`bulk-retry`、`bulk-delete`）；
-   审计日志；
-   去重配置。

**验收：** 不改代码即可通过 UI 接入一个新的 JSON Webhook
来源，并完成消息转发。

### Phase 3：稳定性与更多渠道

-   飞书、企业微信、Email 等 Adapter；
-   熔断机制；
-   死信任务；
-   限流、退避和渠道级并发控制；
-   数据保留和清理；
-   Replay 功能；
-   来源方重试兼容；
-   Prometheus 指标；
-   备份与恢复文档。

### Phase 4：高级能力

根据实际用户需求再评估：

-   事件聚合和定时摘要；
-   更复杂的表达式语言（如
    CEL）；
-   多用户、RBAC；
-   插件系统；
-   PostgreSQL 与多实例部署；
-   Redis/RabbitMQ/Kafka；
-   更多事件来源和输出渠道。

------------------------------------------------------------------------

## 22. 建议代码结构

可按模块化单体组织：

``` text
.
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── api/                 # HTTP 路由、请求/响应、认证中间件
│   ├── config/              # 配置加载和校验
│   ├── source/              # 来源配置与接入校验
│   ├── ingest/              # Webhook 接收与 Event 持久化
│   ├── processor/           # Event Processor：映射、规则、创建 Delivery
│   ├── delivery/            # Delivery Worker：领取、发送、重试、租约回收
│   ├── event/               # Event、EventRun 领域模型
│   ├── mapping/             # JSONPath 提取、转换、验证
│   ├── rules/               # 条件 AST 与规则执行
│   ├── templates/           # 安全模板渲染
│   ├── destination/         # 目标配置与熔断
│   ├── sender/              # Adapter 接口和各渠道实现
│   ├── secret/              # Secret 加密/解密/轮换
│   ├── storage/             # 数据库访问、事务、迁移
│   ├── audit/               # 审计日志
│   └── observability/       # 日志、指标、追踪
├── web/                      # Vue 3 + TypeScript
├── migrations/               # 数据库迁移（含索引）
├── deploy/
│   └── docker-compose.yml
├── docs/
│   ├── architecture.md
│   ├── api.md
│   ├── configuration.md
│   └── operations.md
├── Dockerfile
├── go.mod
└── README.md
```

这只是建议，不应为每个小功能过早拆成独立服务。模块之间应通过接口协作，避免相互循环依赖。

------------------------------------------------------------------------

## 23. 错误码表

所有错误码使用结构化格式，便于程序化处理和国际化。

### 23.1 错误码规范

格式：`<domain>_<number>`，domain 为 3-10 字母小写，number 为 3 位数字。

| 域 | 范围 | 说明 |
|----|------|------|
| `INGEST` | 001-099 | 接入阶段错误 |
| `PARSE` | 100-199 | JSON 解析与字段映射错误 |
| `RULE` | 200-299 | 规则求值错误 |
| `TPL` | 300-399 | 模板渲染错误 |
| `DELIVER` | 400-499 | 投递与发送错误 |
| `CONFIG` | 500-599 | 配置校验错误 |
| `SYSTEM` | 600-699 | 系统级错误 |

### 23.2 常用错误码

| 错误码 | HTTP Status | 描述 |
|--------|-------------|------|
| `INGEST_001` | 401 | 认证失败（Token 无效或缺失） |
| `INGEST_002` | 413 | 请求体超过大小限制 |
| `INGEST_003` | 400 | JSON 语法错误 |
| `INGEST_004` | 429 | 超过速率限制 |
| `INGEST_005` | 404 | Source 不存在或已禁用 |
| `INGEST_006` | 503 | 数据库不可用 |
| `INGEST_007` | 409 | 重复事件（去重命中） |
| `PARSE_001` | 422 | JSONPath 路径语法错误 |
| `PARSE_002` | 422 | 必填字段缺失 |
| `PARSE_003` | 422 | 字段类型转换失败 |
| `PARSE_004` | 422 | multi_result_mode 处理失败 |
| `RULE_001` | 500 | 规则条件语法错误 |
| `RULE_002` | 500 | 规则求值超时 |
| `RULE_003` | 500 | 正则执行错误（ReDoS 保护触发） |
| `TPL_001` | 500 | 模板语法错误 |
| `TPL_002` | 500 | 模板渲染超时 |
| `TPL_003` | 500 | 模板引用了不允许的函数 |
| `DELIVER_001` | 502 | 目标渠道连接失败 |
| `DELIVER_002` | 502 | 目标渠道认证失败（不可重试） |
| `DELIVER_003` | 504 | 目标渠道超时 |
| `DELIVER_004` | 502 | 目标渠道返回业务错误 |
| `DELIVER_005` | 429 | 目标渠道限流（可重试） |
| `DELIVER_006` | 503 | 目标渠道熔断已打开 |
| `DELIVER_007` | 503 | Worker 租约超时（被回收） |
| `CONFIG_001` | 400 | 配置校验失败 |
| `CONFIG_002` | 409 | 配置被引用，无法删除 |
| `SYSTEM_001` | 500 | 数据库迁移失败 |
| `SYSTEM_002` | 500 | 加密密钥无效 |

------------------------------------------------------------------------

## 24. V1 完成定义（Definition of Done）

V1 只有在以下条件满足后才算完成：

-   [ ] 可通过 Docker Compose 启动和停止；
-   [ ] Source 可配置独立认证；
-   [ ] Source slug 默认随机生成，避免路径可猜测；
-   [ ] 接收端限制请求体大小并校验 JSON；
-   [ ] 原始 Payload 在返回 202 前已持久化；
-   [ ] 数据库不可用时不会返回虚假的接收成功；
-   [ ] Mapping 不需要修改代码即可配置；
-   [ ] 缺失值、null、false、0 和类型错误处理正确；
-   [ ] multi_result_mode 各模式行为正确；
-   [ ] 规则条件结构化且可测试；
-   [ ] 规则 priority 升序执行（数值越小越先）；
-   [ ] 模板无法执行任意系统命令或任意代码；
-   [ ] 支持至少一个真实渠道（钉钉）和通用 HTTP；
-   [ ] 通用 HTTP 目标具备 SSRF 防护；
-   [ ] 一个事件可命中多条规则并创建多条 Delivery；
-   [ ] 每条 Delivery 独立记录状态和重试；
-   [ ] 目标 A 成功、目标 B 失败时不会重发 A；
-   [ ] Delivery 幂等键唯一约束生效；
-   [ ] Worker 租约超时后任务可被回收；
-   [ ] 每次发送尝试都有记录；
-   [ ] 具备管理认证；
-   [ ] 日志和 API 不泄露 Secret；
-   [ ] 结构化错误码贯穿全链路；
-   [ ] 数据库索引已创建；
-   [ ] 具备健康检查、基础测试和部署文档；
-   [ ] WUD 端到端测试通过。

------------------------------------------------------------------------

## 25. 给后续 AI / 开发者的执行要求

请按以下原则继续实现，不要只输出概念性建议：

1.  **先输出实现计划和关键技术决策，再开始写代码。** 先明确数据库
    Schema、API 契约、任务状态机和配置结构。
2.  **按可运行的垂直切片推进。** 优先完成"接收 → 持久化 → 映射 → 规则 →
    模板 → 钉钉投递 → 状态查询"闭环。
3.  **每个阶段都提供可运行代码、文件清单、迁移脚本、测试和启动命令。**
4.  **所有用户可配置逻辑都存入数据库或配置实体，不能只写死在代码中。**
5.  **不要伪造已完成的功能。** 如果暂时未实现认证、SSRF
    防护、重试或持久化，必须明确标记为未实现，不能称为生产可用。
6.  **优先安全和正确性。** 特别关注 Secret、Webhook
    认证、SSRF、模板执行、并发领取任务、幂等和数据库事务。
7.  **错误要可诊断。**
    为解析、规则、渲染、发送失败设计结构化错误码与事件关联 ID。
8.  **不要过度设计。** V1 采用模块化单体和 SQLite
    数据库队列，只有出现明确需求后再增加外部消息队列或拆分服务。
9.  **每完成一个阶段，先运行测试并给出实际结果；无法执行的测试应明确说明。**
10. **文档、配置示例和代码必须保持一致。** 特别是 JSONPath
    语法、模板变量、状态名称、API 响应和环境变量。

建议后续 AI 第一轮工作内容：

-   给出最终目录结构；
-   选定 Go HTTP
    框架、数据库访问方案、JSONPath 库、模板语法；
-   定义 SQL Schema
    和迁移（含索引）；
-   定义
    Source、Mapping、Rule、Template、Destination、Event、Delivery 的 API
    DTO；
-   实现 Phase 0 和 Phase 1 的最小闭环；
-   提供 WUD 示例
    Payload、配置样例、curl 测试和 Docker Compose；
-   对安全限制和尚未实现的功能做明确说明。

------------------------------------------------------------------------

## 26. 关键架构结论

1.  本项目不是单纯的 WUD 转钉钉工具，而是通用 Webhook
    事件处理与消息路由平台。
2.  原始 Payload
    是事实来源；解析结果是基于某个映射版本生成的标准化数据。
3.  接收 API 负责认证、校验和可靠持久化；业务处理和外部发送由 Worker
    异步完成。
4.  系统包含两个独立 Worker 角色：Event Processor（解析→规则→创建 Delivery）和 Delivery Worker（发送→重试→记录），职责和并发模型不同。
5.  Mapping、Rule、Template、Destination
    必须配置驱动，不能依赖重新编译发布。
6.  每个目标对应独立 Delivery，目标间失败互不影响。
7.  "成功接收""事件处理成功""消息投递成功"是三个不同状态，必须分开呈现。
8.  Worker 崩溃通过租约超时回收机制恢复，不依赖外部协调。
9.  Destination 熔断机制防止持续向不可用渠道无效重试。
10. 所有敏感凭据统一加密存储，支持多主密钥共存和密钥轮换。
11. 结构化错误码贯穿全链路，每个错误可追溯到具体领域和原因。
12. 先做一个安全、可调试、可恢复的模块化单体，再根据真实负载扩展。

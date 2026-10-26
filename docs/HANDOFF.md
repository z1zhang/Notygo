# 交接文档（2026-10-27 会话）

## 项目背景

Notygo（原 zPush）——通知中转平台：接入点(endpoint)接收 webhook → 映射(mapping)提取字段 → 规则(rules)匹配 → 模板(templates)渲染 → 渠道(channel)投递。
pg-dev 线已完成 PostgreSQL + River 迁移、全栈改名、`notygo_` 表前缀（见 git 历史 133c697 / 42cf18d）。

## 本会话完成的工作

上一会话（endpoint HMAC 认证）因模型不支持图片输入中断；本会话恢复上下文、补齐验证并修通全部 UI e2e：

1. **验证了中断会话的 HMAC 工作**（代码已就位，本会话未改动核心逻辑）：
   - `002_endpoint_auth_hmac.sql`（auth_type CHECK 扩到 0/1/2）、`internal/ingest/handler.go` 的 `validateHMACAuth`（X-Signature 头、HMAC-SHA256 原始 body、`sha256=` 前缀可选、`hmac.Equal` 常数时间比较；限流→读 body→鉴权的新顺序因 HMAC 签的是原始 body）
   - endpoints 密钥改明文返回（与 channels 同语义：前端 `originalSecretRef` 仅变更时提交）+ create/update 校验 auth_type
   - Go e2e `TestEndpointAuth` 15/15 子用例过（真库隔离 schema）
2. **UI e2e 套件跑通**（`web/scripts/ui-e2e/`，中断会话建的，本会话修复后 24/24 全绿）：endpoints 11（含数据面：2KB 边界/HMAC/bearer/令牌桶）、mappings 4、rules 5、templates 4
3. **本会话修的问题**：
   - `helpers.mjs` `deleteRow`：ConfirmDialog 是 radix **AlertDialog**（role=`alertdialog`，非 `dialog`），确认按钮文案是「删除」非「确认」
   - `helpers.mjs` 新增 `finish()`：CDP WebSocket 挂住 Node 事件循环，脚本跑完进程不退出
   - `helpers.mjs` `openApp` 用 CDP `Network.setCacheDisabled` 禁用浏览器缓存（防陈旧 index.html）
   - `endpoints.mjs` ER 限流用例开头等 2.2s 回填令牌桶（限流在 body 读取之前，前面用例已耗令牌）
   - `mappings.mjs` G-02 改测「半填行」：全空行按设计被 `buildPayloadFields` 静默丢弃（计划文档 M-03 语义即半填拦）
   - `templates.mjs`：「内容模板，使用」placeholder 是**全角逗号**
   - `RulesPage.tsx`：条件行字段补 zod `min(1)` + 渲染 `rules.fieldRequired`（原 locale 死键，空字段条件曾直接入库）
   - `router.go` `serveFrontend`：`/assets/*` 发 `Cache-Control: immutable`（内容哈希）；SPA 回退（index.html/favicon 等）发 `no-cache`——修「rebuild/重部署后旧标签页懒加载旧 chunk 404」

## UI e2e 用法

```bash
# 前置：Chrome --remote-debugging-port=9222；服务在 :9003（GoLand MCP "server" 配置，admin/admin）
cd web && node scripts/ui-e2e/endpoints.mjs --pass admin   # 另有 mappings/rules/templates.mjs
```

- 失败自动截图存 `scripts/ui-e2e/artifacts/`（**模型无图片输入，只看控制台输出**；需要时用 `ariaSnapshot()` 文本调试，临时脚本用完即删）
- `npm run build` 后用户开着的旧标签页需 F5（no-cache 头已把复发概率消除，但进程内存里的旧外壳仍要刷新）

## 验证命令

```bash
go build ./... && go vet ./internal/api/
TEST_DATABASE_DSN="postgres://notygo:notygo@master:5432/notygo?sslmode=disable" go test ./test/ -count=1   # e2e 全量，含 TestEndpointAuth
cd web && npm run build        # 含 tsc -b
```

## 当前状态 / 下一步

- 工作区**未提交**：HMAC 认证全套（后端+前端+迁移+Go e2e）+ ui-e2e 套件 + 本会话修复（RulesPage 校验、router 缓存头）+ 文档。`dev` 分支也未推送远端
- 提交建议拆两笔：`feat: endpoint HMAC 认证 + 密钥明文返回（含 e2e）`、`feat: Chrome-CDP UI e2e 套件 + 规则条件必填校验 + 静态资源缓存头`
- 其余待办同 10-26 会话：push origin dev → 合 master 打 tag；GitHub/Docker Hub 改名收尾；压测矩阵

## 约定速查（详见 docs/CLAUDE.md）

- i18n：文案结尾不加标点；插值**单花括号** `{n}`；中文用全角标点（本次踩坑：placeholder 半角逗号导致选择器失配）
- 术语：endpoint=接入点（进）、channel=渠道（出）、delivery=投递
- 提交：仅在用户要求时提交；`feat:/fix:/chore:` + 简短中文
- 模型限制：**不要往上下文读图片**（截图/PNG 会直接报 400 终止会话）

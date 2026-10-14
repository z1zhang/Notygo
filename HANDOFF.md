# zPush 交接文档

> 最后更新：2026-10-14，基于 dev 分支 commit `fda42ae`

## 一、本次完成的工作

### 1. 去重降噪（Dedup）

**完整实现了基于 payload_hash 的规则级去重**：

- 新增 `dedupe_keys` 表（迁移 007），存储 SHA-256 去重键 + 过期时间
- `DedupeConfig` 模型精简为只有 `window_seconds`（去掉了 `key_fields` 和 `action`）
- 去重键固定 = `rule_id + ":payload_hash=" + evt.PayloadHash`，动作固定 = skip
- `processMatchedRule` 返回 `bool`，告知调用方是否实际创建了 delivery
- **去重跳过的事件状态为 `deduped`（区别于 `completed`）**：`MarkDeduped()` 方法
- 前端 Rules 去重配置：开关 + 时间窗口（秒），开关关闭时输入框禁用，默认 300 秒
- 前端 StatusTag 和 i18n 支持 `deduped` 状态（enums.eventStatus.deduped）
- `DedupeCheckResult` 简化：移除 `ExpiresAt` 字段（queue 动作已删除）
- lease_recovery 定期清理过期 dedupe_keys

### 2. 事件处理改进

- **parsedFields vs evalFields 分离**：`parsedFields` 只含业务字段（存入 EventRun），`evalFields` = parsedFields + `payload_hash` + `source_id`（用于规则评估和去重）。payload_hash/source_id 不再出现在 UI 的"解析字段"中
- **run_type 字段**：Event 新增 `run_type`（`initial` / `replay`），重放事件通过 `SetForReplay()` 正确标记为 `replay`（迁移 008）
- **GetEvent API 移除冗余顶层 parsed_fields**：前端已从 `latestRun.parsed_fields` 读取
- **not_exists 操作符修复**：字段不存在时正确返回 `true`（之前返回 `false`）
- **float64 大数格式化修复**：`formatFieldValue()` 对整数值的 float64 用 `%d` 格式化，避免 `1e+09` 导致 eq 比较失败
- **诊断日志升级**：关键日志从 Debug 升级为 Info，增加 `formatFieldSummary()` 输出字段类型+截断值

### 3. 映射-规则关联重构

- **mapping 新增 `rule_ids` 字段**（迁移 004）：规则由映射引用，而非规则引用映射
- **mapping 新增 `continue_on_match` 字段**（迁移 005）：匹配后是否继续评估后续规则
- **rule 移除 `continue_on_match` 字段**（迁移 006）：已移到映射级别
- 前端 Mappings.vue：新增规则多选下拉 + continue_on_match 开关 + Smart Import 功能
- 前端 Mappings 列表：显示关联规则标签 + 字段数

### 4. 映射转换修复

- 前端 transform 选项值从 `"lower"/"upper"` 改为 `"to_lower"/"to_upper"`，与后端 switch case 对齐
- 移除不支持的 `"regex"` 选项
- **不做向后兼容**（用户明确说"不需要兼容"）

### 5. 其他修复

- **MySQL 时区偏移**：`NowExpr()` 从 `NOW()` 改为 `UTC_TIMESTAMP()`，修复 8 小时偏移
- **关联投递表增加 created_at/updated_at 列**（Events 侧栏）
- **连接池调整**：MaxOpenConns 25→50，MaxIdleConns 10→20
- **新增 notify 包**：基于 channel 的事件/投递通知，支持 coalescing，避免空轮询
- **Sources.vue 重构**：slug 点击复制 webhook URL、auth_type 标签、rate-limit 双输入框

## 二、当前状态：没有正在进行的任务

所有改动已提交到 dev 分支（`fda42ae`），Go build 和 Vue type check 均通过。

## 三、待办事项（CLAUDE.md 已记录 + 新增）

| 优先级 | 事项 | 说明 |
|--------|------|------|
| 高 | **XSS 风险** | Deliveries.vue 和 Templates.vue 使用 `v-html` 渲染 Markdown/HTML，需 DOMPurify 消毒 |
| 中 | `/preview/parse` 等预览端点 | 仍是 501 占位，Mappings Smart Import 依赖 `/preview/parse`，需实现 |
| 中 | `destinations/{id}/test` | 投递测试端点未实现 |
| 中 | `EVENT_RETENTION_DAYS` 清理 | 配置存在但无定时清理逻辑 |
| 低 | Dashboard 统计优化 | 4 次 API 拼凑，应改为专用后端端点 |
| 低 | 规则评估只看映射字段 | 未映射字段规则引用不到（设计限制，非 bug） |
| 低 | CLAUDE.md 更新 | 映射-规则关联、去重、deduped 状态等新功能未反映在 CLAUDE.md |

## 四、踩过的坑（绝对不要再踩）

### Go 文件编辑

1. **Go 文件用 tab 缩进**，Edit 工具的 `old_string` 必须精确匹配 tab。如果用空格匹配会找不到。遇到 Edit 反复失败时，改用 Python 脚本按行号替换
2. **Windows CRLF**：部分文件（如 `handlers_event.go`）有 CRLF 换行，导致 Edit 匹配失败。用 Python 显式替换比 Edit 更可靠
3. **反引号字符串**：Go 的 `` ` `` 反引号字符串内不能放函数调用，必须用 `"` + 字符串拼接

### MySQL

4. **UPDATE 子查询不能直接引用同表**，必须双层嵌套：
   ```sql
   -- ❌ MySQL 报错
   UPDATE events SET ... WHERE id = (SELECT id FROM events WHERE ...)
   -- ✅ 双层嵌套绕过
   UPDATE events SET ... WHERE id = (SELECT id FROM (SELECT id FROM events WHERE ...) AS claimable)
   ```
5. **DATETIME 列不接受 `T`/`Z` 和纳秒**，用 `storage.FormatTime()` 而非 `time.RFC3339Nano`
6. **`NOW()` vs `UTC_TIMESTAMP()`**：MySQL 的 `NOW()` 用服务器本地时间（可能 UTC+8），Go 端用 `time.Now().UTC()`，混用会产生 8 小时偏移。**必须用 `UTC_TIMESTAMP()`**

### 前端

7. **el-select 选项值必须与后端 switch case 完全一致**：之前前端发 `"lower"/"upper"`，后端期望 `"to_lower"/"to_upper"`，转换静默失败
8. **`show-overflow-tooltip` 与 `display: flex` 不兼容**：用 `<el-tooltip>` + `.cell-ellipsis` 代替
9. **el-descriptions 收缩**：需要 `:deep(.el-descriptions__table) { table-layout: fixed; width: 100% }` 才能随 drawer 缩小
10. **StatusTag 的 i18n 路径是 `enums.eventStatus.{status}`**：新增状态必须加在 `enums.eventStatus` 下，不是 `events` 下。`events` 下的是列表筛选用的，两处都要加

### 架构设计

11. **parsedFields vs evalFields**：系统元字段（payload_hash、source_id）不要注入到 parsedFields 里再给 UI 显示，要建单独的 evalFields 给规则评估和去重用
12. **processMatchedRule 返回 bool**：调用方需要知道是否实际创建了 delivery，否则无法区分 completed 和 deduped
13. **DedupeConfig 不要过度设计**：key_fields 多选和 queue 动作实际用不到，简化为开关+时间窗口即可，给太多选项反而容易误配

## 五、关键文件速查

| 文件 | 作用 |
|------|------|
| `internal/processor/processor.go` | 核心处理流程：claim→mapping→rules→dedupe→template→delivery |
| `internal/rules/model.go` | Rule + DedupeConfig 模型（DedupeConfig 只有 WindowSeconds） |
| `internal/event/model.go` | Event 状态常量（含 deduped）、EventRun、EventRunType |
| `internal/delivery/repository.go` | CheckAndInsertDedupeKey、CleanExpiredDedupeKeys |
| `internal/mapping/model.go` | Mapping 模型（含 RuleIDs、ContinueOnMatch） |
| `internal/notify/notify.go` | 事件/投递通知 channel（coalescing） |
| `internal/storage/timeutil.go` | FormatTime/ParseTime/NowExpr（UTC_TIMESTAMP） |
| `web/src/views/Rules.vue` | 规则编辑：开关+时间窗口去重配置 |
| `web/src/views/Mappings.vue` | 映射编辑：rule_ids 多选 + continue_on_match + Smart Import |
| `web/src/utils/status.ts` | eventStatusTagType（含 deduped） |
| `web/src/components/StatusTag.vue` | 通用状态标签，读 `enums.eventStatus.{status}` |

## 六、数据库迁移（001-008）

| 编号 | 内容 |
|------|------|
| 001 | 初始建表 |
| 002 | no-op（001 已含 rendered_format） |
| 003 | rules 加 mapping_id（已废弃，004 中删除） |
| 004 | mapping 加 rule_ids，rules 删 mapping_id |
| 005 | mapping 加 continue_on_match |
| 006 | rules 删 continue_on_match |
| 007 | 新建 dedupe_keys 表 |
| 008 | events 加 run_type |

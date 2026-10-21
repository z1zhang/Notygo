# zPush 会话交接文档

> 写给完全没有上下文的新会话。最后更新：2026-10-21（会话结束前）。

## 一、我们在做什么

zPush 是自托管 Webhook 事件路由与通知推送系统（Go 1.26 + chi + MySQL 后端，React 19 前端，项目全景见 `docs/CLAUDE.md`）。

前端 React 重写**已完成并合入**，当前分支 `dev`（主分支 `master`），本地与 `origin/dev` 完全同步（最新提交 `70cabd8`）。

当前大任务：**用户逐页走查 UI/UX，一次提一个视觉/交互问题，即时修复**。用户强诉求是**跨页面完全一致**（同类控件必须长一个样，改一处想到同类处）。工作循环：改代码 → `tsc --noEmit` → 必要时 CDP 实测 → 如实汇报（实测数据附上）。

本轮走查进行到的页面：沙盒（Playground）刚完成深度对齐映射页智能导入，之后是全局悬浮提示统一、弹窗聚焦问题。**用户会继续走查，预计还有新反馈。**

### 2026-10-21 安全加固线（未提交，先于 UI 走查）

用户决策：**不做双端口**，改用路径命名空间分离管理 API 与通知 API；外加登录防爆破。已完成：

1. **通知 API 迁移**：`POST /api/v1/ingest/{slug}` → **`POST /ingest/{slug}`**（改挂顶层，旧路径 `/api/v1/ingest/` 已彻底移除，用户拍板不兼容）。`router.go` 顶层挂载，SPA fallback 对 `/ingest/` 前缀返回 JSON 404。前端 `SourcesPage.tsx` 复制的 webhook URL 已同步
2. **登录审计**：成功/失败都写 `audit_logs`（action `auth.login_success`/`auth.login_failed`，summary 含 `ip=... username=...`）；锁定期间被拒的探测不写审计（防刷爆审计表）
3. **登录锁定**：`internal/api/login_guard.go` 内存滑动窗口，按 IP + 用户名双计数，429 + 新错误码 `AUTH_001`；env `LOGIN_MAX_FAILURES`（默认 5）/`LOGIN_LOCKOUT_SECONDS`（默认 900）。有单测 `login_guard_test.go`
4. **时序侧信道**：登录未知用户名跑假 bcrypt（防用户名枚举）；ingest bearer 改 `subtle.ConstantTimeCompare`
5. 前端：`LoginPage` 对 `AUTH_001` 显示专门文案 `login.locked`（zh/en 已加）
6. **数据库配置拆分**：`DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME` 独立变量拼装 DSN（`config.buildMySQLDSN`，FormatDSN 转义特殊字符密码）；**`DATABASE_DSN` 变量已删除**（用户拍板，不留整串覆盖）。有 `config_test.go` 单测。README/compose/文档全部同步
7. **渠道测试发送已实现**（`POST /api/v1/destinations/{id}/test`）：把 delivery worker 的 `buildDestinationConfig` 抽到 `destination.BuildSenderConfig` 共用；成功判定 = err==nil 且 2xx 且 BusinessCode 为空或 0（覆盖钉钉 errcode）；审计 action `test`；前端渠道页每行新增 Zap 图标按钮，i18n `destinations.test*`
8. **HTTP 渠道自定义请求体**（`body_fields` 树）：经历三轮迭代——占位符方案 → 平铺 title/body 映射（`body_shape`，已删除）→ **最终版递归树**。契约存树形结构 `BodyField{Key, Source, Value, Type, Children}`：叶子行的值来源可选「模板标题/模板内容/格式/自定义」（title/body/format/literal，多个 key 可映射同一来源）；分组行（type `object`/`array`）嵌套子节点，数组内子键名被忽略（按位置）。后端 `buildBodyFields`/`buildGroupValue`/`resolveBodySource`（`http.go`）用 `json.Marshal` 组装（转义天然正确），错误校验：缺来源/非法来源/对象内空键或重复键。优先级 body_fields > 默认 {title,body,format}（**原始请求体 body_template 后来按用户要求整体移除**，树是唯一自定义方式）。前端 `DestinationsPage.tsx` 递归组件 `BodyFieldRows`/`BodyFieldRow`（RHF 递归 `useFieldArray`/`useWatch`，路径用 `as never` 边界转换；zod v4 递归 `z.lazy` 会把推导降级成 `unknown[]`，resolver 需 `as Resolver<DestinationForm>` 收口）；提交前 `cleanBodyFields` 剔除未填完的行。UI 上用户嫌「添加字段/添加分组」两按钮臃肿，已合并为单个「添加字段」——每行带类型选择（字段/对象/数组），切类型就地转换（叶子→分组清来源，分组→叶子清子行并恢复默认来源）。`http_test.go` 5 个单测。用户明确**不要 TG/Bark 预设**，只要自定义模式
9. **i18n 全量交给 react-i18next**（两步走，最终形态为完全库接管）：
   - 第一步迁移 i18next + react-i18next（保留 `useI18n` 适配层）；第二步按用户要求「完全接管」：**23 个调用文件全部改为标准 `useTranslation()` 直接调用**，自研 hook/持久化/回退全删。语言探测/持久化交 `i18next-browser-languagedetector`（order localStorage→navigator，键仍 `zpush-locale`，**首访行为变化：跟随浏览器语言**，fallback en）；`<html lang>` 同步留在 `I18nProvider`（内部就是 I18nextProvider + 10 行 effect）；`LocaleSwitch` 改用 `useLocale()`
   - 键类型校验用 i18next 自带的 `CustomTypeOptions` 模块增强（zh 树为真源）：字面量键拼错直接 tsc 报错（报错还带「Did you mean」提示），**string 类型的动态键天然放行**（枚举拼接不用 cast）；`t(key, 'fb')` 字符串回退改成标准的 `{ defaultValue: fb }`；en 缺键仍由 `_enKeyParity` 编译期断言拦截
   - **复数已接入**：JSON 键带 `_one`/`_other` 后缀（zh 只需 `_other`），调用传 `count`；实证用例 `pagination.range`（en "of 1 record" / "of 5 records"）。**格式化已接入**：i18next v26 内置 Formatter（`{v, number}`、`{v, datetime(dateStyle: medium)}`、`{v, currency}`、`{v, list}`），`{ts, relative}` 是经 `services.formatter.add` 注册的自定义 formatter（时间戳→「3 分钟前」）。运行时已用 node 脚本实测：复数两语言、defaultValue 回退、number/datetime 输出全对
   - 注意：i18next v26 已**移除** `interpolation.format` 回调（老教程写法），必须用内置 Formatter 或 `services.formatter.add` 扩展点
10. **初始管理员用户名可配置**：新增 env `ADMIN_INITIAL_USER`（默认 `admin`），与 `ADMIN_INITIAL_PASSWORD` 一样仅在尚无管理员时生效；`config.AdminConfig.InitialUsername` + `main.go ensureAdminUser` 使用参数化插入。README/CLAUDE.md 环境变量表同步
11. **投递请求摘要捕获线上报文原文**：用户问「请求摘要是实际请求吗」→ 之前只是发送前投影（method/url/脱敏头/标题正文片段），不含实际组装报文。现在 `SendResult` 新增 `RequestPayload` 字段，**五个 sender 全部回传线上原文**（http 的 body_fields 组装 JSON、钉钉/飞书/企微的 marshal 结果、email 的完整 RFC 2822 MIME 文本；GET/HEAD 为空）；worker 在 Send 后用 `attachRequestPayload` 把它合并进 request_summary 的 `payload` 字段（超 2048 字节截断），失败尝试也记录（payload 在发送前就已组装）。前端 `JsonViewer` 自动展示，零前端改动。`worker_test.go` 新增 3 个 attachRequestPayload 测试
12. **前端渠道表单抽离为插件模块**：DestinationsPage 从 1037 行瘦身到 ~460 行，渠道专属代码全部迁入 `web/src/features/destinations/channels/`：
    - `types.ts`：`ChannelModule` 契约（types/formSchema/defaults/defaultMsgType/Fields/toConfig/fromConfig/validate）+ `asString`/`asNumber` + `BodyFieldNode`
    - `webhook.tsx`（钉钉/飞书/企微共用，MSG_TYPE_OPTIONS + fromConfig 的 msg_type 归一化）、`email.tsx`、`httpWebhook.tsx`（含递归 BodyFieldRows/BodyFieldRow 树编辑器、cleanBodyFields）
    - `index.ts`：`DEST_TYPES`/`CHANNELS` 注册表、`channelModule()`（未知类型回退 http）、`destinationSchema`（展开各模块 formSchema + superRefine 委托模块 validate）、`DestinationForm` 交叉类型、`defaultForm`、`destinationResolver`
    - 页面只保留：表格/弹窗框架、公共字段（name/type/enabled/secret/timeout）、`ActiveFields = channelModule(type).Fields` 渲染、saveMutation 委托 `toConfig`、openEdit 委托 `fromConfig`、handleTypeChange 重置所有模块 defaults
    - 边界类型放宽：`AnyForm`（Record<string, any>）+ 少量 `as never`/`as unknown as` 文档化断言（zod v4 递归 z.lazy 推断退化为 unknown[]，与此前单体页面同样的问题）
    - 修过的坑：httpWebhook.tsx 曾误从 react-hook-form 导入 useTranslation；index.ts 的 `uniqueModules` 需导出（页面 handleTypeChange 用）
    - 新增渠道前端侧 = 一个模块文件 + index.ts 两行（DEST_TYPES + CHANNELS）；README「新增一种投递渠道」步骤 3 与 CLAUDE.md 目录结构已同步
13. **渠道类型 `http_webhook` 重命名为 `http`**：sender 注册键/Go 文件（`http.go`）与标识符（HTTPConfig/HTTPSender）/前端 DEST_TYPES/i18n 键全栈统一；新增迁移 010 把存量 `destinations.type='http_webhook'` 平滑改为 `'http'`（含 Down，embed FS 启动自动执行）。外部脚本用 API 创建渠道需改传 `"http"`。同时移除从未使用的 `EVENT_RETENTION_DAYS`（配置加载存在、清理逻辑始终未实现，属死配置）
14. **渠道级投递重试次数（max_attempts）**：destinations 新增 `max_attempts` 列（迁移 011，默认 5），模型/仓库/创建与更新请求全部带指针字段（部分更新语义与 timeout_ms 一致）；handler 校验 1–100；processor 创建投递时逐渠道 `destRepo.GetByID` 取值，≤0 或渠道缺失回退常量 5（原 `DEFAULT_MAX_ATTEMPTS` env 已按用户要求移除——前端已有渠道级配置，回退仅覆盖渠道被删的边缘场景）。前端表单公共区在超时旁加数字输入（zod 1–100），i18n 键 `destinations.maxAttempts`。注意：修改渠道的 max_attempts 只影响之后新建的投递，存量投递行创建时已固化的次数不变
15. **迁移链压平为单一基线**：用户重建了空数据库，002–011 共 9 个增量迁移（含 no-op 的 002、动态 SQL 的 004）全部折进 `001_init.sql` 最终形态：mappings 带 rule_ids/continue_on_match，rules 去掉 condition_mode/continue_on_match，events 带 run_type、无 expire_at，destinations 带 max_attempts、无 rate_limit/circuit_breaker，sources 无 expected_response，dedupe_keys 表并入。timeout_ms 列默认从 5000 对齐为 10000（应用层默认值，列默认实际不会被用到）。**只适用全新数据库**——老库 goose 版本已到 11，切到压平版会因版本表不匹配而重复建表报错

验证：`go build/vet/test` 全过，`tsc -b --noEmit` 过、`npm run build` 过。**Docker/运行时冒烟未做（本机无 docker/MySQL）**；用户现有 webhook 调用方需要把 URL 改成 `/ingest/{slug}`（曾短暂定为 `/push/{slug}`，最终用户改回 ingest）。

## 二、最近一次提交内容（`70cabd8`，已推送）

1. **移除 next-themes**，自研 `web/src/app/theme.tsx`：`ThemeProvider` + `applyTheme` + `disableTransitionsTemporarily()`（切换主题瞬间生效，注入一帧 `transition:none!important`）。`index.html` 里有 parse-time 内联脚本防 FOUC，读 localStorage `zpush-theme`。`providers.tsx` / `sonner.tsx` / `ThemeToggle.tsx` / `SettingsPage.tsx` 的 `useTheme` 都改从 `@/app/theme` 导入
2. **沙盒对齐映射智能导入**（用户点名"细节必须一样"）：+号点击无抖动（✓ 占位用 `size-8`，与 `icon-sm` 按钮同宽）、路径 `w-40 shrink-0 truncate`、样本值 `truncate`、类型徽章按值着色（string=secondary/number=outline/boolean=default/array=destructive）、容器 `max-h-60`、payload 文本域 `field-sizing-fixed`（固定高度+滚动条）
3. **布局对齐**：「添加字段」按钮上移到区块标题行右侧；左右列统一 `space-y-3`
4. **`main` 加 `[scrollbar-gutter:stable]`**（AppShell.tsx）：修"分析 JSON 时整页像被放大"的抖动。**注意滚动容器是 `main` 不是 `html`**（第一次改错位置，无效）
5. **全站原生 `title` 悬浮 → shadcn Tooltip**：沙盒、映射智能导入、`TruncatedId`（日志表格 ID）、投递批量重试禁用原因（外面套 span——禁用按钮不触发鼠标事件）、规则 AND/OR 切换、侧栏收起导航（`side="right"`）。代码里剩余 `title=` 全是组件 prop（PageHeader/EmptyState/ConfirmDialog/DetailSection/SectionHeader），**不是**原生悬浮，别再改
6. **`DialogContent` 默认阻止自动聚焦**（`onOpenAutoFocus` → `preventDefault`，可被调用方覆盖）；沙盒弹窗的 `autoFocus` 已移除
7. 侧栏图标（渠道=Send、投递日志=SendHorizontal、规则=Filter、审计=ScrollText）、destination 译名「渠道」（auditTargetType 等）、尝试次数灰字、映射规则 tag 灰底（secondary）

## 三、当前卡在哪 / 待办

**没有卡住的东西。** 待办清单：

1. **Sheet（右侧详情抽屉）和 AlertDialog（确认框）仍会自动聚焦**——用户只要了弹窗（Dialog）不聚焦，Sheet/AlertDialog 没动。用户若再提，照 `dialog.tsx` 的 `onOpenAutoFocus` 模式改 `sheet.tsx` / `alert-dialog.tsx`
2. 后端遗留（见 `docs/CLAUDE.md` 已知问题）：规则引用不到未映射字段、Settings 无配置 API、Docker 构建未验证（`destinations/{id}/test` 已于 2026-10-21 实现）
3. **`docs/CLAUDE.md` 有几行已过时**：还写着 "next-themes"、"Dashboard 统计用 4 次 API"、favicon 问题——这三项本会话/上会话已修，新会话顺手更新掉
4. 上会话的 Go 后端修复（PROCESSOR_005 渲染失败拦截、`/dashboard/stats` 端点、批量重试校验、favicon 静态服务）在提交 `68455ac` 里，**需要重启 Go 服务才生效**，用户未确认重启过

## 四、CDP 验证工作流（重要资产，照此操作）

用户开着 debug Chrome（CDP 端口 9222），Vite dev server 在 `localhost:3000`（HMR，改文件即时生效）。**必须**把脚本写成 `.tmp-cdp/*.mjs` 文件再 `node` 运行（Node 内置 WebSocket，不用 `ws` 包）：

```js
const tabs = (await (await fetch('http://127.0.0.1:9222/json')).json())
  .filter((t) => t.type === 'page' && t.url.includes('localhost:3000')); // 用户常开很多别的 tab，必须按 URL 过滤！
const ws = new WebSocket('ws://127.0.0.1:9222/devtools/page/' + tabs[0].id);
// send/evalJs 样板见旧脚本 .tmp-cdp/*.mjs（check-playground.mjs、check-jitter.mjs 等）
```

- React 受控 textarea 填值要用原生 setter + `dispatchEvent(new Event('input', {bubbles:true}))`
- 截图存 `.tmp-cdp/` 内（Read 工具读不了工作目录外路径），看完清理
- **量 DOM 必须在同一次 `Runtime.evaluate` 里量所有元素**（分开两次会因用户滚动/页面变化出现假差异，本会话踩过：两次测量差 14px 误判不齐平）

## 五、踩过的坑 —— 绝对不要再踩

### 本会话新增
- **页面滚动容器是 AppShell 的 `<main class="overflow-y-auto">`，不是 `html/body`**——`scrollbar-gutter`、防滚动条抖动之类的修复必须打在 `main` 上
- **Radix DropdownMenu 无法用合成点击打开**（`.click()` 和 `Input.dispatchMouseEvent` 都失败），自动化验证主题下拉曾放弃改人工。Radix 普通按钮（如智能导入的 + 号）`.click()` 是有效的
- **`node -e` 里嵌套模板字符串 + bash 双引号转义地狱**（`\\/` 等）——一律写 `.mjs` 文件，别内联
- **Edit 工具替换后行首缩进可能错位**，后续匹配会 "String not found"；改完立刻 Read 回看该区域，别硬猜缩进
- **claim 类陈述（"领先 3 个提交"）出自压缩前的旧摘要，可能过时**——汇报前先 `git log origin/dev` 核实，本会话因此报过一次错误的推送内容
- **悬浮提示的判定**：`title={t(...)}` 出现在 `PageHeader`/`EmptyState`/`DetailSection`/`SectionHeader`/`ConfirmDialog` 上是**组件 prop**，不是浏览器原生悬浮，不要替换

### 沿用旧坑（仍然有效）
- Tailwind v4 `space-y-*` + Radix Select 隐藏 `<select>` 的 margin bug 已在 `globals.css` 用 `:has()` 全局修复，**不要再从组件层面修**
- CDP 测 DOM 前先 `location.href` 重新导航（瞬态 portal 会污染测量）；点击后留 ≥600ms 再量
- Radix Select trigger 是 `button[role="combobox"]`，选项在 body 级 `[data-radix-popper-content-wrapper]` 里
- Tailwind v4 位移用独立 `translate` 属性，`computed transform` 恒 "none"
- Button cva 里用户手加的 `active:scale-[0.9]` 不是垃圾，别清
- MySQL 陷阱见 `docs/CLAUDE.md`（UPDATE 同表子查询双层嵌套、DATETIME 用 `storage.FormatTime()`、Go 反引号字符串内不能放函数调用）

## 六、文案/排版约定（用户已确认，写进了持久记忆，但这里再列一遍）

- zh 文案结尾**不加**「。」/「.」；中文引用用全角引号
- 术语：destination=**渠道**、delivery=**投递**、source=**数据源**（「接收」被用户否决过，别再提）
- 排版：灰色=元数据、黑色=数据值、非标题黑字不加粗、**mono 仅限代码块**（正文/悬浮内容一律 font-sans）
- 类型徽章（智能导入/沙盒推荐列表）显示**原始英文** `string`/`boolean`（用户明确撤掉过汉化）；但表单里的值类型下拉是汉化的，两处不一致是**故意的**

## 七、给新会话的开工建议

1. 先读 `docs/CLAUDE.md`（项目全景）和本文件
2. 问用户当前在走查哪一页、有什么新反馈
3. 改任何 UI 前先 Grep 找同类页面的既有模式，**先对齐再创新**；改完必跑 `cd web && npx tsc -p tsconfig.app.json --noEmit`
4. 涉及布局尺寸的结论要用 CDP 实测背书（同一 evaluate 内测量），别肉眼判断

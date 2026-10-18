# zPush 会话交接文档

> 写给完全没有上下文的新会话。最后更新：2026-10-18。

## 一、我们在做什么

zPush 是自托管 Webhook 事件路由与通知推送系统（Go + MySQL 后端，详见根目录 `CLAUDE.md`）。

当前大任务：**把前端从 Vue 重写为 React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + shadcn/ui（new-york 风格）+ TanStack Query 5 + React Hook Form/Zod + sonner**，新代码在 `web/`，旧 `web/`（Vue）已删除。分支：`react-rewrite`（主分支是 `master`）。

工作模式：**用户逐页走查 UI，一次提一个视觉/交互问题**，我即时修复 → CDP 实测验证 → 类型检查 → 如实汇报。用户的强诉求是**跨页面视觉完全统一**（所有弹窗里的开关、下拉框、添加按钮、label 摆法必须长一个样）。

## 二、已完成

### 本次（最新）已提交
用户已手动提交两个 commit（本会话因权限分类器故障无法执行 git 写命令，最终用户用 `!` 前缀自己提交的）：
- `bcbf180` — chore: 移除旧 Vue 前端 web/ 及遗留 zpush.sql（-10294 行）
- `dccb4b9` — feat: 新增 web/（React 19 重写）+ Dockerfile 路径调整（web/ → web/，产物仍复制到镜像内 web/dist）+ .gitattributes（统一 LF）+ .gitignore（web/node_modules、web/dist）+ `internal/api/handlers_preview.go` 双模式改造

注意：这两个 commit 的 message 是重复的（用户提交时把两个 message 拼在了每个 commit 里），内容拆分是正确的，只是 message 不精确。**不要去改历史**。

### 本会话窗口完成的 UI 统一改造（均已包含在上述提交中）
1. **启用开关统一模式**（sources/rules/destinations 全部一致）：label 在上、Switch 在下，包在 `flex h-9 items-center` 里，**无描边盒子**；启用列永远放行尾
2. **PasswordInput 组件**（`web/src/components/ui/password-input.tsx`）：Input + 眼睛按钮（icon-sm ghost，`absolute top-1/2 right-1 -translate-y-1/2`），sources 密钥、destinations 密钥都用它
3. **共享 MultiSelect**（`web/src/components/form/MultiSelect.tsx`）：badge-in-trigger 风格、`w-(--radix-popover-trigger-width)`、未知 id 兜底。旧的 `features/mappings/MultiRuleSelect.tsx` 和 `features/rules/MultiSelect.tsx` 已删除，统一用这个
4. **区块标题行模式**（映射"字段"、规则"条件"、投递"自定义请求头"完全一致）：`flex items-center justify-between`，左 `<Label>`，右 `<Button variant="outline" size="sm"><Plus/>文案</Button>`
5. **规则弹窗条件模式 AND/OR**：改为滑块分段控件（sliding pill），移入"条件"标题行：`[条件] [全部匹配|任一匹配] ··· [+ 添加条件]`。pill 用 `absolute inset-y-0.5 left-0.5 w-[calc(50%-2px)] bg-primary transition-transform` + `translate-x-full`；按钮 `text-sm`（与 sm 按钮字号一致，14px，实测对齐）。i18n 键 `rules.andShort`/`rules.orShort`（全部匹配/任一匹配），长文案 `andMode`/`orMode` 留作 title tooltip
6. **映射值类型对齐后端**：`VALUE_TYPES = ['string','number','boolean','object','array']`（mappings 页 + playground 页），`normalizeValueType()` 兼容旧数据（'bool'→'boolean'，'json'→'object'）；智能导入点 + 时默认选中正确类型
7. **模板"渲染"改为纯预览**：后端 `/api/v1/preview/template` 支持 template_id 或 inline 内容（title_template/body_template/format）双模式，前端 `previewTemplate()` 传 inline spec，渲染不再隐式创建模板
8. **Tailwind v4 space-y + Radix Select 隐藏元素 bug 的全局修复**（`web/src/styles/globals.css`）：`:where([class*="space-y"] > *:has(+ select[aria-hidden="true"]:is([tabindex="-1"])))` 置零多余 margin

## 三、当前卡在哪 / 待办

1. **⚠️ 本地 Go 服务跑的是旧二进制** — `/preview/template` 的 inline 模式支持需要重启才生效。用户尚未重启（之前报过 "template_id is required (CONFIG_001)"）。让用户执行 `! go run ./cmd/server`（需要 MySQL 和环境变量就绪）。验证方式：`curl -X POST localhost:9003/api/v1/preview/template -H 'Content-Type: application/json' -d '{"title_template":"{{.x}}","format":"text","sample_data":{"x":"hi"}}'`
2. `destinations/{id}/test` 仍是 501 占位
3. EVENT_RETENTION_DAYS 无清理逻辑；Dashboard 4 次 API 拼凑；Settings 无配置 API（见 CLAUDE.md 已知问题清单）
4. Docker 构建未在本机验证（本机无 docker 命令）
5. 用户会继续逐页走查 UI，**预计还有新的反馈**

## 四、CDP 验证工作流（重要资产，照此操作）

用户开着 debug Chrome（CDP 端口 9222，取第一个 tab），dev server 在 `localhost:3000`（Vite HMR，改文件即时生效）。

**必须**把脚本写成 `.tmp-cdp/*.mjs` 文件再 `node` 运行（用 Node 内置 WebSocket，不用 `ws` 包）。模板：

```js
const ws = new WebSocket('ws://127.0.0.1:9222/devtools/page/' + (await (await fetch('http://127.0.0.1:9222/json')).json())[0].id);
let id = 0; const pending = new Map();
function send(method, params = {}) {
  return new Promise((resolve, reject) => {
    const mid = ++id; pending.set(mid, { resolve, reject });
    ws.send(JSON.stringify({ id: mid, method, params }));
  });
}
ws.onmessage = (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id); pending.delete(msg.id);
    msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result);
  }
};
await new Promise((r) => (ws.onopen = r));
async function evalJs(expr) {
  const res = await send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true });
  if (res.exceptionDetails) throw new Error('JS exception: ' + JSON.stringify(res.exceptionDetails.exception?.description ?? res.exceptionDetails.text));
  return res.result.value;
}
await send('Page.enable');
await evalJs(`location.href = 'http://localhost:3000/rules'`);
await new Promise((r) => setTimeout(r, 1500));
// ...点按钮开弹窗、量 DOM、截图
const shot = await send('Page.captureScreenshot', { format: 'png' });
const { writeFileSync } = await import('node:fs');
writeFileSync('.tmp-cdp/shot.png', Buffer.from(shot.data, 'base64'));
```

截图后用 Read 工具看图，验证完 `rm -rf .tmp-cdp`（有时 Windows 句柄占用删不掉目录，清空内容即可，无害）。

## 五、踩过的坑 —— 绝对不要再踩

### 工具/流程类
- **`node -e` 里写多行 JS 会语法报错**（"Expected ',', got 'const'"）——一律写脚本文件
- **截图必须存在仓库目录内**（`.tmp-cdp/`），Read 工具被 `permissions.blockReadsOutsideWorkingDirectories` 限制，读不了仓库外路径
- **权限分类器可能长时间超时**（本会话末尾发生过，git commit 全被挡）。遇到时：不要疯狂重试，告知用户可以用 `! <命令>` 前缀自己执行；读操作不受影响
- **CDP 测 DOM 前先 `location.href` 重新导航**：弹窗状态、Radix portal 都是瞬态的，旧会话残留状态会污染测量结果

### 前端技术类
- **Tailwind v4 的 `space-y-*` 用 margin-bottom 打在非末尾子元素上**（`:where(& > :not(:last-child))`，零特异性、layered）。Radix Select 会在末尾塞一个隐藏原生 `<select>`（absolute 1px aria-hidden），导致前面的控件凭空多出 margin —— 这就是"启用开关下移 8px"的根因，已用 globals.css 的 `:has()` 规则全局修复，**不要再从组件层面修**
- **Radix Select 的 trigger 是 `button[role="combobox"]`**，不是 `aria-haspopup`；popover 内容渲染在 body 级的 `[data-radix-popper-content-wrapper]` portal 里，**在 `[role=dialog]` 里查不到选项**，必须全 document 查
- **Tailwind v4 的 translate 用独立的 `translate` 属性**，computed `transform` 恒为 "none"，测位移要读 `getComputedStyle(el).translate`
- **CDP 里 `button.click()` 是有效的**，React 合成事件能收到；但测量要留足时间（≥600ms，等 transition 和 React commit），否则读到过渡中间态会误判 bug（本会话就误判过一次"滑块不动"，实际是好的）
- **Button 组件的 cva 里有用户手加的 `active:scale-[0.9]`**，不要当垃圾清掉
- 按钮尺寸约定：default=h-9/14px 字、sm=h-8/14px 字、xs=h-6/12px 字。**sm 是"紧凑档"，字号仍是 14px** —— 自定义控件要与 sm 并排时字号必须 14px

### 后端类
- MySQL 陷阱见 CLAUDE.md（UPDATE 同表子查询双层嵌套、DATETIME 用 `storage.FormatTime()`、Go 反引号字符串里不能放函数调用）
- `handlers_preview.go` 里曾因 `tmpl, err :=` 在 if 块内 shadowing 出过编译问题，改用局部 `t, err :=` 再赋值

## 六、常用命令

```bash
go build ./... && go test ./...                                  # 后端
cd web && npx tsc -p tsconfig.app.json --noEmit            # 前端类型检查（每次改完必跑）
cd web && npm run build                                     # 前端生产构建
cd web && node .tmp-cdp/xxx.mjs                             # CDP 验证脚本
```

## 七、给新会话的开工建议

1. 先读根目录 `CLAUDE.md`（项目全景）和本文件
2. 问用户当前在走查哪一页、有什么新反馈；或从"待办"第 1 项开始（提醒重启 Go 服务并验证 inline 预览）
3. 改任何 UI 前先 Grep 找同类页面的既有模式（本文件第二节列的 4 个统一模式是基准），**先对齐再创新**

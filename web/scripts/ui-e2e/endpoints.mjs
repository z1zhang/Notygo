// 接入点 UI 用例(functional-test-plan.md §4)+ 数据面边界/限流/鉴权。
// UI 部分:三种认证各建一个(含 2KB / 速率 2/4 表单值)、编辑回填明文 secret、CRUD。
// 数据面部分:字节级边界、令牌桶行为、HMAC 签名 —— 直接 fetch,不走浏览器。
//
// 前置:Chrome --remote-debugging-port=9222;被测服务已运行且指向目标库。
// 运行:node scripts/ui-e2e/endpoints.mjs --base http://127.0.0.1:9003 --user admin --pass <密码>
// 或整套跑:npm run test:ui -- --pass <密码>(参数透传,前置条件相同)

import { createHmac } from 'node:crypto'
import { openApp, login, gotoNav, expectToast, expectRow, expectNoRow,
  openCreate, saveDialog, editRow, deleteRow, selectOption, runCases, finish } from './helpers.mjs'

const PREFIX = '[E2E]'
const HMAC_SECRET = 'ui-e2e-hmac-secret'

const hmacBody = (body, key) => createHmac('sha256', key).update(body).digest('hex')
// `{"pad":"<n×x>"}` total length = 10 + n
const exactBody = (total) => `{"pad":"${'x'.repeat(total - 10)}"}`

async function ingest(base, slug, body, headers = {}) {
  const res = await fetch(`${base}/endpoint/${slug}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...headers },
    body,
  })
  const text = await res.text()
  return { status: res.status, body: text }
}

const { args, page } = await openApp()
await login(page, args)
await gotoNav(page, '接入点')

const cases = [
  {
    name: 'E-01 新建 none 接入点(2KB / 速率 2/4)',
    fn: async () => {
      const dlg = await openCreate(page, '新建接入点')
      await dlg.getByLabel('名称').fill(`${PREFIX}none`)
      await dlg.getByLabel('最大请求体').fill('2')
      await dlg.getByLabel('每秒请求数').fill('2')
      await dlg.getByLabel('瞬时峰值').fill('4')
      await saveDialog(page, dlg)
      await expectToast(page, '接入点已创建')
      await expectRow(page, `${PREFIX}none`)
    },
  },
  {
    name: 'E-02 新建 bearer 接入点(带密钥)',
    fn: async () => {
      const dlg = await openCreate(page, '新建接入点')
      await dlg.getByLabel('名称').fill(`${PREFIX}bearer`)
      await selectOption(page, dlg, 'Bearer')
      await dlg.getByLabel('密钥 / Token').fill('ui-e2e-bearer-token')
      await saveDialog(page, dlg)
      await expectToast(page, '接入点已创建')
      await expectRow(page, `${PREFIX}bearer`)
    },
  },
  {
    name: 'E-03 新建 hmac 接入点(随机生成密钥)',
    fn: async () => {
      const dlg = await openCreate(page, '新建接入点')
      await dlg.getByLabel('名称').fill(`${PREFIX}hmac`)
      await selectOption(page, dlg, 'HMAC')
      await dlg.getByLabel('密钥 / Token').fill(HMAC_SECRET)
      await saveDialog(page, dlg)
      await expectToast(page, '接入点已创建')
      await expectRow(page, `${PREFIX}hmac`)
    },
  },
  {
    name: 'E-04 编辑回填明文 secret(留空=保留)',
    fn: async () => {
      const dlg = await editRow(page, `${PREFIX}hmac`)
      const secret = dlg.getByLabel('密钥 / Token')
      const value = await secret.inputValue()
      if (value !== HMAC_SECRET) throw new Error(`secret 回填值不符: ${JSON.stringify(value)}`)
      await dlg.getByLabel('名称').fill(`${PREFIX}hmac-改名`)
      await saveDialog(page, dlg)
      await expectToast(page, '接入点已更新')
      await expectRow(page, `${PREFIX}hmac-改名`)
    },
  },
  {
    name: 'G-02 必填校验:空名称提交被拦截',
    fn: async () => {
      const dlg = await openCreate(page, '新建接入点')
      await saveDialog(page, dlg).catch(() => {})
      await dlg.getByText('名称为必填项').waitFor()
      await dlg.getByRole('button', { name: '取消' }).click()
      await dlg.waitFor({ state: 'detached' })
    },
  },
]

// 数据面用例需要 slug:从页面表格拿不到,通过管理 API 查(token 复用页面
// localStorage 里 api-client 存的原值,非 JSON 编码)。
async function listEndpoints() {
  const token = await page.evaluate(() => localStorage.getItem('token'))
  const res = await fetch(`${args.base}/api/v1/endpoints?page=1&size=100`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  return (await res.json()).data
}

const dataCases = [
  {
    name: 'EB-01/02/03 2KB 边界:2047/2048 过、2049 拒',
    fn: async () => {
      const eps = await listEndpoints()
      const ep = eps.find((e) => e.name === `${PREFIX}none`)
      const push = (total) => ingest(args.base, ep.slug, exactBody(total))
      if ((await push(2047)).status !== 202) throw new Error('2047 字节应 202')
      if ((await push(2048)).status !== 202) throw new Error('2048 字节(=上限)应 202')
      const r = await push(2049)
      if (r.status !== 413) throw new Error(`2049 字节应 413,实际 ${r.status}: ${r.body}`)
    },
  },
  {
    name: 'EB-05 空 body → 400 invalid JSON',
    fn: async () => {
      const eps = await listEndpoints()
      const ep = eps.find((e) => e.name === `${PREFIX}none`)
      const r = await ingest(args.base, ep.slug, '')
      if (r.status !== 400) throw new Error(`空 body 应 400,实际 ${r.status}`)
    },
  },
  {
    name: 'EA-01/02 HMAC:正确签名 202、错误密钥/缺头 401',
    fn: async () => {
      const eps = await listEndpoints()
      const ep = eps.find((e) => e.name === `${PREFIX}hmac-改名`)
      const body = '{"event":"ui"}'
      const ok = await ingest(args.base, ep.slug, body, { 'X-Signature': hmacBody(body, HMAC_SECRET) })
      if (ok.status !== 202) throw new Error(`正确签名应 202,实际 ${ok.status}: ${ok.body}`)
      const bad = await ingest(args.base, ep.slug, body, { 'X-Signature': hmacBody(body, 'wrong') })
      if (bad.status !== 401) throw new Error(`错误密钥应 401,实际 ${bad.status}`)
      const none = await ingest(args.base, ep.slug, body)
      if (none.status !== 401) throw new Error(`缺头应 401,实际 ${none.status}`)
    },
  },
  {
    name: 'EA-03 Bearer:正确 token 202、错误 401',
    fn: async () => {
      const eps = await listEndpoints()
      const ep = eps.find((e) => e.name === `${PREFIX}bearer`)
      const body = '{"a":1}'
      const ok = await ingest(args.base, ep.slug, body, { Authorization: 'Bearer ui-e2e-bearer-token' })
      if (ok.status !== 202) throw new Error(`正确 token 应 202,实际 ${ok.status}: ${ok.body}`)
      const bad = await ingest(args.base, ep.slug, body, { Authorization: 'Bearer nope' })
      if (bad.status !== 401) throw new Error(`错误 token 应 401,实际 ${bad.status}`)
    },
  },
  {
    name: 'ER-01/02/03 速率 2/峰值 4:突发 4 全过、第 5 个 429、1.2s 后补 2',
    fn: async () => {
      // Rate limiting runs BEFORE the body read, so the earlier EB/EA cases
      // already consumed tokens on this endpoint — wait for the bucket to
      // refill to burst (4 tokens @ 2/s) before asserting on it.
      await new Promise((r) => setTimeout(r, 2200))
      const eps = await listEndpoints()
      const ep = eps.find((e) => e.name === `${PREFIX}none`)
      for (let i = 0; i < 4; i++) {
        const r = await ingest(args.base, ep.slug, `{"burst":${i}}`)
        if (r.status !== 202) throw new Error(`突发第 ${i + 1} 个应 202,实际 ${r.status}`)
      }
      const r5 = await ingest(args.base, ep.slug, '{"burst":5}')
      if (r5.status !== 429) throw new Error(`第 5 个应 429,实际 ${r5.status}`)
      const r6 = await ingest(args.base, ep.slug, '{"burst":6}')
      if (r6.status !== 429) throw new Error(`第 6 个(被拒不耗令牌)应 429,实际 ${r6.status}`)
      await new Promise((r) => setTimeout(r, 1200))
      const a = await ingest(args.base, ep.slug, '{"refill":1}')
      if (a.status !== 202) throw new Error(`补水后第 1 个应 202,实际 ${a.status}`)
      const b = await ingest(args.base, ep.slug, '{"refill":2}')
      if (b.status !== 202) throw new Error(`补水后第 2 个应 202,实际 ${b.status}`)
      const c = await ingest(args.base, ep.slug, '{"refill":3}')
      if (c.status !== 429) throw new Error(`补水后第 3 个应 429,实际 ${c.status}`)
    },
  },
  {
    name: 'E-90 清理:三个接入点逐一删除',
    fn: async () => {
      await gotoNav(page, '接入点')
      for (const name of [`${PREFIX}none`, `${PREFIX}bearer`, `${PREFIX}hmac-改名`]) {
        await deleteRow(page, name)
        await expectNoRow(page, name)
      }
    },
  },
]

console.log('接入点 UI 用例:')
await runCases('endpoints-ui', page, cases)
console.log('\n接入点数据面用例:')
await runCases('endpoints-data', page, dataCases)
await finish()

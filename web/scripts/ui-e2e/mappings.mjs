// 映射 UI 用例(functional-test-plan.md §5):CRUD、字段行增删、
// 取值路径/目标键回填、必填校验。
//
// 运行:node scripts/ui-e2e/mappings.mjs --base http://127.0.0.1:9003 --user admin --pass <密码>

import { openApp, login, gotoNav, expectToast, expectRow, expectNoRow,
  openCreate, saveDialog, editRow, deleteRow, runCases, finish } from './helpers.mjs'

const PREFIX = '[E2E]'

const { args, page } = await openApp()
await login(page, args)
await gotoNav(page, '映射')

const cases = [
  {
    name: 'M-01/02 新建映射(一个字段:取值路径 + 目标键 + 必填开)',
    fn: async () => {
      const dlg = await openCreate(page, '新建映射')
      await dlg.getByLabel(/名称/).fill(`${PREFIX}map`)
      await dlg.getByRole('button', { name: '添加字段' }).click()
      const card = dlg.locator('div.rounded-lg.border', { hasText: '字段 #1' }).first()
      await card.getByPlaceholder('例如 payload.action 或 commits.0.message').fill('payload.action')
      await card.getByPlaceholder('例如 action_type 或 commit_msg').fill('action_type')
      // 必填开关默认关,点开验证交互
      await card.getByLabel('必填').click()
      await saveDialog(page, dlg)
      await expectToast(page, '映射已创建')
      await expectRow(page, `${PREFIX}map`)
    },
  },
  {
    name: 'M-03 编辑回填:字段值正确回显',
    fn: async () => {
      const dlg = await editRow(page, `${PREFIX}map`)
      const card = dlg.locator('div.rounded-lg.border', { hasText: '字段 #1' }).first()
      const path = await card.getByPlaceholder('例如 payload.action 或 commits.0.message').inputValue()
      const key = await card.getByPlaceholder('例如 action_type 或 commit_msg').inputValue()
      if (path !== 'payload.action') throw new Error(`取值路径回填: ${JSON.stringify(path)}`)
      if (key !== 'action_type') throw new Error(`目标键回填: ${JSON.stringify(key)}`)
      await saveDialog(page, dlg)
      await expectToast(page, '映射已更新')
    },
  },
  {
    name: 'M-03 必填校验:半填行(只填目标键)提交被拦截',
    fn: async () => {
      const dlg = await openCreate(page, '新建映射')
      await dlg.getByLabel(/名称/).fill(`${PREFIX}bad`)
      await dlg.getByRole('button', { name: '添加字段' }).click()
      // A fully-empty row is dropped silently by design (buildPayloadFields
      // filters it); only a PARTIALLY-filled row is blocked (plan §5 M-03).
      const card = dlg.locator('div.rounded-lg.border', { hasText: '字段 #1' }).first()
      await card.getByPlaceholder('例如 action_type 或 commit_msg').fill('bad_key')
      await saveDialog(page, dlg).catch(() => {})
      await dlg.getByText('取值路径为必填项').waitFor()
      await dlg.getByRole('button', { name: '取消' }).click()
      await dlg.waitFor({ state: 'detached' })
    },
  },
  {
    name: 'M-90 清理:删除映射',
    fn: async () => {
      await deleteRow(page, `${PREFIX}map`)
      await expectNoRow(page, `${PREFIX}map`)
    },
  },
]

console.log('映射 UI 用例:')
await runCases('mappings', page, cases)
await finish()

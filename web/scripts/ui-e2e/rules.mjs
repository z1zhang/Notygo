// 规则 UI 用例(functional-test-plan.md §6):CRUD、条件行增删、
// 无值操作符(存在)表单联动、必填校验。
//
// 运行:node scripts/ui-e2e/rules.mjs --base http://127.0.0.1:9003 --user admin --pass <密码>
// 或整套跑:npm run test:ui -- --pass <密码>(参数透传,前置条件相同)

import { openApp, login, gotoNav, expectToast, expectRow, expectNoRow,
  openCreate, saveDialog, editRow, deleteRow, selectOption, runCases, finish } from './helpers.mjs'

const PREFIX = '[E2E]'

const { args, page } = await openApp()
await login(page, args)
await gotoNav(page, '规则')

const cases = [
  {
    name: 'R-01/03 新建规则(条件:字段 + 无值操作符"存在")',
    fn: async () => {
      const dlg = await openCreate(page, '新建规则')
      await dlg.getByLabel(/名称/).fill(`${PREFIX}rule`)
      await dlg.getByRole('button', { name: '添加条件' }).click()
      const card = dlg.locator('div.rounded-lg.border', { hasText: '条件 #1' }).first()
      await card.getByPlaceholder('例如 action_type 或 fields.repo_name').fill('status')
      // 无值操作符:选中后"值"输入框应消失
      const operator = card.getByRole('combobox').first()
      await operator.click()
      await page.getByRole('option', { name: '存在', exact: true }).click()
      await card.getByLabel('值').waitFor({ state: 'hidden' })
      await saveDialog(page, dlg)
      await expectToast(page, '规则已创建')
      await expectRow(page, `${PREFIX}rule`)
    },
  },
  {
    name: 'R-04 编辑回填:条件字段与操作符回显',
    fn: async () => {
      const dlg = await editRow(page, `${PREFIX}rule`)
      const card = dlg.locator('div.rounded-lg.border', { hasText: '条件 #1' }).first()
      const field = await card.getByPlaceholder('例如 action_type 或 fields.repo_name').inputValue()
      if (field !== 'status') throw new Error(`条件字段回填: ${JSON.stringify(field)}`)
      const opText = await card.getByRole('combobox').first().textContent()
      if (!opText?.includes('存在')) throw new Error(`操作符回填: ${JSON.stringify(opText)}`)
      await saveDialog(page, dlg)
      await expectToast(page, '规则已更新')
    },
  },
  {
    name: 'R-03b 有值操作符切换回"等于"后值输入框恢复',
    fn: async () => {
      const dlg = await editRow(page, `${PREFIX}rule`)
      const card = dlg.locator('div.rounded-lg.border', { hasText: '条件 #1' }).first()
      await card.getByRole('combobox').first().click()
      await page.getByRole('option', { name: '等于 (=)', exact: true }).click()
      await card.getByLabel('值').waitFor({ state: 'visible' })
      await card.getByLabel('值').fill('ok')
      await saveDialog(page, dlg)
      await expectToast(page, '规则已更新')
    },
  },
  {
    name: 'G-02 必填校验:条件字段留空提交被拦截',
    fn: async () => {
      const dlg = await openCreate(page, '新建规则')
      await dlg.getByLabel(/名称/).fill(`${PREFIX}bad`)
      await dlg.getByRole('button', { name: '添加条件' }).click()
      await saveDialog(page, dlg).catch(() => {})
      await dlg.getByText('字段为必填项').waitFor()
      await dlg.getByRole('button', { name: '取消' }).click()
      await dlg.waitFor({ state: 'detached' })
    },
  },
  {
    name: 'R-90 清理:删除规则',
    fn: async () => {
      await deleteRow(page, `${PREFIX}rule`)
      await expectNoRow(page, `${PREFIX}rule`)
    },
  },
]

console.log('规则 UI 用例:')
await runCases('rules', page, cases)
await finish()

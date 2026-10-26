// 模板 UI 用例(functional-test-plan.md §7):CRUD、格式选择、
// 标题/内容模板回填、必填校验。
//
// 运行:node scripts/ui-e2e/templates.mjs --base http://127.0.0.1:9003 --user admin --pass <密码>

import { openApp, login, gotoNav, expectToast, expectRow, expectNoRow,
  openCreate, saveDialog, editRow, deleteRow, selectOption, runCases, finish } from './helpers.mjs'

const PREFIX = '[E2E]'

const { args, page } = await openApp()
await login(page, args)
await gotoNav(page, '模板')

const cases = [
  {
    name: 'T-01/02 新建模板(Markdown 格式 + 变量模板)',
    fn: async () => {
      const dlg = await openCreate(page, '新建模板')
      await dlg.getByLabel(/名称/).fill(`${PREFIX}tpl`)
      await selectOption(page, dlg, 'Markdown')
      await dlg.getByLabel('标题模板').fill('{{.title}}')
      await dlg.getByPlaceholder('内容模板，使用 Go template 语法').fill('部署:{{.status}}')
      await saveDialog(page, dlg)
      await expectToast(page, '模板已创建')
      await expectRow(page, `${PREFIX}tpl`)
    },
  },
  {
    name: 'T-03 编辑回填:标题/内容模板回显',
    fn: async () => {
      const dlg = await editRow(page, `${PREFIX}tpl`)
      const title = await dlg.getByLabel('标题模板').inputValue()
      if (title !== '{{.title}}') throw new Error(`标题回填: ${JSON.stringify(title)}`)
      const body = await dlg.getByPlaceholder('内容模板，使用 Go template 语法').inputValue()
      if (body !== '部署:{{.status}}') throw new Error(`内容回填: ${JSON.stringify(body)}`)
      await saveDialog(page, dlg)
      await expectToast(page, '模板已更新')
    },
  },
  {
    name: 'G-02 必填校验:空名称提交被拦截',
    fn: async () => {
      const dlg = await openCreate(page, '新建模板')
      await saveDialog(page, dlg).catch(() => {})
      await dlg.getByText('名称为必填项').waitFor()
      await dlg.getByRole('button', { name: '取消' }).click()
      await dlg.waitFor({ state: 'detached' })
    },
  },
  {
    name: 'T-90 清理:删除模板',
    fn: async () => {
      await deleteRow(page, `${PREFIX}tpl`)
      await expectNoRow(page, `${PREFIX}tpl`)
    },
  },
]

console.log('模板 UI 用例:')
await runCases('templates', page, cases)
await finish()

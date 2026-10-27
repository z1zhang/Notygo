// Shared helpers for the Chrome-CDP UI e2e scripts.
//
// Usage contract: Chrome must be running with --remote-debugging-port=9222
// (see docs/functional-test-plan.md §2). All scripts take:
//   node scripts/ui-e2e/<entity>.mjs --base http://127.0.0.1:9003 --user admin --pass <密码>
// 或整套跑:npm run test:ui -- --pass <密码>(参数透传,前置条件相同)
//
// Selectors are grounded in the zh locale labels (src/i18n/locales/zh.json)
// and the shadcn/ui structure (radix roles, [data-sonner-toast]).

import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const ARTIFACTS = join(dirname(fileURLToPath(import.meta.url)), 'artifacts')

export function parseArgs(argv) {
  const args = { base: 'http://127.0.0.1:9003', user: 'admin', pass: '', cdp: 'http://127.0.0.1:9222' }
  for (let i = 2; i < argv.length; i += 2) {
    const key = String(argv[i]).replace(/^--/, '')
    if (key in args) args[key] = argv[i + 1]
  }
  return args
}

// openApp connects to the user-launched debug Chrome and picks a usable page.
export async function openApp() {
  const args = parseArgs(process.argv)
  if (!args.pass) {
    console.error('缺少 --pass <管理员密码> 参数(其余可选: --base --user --cdp)')
    process.exit(2)
  }
  const browser = await chromium.connectOverCDP(args.cdp)
  const ctx = browser.contexts()[0] ?? (await browser.newContext())
  const page = ctx.pages().find((p) => !p.isClosed()) ?? (await ctx.newPage())
  page.setDefaultTimeout(10_000)
  // Stale cached index.html references deleted chunk hashes after a rebuild
  // (every lazy route then dies with "Failed to fetch dynamically imported
  // module"). The server sends no-cache for index.html, but disable the cache
  // entirely so tests never depend on it.
  const cdp = await ctx.newCDPSession(page)
  await cdp.send('Network.setCacheDisabled', { cacheDisabled: true })
  return { args, browser, page }
}

export async function login(page, { base, user, pass }) {
  await page.goto(base + '/login')
  if (!page.url().includes('/login')) return // already authenticated
  await page.getByPlaceholder('请输入用户名').fill(user)
  await page.getByPlaceholder('请输入密码').fill(pass)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL((u) => !String(u).includes('/login'), { timeout: 15_000 })
}

// gotoNav clicks a sidebar entry (nav.* labels: 接入点/映射/规则/模板…).
export async function gotoNav(page, label) {
  await page.locator('a', { hasText: label }).first().click()
  await page.waitForLoadState('networkidle')
}

// expectToast waits for a sonner toast containing text.
export async function expectToast(page, text) {
  await page
    .locator('[data-sonner-toast]', { hasText: text })
    .first()
    .waitFor({ timeout: 8_000 })
}

// expectRow / expectNoRow assert a table row by name text.
export async function expectRow(page, name) {
  await page.locator('tbody tr', { hasText: name }).first().waitFor()
}
export async function expectNoRow(page, name) {
  await page.locator('tbody tr', { hasText: name }).first().waitFor({ state: 'detached' })
}

// openCreate opens the create dialog (新建Xxx button) and returns its locator.
export async function openCreate(page, buttonText) {
  await page.getByRole('button', { name: buttonText }).click()
  const dlg = page.getByRole('dialog')
  await dlg.waitFor()
  return dlg
}

// saveDialog clicks 保存 and closes the dialog afterwards.
export async function saveDialog(page, dlg) {
  await dlg.getByRole('button', { name: '保存', exact: true }).click()
  await dlg.waitFor({ state: 'detached', timeout: 10_000 })
}

// editRow opens the edit dialog for a table row (aria-label 编辑 pencil).
export async function editRow(page, name) {
  const row = page.locator('tbody tr', { hasText: name }).first()
  await row.getByRole('button', { name: '编辑' }).click()
  const dlg = page.getByRole('dialog')
  await dlg.waitFor()
  return dlg
}

// deleteRow deletes a row via the trash button + confirmation dialog.
// ConfirmDialog is built on radix AlertDialog, so its role is "alertdialog",
// NOT "dialog" (that role belongs to the shadcn edit Dialog). All CRUD pages
// pass confirmLabel = t('common.delete') — the confirm button is labeled 删除.
export async function deleteRow(page, name) {
  const row = page.locator('tbody tr', { hasText: name }).first()
  await row.getByRole('button', { name: '删除' }).click()
  const confirm = page.getByRole('alertdialog')
  await confirm.getByRole('button', { name: '删除', exact: true }).click()
  await expectNoRow(page, name)
}

// finish exits the process explicitly: the CDP WebSocket keeps the Node event
// loop alive forever otherwise. The short delay lets piped stdout flush
// (console.log to a pipe is async; process.exit would truncate it).
export async function finish() {
  await new Promise((r) => setTimeout(r, 200))
  process.exit(process.exitCode ?? 0)
}

// selectOption clicks a radix Select trigger inside a dialog and picks the option.
export async function selectOption(page, dlg, optionText, { nth = 0 } = {}) {
  await dlg.getByRole('combobox').nth(nth).click()
  await page.getByRole('option', { name: optionText, exact: true }).click()
}

// runCases executes cases sequentially, screenshots failures, prints a summary.
export async function runCases(title, page, cases) {
  const results = []
  for (const c of cases) {
    try {
      await c.fn(page)
      results.push({ name: c.name, ok: true })
      console.log(`  PASS  ${c.name}`)
    } catch (e) {
      const file = `${title}-${c.name}`.replace(/[^\w一-鿿-]+/g, '_') + '.png'
      try {
        mkdirSync(ARTIFACTS, { recursive: true })
        await page.screenshot({ path: join(ARTIFACTS, file), fullPage: true })
      } catch { /* screenshot best-effort */ }
      const msg = String(e).split('\n')[0].slice(0, 160)
      results.push({ name: c.name, ok: false, msg, file })
      console.log(`  FAIL  ${c.name}\n        ${msg}\n        截图: scripts/ui-e2e/artifacts/${file}`)
    }
  }
  const failed = results.filter((r) => !r.ok).length
  console.log(`\n${title}: ${results.length - failed}/${results.length} 通过${failed ? `,${failed} 失败` : ''}`)
  if (failed) process.exitCode = 1
}

// 顺序跑完 ui-e2e 四个实体套件(单个套件失败不中断,最后汇总)。
//
// 运行:npm run test:ui -- --pass <管理员密码>
// 可选参数 --base --user --cdp 会原样透传给每个套件。
// 前置:Chrome 以 --remote-debugging-port=9222 启动;被测服务已运行且指向目标库。

import { spawnSync } from 'node:child_process'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const SUITES = ['endpoints.mjs', 'mappings.mjs', 'rules.mjs', 'templates.mjs']
const here = dirname(fileURLToPath(import.meta.url))
const args = process.argv.slice(2)

// 提前校验必传参数,免得四个套件各自报一遍(helpers 的 parseArgs 只认 --pass 值 分离写法)
if (!args.includes('--pass')) {
  console.error('缺少 --pass <管理员密码> 参数(其余可选: --base --user --cdp)')
  process.exit(2)
}

const failed = []
for (const suite of SUITES) {
  console.log(`\n===== ${suite} =====`)
  const res = spawnSync(process.execPath, [join(here, suite), ...args], { stdio: 'inherit' })
  if (res.status !== 0) failed.push(suite)
}

console.log(`\nui-e2e 汇总: ${SUITES.length - failed.length}/${SUITES.length} 套件通过`)
if (failed.length) {
  console.log(`失败套件: ${failed.join(', ')}(失败用例截图见 scripts/ui-e2e/artifacts/)`)
  process.exitCode = 1
}

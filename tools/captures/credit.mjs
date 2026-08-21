// Contrôle visuel de la page Créances.
import puppeteer from 'puppeteer-core'
const S = '/private/tmp/claude-501/-Users-abdatytechnologie/9ac72bf8-a04e-4e29-93c8-086b4012ff87/scratchpad/guide'
const a = (ms) => new Promise((r) => setTimeout(r, ms))
const nav = await puppeteer.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: 'new', defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 2 },
  args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars'],
})
const page = await nav.newPage()
const err = []
page.on('pageerror', (e) => err.push(e.message))
await page.goto('http://localhost:34115', { waitUntil: 'networkidle2' })
await a(2500)
if (await page.$('.gate-card input')) {
  const c = await page.$$('.gate-card input')
  await c[0].type('aissata'); await c[1].type('demo2026a')
  await page.evaluate(() => [...document.querySelectorAll('button')]
    .find((b) => b.textContent.includes('Se connecter')).click())
  await page.waitForSelector('.shell'); await a(2500)
}
await page.evaluate(() => [...document.querySelectorAll('.nav-item')]
  .find((e) => e.textContent.includes('Créances')).click())
await a(2500)
await page.screenshot({ path: `${S}/creances.png` })
console.log('  page Créances capturée', err.length ? 'ERREURS: ' + err.join(' | ') : '')
await nav.close()

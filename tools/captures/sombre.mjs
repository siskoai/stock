import puppeteer from 'puppeteer-core'
const S = '/private/tmp/claude-501/-Users-abdatytechnologie/9ac72bf8-a04e-4e29-93c8-086b4012ff87/scratchpad/guide'
const a = (ms) => new Promise((r) => setTimeout(r, ms))
const nav = await puppeteer.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: 'new', defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 2 },
  args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars'],
})
const page = await nav.newPage()
await page.goto('http://localhost:34115', { waitUntil: 'networkidle2' })
await a(2500)
if (await page.$('.gate-card input')) {
  const c = await page.$$('.gate-card input')
  await c[0].type('aissata'); await c[1].type('demo2026a')
  await page.evaluate(() => [...document.querySelectorAll('button')]
    .find((b) => b.textContent.includes('Se connecter')).click())
  await page.waitForSelector('.shell'); await a(2500)
}
console.log('  thème appliqué :', await page.evaluate(() => document.documentElement.dataset.theme))
for (const [nom, fichier] of [['Créances', 'sombre-creances'], ['Tableau de bord', 'sombre-tdb']]) {
  await page.evaluate((l) => [...document.querySelectorAll('.nav-item')]
    .find((e) => e.textContent.includes(l)).click(), nom)
  await a(2200)
  await page.screenshot({ path: `${S}/${fichier}.png` })
}
await nav.close()
console.log('  captures en thème sombre prises')

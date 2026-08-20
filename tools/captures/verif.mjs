// Vérifie les quatre corrections sur l'application réelle.
import puppeteer from 'puppeteer-core'
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const SORTIE = '/private/tmp/claude-501/-Users-abdatytechnologie/9ac72bf8-a04e-4e29-93c8-086b4012ff87/scratchpad/guide'
const attendre = (ms) => new Promise((r) => setTimeout(r, ms))
const nav = await puppeteer.launch({
  executablePath: CHROME, headless: 'new',
  defaultViewport: { width: 1024, height: 700, deviceScaleFactor: 2 },
  args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars'],
})
const page = await nav.newPage()
await page.goto('http://localhost:34115', { waitUntil: 'networkidle2' })
await attendre(2500)
if (await page.$('.shell')) {
  const b = await page.$$('.sidebar-user .nav-item')
  await b[b.length - 1].click()
  await attendre(1500)
}
// 1. Mot de passe oublié
await page.evaluate(() => [...document.querySelectorAll('button')]
  .find((b) => b.textContent.includes('Mot de passe oublié')).click())
await page.waitForSelector('.modal'); await attendre(900)
await page.screenshot({ path: `${SORTIE}/v-oubli.png` })
await page.keyboard.press('Escape'); await attendre(600)
// Connexion
const ch = await page.$$('.gate-card input')
await ch[0].type('aissata'); await ch[1].type('demo2026a')
await page.evaluate(() => [...document.querySelectorAll('button')]
  .find((b) => b.textContent.includes('Se connecter')).click())
await page.waitForSelector('.shell'); await attendre(2500)
// 2. Onglet « Ce poste »
await page.evaluate(() => [...document.querySelectorAll('.nav-item')]
  .find((e) => e.textContent.includes('Paramètres')).click())
await attendre(1800)
await page.evaluate(() => [...document.querySelectorAll('.tab')]
  .find((e) => e.textContent.trim() === 'Ce poste').click())
await attendre(1600)
await page.screenshot({ path: `${SORTIE}/v-poste.png` })
await nav.close()
console.log('  vérifications capturées')

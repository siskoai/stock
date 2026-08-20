// Capture les rapports à la largeur minimale de la fenêtre, celle où les
// colonnes se disputent la place.
import puppeteer from 'puppeteer-core'
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const attendre = (ms) => new Promise((r) => setTimeout(r, ms))
const nav = await puppeteer.launch({
  executablePath: CHROME, headless: 'new',
  defaultViewport: { width: 1024, height: 680, deviceScaleFactor: 2 },
  args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars'],
})
const page = await nav.newPage()
await page.goto('http://localhost:34115', { waitUntil: 'networkidle2' })
await attendre(2500)
if (await page.$('.gate-card input')) {
  const champs = await page.$$('.gate-card input')
  await champs[0].type('aissata'); await champs[1].type('demo2026a')
  await page.evaluate(() => [...document.querySelectorAll('button')]
    .find((b) => b.textContent.includes('Se connecter')).click())
  await page.waitForSelector('.shell'); await attendre(2500)
}
const aller = async (libelle) => {
  await page.evaluate((l) => [...document.querySelectorAll('.nav-item')]
    .find((e) => e.textContent.includes(l)).click(), libelle)
  await attendre(1800)
}
const onglet = async (l) => {
  await page.evaluate((t) => [...document.querySelectorAll('.tab')]
    .find((e) => e.textContent.trim() === t).click(), l)
  await attendre(1800)
}
await aller('Rapports')
const SORTIE = '/private/tmp/claude-501/-Users-abdatytechnologie/9ac72bf8-a04e-4e29-93c8-086b4012ff87/scratchpad/guide'
await page.screenshot({ path: `${SORTIE}/etroit-evolution.png` })
await onglet('Compte de résultat')
await page.screenshot({ path: `${SORTIE}/etroit-resultat.png` })
await onglet('Statistiques')
await page.screenshot({ path: `${SORTIE}/etroit-stats.png` })
await nav.close()
console.log('  captures étroites prises')

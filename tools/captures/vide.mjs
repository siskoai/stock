// Vérifie les trois écrans qui noircissaient, sur une boutique clairsemée.
import puppeteer from 'puppeteer-core'
const S = '/private/tmp/claude-501/-Users-abdatytechnologie/9ac72bf8-a04e-4e29-93c8-086b4012ff87/scratchpad/guide'
const a = (ms) => new Promise((r) => setTimeout(r, ms))
const nav = await puppeteer.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: 'new', defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 2 },
  args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars'],
})
const page = await nav.newPage()
const erreurs = []
page.on('pageerror', (e) => { if (!/ipc\.js/.test(e.stack ?? '')) erreurs.push(e.message) })
await page.goto('http://localhost:34115', { waitUntil: 'networkidle2' })
await a(2500)
if (await page.$('.gate-card input')) {
  const c = await page.$$('.gate-card input')
  await c[0].type('aissata'); await c[1].type('demo2026a')
  await page.evaluate(() => [...document.querySelectorAll('button')]
    .find((b) => b.textContent.includes('Se connecter')).click())
  await page.waitForSelector('.shell'); await a(2500)
}

const controler = async (libelle) => {
  const r = await page.evaluate(() => ({
    filet: !!document.querySelector('.filet'),
    contenu: (document.querySelector('.content')?.innerText ?? '').replace(/\n/g, ' | ').slice(0, 90),
  }))
  console.log(`  ${libelle.padEnd(22)} ${r.filet ? 'EN ERREUR' : 'affiché  '}  ${JSON.stringify(r.contenu)}`)
}

await page.evaluate(() => [...document.querySelectorAll('.nav-item')]
  .find((e) => e.textContent.includes('Créances')).click())
await a(2200); await controler('Créances')
await page.screenshot({ path: `${S}/vide-creances.png` })

await page.evaluate(() => [...document.querySelectorAll('.nav-item')]
  .find((e) => e.textContent.includes('Rapports')).click())
await a(2200)
for (const onglet of ['Compte de résultat', 'Statistiques', 'Situation', 'Évolution']) {
  await page.evaluate((t) => [...document.querySelectorAll('.tab')]
    .find((e) => e.textContent.trim() === t)?.click(), onglet)
  await a(2000)
  await controler(onglet)
}
await page.screenshot({ path: `${S}/vide-stats.png` })
console.log('\n  erreurs applicatives :', erreurs.length ? erreurs.join(' | ') : 'aucune')
await nav.close()

// Diagnostic de la page Créances : on veut la pile d'appels, pas un résumé.
import puppeteer from 'puppeteer-core'
const a = (ms) => new Promise((r) => setTimeout(r, ms))
const nav = await puppeteer.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: 'new', defaultViewport: { width: 1440, height: 900 },
  args: ['--no-sandbox', '--disable-gpu'],
})
const page = await nav.newPage()
page.on('pageerror', (e) => console.log('\n=== ERREUR DE PAGE ===\n' + (e.stack || e.message)))
page.on('console', (m) => {
  if (m.type() === 'error') console.log('\n=== CONSOLE ===\n' + m.text())
})
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
await a(3000)
const etat = await page.evaluate(() => ({
  racine: document.getElementById('root')?.children.length ?? 0,
  shell: !!document.querySelector('.shell'),
  texte: (document.querySelector('.content')?.innerText ?? '').slice(0, 120),
}))
console.log('\n=== ÉTAT APRÈS CLIC ===')
console.log('  enfants de #root :', etat.racine)
console.log('  cadre présent    :', etat.shell)
console.log('  contenu          :', JSON.stringify(etat.texte.replace(/\n/g, ' | ')))
await nav.close()

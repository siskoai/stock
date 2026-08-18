// Rend le guide d'utilisation en PDF.
//
// Le rendu passe par le moteur d'un navigateur plutôt que par une bibliothèque
// de mise en page : la typographie, les coupures de page et les images se
// contrôlent alors en CSS, et le résultat est identique à ce que l'on voit à
// l'écran. Le pied de page numéroté est ajouté par le moteur d'impression.
//
//   node tools/captures/pdf.mjs

import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { stat } from 'node:fs/promises'
import puppeteer from 'puppeteer-core'

const ICI = dirname(fileURLToPath(import.meta.url))
const RACINE = resolve(ICI, '..', '..')
const SOURCE = join(RACINE, 'docs', 'guide', 'guide.html')
const SORTIE = join(RACINE, 'docs', 'Comptoir-guide-utilisation.pdf')
const CHROME = process.env.CHROME_PATH
  ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'

const pied = `
<div style="width:100%;font-family:-apple-system,Helvetica,Arial,sans-serif;
            font-size:7pt;color:#6E767D;padding:0 20mm;
            display:flex;justify-content:space-between;align-items:center">
  <span>Comptoir, guide d'utilisation</span>
  <span>Un logiciel SISKO</span>
  <span class="pageNumber"></span>
</div>`

const navigateur = await puppeteer.launch({
  executablePath: CHROME,
  headless: 'new',
  args: ['--no-sandbox', '--disable-gpu', '--font-render-hinting=none',
         '--force-color-profile=srgb', '--allow-file-access-from-files'],
})
const page = await navigateur.newPage()
await page.goto(pathToFileURL(SOURCE).href, { waitUntil: 'networkidle0' })
// Les images sont volumineuses : on attend leur décodage effectif, sinon
// certaines pages sortiraient vides.
await page.evaluate(() => Promise.all(
  [...document.images].filter((i) => !i.complete).map((i) => i.decode().catch(() => {})),
))

await page.pdf({
  path: SORTIE,
  format: 'A4',
  printBackground: true,
  displayHeaderFooter: true,
  headerTemplate: '<div></div>',
  footerTemplate: pied,
  // La couverture et les ouvertures de partie occupent toute la page : les
  // marges sont gérées en CSS, le moteur n'ajoute que le pied.
  margin: { top: '0mm', bottom: '14mm', left: '0mm', right: '0mm' },
  preferCSSPageSize: true,
})
await navigateur.close()

const { size } = await stat(SORTIE)
console.log(`${SORTIE}\n${(size / 1048576).toFixed(1)} Mo`)

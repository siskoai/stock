// Captures d'écran du guide d'utilisation.
//
// Elles sont prises sur l'application réelle, pilotée dans un navigateur sans
// affichage, branchée sur le vrai moteur Go et sur une boutique de
// démonstration cohérente. Rien n'est reconstitué : ce que montre le guide est
// ce que fait le logiciel.
//
// Préparation :
//   go run ./tools/demo -dir /tmp/demo
//   COMPTOIR_DATA_DIR=/tmp/demo wails dev -s
//   node tools/captures/capture.mjs
//
// La résolution est doublée (deviceScaleFactor 2) : les captures finissent
// dans un PDF, où une image à l'échelle 1 paraîtrait floue à l'impression.

import { mkdir, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import puppeteer from 'puppeteer-core'

const ICI = dirname(fileURLToPath(import.meta.url))
const SORTIE = join(ICI, '..', '..', 'docs', 'images')
const ADRESSE = process.env.COMPTOIR_URL ?? 'http://localhost:34115'
const CHROME = process.env.CHROME_PATH
  ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'

const IDENTIFIANT = 'aissata'
const MOT_DE_PASSE = 'demo2026a'

const attendre = (ms) => new Promise((r) => setTimeout(r, ms))

/** clique sur le premier élément dont le texte correspond. */
async function cliquerTexte(page, selecteur, texte) {
  const cible = await page.evaluateHandle(
    (sel, txt) => [...document.querySelectorAll(sel)]
      .find((el) => el.textContent.trim().toLowerCase().includes(txt.toLowerCase())),
    selecteur, texte,
  )
  const element = cible.asElement()
  if (!element) throw new Error(`introuvable : ${selecteur} contenant « ${texte} »`)
  await element.click()
  return element
}

/** capture attend que le rendu se stabilise avant de déclencher. */
async function capture(page, nom, { delai = 900 } = {}) {
  await attendre(delai)
  const chemin = join(SORTIE, `${nom}.png`)
  await page.screenshot({ path: chemin, captureBeyondViewport: false })
  console.log(`  ${nom}.png`)
}

async function main() {
  await mkdir(SORTIE, { recursive: true })

  const navigateur = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 2 },
    args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--force-color-profile=srgb'],
  })
  const page = await navigateur.newPage()
  page.setDefaultTimeout(20000)
  await page.goto(ADRESSE, { waitUntil: 'networkidle2' })
  await attendre(1800) // le pont vers le moteur Go s'établit après le chargement

  // --- Connexion -----------------------------------------------------------
  // Le moteur garde la session ouverte entre deux exécutions : si l'application
  // est déjà déverrouillée, on se déconnecte pour retrouver l'écran d'entrée.
  if (await page.$('.shell')) {
    await cliquerTexte(page, '.sidebar-user .nav-item[title="Se déconnecter"]', '')
      .catch(async () => {
        const boutons = await page.$$('.sidebar-user .nav-item')
        await boutons[boutons.length - 1].click()
      })
    await attendre(1500)
  }
  await page.waitForSelector('.gate-card input')
  await capture(page, '01-connexion')

  const champs = await page.$$('.gate-card input')
  await champs[0].type(IDENTIFIANT, { delay: 12 })
  await champs[1].type(MOT_DE_PASSE, { delay: 12 })
  await cliquerTexte(page, 'button', 'Se connecter')
  await page.waitForSelector('.shell', { timeout: 20000 })
  await attendre(2200)

  // --- Tableau de bord ------------------------------------------------------
  await capture(page, '02-tableau-de-bord')

  // --- Chaque page de la barre latérale -------------------------------------
  const pages = [
    ['Ventes', '03-ventes'],
    ['Créances', '24-creances'],
    ['Achats', '04-achats'],
    ['Articles', '05-articles'],
    ['Catégories', '06-categories'],
    ['Mouvements', '07-mouvements'],
    ['Clients', '08-tiers'],
    ['Charges', '09-charges'],
    ['Rapports', '10-rapports'],
    ['Comptes', '11-comptes'],
    ['Paramètres', '12-parametres'],
  ]
  for (const [libelle, nom] of pages) {
    await cliquerTexte(page, '.nav-item', libelle)
    await attendre(1400)
    await capture(page, nom, { delai: 400 })
  }

  // --- Écrans de saisie, ouverts en fenêtre ---------------------------------
  await cliquerTexte(page, '.nav-item', 'Ventes')
  await attendre(1200)
  await cliquerTexte(page, 'button', 'Nouvelle vente')
  await page.waitForSelector('.modal')
  await attendre(800)
  await capture(page, '13-nouvelle-vente-vide')

  // Recherche d'un article, résultats affichés
  const recherche = await page.$('.modal .search input')
  await recherche.type('imprimante', { delay: 28 })
  await attendre(1400)
  await capture(page, '14-recherche-article')

  // Ajout de deux lignes
  await page.keyboard.press('Enter')
  await attendre(900)
  await recherche.type('toner', { delay: 28 })
  await attendre(1300)
  await page.keyboard.press('Enter')
  await attendre(1100)
  await capture(page, '15-vente-remplie')
  await page.keyboard.press('Escape')
  await attendre(700)

  // --- Détail d'une facture -------------------------------------------------
  await attendre(600)
  const lignes = await page.$$('table.data tbody tr')
  if (lignes.length) {
    await lignes[0].click()
    await page.waitForSelector('.modal')
    await attendre(1200)
    await capture(page, '16-detail-facture')
    await page.keyboard.press('Escape')
    await attendre(700)
  }

  // --- Fiche article --------------------------------------------------------
  await cliquerTexte(page, '.nav-item', 'Articles')
  await attendre(1400)
  const modifier = await page.$$('table.data tbody tr button')
  if (modifier.length) {
    await modifier[0].click()
    await page.waitForSelector('.modal')
    await attendre(1000)
    await capture(page, '17-fiche-article')
    await page.keyboard.press('Escape')
    await attendre(700)
  }

  // --- Import de catalogue --------------------------------------------------
  await cliquerTexte(page, 'button', 'Importer')
  await page.waitForSelector('.modal')
  await attendre(900)
  await capture(page, '18-import-catalogue')
  await page.keyboard.press('Escape')
  await attendre(700)

  // --- Opérations de stock --------------------------------------------------
  await cliquerTexte(page, '.nav-item', 'Mouvements')
  await attendre(1400)
  await cliquerTexte(page, 'button', 'Déclarer défectueux')
  await page.waitForSelector('.modal')
  await attendre(900)
  await capture(page, '19-operation-stock')
  await page.keyboard.press('Escape')
  await attendre(700)

  // --- Rapports, onglet par onglet ------------------------------------------
  await cliquerTexte(page, '.nav-item', 'Rapports')
  await attendre(1600)
  for (const [onglet, nom] of [
    ['Compte de résultat', '20-compte-de-resultat'],
    ['Statistiques', '21-statistiques'],
  ]) {
    await cliquerTexte(page, '.tab', onglet)
    await attendre(1600)
    await capture(page, nom, { delai: 500 })
  }

  // --- Mon compte -----------------------------------------------------------
  await cliquerTexte(page, '.sidebar-user .nav-item', '')
  await page.waitForSelector('.modal')
  await attendre(900)
  await capture(page, '22-mon-compte')
  await page.keyboard.press('Escape')
  await attendre(700)

  // --- Sauvegardes ----------------------------------------------------------
  await cliquerTexte(page, '.nav-item', 'Paramètres')
  await attendre(1400)
  await cliquerTexte(page, '.tab', 'Sauvegardes')
  await attendre(1400)
  await capture(page, '23-sauvegardes')

  await navigateur.close()
  await writeFile(join(SORTIE, 'SOURCE.txt'),
    "Captures prises sur l'application réelle, pilotée par tools/captures/capture.mjs\n" +
    "sur la boutique de démonstration produite par tools/demo.\n")
  console.log('\ncaptures terminées')
}

main().catch((err) => {
  console.error('échec :', err.message)
  process.exit(1)
})

import puppeteer from 'puppeteer-core'
const SORTIE='/private/tmp/claude-501/-Users-abdatytechnologie/9ac72bf8-a04e-4e29-93c8-086b4012ff87/scratchpad/guide'
const attendre=(ms)=>new Promise(r=>setTimeout(r,ms))
const nav=await puppeteer.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless:'new', defaultViewport:{width:1440,height:900,deviceScaleFactor:2},
  args:['--no-sandbox','--disable-gpu','--hide-scrollbars']})
const page=await nav.newPage()
const erreurs=[]
page.on('pageerror',e=>erreurs.push('ERREUR JS: '+e.message))
page.on('console',m=>{ if(m.type()==='error') erreurs.push('CONSOLE: '+m.text().slice(0,200)) })
await page.goto('http://localhost:34115',{waitUntil:'networkidle2'}); await attendre(2500)
if(await page.$('.gate-card input')){
  const c=await page.$$('.gate-card input'); await c[0].type('aissata'); await c[1].type('demo2026a')
  await page.evaluate(()=>[...document.querySelectorAll('button')].find(b=>b.textContent.includes('Se connecter')).click())
  await page.waitForSelector('.shell'); await attendre(2500)
}
await page.evaluate(()=>[...document.querySelectorAll('.nav-item')].find(e=>e.textContent.includes('Rapports')).click())
await attendre(2000)
for(const [onglet,nom] of [['Compte de résultat','bug-resultat'],['Statistiques','bug-stats'],['Situation','bug-situation']]){
  erreurs.length=0
  await page.evaluate((t)=>[...document.querySelectorAll('.tab')].find(e=>e.textContent.trim()===t)?.click(),onglet)
  await attendre(2500)
  const texte=await page.evaluate(()=>document.querySelector('.content')?.innerText.slice(0,200))
  console.log(`\n--- ${onglet} ---`)
  console.log('  contenu :', JSON.stringify(texte?.replace(/\n/g,' | ').slice(0,160)))
  if(erreurs.length) erreurs.forEach(e=>console.log('  '+e))
  await page.screenshot({path:`${SORTIE}/${nom}.png`})
}
await nav.close()

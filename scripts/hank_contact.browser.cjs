// Capture the real 8080 profile, then compare its visible SVG with sheet SVG.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.SP_PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const state=JSON.parse(fs.readFileSync(process.env.SP_HANK_PROFILE_STATE||'/tmp/hank-contact-profile.json','utf8'));
 const out=process.env.SP_HANK_CONTACT_OUT||'docs/avatar-check/pip/grip-contact';fs.mkdirSync(out,{recursive:true});
 const stage=process.env.SP_HANK_CONTACT_STAGE||'after';
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1100},deviceScaleFactor:1});await ctx.addCookies(state.cookies);
  const page=await ctx.newPage();await page.goto(state.profile_url+'?v='+Date.now());await page.locator('#profile-figure svg[data-character="pip"]').waitFor();
  await page.screenshot({path:path.join(out,stage+'-profile-app.png'),fullPage:true});
  const actual=await page.locator('#profile-figure svg').evaluate(svg=>{
   const clone=svg.cloneNode(true);const css=getComputedStyle(svg);
   for(const key of ['fur','fur-dark','headdress','headdress-dark','handle','handle-dark','paddle','emblem']){const v=css.getPropertyValue('--'+key);if(v.trim())clone.style.setProperty('--'+key,v)}
   clone.querySelectorAll('.sthings-face').forEach(g=>{if(g.dataset.face!=='neutral')g.remove();else g.style.display='inline'});
   clone.setAttribute('width','420');clone.setAttribute('height','420');return clone.outerHTML;
  });
  const response=await ctx.request.get(state.svg_url+'&v='+Date.now());assert.equal(response.status(),200);fs.writeFileSync(path.join(out,stage+'-http-neutral.svg'),await response.text());
  fs.writeFileSync(path.join(out,stage+'-profile-visible.svg'),actual);
  if(stage==='before')return;
  const reference=fs.readFileSync(path.join(out,'reference-neutral.svg'),'utf8');
  const compare=await ctx.newPage({viewport:{width:880,height:470}});
  const css='<style>body{margin:0;background:#0a0a12;color:white;font:16px system-ui}.image{width:420px;height:420px}svg{display:block;width:420px;height:420px}.sthings-face{display:inline}</style>';
  // Rasterize at the SAME viewport origin; SVG antialiasing can otherwise
  // differ by a channel value merely because the second column is offset.
  await compare.setContent(css+'<div class="image" id="single">'+actual+'</div>');
  await compare.locator('#single').screenshot({path:path.join(out,'app-render-420.png')});
  await compare.locator('#single').evaluate((el,svg)=>el.innerHTML=svg,reference);
  await compare.locator('#single').screenshot({path:path.join(out,'sheet-render-420.png')});
  const actualPng=fs.readFileSync(path.join(out,'app-render-420.png')).toString('base64');
  const referencePng=fs.readFileSync(path.join(out,'sheet-render-420.png')).toString('base64');
  await compare.setContent('<style>body{margin:0;background:#0a0a12;color:white;font:16px system-ui}.row{display:flex;gap:20px}section{width:420px}header{height:30px}img{display:block;width:420px;height:420px}</style><div class="row" id="pair"><section><header>Echte Profilseite · Port 8080</header><img src="data:image/png;base64,'+actualPng+'"></section><section><header>Vergleichsblatt · gleicher Assetstand</header><img src="data:image/png;base64,'+referencePng+'"></section></div>');
  await compare.locator('#pair').screenshot({path:path.join(out,'app-vs-sheet-420.png')});
  const screenshot=fs.readFileSync(path.join(out,stage+'-profile-app.png')).toString('base64');
  const expected=fs.readFileSync(path.join(out,'sheet-render-420.png')).toString('base64');
  await compare.setViewportSize({width:1100,height:1400});
  await compare.setContent('<style>body{background:#0a0a12;color:white;font:16px system-ui;margin:16px}.row{display:flex;gap:24px;align-items:start}section:first-child{width:620px}section:last-child{width:420px}img{width:100%;height:auto}</style><h1>hank · echte App und Vergleichsblatt</h1><div class="row"><section><h2>Profilseite auf Port 8080</h2><img src="data:image/png;base64,'+screenshot+'"></section><section><h2>Referenzrendering · 420 px</h2><img src="data:image/png;base64,'+expected+'"></section></div>');
  await compare.screenshot({path:path.join(out,'profile-app-vs-sheet.png'),fullPage:true});
  fs.writeFileSync(path.join(out,'browser-verification.json'),JSON.stringify({profile_url:state.profile_url,svg_url:state.svg_url,source:'real profile DOM and HTTP response from port 8080',reference:'reference-neutral.svg rendered from current embedded assets',stage},null,2)+'\n');
  console.log('Real profile and 420px comparison captured.');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});

// Real HTTP SVGs, same 300-unit detail window for sthings and both hank versions.
const fs=require('node:fs');const path=require('node:path');const assert=require('node:assert/strict');
const {chromium}=require(process.env.SP_PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const out='docs/avatar-check/pip/framing';const base=process.env.SP_PREVIEW_BASE_URL||'http://127.0.0.1:8080';
 const assets=['base','racket'].map(name=>fs.readFileSync('web/static/img/pip/'+name+'.svg','utf8')).join('');
 const expected=[...assets.matchAll(/ d="([^"]+)"/g)].map(m=>m[1]);let actual,cache;
 for(let i=0;i<60;i++){
  const r=await fetch(base+'/avatars/random.svg?face=neutral&framing='+Date.now(),{headers:{'Cache-Control':'no-cache'}});assert.equal(r.status,200);
  const body=await r.text();if(body.includes('data-character="pip"')&&expected.every(d=>body.includes('d="'+d+'"'))){actual=body;cache=r.headers.get('cache-control');break;}
 }
 assert.ok(actual,'Actual app does not deliver current framed base/racket');fs.writeFileSync(path.join(out,'served-after.svg'),actual);
 fs.writeFileSync(path.join(out,'served-verification.json'),JSON.stringify({endpoint:'/avatars/random.svg?face=neutral',cache_control:cache,current_base_and_racket_paths_match:true,crop_native:[120,120,420,420],detail_pixels:420},null,2)+'\n');
 const browser=await chromium.launch({executablePath:process.env.SP_PREVIEW_BROWSER_BINARY||undefined,headless:true,args:['--no-sandbox']});
 try{
  const page=await browser.newPage({viewport:{width:1260,height:460},deviceScaleFactor:1});
  const entries=[['sthings','sthings'],['hank vorher','before'],['hank nachher','after']];
  let html='<style>body{margin:0;background:#0a0a12;color:white;font:16px system-ui}#comparison{display:flex;width:1260px}section{width:420px}header{height:28px}.detail{position:relative;width:420px;height:420px;overflow:hidden}svg{position:absolute;width:588px;height:588px;left:-168px;top:-168px}.sthings-face{display:none}.sthings-face[data-face="neutral"]{display:inline}</style><div id="comparison">';
  for(const [label,key] of entries)html+='<section><header>'+label+'</header><div class="detail" id="'+key+'">'+fs.readFileSync(path.join(out,'served-'+key+'.svg'),'utf8')+'</div></section>';
  await page.setContent(html+'</div>');
  assert.equal(await page.locator('#after svg > :last-child').getAttribute('data-pip-part'),'racket');
  for(const [,key] of entries)await page.locator('#'+key).screenshot({path:path.join(out,'cheek-racket-'+key+'-420.png')});
  await page.locator('#comparison').screenshot({path:path.join(out,'cheek-racket-comparison-420.png')});
 }finally{await browser.close()}
 console.log('Actual SVG matches framed assets; foreground racket verified; three 420px details captured.');
})().catch(e=>{console.error(e);process.exit(1)});

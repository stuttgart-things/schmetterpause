// Capture the actual HTTP-delivered hank SVG; no synthetic replacement geometry.
const fs=require('node:fs');
const path=require('node:path');
const assert=require('node:assert/strict');
const {chromium}=require(process.env.SP_PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const out='docs/avatar-check/pip/sthings-style';
 const base=process.env.SP_PREVIEW_BASE_URL||'http://127.0.0.1:8080';
 const current=fs.readFileSync('web/static/img/pip/base.svg','utf8');
 const expected=[...current.matchAll(/ d="([^"]+)"/g)].map(m=>m[1]);
 let svg,cache;
 for(let attempt=0;attempt<60;attempt++){
  const response=await fetch(base+'/avatars/random.svg?face=neutral&review='+Date.now(),{headers:{'Cache-Control':'no-cache'}});
  assert.equal(response.status,200);
  const body=await response.text();
  if(body.includes('data-character="pip"')&&expected.every(d=>body.includes('d="'+d+'"'))){svg=body;cache=response.headers.get('cache-control');break;}
 }
 assert.ok(svg,'Running server does not serve the current hank base');
 fs.writeFileSync(path.join(out,'served-after.svg'),svg);
 fs.writeFileSync(path.join(out,'served-after.json'),JSON.stringify({endpoint:'/avatars/random.svg?face=neutral',cache_control:cache,base_paths_match_working_tree:true},null,2)+'\n');
 const browser=await chromium.launch({executablePath:process.env.SP_PREVIEW_BROWSER_BINARY||undefined,headless:true,args:['--no-sandbox']});
 try{
  const page=await browser.newPage({viewport:{width:500,height:500},deviceScaleFactor:1});
  for(const stage of ['before','after']){
   const actual=fs.readFileSync(path.join(out,'served-'+stage+'.svg'),'utf8');
   await page.setContent('<style>body{margin:0;background:#0a0a12}#detail{position:relative;width:420px;height:420px;overflow:hidden}svg{position:absolute;width:1764px;height:1764px;left:-1197px;top:-735px}.sthings-face{display:none}.sthings-face[data-face="neutral"]{display:inline}</style><div id="detail">'+actual+'</div>');
   await page.locator('#detail').screenshot({path:path.join(out,'cheek-served-'+stage+'-420.png')});
  }
 }finally{await browser.close();}
 console.log('HTTP SVG matches current embedded base; 420 px browser crops captured, cache='+cache);
})().catch(e=>{console.error(e);process.exit(1)});

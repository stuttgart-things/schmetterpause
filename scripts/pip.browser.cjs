// Run through TestPipBrowser with SP_RUN_PIP_BROWSER=1; uses disposable test players.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.SP_PLAYWRIGHT_MODULE || 'playwright');

(async () => {
  const base = process.env.SP_PREVIEW_BASE_URL;
  const id = process.env.SP_PREVIEW_PLAYER_ID;
  const out = process.env.SP_PREVIEW_SCREENSHOT_DIR || '/tmp/pip-browser';
  fs.mkdirSync(out, {recursive:true});
  const browser = await chromium.launch({executablePath:process.env.SP_PREVIEW_BROWSER_BINARY || undefined,headless:true,args:['--no-sandbox']});
  try {
    const ctx = await browser.newContext({viewport:{width:1440,height:1200}});
    const cookie = JSON.parse(process.env.SP_PREVIEW_COOKIE);
    await ctx.addCookies([{...cookie,url:base}]);
    const page = await ctx.newPage();
    const errors=[];page.on('pageerror',e=>errors.push(e.message));
    const profile=base+'/players/'+id;
    await page.goto(profile);
    await page.locator('#figure-editor').waitFor();
    assert.equal(await page.locator('.character-pick').filter({has:page.locator('input[name=character][value=pip]')}).innerText(), 'hank');
    const pip=page.locator('input[name=character][value=pip]');
    if (!(await pip.isChecked())) await Promise.all([
      page.waitForResponse(r=>r.url().includes('/fragments/figure-character')&&r.status()===200),
      page.locator('.character-pick').filter({has:pip}).click(),
    ]);
    await page.waitForFunction(()=>document.querySelector('#figure-preview svg')?.getAttribute('data-character')==='pip');
    assert.deepEqual(await page.locator('input[name=fur]').evaluateAll(es=>es.map(e=>e.value)),['','grey','blue-grey']);
    assert.equal(await page.locator('input[name=headdress][value=lapis]').count(),0);
    assert.deepEqual(await page.locator('input[name=bling]').evaluateAll(es=>es.map(e=>e.value)),['','initial']);
    const faces=['neutral','happy','surprised','skeptical','angry','sad'];
    assert.deepEqual(await page.locator('.face-preview').evaluateAll(es=>es.map(e=>e.dataset.expression)),faces);
    async function choose(name,value) {
      const input=page.locator(`input[name=${name}][value="${value}"]`);
      if (!(await input.isChecked())) {
        const version=await page.evaluate(()=>{
          const v=(window.__hankVersion||0)+1;window.__hankVersion=v;
          document.body.addEventListener('htmx:afterSettle',()=>{window.__hankSettled=v},{once:true});
          return v;
        });
        await Promise.all([
          page.waitForResponse(r=>r.url().includes('/fragments/figure-preview')&&r.status()===200),
          page.locator('.figure-parts label.pick').filter({has:input}).click(),
        ]);
        await page.waitForFunction(v=>window.__hankSettled===v,version);
      }
    }
    const originalLabel=page.locator('.figure-parts label.pick').filter({has:page.locator('input[name=headdress][value=fur]')});
    assert.match(await originalLabel.innerText(),/Original/);
    assert.doesNotMatch(await originalLabel.innerText(),/Wie das Fell/);
    await choose('headdress','silver');
    await choose('fur','grey');
    assert.equal(await page.locator('input[name=headdress][value=silver]').count(),0);
    assert.ok(await page.locator('input[name=headdress][value=fur]').isChecked());
    for(const card of await page.locator('.face-preview').all()) assert.ok((await card.innerHTML()).includes('--headdress:#d742c5'));
    await choose('fur','blue-grey');
    assert.equal(await page.locator('input[name=headdress][value=silver]').count(),1);
    await choose('headdress','ivory');await choose('handle','blue');await choose('blade','steel');
    for (const prop of ['--fur:#587e9b','--headdress:#f2ead8','--handle:#2f6fd0','--paddle:#8c8f9c']) {
      assert.ok((await page.locator('#character-thumbnail-pip').innerHTML()).includes(prop));
      for(const card of await page.locator('.face-preview').all()) assert.ok((await card.innerHTML()).includes(prop));
    }
    await choose('bling','initial');
    await page.locator('#figure-preview [data-initial=A]').waitFor({state:'visible'});
    assert.equal(await page.locator('#figure-preview [data-initial=A]').count(),1);
    assert.equal(await page.locator('.face-preview [data-initial=A]').count(),6);
    assert.equal(await page.locator('#character-thumbnail-pip [data-initial=A]').count(),1);
    await page.locator('#figure-editor').screenshot({path:path.join(out,'editor-desktop.png')});
    const save=page.locator('#figure-editor button[type=submit]').filter({hasText:'Speichern'});
    await Promise.all([page.waitForResponse(r=>r.url().endsWith('/figure')&&r.status()===200),save.click()]);
    await page.goto(profile);await page.locator('#figure-editor').waitFor();
    assert.ok(await page.locator('input[name=character][value=pip]').isChecked());
    assert.ok(await page.locator('input[name=fur][value=blue-grey]').isChecked());
    const svg=await (await ctx.request.get(base+'/avatars/'+id+'.svg')).text();
    assert.ok(svg.includes('data-character="pip"')&&svg.includes('--fur:#587e9b')&&svg.includes('data-initial="A"'));
    const gif=await ctx.request.get(base+'/avatars/'+id+'.gif?face=angry');
    assert.equal(gif.status(),200);assert.match(gif.headers()['content-type'],/image\/gif/);
    await page.setViewportSize({width:390,height:844});
    await page.reload();await page.locator('#figure-editor').waitFor();
    assert.equal(await page.locator('.face-preview').count(),6);
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    await page.locator('#figure-editor').screenshot({path:path.join(out,'editor-mobile.png')});
    const reset=page.locator('#figure-editor button').filter({hasText:'Zurücksetzen'});
    await Promise.all([page.waitForResponse(r=>r.url().endsWith('/figure/reset')&&r.status()===200),reset.click()]);
    await page.goto(profile);await page.locator('#figure-editor').waitFor();
    assert.ok(await page.locator('input[name=character][value=""]').isChecked());
    assert.deepEqual(errors,[]);
    console.log('Hank browser: palettes, six faces, colour previews, save/reload, SVG/GIF, mobile and reset passed');
  } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exit(1);});

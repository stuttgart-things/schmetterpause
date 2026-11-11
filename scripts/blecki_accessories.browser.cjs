// Read-only browser verification against an authenticated local preview.
// Supply SP_PREVIEW_BASE_URL, SP_PREVIEW_PLAYER_ID and SP_PREVIEW_COOKIE_FILE.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.SP_PLAYWRIGHT_MODULE || 'playwright');

async function main() {
  const required = name => {
    if (!process.env[name]) throw new Error(`Set ${name} before running this check`);
    return process.env[name];
  };
  const base = required('SP_PREVIEW_BASE_URL').replace(/\/$/, '');
  const id = required('SP_PREVIEW_PLAYER_ID');
  const out = process.env.SP_PREVIEW_SCREENSHOT_DIR || '/tmp/blecki-accessories-browser';
  fs.mkdirSync(out, { recursive: true });
  const browser = await chromium.launch({
    executablePath: process.env.SP_PREVIEW_BROWSER_BINARY || undefined,
    headless: true,
    args: ['--no-sandbox'],
  });
  try {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 1200 } });
    const cookies = fs.readFileSync(required('SP_PREVIEW_COOKIE_FILE'), 'utf8')
      .split('\n').filter(l => l && (!l.startsWith('#') || l.startsWith('#HttpOnly_')))
      .map(l => {
        const [domain, , cookiePath, secure, , name, value] = l.replace('#HttpOnly_', '').split('\t');
        return { name, value, domain, path: cookiePath, secure: secure === 'TRUE', httpOnly: l.startsWith('#HttpOnly_'), sameSite: 'Lax' };
      });
    await ctx.addCookies(cookies);
    const page = await ctx.newPage();
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    const profile = `${base}/players/${id}`;
    const endpoint = `${base}/avatars/${id}.svg?review=ball-only`;
    await page.goto(profile);
    await page.locator('#figure-editor').waitFor();
    const saved = await (await ctx.request.get(endpoint)).text();
    const wolf = page.locator('input[name=character][value=vex]');
    if (!(await wolf.isChecked())) {
      await Promise.all([
        page.waitForResponse(r => r.url().includes('/fragments/figure-character') && r.status() === 200),
        page.locator('.character-pick').filter({ has: wolf }).click(),
      ]);
    }
    assert.equal(await page.locator('.character-pick').filter({has: wolf}).innerText(), 'blecki');
    assert.deepEqual(await page.locator('input[name=bling]').evaluateAll(es => es.map(e => e.value)), ['', 'ball']);
    async function choose(name, value) {
      const input = page.locator(`input[name=${name}][value="${value}"]`);
      if (!(await input.isChecked())) await Promise.all([
        page.waitForResponse(r => r.url().includes('/fragments/figure-preview') && r.status() === 200),
        page.locator('.figure-parts label.pick').filter({ has: input }).click(),
      ]);
    }
    for (const key of ['ball', '']) {
      await choose('bling', key);
      await page.waitForFunction(key => key
        ? document.querySelectorAll(`.face-preview [data-bling="${key}"]`).length === 6
        : !document.querySelector('.face-preview [data-bling]'), key);
      assert.equal(await page.locator('#character-thumbnail-vex [data-bling]').count(), key ? 1 : 0);
      assert.equal(await page.locator('#figure-ball-color fieldset').count(), key === 'ball' ? 1 : 0);
      await page.locator('#figure-editor').screenshot({ path: path.join(out, `${key || 'none'}-desktop.png`) });
      assert.equal(await (await ctx.request.get(endpoint)).text(), saved, 'preview changed saved avatar');
    }
    await choose('bling', 'ball');
    assert.deepEqual(await page.locator('input[name=ballColor][type=radio]').evaluateAll(es => es.map(e => e.value)), ['', 'orange', 'yellow']);
    for (const [value, expected] of [['', '#f8f8f8'], ['orange', '#ff8a24'], ['yellow', '#ffe23b']]) {
      await choose('ballColor', value);
      await page.waitForFunction(hex => [...document.querySelectorAll('.face-preview svg')].every(e => getComputedStyle(e).getPropertyValue('--ball').trim() === hex), expected);
      await page.locator('#figure-editor').screenshot({path: path.join(out, `ball-colour-${value || 'white'}.png`)});
    }
    await choose('bling', '');
    assert.equal(await page.locator('#figure-ball-color fieldset').count(), 0);
    assert.equal(await page.locator('input[name=ballColor][type=hidden]').inputValue(), 'yellow');
    await choose('bling', 'ball');
    assert.ok(await page.locator('input[name=ballColor][value=yellow]').isChecked());
    for (const blade of ['light-blue', 'blue', 'petrol', 'moss', 'pink', 'orchid', 'steel']) {
      await choose('blade', blade);
      assert.equal(await page.locator('.face-preview [data-bling=ball]').count(), 6);
      await page.locator('#figure-preview').screenshot({ path: path.join(out, `ball-${blade}.png`) });
    }
    const form = await page.locator('#figure-editor form').evaluate(f => Object.fromEntries(new FormData(f)));
    for (const bling of ['towel', 'captain', 'bandana', 'moon', 'wolf-tooth']) {
      const r = await ctx.request.get(`${base}/fragments/figure-preview?${new URLSearchParams({ ...form, bling })}`);
      assert.equal(r.status(), 422);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.locator('#figure-editor').screenshot({ path: path.join(out, 'ball-mobile.png') });
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    assert.deepEqual(errors, []);
    assert.equal(await (await ctx.request.get(endpoint)).text(), saved);
    await page.reload();
    console.log('blecki: none/ball, six expressions, conditional ball colours, dormant-colour preservation, live thumbnail, seven blades, retired-key rejection, unchanged persistence and mobile layout OK');
  } finally {
    await browser.close();
  }
}
main().catch(e => { console.error(e); process.exitCode = 1; });

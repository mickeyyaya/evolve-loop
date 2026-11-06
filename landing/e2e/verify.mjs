import { chromium } from 'playwright';
import AxeBuilder from '@axe-core/playwright';
import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const BASE = process.env.SITE_BASE;
const REPO = process.argv[2];
const OUT = process.argv[3];
const site = JSON.parse(readFileSync(join(REPO, 'landing/shared/content.json'), 'utf8'));
const EXPLAIN = ['index', 'fleet-width', 'phase-pipeline', 'trust-kernel', 'bridge-drivers', 'model-routing', 'quality-gates', 'ship-release', 'token-telemetry', 'queue-integrity', 'chronicle', 'resilience'];
const PAGES = ['/', '/noir/', ...EXPLAIN.map((p) => `/explain/${p}.html`)];
const STALE = [/(?<!since )\bv?22\.27\.0\b/, /1,830/, /demotes itself/, /no loop calls/, /no diff to read/, /21 core\s+documents/, /not a lost cycle/, /<no value>/, /landing page concepts/, /\{\{(?:version|semver)\}\}/];
const VIEWPORTS = { desktop: { width: 1280, height: 900 }, phone: { width: 390, height: 844 } };

const results = [];
if (process.env.EXPECT_MARKER) {
  const got = await fetch(`${BASE}/build-marker.txt`).then((r) => r.text()).catch(() => '');
  if (got.trim() !== process.env.EXPECT_MARKER) {
    console.error(`the server gives build marker "${got.trim()}", want "${process.env.EXPECT_MARKER}"`);
    process.exit(1);
  }
}
const check = (area, name, ok, detail = '') => results.push({ area, name, ok: !!ok, detail: String(detail).slice(0, 400) });

const browser = await chromium.launch();
const links = new Map();

for (const [vname, viewport] of Object.entries(VIEWPORTS)) {
  const ctx = await browser.newContext({ viewport, permissions: ['clipboard-read', 'clipboard-write'] });
  for (const path of PAGES) {
    const page = await ctx.newPage();
    const errors = [];
    const failed = [];
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
    page.on('requestfailed', (r) => { if (r.url().startsWith(BASE)) failed.push(r.url()); });
    page.on('response', (r) => { if (r.url().startsWith(BASE) && r.status() >= 400) failed.push(`${r.status()} ${r.url()}`); });
    const res = await page.goto(BASE + path, { waitUntil: 'load' });
    await page.waitForTimeout(400);
    const label = `${path} ${vname}`;
    check('load', `${label} returns 200`, res.status() === 200, res.status());
    check('load', `${label} no console or page errors`, errors.length === 0, errors.join(' | '));
    check('load', `${label} no failed local requests`, failed.length === 0, failed.join(' | '));
    const m = await page.evaluate(() => ({
      over: document.documentElement.scrollWidth - window.innerWidth,
      h1: document.querySelectorAll('h1').length,
      title: document.title,
      text: document.body.innerText,
      broken: [...document.images].filter((i) => i.complete && i.naturalWidth === 0).map((i) => i.src),
      hrefs: [...document.querySelectorAll('a[href]')].map((a) => a.href),
    }));
    check('layout', `${label} no horizontal overflow`, m.over <= 0, `${m.over}px`);
    check('structure', `${label} has one h1 and a title`, m.h1 === 1 && m.title.length > 0, `h1=${m.h1} title=${m.title}`);
    check('images', `${label} no broken images`, m.broken.length === 0, m.broken.join(' '));
    for (const re of STALE) {
      const hit = m.text.match(re);
      if (hit) {
        const at = m.text.indexOf(hit[0]);
        check('stale', `${label} has no ${re}`, false, m.text.slice(Math.max(0, at - 70), at + 70));
      }
    }
    if (vname === 'desktop') {
      for (const h of m.hrefs) { if (!links.has(h)) links.set(h, path); }
      const axe = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze();
      for (const v of axe.violations) {
        check('a11y', `${path} ${v.id} (${v.impact})`, !['critical', 'serious'].includes(v.impact), `${v.nodes.length} node(s): ${v.nodes[0]?.target}`);
      }
      if (axe.violations.length === 0) check('a11y', `${path} no WCAG A or AA violation`, true);
    }
    await page.close();
  }

  const page = await ctx.newPage();
  await page.goto(`${BASE}/`, { waitUntil: 'load' });
  const v = vname;

  const order = await page.evaluate(() => [...document.querySelectorAll('h2')].map((h) => h.textContent.trim()));
  check('content', `${v} install comes before the demos`, order.indexOf(site.tryIt.heading) >= 0 && order.indexOf(site.tryIt.heading) < order.indexOf(site.gateLab.heading), order.join(' > '));
  const h1 = await page.locator('h1').innerText();
  check('content', `${v} headline`, h1.replace(/\s+/g, ' ').includes('Your AI wrote the code. Who decides it ships?'), h1);
  const body = await page.evaluate(() => document.body.innerText);
  check('content', `${v} shows the version in the terminal`, body.includes(`evolve ${site.product.version.slice(1)}`), site.product.version);
  check('content', `${v} shows the cycle count`, body.includes(site.proofBar[0].value), site.proofBar[0].value);
  for (const p of site.pillars) check('content', `${v} pillar "${p.title}"`, body.toLowerCase().includes(p.title.toLowerCase()));
  for (const removed of [site.inboxLab.heading, site.concurrency.heading, site.examples.heading]) check('content', `${v} removed section "${removed}" is gone`, !order.includes(removed));

  const head = await page.evaluate(() => Object.fromEntries(['og:title', 'og:description', 'og:image', 'og:url'].map((k) => [k, document.querySelector(`meta[property="${k}"]`)?.content]).concat([['twitter', document.querySelector('meta[name="twitter:card"]')?.content], ['canonical', document.querySelector('link[rel=canonical]')?.href]])));
  check('meta', `${v} preview tags`, Object.values(head).every(Boolean), JSON.stringify(head));
  const local = head['og:image']?.replace(site.product.siteUrl, `${BASE}/`);
  const img = await page.request.get(local);
  check('meta', `${v} preview image exists`, img.ok(), local);

  const copy = page.locator('#tryit .tri-copy').first();
  await copy.scrollIntoViewIfNeeded();
  await copy.click();
  await page.waitForTimeout(150);
  const clip = await page.evaluate(() => navigator.clipboard.readText().catch((e) => `ERR ${e}`));
  check('demo', `${v} the copy button copies the install command`, clip === site.tryIt.command, clip);

  const lamp = page.locator('#gatelab .gtl-lamp .big');
  for (const s of site.gateLab.scenarios) {
    const b = page.locator('#gatelab .gtl-sc', { hasText: s.label });
    await b.scrollIntoViewIfNeeded();
    await b.click();
    await page.waitForTimeout(350);
    const shown = (await lamp.innerText()).toLowerCase();
    const ship = /ship/.test(shown) && !/block/.test(shown);
    check('demo', `${v} gate scenario "${s.label}" reads ${s.expect}`, s.expect === 'ship' ? ship : !ship, shown);
  }
  const sw = page.locator('#gatelab .gtl-sw[data-key="audit-pass"]');
  await page.locator('#gatelab .gtl-sc', { hasText: site.gateLab.scenarios[0].label }).click();
  await page.waitForTimeout(200);
  await sw.click();
  await page.waitForTimeout(300);
  check('demo', `${v} turning audit off blocks the ship`, !/^ship/i.test((await lamp.innerText()).trim()), await lamp.innerText());

  const evs = page.locator('#recoverylab .rcl-ev');
  check('demo', `${v} recovery lab lists each event`, (await evs.count()) === site.recoveryLab.events.length, await evs.count());
  for (let i = 0; i < site.recoveryLab.events.length; i++) {
    const e = site.recoveryLab.events[i];
    await evs.nth(i).scrollIntoViewIfNeeded();
    await evs.nth(i).click();
    await page.waitForFunction((want) => document.querySelector('#recoverylab .rcl-out .txt')?.textContent.includes(want), e.outcome, { timeout: 15000 }).catch(() => {});
    const out = await page.locator('#recoverylab .rcl-out .txt').innerText();
    const verdict = await page.locator('#recoverylab .rcl-verdict').innerText();
    check('demo', `${v} recovery "${e.label}" shows its outcome and verdict`, out.includes(e.outcome) && verdict.trim().toUpperCase() === e.verdict, `${verdict} | ${out}`);
  }

  await page.locator('#pipeline').scrollIntoViewIfNeeded();
  const goals = new Set();
  const want = site.pipelineDemo.cases.map((c) => c.goal.replace(/`/g, ''));
  const until = Date.now() + (v === 'desktop' ? 150000 : 30000);
  while (Date.now() < until && goals.size < want.length) {
    const g = (await page.locator('#pipeline .pld-goal').innerText()).replace(/`/g, '').trim();
    const full = want.find((w) => w === g);
    if (full) goals.add(full);
    await page.waitForTimeout(500);
  }
  check('demo', `${v} pipeline demo plays its goals (${goals.size} of ${want.length})`, v === 'desktop' ? goals.size === want.length : goals.size >= 1, [...goals].join(' | '));
  await page.screenshot({ path: `${OUT}.${v}.png`, fullPage: true });
  await page.close();
  await ctx.close();
}
await browser.close();
writeFileSync(OUT, JSON.stringify({ results, links: [...links] }, null, 1));
const failed = results.filter((r) => !r.ok);
for (const f of failed) console.error(`FAIL [${f.area}] ${f.name} :: ${f.detail}`);
console.log(`site check: ${results.length - failed.length}/${results.length} PASS`);
process.exit(failed.length ? 1 : 0);

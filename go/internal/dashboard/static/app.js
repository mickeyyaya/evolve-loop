/* evolve dashboard — vanilla JS, no framework. Every artifact string is
   LLM-authored and reaches the DOM through textContent only (the h() helper);
   innerHTML is never used. */
'use strict';

const $ = (id) => document.getElementById(id);

// h(tag, props, ...children): props = {class, title, href, onclick, style}; children are
// strings (become text nodes) or nodes. Strings are never parsed as HTML.
function h(tag, props, ...children) {
  const el = document.createElement(tag);
  if (props) for (const [k, v] of Object.entries(props)) {
    if (v == null) continue;
    if (k === 'class') el.className = v;
    else if (k === 'onclick') el.addEventListener('click', v);
    else if (k === 'style') el.style.cssText = v;
    else if (k === 'hidden') el.hidden = !!v;
    else el.setAttribute(k, v);
  }
  for (const c of children.flat()) {
    if (c == null || c === false) continue;
    el.appendChild(typeof c === 'string' || typeof c === 'number' ? document.createTextNode(String(c)) : c);
  }
  return el;
}
function clear(el) { while (el.firstChild) el.removeChild(el.firstChild); return el; }
function pct(x) { return Math.round((x || 0) * 100) + '%'; }
function fmtTime(s) { if (!s || s.startsWith('0001')) return '—'; const d = new Date(s); return isNaN(d) ? '—' : d.toLocaleString(); }
function fmtDur(ms) { if (!ms) return ''; const s = Math.round(ms / 1000); if (s < 90) return s + 's'; const m = Math.round(s / 60); return m < 120 ? m + 'm' : (m / 60).toFixed(1) + 'h'; }
function ago(s) { if (!s || s.startsWith('0001')) return ''; const ms = Date.now() - new Date(s).getTime(); return ms < 0 ? '' : fmtDur(ms) + ' ago'; }
function short(sha) { return sha ? sha.slice(0, 8) : ''; }
function pill(state, text) { return h('span', { class: 'pill ' + (state || 'neutral') }, text ?? state); }
function verdictClass(v) { return ({ PASS: 'pass', FAIL: 'fail', WARN: 'warn' })[v] || ''; }

let snap = null;

async function load() {
  const r = await fetch('/api/snapshot', { cache: 'no-store' });
  snap = await r.json();
  render();
  if (location.hash.startsWith('#cycle/')) openDetail(parseInt(location.hash.slice(7), 10), true);
}

function render() {
  renderTiles(); renderLoop(); renderTrend(); renderCycles(); renderQueue(); renderGrid(); renderFingerprints(); renderWarnings();
}

function renderTiles() {
  const c = snap.cycles || [];
  const count = (st) => c.filter((x) => x.state === st).length;
  const t = snap.trend || {};
  const tiles = [
    ['running', count('running'), 'running'], ['queued', (snap.queue.pending || []).length, 'queued'],
    ['pass', count('pass'), 'pass (listed)'], ['fail', count('fail') + count('halted'), 'fail (listed)'],
    ['rate20', pct(t.ship_rate_last_20), 'ship rate · last 20'], ['rateAll', pct(t.ship_rate_all), 'ship rate · all ' + (t.closed || 0)],
  ];
  clear($('tiles')).append(...tiles.map(([k, v, l]) => h('div', { class: 'tile ' + k }, h('b', null, v), h('small', null, l))));
}

function renderLoop() {
  const l = snap.loop;
  const box = clear($('loop'));
  box.append(h('h2', null, 'loop', h('small', null, snap.root)));
  const state = l.running ? 'running' : (l.brake_engaged ? 'incomplete' : 'neutral');
  const label = l.running ? 'RUNNING' : (l.brake_engaged ? 'PAUSED (brake)' : (l.cycle_id ? (l.checkpointed ? 'STOPPED (checkpointed)' : 'IDLE / STOPPED') : 'NO CYCLE'));
  const dl = h('dl', { class: 'kv' });
  const kv = (k, v) => dl.append(h('dt', null, k), h('dd', null, v));
  kv('status', pill(state, label));
  if (l.cycle_id) {
    kv('cycle', h('button', { class: 'link', onclick: () => go('#cycle/' + l.cycle_id) }, '#' + l.cycle_id));
    kv('phase', h('span', null, l.phase || '—', ' ', h('span', { class: 'muted' }, l.phase_started_at ? '· since ' + ago(l.phase_started_at) : '')));
    kv('dispatch', h('span', { class: 'mono' }, [l.cli, l.model].filter(Boolean).join(' · ') || '—'));
    kv('audit rounds', String(l.audit_rounds || 0));
    kv('lease', l.lease_heartbeat && !l.lease_heartbeat.startsWith('0001') ? 'heartbeat ' + ago(l.lease_heartbeat) : 'none');
    if (l.active_worktree) kv('worktree', h('span', { class: 'mono small' }, l.active_worktree));
  }
  kv('brake', l.brake_engaged ? '.evolve/loop-stop present' : 'off');
  box.append(dl);
}

function renderTrend() {
  const t = snap.trend || {};
  const box = clear($('trend'));
  box.append(h('h2', null, 'ship rate', h('small', null, `${t.shipped || 0} shipped / ${t.closed || 0} closed · last 20 ${pct(t.ship_rate_last_20)} · last 50 ${pct(t.ship_rate_last_50)} · all ${pct(t.ship_rate_all)}`)));
  const strip = h('div', { class: 'strip', title: 'one bar per closed cycle, oldest → newest' });
  for (const p of t.points || []) strip.append(h('i', { class: verdictClass(p.verdict), title: `#${p.cycle} ${p.verdict}` }));
  box.append(strip);
  const hist = t.round_histogram || [];
  if (hist.length) {
    const tbl = h('table', null, h('tr', null, h('th', null, 'audit rounds'), h('th', { class: 'right' }, 'cycles'), h('th', { class: 'right' }, 'shipped'), h('th', { class: 'right' }, 'ship %')));
    for (const b of hist) tbl.append(h('tr', null, h('td', null, String(b.rounds)), h('td', { class: 'right' }, String(b.cycles)), h('td', { class: 'right' }, String(b.shipped)), h('td', { class: 'right' }, b.cycles ? pct(b.shipped / b.cycles) : '—')));
    box.append(h('p', { class: 'muted small' }, 'repair-loop convergence (cycles that still have a workspace on disk)'), tbl);
  }
}

function stepper(c) {
  const s = h('div', { class: 'stepper' });
  for (const p of c.phases || []) s.append(h('span', { class: 'step ' + verdictClass(p.verdict), title: `${p.phase}${p.round > 1 ? ' r' + p.round : ''} ${p.verdict} ${fmtDur(p.duration_ms)}` }));
  if (c.state === 'running') s.append(h('span', { class: 'step cur', title: c.current_phase }));
  return s;
}

function renderCycles() {
  const box = clear($('cycles'));
  box.append(h('h2', null, 'cycles', h('small', null, 'newest first · click a row')));
  const tbl = h('table', null, h('tr', null, h('th', null, '#'), h('th', null, 'state'), h('th', null, 'phases'), h('th', null, 'rounds'), h('th', null, 'task'), h('th', null, 'what went wrong'), h('th', null, 'ended')));
  for (const c of snap.cycles || []) {
    const f = c.failure;
    const wrong = f ? [f.category || f.pre_class || '', f.fingerprint ? h('span', { class: 'mono muted' }, ' ' + f.fingerprint.split('|').pop()) : ''] : (c.commit_sha ? [h('span', { class: 'mono muted' }, 'shipped ' + short(c.commit_sha))] : '');
    tbl.append(h('tr', { class: 'click', onclick: () => go('#cycle/' + c.id) },
      h('td', { class: 'mono' }, '#' + c.id), h('td', null, pill(c.state, c.state_name)), h('td', null, stepper(c)),
      h('td', null, c.audit_rounds ? String(c.audit_rounds) : ''), h('td', { class: 'small' }, (c.tasks || []).join(', ')),
      h('td', { class: 'small' }, wrong), h('td', { class: 'small muted' }, c.ended_at && !c.ended_at.startsWith('0001') ? ago(c.ended_at) : '')));
  }
  box.append(tbl);
}

function renderQueue() {
  const q = snap.queue;
  const box = clear($('queue'));
  box.append(h('h2', null, 'inbox', h('small', null, `${q.pending.length} pending · ${q.processing} processing · ${q.retry} retry · ${q.consumed} consumed · ${q.processed} processed`)));
  const tbl = h('table', null, h('tr', null, h('th', null, 'w'), h('th', null, 'item'), h('th', null, 'kind'), h('th', null, 'route')));
  for (const it of q.pending.slice(0, 25)) tbl.append(h('tr', { title: it.title }, h('td', { class: 'mono' }, it.weight.toFixed(2)), h('td', null, h('div', null, it.id), h('div', { class: 'small muted' }, (it.title || '').slice(0, 110))), h('td', { class: 'small' }, it.kind || it.class || ''), h('td', { class: 'small' }, it.route || '')));
  if (q.pending.length > 25) tbl.append(h('tr', null, h('td', { colspan: '4', class: 'muted small' }, `… ${q.pending.length - 25} more`)));
  box.append(tbl);
}

function renderGrid() {
  const cycles = (snap.cycles || []).filter((c) => c.has_workspace || (c.phases || []).length).slice(0, 24).reverse();
  const box = clear($('grid'));
  box.append(h('h2', null, 'phase × cycle', h('small', null, 'cell = phase verdict · rN = repeated rounds · click a column header')));
  if (!cycles.length) { box.append(h('p', { class: 'muted' }, 'no cycle workspaces on disk')); return; }
  const phases = [];
  for (const c of cycles) for (const p of c.phases || []) if (!phases.includes(p.phase)) phases.push(p.phase);
  const head = h('tr', null, h('th', null, 'phase'));
  for (const c of cycles) head.append(h('th', { class: 'click mono', onclick: () => go('#cycle/' + c.id) }, String(c.id)));
  const tbl = h('table', { class: 'grid' }, head);
  for (const ph of phases) {
    const tr = h('tr', null, h('td', null, ph));
    for (const c of cycles) {
      const runs = (c.phases || []).filter((p) => p.phase === ph);
      if (!runs.length) { tr.append(h('td', null, h('span', { class: 'cell none' }))); continue; }
      const last = runs[runs.length - 1];
      tr.append(h('td', null, h('span', { class: 'cell ' + (c.state === 'running' && c.current_phase === ph ? 'cur' : verdictClass(last.verdict)), title: `#${c.id} ${ph} ${last.verdict} ${fmtDur(last.duration_ms)} ${last.cli || ''} ${last.model || ''}` }, runs.length > 1 ? 'r' + runs.length : '')));
    }
    tbl.append(tr);
  }
  const vr = h('tr', null, h('td', null, 'verdict'));
  for (const c of cycles) vr.append(h('td', null, pill(c.state, (c.verdict || c.state).slice(0, 1))));
  tbl.append(vr);
  box.append(h('div', { class: 'grid-wrap' }, tbl));
}

function renderFingerprints() {
  const box = clear($('fps'));
  const fps = snap.fingerprints || [];
  box.append(h('h2', null, 'failure classes', h('small', null, `${fps.length} fingerprints · grouped from committed dossiers`)));
  const tbl = h('table', null, h('tr', null, h('th', null, 'fingerprint'), h('th', null, 'class'), h('th', { class: 'right' }, 'count'), h('th', null, 'first'), h('th', null, 'last'), h('th', null, 'flags'), h('th', null, 'latest reason')));
  for (const f of fps.slice(0, 30)) tbl.append(h('tr', { class: 'click', onclick: () => go('#cycle/' + f.last_cycle) },
    h('td', { class: 'mono' }, f.fingerprint), h('td', null, f.pre_class || ''), h('td', { class: 'right' }, String(f.count)),
    h('td', { class: 'mono' }, '#' + f.first_cycle), h('td', { class: 'mono' }, '#' + f.last_cycle),
    h('td', null, f.regressed ? pill('halted', 'REGRESSED') : (f.count > 1 ? pill('warn', 'recurring') : pill('neutral', 'new'))),
    h('td', { class: 'small' }, (f.reason || '').slice(0, 140))));
  box.append(tbl);
}

function renderWarnings() {
  const w = snap.warnings || [];
  const f = clear($('warnings'));
  f.append(h('span', null, `generated ${fmtTime(snap.generated_at)} · ${w.length} unreadable artifact${w.length === 1 ? '' : 's'}`));
  if (w.length) f.append(h('details', null, h('summary', null, 'show'), h('ul', null, ...w.map((x) => h('li', { class: 'mono' }, x)))));
}

/* ---------- detail ---------- */
function go(hash) { location.hash = hash; }
window.addEventListener('hashchange', () => {
  if (location.hash.startsWith('#cycle/')) openDetail(parseInt(location.hash.slice(7), 10), false);
  else { $('detail').hidden = true; $('board').hidden = false; }
});

async function openDetail(id, silent) {
  if (!id) return;
  const r = await fetch('/api/cycle/' + id, { cache: 'no-store' });
  if (!r.ok) { if (!silent) alert('cycle ' + id + ' not found'); return; }
  const d = await r.json();
  const c = d.cycle;
  const box = clear($('detail'));
  $('board').hidden = true; box.hidden = false;
  box.append(h('h2', null, h('span', null, h('button', { class: 'link', onclick: () => go('#') }, '← board'), '  cycle #' + c.id), h('small', null, pill(c.state, c.state_name), ' ', c.commit_sha ? h('span', { class: 'mono' }, 'shipped ' + short(c.commit_sha)) : '', ' ', c.tokens ? `${c.tokens.toLocaleString()} tokens` : '')));
  const dl = h('dl', { class: 'kv' });
  const kv = (k, v) => dl.append(h('dt', null, k), h('dd', null, v));
  if (c.goal) kv('goal', c.goal);
  if ((c.tasks || []).length) kv('tasks', c.tasks.join(', '));
  kv('window', `${fmtTime(c.started_at)} → ${fmtTime(c.ended_at)}`);
  kv('sources', [c.has_workspace ? 'run workspace' : null, c.has_dossier ? 'committed dossier' : null].filter(Boolean).join(' + ') || 'none');
  box.append(dl);
  if (c.failure) box.append(failurePanel(c.failure, snap.fingerprints || [], c.state === 'halted'));
  box.append(timeline(c));
  box.append(artifactBrowser(c.id, d.artifacts || [], d.primary_report || ''));
  if ((d.warnings || []).length) box.append(h('p', { class: 'muted small' }, 'unreadable: ' + d.warnings.join('; ')));
}

function failurePanel(f, fps, halted) {
  const stat = fps.find((x) => x.fingerprint === f.fingerprint);
  const p = h('div', { class: 'panel' }, h('h2', null, 'what went wrong'));
  p.append(h('div', { class: 'rowline' }, f.category ? pill('fail', f.category) : '', f.level ? pill(halted ? 'halted' : 'neutral', f.level) : '', f.action ? pill('neutral', f.action + (f.fix_type ? ' · ' + f.fix_type : '')) : '', f.urgency ? pill('warn', f.urgency) : ''));
  if (f.fingerprint) p.append(h('div', { class: 'rowline' }, h('span', { class: 'mono' }, f.fingerprint), stat ? h('span', { class: 'muted' }, `seen ${stat.count}× · first #${stat.first_cycle} · last #${stat.last_cycle}`) : '', stat && stat.regressed ? pill('halted', 'REGRESSED') : ''));
  if (f.root_cause || f.legitimacy || f.layer) p.append(h('div', { class: 'rowline' }, f.legitimacy ? pill(f.legitimacy === 'false-rejection' ? 'halted' : 'neutral', f.legitimacy) : '', f.layer ? pill('neutral', 'layer: ' + f.layer) : '', h('span', null, f.root_cause || '')));
  if ((f.gate_reasons || []).length) p.append(h('div', null, h('b', null, 'deterministic gate reasons'), h('ul', null, ...f.gate_reasons.map((r) => h('li', { class: 'small' }, r)))));
  if ((f.findings || []).length) p.append(h('div', null, h('b', null, 'auditor findings (final round)'), ...f.findings.map((x) => h('div', { class: 'finding' }, h('b', { class: 'mono sev-' + x.severity }, x.id, ' ', x.severity), x.title))));
  if ((f.rounds || []).length) {
    const parts = f.rounds.map((r, i) => `r${r.round} ${r.verdict || '?'} (${r.findings.length}${i ? `: ${r.resolved} resolved, ${r.new} new, ${r.carried} carried` : ''})`);
    p.append(h('div', null, h('b', null, 'repair-round history'), ' ', h('span', { class: 'mono small' }, parts.join(' → '))));
  }
  if (f.salvage) p.append(h('div', { class: 'rowline' }, h('b', null, 'salvage'), h('span', { class: 'mono small' }, f.salvage)));
  return p;
}

function timeline(c) {
  const box = h('div', null, h('h2', null, 'phase timeline', h('small', null, 'bars = wall clock · label = cli · model')));
  const ph = c.phases || [];
  if (!ph.length) { box.append(h('p', { class: 'muted' }, 'no phase timing recorded for this cycle')); return box; }
  const t0 = Math.min(...ph.map((p) => new Date(p.started_at).getTime()).filter((x) => !isNaN(x)));
  const t1 = Math.max(...ph.map((p) => new Date(p.ended_at).getTime()).filter((x) => !isNaN(x)));
  const span = Math.max(1, t1 - t0);
  const bars = h('div', { class: 'bars' });
  for (const p of ph) {
    const s = new Date(p.started_at).getTime(), e = new Date(p.ended_at).getTime();
    const left = isNaN(s) ? 0 : ((s - t0) / span) * 100, width = isNaN(e) || isNaN(s) ? 2 : Math.max(0.6, ((e - s) / span) * 100);
    bars.append(h('div', { class: 'small' }, p.phase, p.round > 1 ? h('span', { class: 'muted' }, ' r' + p.round) : ''),
      h('div', { class: 'lane' }, h('div', { class: 'bar ' + verdictClass(p.verdict), style: `left:${left}%;width:${width}%`, title: `${p.phase} ${p.verdict} ${fmtDur(p.duration_ms)} · ${p.cli || ''} ${p.model || ''} · ${p.tokens || 0} tokens` }, `${p.cli || ''} ${p.model || ''}`.trim())));
  }
  box.append(bars);
  return box;
}

function artifactBrowser(id, arts, primary) {
  const box = h('div', null, h('h2', null, 'artifacts', h('small', null, `${arts.length} readable files · reports first`)));
  const view = h('pre', { class: 'doc' }, 'select an artifact');
  const tabs = h('div', { class: 'tabs' });
  // Display order only: the server names the primary report (registry-derived); the rest group by shape.
  const order = (n) => (n === primary ? 0 : /-report\.md$/.test(n) ? 1 : /\.json$/.test(n) ? 2 : /-prompt\.txt$/.test(n) ? 3 : 4);
  const sorted = [...arts].sort((a, b) => order(a.name) - order(b.name) || a.name.localeCompare(b.name));
  let on = null;
  const show = async (a, tab) => {
    if (on) on.classList.remove('on'); on = tab; tab.classList.add('on');
    const r = await fetch(`/api/artifact/${id}/${encodeURIComponent(a.name)}`, { cache: 'no-store' });
    clear(view);
    if (!r.ok) { view.append(`(${r.status}) ${await r.text()}`); return; }
    const text = await r.text();
    // Visual heading hints only: lines starting with '#' get a class; the text itself is a text node.
    for (const line of text.split('\n')) view.append(line.startsWith('#') ? h('span', { class: 'hd' }, line + '\n') : line + '\n');
  };
  for (const a of sorted) { const tab = h('span', { class: 'tab', title: `${a.size} bytes · ${fmtTime(a.mod_time)}` }, a.name); tab.addEventListener('click', () => show(a, tab)); tabs.append(tab); }
  box.append(tabs, view);
  const first = sorted.find((a) => a.name === primary) || sorted[0];
  if (first) show(first, tabs.children[sorted.indexOf(first)]);
  return box;
}

/* ---------- live updates ---------- */
function connect() {
  const es = new EventSource('/events');
  es.addEventListener('snapshot', () => { $('live').textContent = 'live'; $('live').className = 'live ok'; load().catch(() => {}); });
  es.onerror = () => { $('live').textContent = 'reconnecting…'; $('live').className = 'live bad'; };
}
load().then(connect).catch((e) => { $('live').textContent = 'load failed: ' + e; $('live').className = 'live bad'; });
setInterval(() => load().catch(() => {}), 60000); // safety net if the stream is silently dead

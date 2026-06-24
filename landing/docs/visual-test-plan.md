# Landing — Visual Test Plan

Coverage plan for the landing site's visual behavior, with the **AI-composed-pipeline
connector-line stacking** fix as a first-class case. Two layers:

1. **Automated invariants** (Go, runs in CI) — catch structural/CSS regressions deterministically.
2. **Visual matrix** (manual / headless-browser) — catch rendering regressions a static check can't see.

All five themes draw from one shared partial (`templates/partials.html`) and one content
SSOT (`shared/content.json`), so most regressions reproduce across every theme at once.

---

## 1. Invariant under test (the line fix)

> The moving connector line in the "AI-composed pipeline" section (`.pld-railfill`, with its
> glowing leading dot) runs along a rail **behind** the phase cards. It must stay **hidden behind
> every on-rail phase card** and appear only in the **gaps** between cards as a connector — never
> on top of, or showing through, a card.

Two independent failure modes, both previously shipped and now fixed:

| Failure mode | Cause | Fix |
|---|---|---|
| Line shows **through** a lit card | Tinted state used a translucent background (`pick` = `rgba(accent,0.1)`, `floor.on`/`mint` soft tints) with nothing opaque behind it | Composite each tint over the opaque `--pl-node-base` (`background: <tint>, var(--pl-node-base)`) |
| Line shows **through** a dim/placeholder card | `default`/`considering`/`ghost` used element `opacity < 1`, making the whole card (opaque base included) translucent | Dim via muted color/border, **not** element opacity; cards stay `opacity:1` and opaque |

**Exempt states** (correct to remain non-opaque):
- `.pld-node.skip` — translated 48px **down**, off the rail; nothing is behind it (reads as a gap).
- `.pld-node.premint` — `opacity:0` invisible pre-pop placeholder; carries no visible card.

---

## 2. Automated coverage (Go — `go test ./...`)

| Test | File | Guards |
|---|---|---|
| `TestPipelineNodesAreOpaque_NoElementOpacity` | `internal/buildsite/pipeline_demo_stacking_test.go` | No on-rail visible state (`base, considering, pick, floor.on, mint, ghost`) uses `opacity<1` |
| `TestPipelineTintedNodesCompositeOverOpaqueBase` | same | Every tinted state layers its tint over `var(--pl-node-base)` |
| `TestPipelineNodeBaseIsOpaqueByConstruction` | same | `--pl-node-base` exists, never mixes `transparent`, and is the base node background |
| `TestPipelineRailStaysBehindNodes` | same | Nodes keep `z-index:2`; rail/fill never claim `z-index>=2` |
| `TestValidate_*`, content/structure tests | `internal/content`, `internal/buildsite` | Content SSOT shape: proofBar non-empty, `examples.items>=5`, `concurrency.lanes>=2`, section blocks present |

These run in the default CI tier (the `validate` job). They are deterministic and need no browser.
**Limitation:** they assert the *CSS contract*, not pixels — they cannot prove the rendered result.
That is the job of the visual matrix below.

---

## 3. Visual matrix (manual / headless)

Render target: each theme page + the gallery index. Drive the pipeline demo through its states.

### 3a. Pipeline-demo connector line — the core matrix

For every theme, freeze frames covering each phase-card state with the rail/fill behind it, and
confirm the line is **hidden behind the card** (visible only in the inter-card gaps):

| Phase-card state | Expected vs. the line |
|---|---|
| default (not yet decided) | opaque card; line hidden behind it |
| `considering` (pulsing) | opaque card; line hidden behind it |
| `pick` (accent) | opaque tinted card; line hidden |
| `floor.on` (mandated, lock) | opaque tinted card; line hidden |
| `mint` (pop + "minted") | opaque card; line hidden, including during the pop |
| `ghost` ("+ unlimited") | opaque dashed card; line hidden |
| `skip` (dropped) | card sits below the rail; line shows in the gap above it (correct) |
| leading glow dot crossing a card | dot occluded by the card; visible only in gaps |

### 3b. Cross-cutting cases

| Case | Expectation |
|---|---|
| Themes | `aurora-glass`, `blueprint`, `editorial`, `luminous`, `noir`, + gallery — line hidden in all (opaque base verified light **and** dark) |
| Section order | `pipelinedemo` → `concurrency` ("Run several loops at once") → `examples` |
| Examples #4 | "Stack the levers" (`/evo:loop --cycles 3 harden …`), not `/evo:setup` |
| Proof bar | no `v20.4 / shipping` chip; reads `367+ / 61 / Apache-2.0` |
| Breakpoints | 320 / 375 / 768 / 1024 / 1440 / 1920 — no overflow; rail/cards reflow without the line escaping a card |
| `prefers-reduced-motion` | demo animations stop (`.pld-node.mint`, `.considering`, `.spot`, cursor) — the static frame still shows the line hidden behind cards |

---

## 4. How to verify visually (headless recipe)

Build + serve, then drive the demo. The demo rebuilds nodes per goal, so **freeze** it to capture a
stable frame:

```
# build + serve
(cd landing && go run ./cmd/build && go run ./cmd/serve dist 127.0.0.1:8081)
```

In a headless browser (e.g. the Playwright MCP `/browse` tooling), per theme page:

```js
// 1) let a goal build, then kill all timers so nodes stop rebuilding
let hi = setTimeout(()=>{},0); for (let i=0;i<=hi;i++){ clearTimeout(i); clearInterval(i); }
const rf = document.querySelector('.pld-railfill');
rf.style.transition = 'none';                 // pin the sweep
document.head.appendChild(Object.assign(document.createElement('style'),
  {textContent:'.pld *{animation-play-state:paused!important}'}));

// 2) ASSERTION HINT: opacity is the real signal. elementFromPoint() hit-tests by
//    geometry and IGNORES opacity, so a translucent card still "wins" it — do NOT
//    rely on elementFromPoint to prove occlusion. Check opacity + screenshot instead.
[...document.querySelectorAll('.pld-node')].map(n =>
  ({c:n.className, op:getComputedStyle(n).opacity}));   // every on-rail card must be op==1

// 3) screenshot .pld-board and eyeball: line only in gaps, never crossing a card body.
```

Capture `.pld-board` per theme; compare against the expectation table in §3a.

---

## 5. Sign-off checklist

- [ ] `go test ./...` green (incl. the four `TestPipeline*` invariants)
- [ ] Pipeline line hidden behind every on-rail card state — verified on `aurora-glass` (light) and `noir` (dark)
- [ ] Spot-check `blueprint`, `editorial`, `luminous`, gallery
- [ ] No overflow at 320 / 768 / 1440
- [ ] Reduced-motion: animations stop, line still hidden
- [ ] Section order + examples #4 + proof bar match §3b

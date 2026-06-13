# ADR-0047: Surface Classification — One Channel Separator per Mixed Surface

- **Status:** Proposed → Building (Stage 1 corpus gate + Stage 2 pane classifier first)
- **Date:** 2026-06-13
- **Driver:** the single most-recurring bug class of the W4 campaign — a detector reads a
  surface that co-mingles **agent content** with **infrastructure chrome/control** and matches
  the wrong layer. Named the "content-vs-chrome disease" in the campaign memory after its 3rd
  instance; it is now at **8** (catalog below).
- **Relates to:** ADR-0026 (self-healing review layer / pane liveness), ADR-0034/0035
  (deliverable contracts), ADR-0046 (gate epistemics — production-harvested golden corpora),
  the no-duplication / single-source-with-projection standing rule.

## Problem

The loop drives external CLIs (claude/codex/agy) and reads agent-authored artifacts through
**mixed surfaces** — one byte stream carrying two logically distinct channels with no structural
delimiter between them:

| Surface | Content channel (agent's work) | Control/chrome channel (infra signal) |
|---|---|---|
| tmux pane | transcript: tool calls, file contents, prose | spinner frames, elapsed clock, token counter, "esc to interrupt", rate-limit banner |
| triage-report.md | the human-readable decision prose | `top_n`/`deferred` declarations, floor commitments, `evidence=`/`source=` metadata |
| git working tree / diff | the cycle's deliverable files | a phase leaking writes outside its worktree |

Every place the loop makes a control decision — *is the agent busy? did it progress? is this a
rate-limit wall? is this a deliverable or a leak? is this a floor declaration?* — it
pattern-matches against the **raw** surface. Two structural facts make this fragile:

1. **The content channel is adversarial.** An agent editing `clihealth.go` renders the literal
   string "You've hit your usage limit" into the pane (clihealth *is* the rate-limit parser); a
   scout writes `.evolve/evals/*.md` that look exactly like a tree leak; a triage bullet's
   `evidence=go/internal/paths/...` names a package the floor counter is hunting. Content can
   contain *any* substring a control detector keys on.
2. **The separation logic is duplicated and drifts.** "What is chrome vs content on a pane" is
   re-implemented in `cleanPane` (progress detection) and again in `PaneBusy`
   (liveness) — and they **disagree**: the claude `· Schlepping… (50s · ↑ 3.1k tokens)` line is
   *chrome* to `PaneBusy` but *content* to `cleanPane`. Each re-implementation rots
   independently when a CLI updates its TUI.

### The disease catalog (every instance was a separate point-fix)

| Fix | Surface | Misclassification |
|---|---|---|
| `0f0b1ff9` triagecap floor counter | triage-report | contract metadata (`evidence=`,`source=scout`) + prose ("paths") counted as package floor declarations |
| `f3766fe3` Gate-C defer_reason | triage-report | a `defer_reason` naming a package blocked that package's predicates |
| `4e882900` bridge agent-diff | pane | the agent's own test-fixture text matched the rate-limit auto-responder rule |
| `c6efc29f` bench evidence line | pane | `evidenceLine` picked an agent edit-diff frame as the rate-limit banner |
| `beb82d66` progressing-agent timeout | pane | `Progressed` (content) and `Busy` (chrome) conflated under one extend budget |
| `d26eaf7a` claude 2.1.173 busy | pane | CLI dropped "esc to interrupt" → real busy chrome no longer recognized |
| `c62cd1ef` blank-pane wedge | pane | a render glitch (blank chrome) read as a dead agent |
| `a01d666a` scout eval tree-guard | tree-diff | scout's contractual eval deliverable read as a leak |

Six of eight are the **pane**; two are the **triage-report**; one is the **tree-diff**. Each was
fixed by narrowing one regex or adding one carve-out. None fixed the *class*, so the class kept
recurring — and the two channel-separators on the pane are still divergent today.

## Decision

Introduce a **Surface Classification layer**: exactly one *channel separator* per mixed surface,
consumed by every detector on that surface. Four design rules, each a named pattern:

### 1. Strategy + single source — one classifier per surface (kills the duplication/drift)

Each surface gets ONE classifier that maps a unit (pane line, report item, diff path) to a small
**closed** set of layers. All detectors are *projections* of that one classification — never their
own regex:

```
panestream.ClassifyLine(line, profile) → Layer{Content | Chrome | Affordance}
    PaneBusy            = (Affordance present)            // liveness  = content ∨ affordance
    cleanPane/Progress  = (keep Content only)             // progress  = real output only
```

This is the no-duplication / single-source rule applied to *classification logic*. It
structurally eliminates the `cleanPane`-vs-`PaneBusy` divergence: the `· Schlepping…` line is
`Affordance` for both — busy=true (liveness) **and** excluded from progress (it is not new work).
That is strictly *more* correct than today and closes the "ticking-clock extends forever" hole
the `beb82d66` fix left open, while keeping cycle-311/312 resolved (that scout emitted real
`⏺ Bash` tool-call `Content`).

### 2. Specification — control detectors match a closed, structurally-anchored vocabulary

A control signal is recognized by **structure in its own channel**, never by an open substring
anywhere on the surface. A rate-limit wall is a banner *shape* in the chrome region, not the
substring "usage limit" wherever it appears. A floor is `coverage|floor` ∧ `%` ∧ a known package
token *after metadata is stripped*. This generalizes every "narrow the match" point-fix into a
standing rule: **the content channel is untrusted input; match the control grammar, not a word.**

### 3. Region awareness — separate the channels at the source where the surface has zones

The deepest defence is to not co-mingle in the first place. A tmux pane has a **transcript zone**
(agent content) and a **chrome/footer zone** (CLI affordances + banners). Control detectors are
scoped to the chrome zone; content detectors to the transcript. A rate-limit banner found in the
transcript zone is the *agent quoting one*, not a wall — exactly the `4e882900` / `c6efc29f`
class. (`PaneRegion` decomposition; the bridge-ratelimit "footer-scoping" remainder is the first
consumer.)

### 4. Adversarial corpus gate — the regression that catches the NEXT drift at CI

A test corpus harvested from real `.evolve/runs/cycle-*/` panes, triage-reports, and diffs, with
one invariant: **no classifier produces a false positive on real agent content, nor a false
negative on real control chrome.** When a CLI ships a new spinner verb or drops an affordance
(claude did both, 2.1.173 and 2.1.175), the corpus fails at CI — not at 3am in a soak. This
generalizes ADR-0046 Layer 2's production-harvested golden corpora to every surface classifier.

## Architecture

```
                 ┌──────────────────────────────────────────┐
   raw surface → │  SurfaceClassifier (Strategy, 1 per kind) │ → Layer/Region
                 │   panestream.ClassifyLine  (pane)          │
                 │   triagecap.report parser  (triage-report) │
                 │   contract-driven leak gate (tree-diff)    │
                 └──────────────────────────────────────────┘
                        │ every detector is a PROJECTION, never its own regex
        ┌───────────────┼───────────────┬────────────────────┐
     PaneBusy     cleanPane/Progress  rate-limit          floor counter / Gate-C / projection
   (liveness)      (real output)   (chrome-zone banner)   (declarations, not prose)
                        ▲
                        └── adversarial corpus gate asserts no detector misfires on real data
```

Deliverable-vs-leak (tree-diff) becomes **contract-driven**: the phase's declared deliverable
prefixes (ADR-0034 contracts) define what is legitimate, replacing the hardcoded prefix list +
per-phase carve-outs (`a01d666a` was a carve-out the contract would have made unnecessary).

## Build plan (staged, TDD + dual-review + ship per stage; pane first — it owns 6/8)

- **Stage 1 — Adversarial corpus gate (pure prevention, zero behaviour change).** Harvest real
  panes/reports from `.evolve/runs/`; assert the *current* classifiers don't misfire. Ships the
  safety net first so Stage 2's refactor is guarded.
- **Stage 2 — Pane `ClassifyLine` single source.** Introduce the classifier; migrate `PaneBusy`
  (byte-identical) and `cleanPane`/`PaneHasSubstantiveChange` (Schlepping → `Affordance`,
  re-validating cycle-311/312 and pinning the closed ticking-clock hole) onto it.
- **Stage 3 — Triage-report single source.** `topNSection` already delegates to `sectionBody`
  (project.go); fold the floor counter, Gate-C, and the new `ProjectDecisionJSON` onto one item
  parser. (The projection — `triage-decision-json-not-emitted` — lands here.)
- **Stage 4 — Contract-driven tree-diff leak gate + `PaneRegion` chrome-zone scoping** (the
  bridge-ratelimit footer remainder).

## Consequences

- **Positive:** the largest recurring bug class becomes structurally impossible; CLI-chrome drift
  is caught at CI; one home per surface to update when a CLI changes; closes the ticking-clock
  hole; retires a pile of divergent regexes.
- **Cost:** load-bearing pane code is touched; Stage 2 is migrated behind the Stage 1 corpus net
  and full TDD, byte-identical where possible, behaviour-change (Schlepping) explicitly pinned.
- **Non-goal:** out-of-band structured CLI signaling (an agent-SDK event stream instead of pane
  scraping) is the eventual deepest fix but requires CLI cooperation; this ADR hardens the
  scrape-the-pane reality we have.

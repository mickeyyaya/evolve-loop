# Go-only consolidation — architecture review & gap bridge (2026-05-31)

Review of [the consolidation plan](../../../.claude/plans/first-let-s-summarize-to-cheerful-blanket.md)
against the goal: *everything learned is carefully summarized in a well-structured filesystem usable for
future development.* Verdict: **the plan is sound and safety-first; Stage 1 is strong. Seven architectural
gaps must be bridged — most stem from one root: the plan optimizes for "reconstruct the past" but
under-serves "extend the future," and its stage ordering captures knowledge BEFORE the keepers that
change the architecture.** Settled decisions are not re-opened.

## Validated (keep as-is)

- **Safety model** — rewrite-knowledge → merge → verify → delete; tag + bundle reversibility. Correct.
- **Stage-1 taxonomy** — `00-overview / architecture / evolution / incidents / reference / _migration`
  is navigable and non-redundant; README has reading-paths + a governance paragraph. 3690 lines, synthesized.
- **Test redesign (Stage 3)** — tiered `unit / integration / e2e / trustkernel / commitgate` + one authoritative
  `go/docs/testing.md`, behavior-named not cycle-pegged. Idiomatic.

## The seven gaps (each: gap → why it matters → bridge)

### G1 — No extension layer (`guides/`). THE biggest gap vs the goal.
The taxonomy is entirely *understand-the-system* (overview/architecture/evolution/incidents/reference). The
goal says **"used for future development"** — a new dev cannot, from this base, learn *how to extend* the
system. Missing actionable guides.
**Bridge:** add `knowledge/guides/` — the extension cookbook:
`add-a-phase.md`, `add-a-cli-driver.md` (the bridge driver contract), `write-an-eval-and-predicate.md`,
`run-and-debug-locally.md`, `the-dev-workflow.md` (worktree → TDD → commit-gate → ship). Each guide ends with
"the tests that pin this" (see G3).

### G2 — Knowledge is STALE relative to keepers + today's work (ordering gap).
Stage 1 was committed at `b06ed8f` (= main `5436919`). It does **not** describe: the 7 `feat/ws-*` keepers
(abs-root, **sandbox confinement**, soft-fail, **observer liveness**, ollama driver, **any-CLI/any-model matrix**),
nor today's **ship-recovery** (`b212a33`: advisor-centric ship-error recovery + debugger phase + CoR), nor
today's hard-won lessons (headless-driver **cwd=worktree parity**, "**workspace IS the cycle root**", agent
**write-location contracts**, the `assert_go_build` library gap). "Summarize before deleting" is right for
*history*; but the keepers are *current architecture* whose knowledge can only be written *after* they merge.
**Bridge (stage-ordering fix):** Stage 1 is **not one-shot** — it has a mandatory **Stage 2.5 "knowledge
reconciliation"** pass: after keepers merge, update `architecture/routing-and-advisor.md` (+ debugger phase,
ship-error recovery CoR), `architecture/bridge-and-adapters.md` (+ headless cwd parity, driver contract),
`incidents/pattern-library.md` (+ workspace-is-cycle-root, write-location, invented-helper patterns), and a new
`architecture/cli-matrix-and-drivers.md` (G6). The Stage-1 coverage gate must run AGAINST post-Stage-2 main.

### G3 — Knowledge ↔ tests are not cross-wired.
Stage 3 builds `go/test/trustkernel/`; Stage 1 `architecture/*` documents the same invariants — but nothing
links them. A future dev changing an invariant cannot find its guard.
**Bridge:** convention — every `architecture/*.md` invariant cites its pinning test (`go/test/trustkernel/...`),
and `go/docs/testing.md` carries an **invariant → test → knowledge-doc** table. Cheap, high-leverage for "future dev."

### G4 — Governance has a policy but no enforcement → re-sprawl risk.
The README says "refine in place," but nothing prevents the next 50 cycles from re-flattening into sprawl.
**Bridge:** `knowledge/CONTRIBUTING.md` (where each artifact type lands) + a lightweight `evolve doctor` /
CI check: a new ADR or `docs/incidents/` entry must touch `knowledge/`. Documented policy + one grep gate —
not heavy machinery (respects no-flag-sprawl).

### G5 — Stage 4 bundles mechanical couplings with the 8–12-week narrative rewrite.
The plan itself flags Stage 4 as the long pole likely to split. Architecturally it conflates two risk classes.
**Bridge:** split — **4a (mechanical, do this session):** fix the 16 Go-code `docs/...` path consts, both CI
workflows' doc paths, retire the 575 `legacy/scripts/` refs, add `doc.go` per package. Small, unblocks Go-only,
verifiable by `grep`/`go build`. **4b (incremental follow-up):** distill 187 docs → ~20–30 narrative docs.
A clean Go-only tree ships after 4a regardless of 4b's timeline.

### G6 — The "any CLI × any phase × any model" invariant deserves first-class architecture, not just a matrix.
Today's entire debugging session was cross-CLI behavior (claude-p completes nested where tmux REPL hangs; the
headless drivers lacked the `cd $worktree` the tmux driver had). `reference/cli-capability-matrix.md` is a lookup
table; the **invariant + failure modes + the driver contract (what a new CLI driver must satisfy: cwd, sandbox,
write-location, completion signal)** is architecture.
**Bridge:** `architecture/cli-matrix-and-drivers.md` — the invariant, the `Driver` contract, cross-CLI parity
checklist; the reference matrix becomes its lookup appendix. Pairs with the G1 `add-a-cli-driver.md` guide.

### G7 — Cross-CLI trust is an unresolved security-architecture decision (`stash@{1}`, issue#2, ws-g).
The plan flags `stash@{1}` (cross-CLI trust bypass) for security review but offers no design. With ws-g (any-CLI)
merging, "how is a non-Claude CLI's phase output trusted?" (challenge tokens, sandbox confinement from ws-b,
ledger attestation) needs a coherent decision, not a keep/drop coin-flip.
**Bridge:** Stage 2 produces an explicit `architecture/trust-kernel-and-egps.md` subsection (or ADR) on the
**cross-CLI trust model**, reconciling ws-b sandbox + ws-g any-CLI + the challenge-token chain. Gate the
`stash@{1}` keep/drop on that decision.

## Revised stage sequencing (the architectural correction)

```
Stage 1   knowledge capture (history)            ✅ DONE (b06ed8f) — strong
Stage 2   merge keepers (ship-recovery + ws-*)   ← includes TODAY's validated ship-recovery branch
Stage 2.5 KNOWLEDGE RECONCILIATION (NEW)         ← capture keeper + today's architecture; re-run coverage gate
Stage 3   test architecture + knowledge↔test wiring (G3)
Stage 4a  mechanical doc/CI/code-path couplings  ← unblocks Go-only this session
Stage 1.5 guides/ extension layer (NEW, G1)      ← the future-dev cookbook
Stage 4b  narrative doc distillation (187→~25)   ← incremental; may follow-up
Stage 5   destructive cleanup
Stage 6   review + ship (+ governance gate, G4)
```

The one structural insight: **knowledge capture must straddle the keeper-merge, not precede it.** Stage 1
captured history correctly; Stage 2.5 captures the *current* architecture the keepers establish. Without it,
the knowledge base ships describing a codebase that no longer exists.

## Execution order this session (after the running E2E completes)

1. **Finish/validate ship-recovery** (the running cycle-12) — it is a Stage-2 keeper; validating it first is the
   natural order, and its merge + today's fixes feed Stage 2.5.
2. **Stage 2** — triage + merge keepers (ship-recovery first; ws-* per ledger; G7 trust decision gates `stash@{1}`).
3. **Stage 2.5** — reconcile knowledge to post-merge architecture (debugger phase, cwd parity, write-location,
   workspace-is-cycle-root, invented-helper, assert lib gap).
4. **Stage 1.5 + G3 + G4** — add `guides/`, wire knowledge↔tests, add governance.
5. **Stage 4a** — mechanical couplings; defer 4b narrative if time-boxed.

Stages 0–3 + 4a + 5 deliver a clean, future-usable Go-only tree regardless of 4b's timeline.

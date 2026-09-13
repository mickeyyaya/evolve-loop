---
score_cap:
  - criterion: "llmroute's PACKAGE doc (what `go doc ./internal/llmroute` prints before the symbol list) states in one sentence that a bare overlay CLI is a family selector, a hyphen-qualified one a driver selector, and an exact chain entry outranks both"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_001_' ./acs/cycle1647/"
  - criterion: "Through the production advisor projection (runner.resolveDispatchPlan, routing.go), a bare 'claude' overlay over the resolved chain [claude-p codex] dispatches claude-p and never claude-tmux"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_002_' ./acs/cycle1647/"
  - criterion: "Through the same production path, an explicit 'claude-tmux' overlay over chain [claude-p] dispatches claude-tmux FIRST (never satisfied by promoting claude-p) and retains claude-p as the fallback the walk reaches on a trigger exit"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_003_' ./acs/cycle1647/"
  - criterion: "A focused runner test named TestResolveRouting_AdvisorOverlayPreservesFamilyTransport passes alongside the contract-escalation runner test, names resolveDispatchPlan/routing.go as the advisor caller, distinguishes the contract-escalation projection, and drives the literal claude-p/codex/claude-tmux inputs"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_004_' ./acs/cycle1647/"
  - criterion: "internal/llmroute and internal/router are race-clean and apicover -enforce reports 0 uncovered, 0 false-green exports for internal/llmroute"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_005_' ./acs/cycle1647/"
  - criterion: "The cycle's ACS predicate package and this eval are git-tracked so the audit's predicate tree and the ship tree agree"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_006_' ./acs/cycle1647/"
  - criterion: "Every inherited go/acs/cycle* predicate package the base-bound diff ADDS is green on the shipping tree, each run as one named package — the tracked unified-synthesis eval names their tests as evidence, so a red inherited package is a red ship tree"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_007_' ./acs/cycle1647/"
  - criterion: "The shipping build explanation carries no path:line citation into a path this diff deletes — a line citation must be openable on the shipped tree (the consumed inbox copy survives; the root copy does not)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1647_008_' ./acs/cycle1647/"
---

# Eval: A routing overlay naming a bare FAMILY must not cross transport on the advisor path

> Pins the decided semantics of the overlay CLI name ambiguity
> (`llmroute.Overlay.CLI`): a BARE name is a FAMILY selector satisfied by the
> chain's existing same-family entry whatever its transport, a hyphen-QUALIFIED
> name is a DRIVER selector honored as written, and an exact chain entry
> outranks both. PR #390 fixed the contract-escalation instance (chain
> `[claude-p codex]`, overlay `codex` → was dispatching `codex-tmux`, exit=10
> on CI macOS without tmux); commit `797b8518` added the family rung for the
> ADVISOR instance (overlay `claude` over the same chain → was rewritten onto
> `claude-tmux`). What was still missing at cycle 1647 was (a) the decision
> stated at the PACKAGE boundary, where the ambiguity was the actual defect,
> and (b) proof through the production caller — `runner.resolveDispatchPlan`
> (`go/internal/phases/runner/routing.go:71`), the one place the advisor's
> `PhaseRequest.ModelRoutingCLI` reaches `ApplySoftOverlay` — rather than
> resolver-only unit coverage that a caller normalizing the name upstream
> would silently defeat. Source incident: inbox record
> `overlay-family-name-transport-ambiguity` (console 2026-07-30), materialized
> in cycle 1647 (fleet lane `cycle-cd3ae73e-1647`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| decision-at-package-boundary | package doc names family selector / driver selector / exact-entry precedence | 6/10 | `go test -tags acs -run TestC1647_001_ ./acs/cycle1647/` |
| bare-family-keeps-transport | runner dispatches `claude-p` for overlay `claude` over `[claude-p codex]`; `claude-tmux` never dispatched | 8/10 | `go test -tags acs -run TestC1647_002_ ./acs/cycle1647/` |
| explicit-driver-wins-soft | runner dispatches `claude-tmux` first for overlay `claude-tmux` over `[claude-p]`, then falls back to `claude-p` on exit 80 | 8/10 | `go test -tags acs -run TestC1647_003_ ./acs/cycle1647/` |
| advisor-path-covered-caller-named | focused runner test passes with the contract-escalation test; names `resolveDispatchPlan` / `routing.go`; distinguishes contract-escalation | 7/10 | `go test -tags acs -run TestC1647_004_ ./acs/cycle1647/` |
| race-and-apicover-clean | `-race` llmroute + router; apicover -enforce llmroute `0 uncovered, 0 false-green` | 6/10 | `go test -tags acs -run TestC1647_005_ ./acs/cycle1647/` |
| tracked-artifacts | predicate package + this eval are in the git index | 5/10 | `go test -tags acs -run TestC1647_006_ ./acs/cycle1647/` |
| inherited-packages-green | every go/acs/cycle* package the diff adds runs green (cycle1633, cycle1637, cycle1638 on this tree), one named package each | 7/10 | `go test -tags acs -run TestC1647_007_ ./acs/cycle1647/` |
| no-dead-line-citations | the explanation cites no `path:line` into a path the diff deletes | 5/10 | `go test -tags acs -run TestC1647_008_ ./acs/cycle1647/` |

The `go test -race ./internal/core` half of the inbox's fifth criterion is a
whole-suite shape the predicate lint bans (40s+ under fleet load, cycles
1173/1175/1178); it is delegated to CI's regression suite and to the Builder's
pasted run in `build-report.md`, which the Auditor checks by hand.

## Cycle-1647 audit round 1 — continuation hygiene (007-008)

Round 1 PASSED every task predicate (001-006) and FAILED the SHIPPING TREE:
`go/acs/cycle1638/predicates_test.go` resolved "this cycle's explanation" by a
`docs/explain/builds/cycle-1638-*.md` glob, but the host archives every
unshipped predecessor record on a continuation
(`explanationdocs.ArchiveUnpublishedContinuationRecords`), so
`TestC1638_010/011` — named as evidence by the tracked
`.evolve/evals/triage-unified-solution-synthesis.md` — were RED on any
continuation tree, and neither TDD nor Builder ran that package (H1). The
1647 explanation also regressed the two defects those predicates pin: it
omitted `docs/architecture/phase-registry.json` from `## Changed Areas` and
called this diff's own cycle-1638 package "inherited … preserved" (M1), and
cited the two root inbox records at `:1` although this diff deletes them
(correction 3). Reconciled at the TDD seam: the cycle-1638 helper now resolves
the ONE record the tree adds (from `git status`/`git log`, never a cycle
number), so 010/011 bind to whichever record ships; `inherited-packages-green`
makes the harness lane (`./acs/cycle1647` only) reach every inherited package
the diff adds; `no-dead-line-citations` pins the third correction. All three
go green with edits to the DOCUMENT only (probed at RED: naming the registry
file, dropping the provenance claim, citing the `consumed/` copies).

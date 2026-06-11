# Batch Failure Review — Coverage/Adversarial Campaign (cycles 277–284, 2026-06-10/11)

Operator-stopped after cycle 284 scout for this review. Goal under test: ≥98% behavior coverage
across all three LLM CLI driver families (claude/codex/agy) + adversarial fault-injection capability.

## Scoreboard

| Cycle | Verdict | Shipped | Failure class | Work fate |
|---|---|---|---|---|
| 277 | audit FAIL | — | amplification gap-tests tripped whole-suite predicate (`C277_007`) | **lost** (worktree destroyed) |
| 278–279 | — | — | burned numbers: two-session launch race | nothing produced |
| 280 | guard FATAL → batch exit | — | inserted-phase `worktree=""` + abort-cleanup destruction | **lost** except 2 leaked files |
| 281 | PASS | `02b778ef` | — | worktree-dispatch fix + adversarial suite (agy incl.) + 2 floors |
| 282 | guard FATAL → batch exit | — | same as 280 (binary unfixed mid-batch) | **salvaged** (reviewed; parked in `.evolve/operator-salvage/cycle-282-main-tree/`) |
| — | binary chore | `ec4de60e` | — | fixed binary deployed |
| 283 | build FAIL → batch continued | — | overpacked floors + codex quota wall | **preserved** (worktree alive; salvage copies taken) |
| 284 | sealed at scout | — | operator stop for this review | nothing lost |

## Root causes (5 distinct diseases)

### RC1 — Inserted-phase dispatch with empty worktree ✅ FIXED, verified in production
Advisor/plan-inserted write-capable phases (test-amplification) dispatched with `Worktree=""`;
their writes landed in the main tree; the phase-universal tree-diff guard (`7e0df0b5`) correctly
FAILed the cycle; `cmd_loop.go:374` treats a guard breach as non-recoverable → batch exit.
Killed cycles 280 and 282. Fixed in `02b778ef` (mint template `writes_source` propagation,
`phase_advisor.go`), live in binary `ec4de60e`. **Production-verified in 283**: build leaked 6 paths,
`build-leak-recover` relocated them into the worktree and the cycle continued.

### RC2 — Abort-cleanup destroyed uncommitted worktrees ✅ FIXED, verified
Guard-abort cleanup deleted `.evolve/worktrees/cycle-N` holding the entire top_n output
(asymmetric with the v16.8.0 ship-failure path which preserves). Destroyed 277, 280, 282 work.
Fixed in `02b778ef` (`orchestrator.go` preservation); **283's worktree survived its FAIL**.

### RC3 — Coverage-floor overpacking (OPEN — the recurring content failure)
Three consecutive cycles failed the same way:
- 280: 3 tasks; builder starved committed `cmd/evolve` task entirely ("turn budget")
- 282: 3 tasks; build PARTIAL — Tasks 2+3 unimplemented
- 283: 3 tasks (~12 package floors ≥98%); build FAIL — only Task 1 attempted, 91.5/100/89.9%

Observed sustainable throughput: **~1 task ≈ 4–5 package floors per builder turn-budget (25 turns)**
(cycle 281, the only PASS, was exactly that size). Compounder in 280: TDD wrote binding floor
predicates for tasks triage had *deferred*.

Filed: `.evolve/inbox/2026-06-11T00-45-00Z-coverage-floor-overpacking.json` (HIGH).
Fix direction: (a) triage caps committed floors at observed rolling throughput, defers the rest;
(b) TDD floor predicates restate ONLY committed-task floors; (c) stage floors (88→92→95→98)
instead of jumping to 98 everywhere.

### RC4 — The last coverage miles need seams, not more tests (OPEN — task-class mismatch)
283's builder correctly diagnosed its own wall: the remaining uncovered functions invoke real
external processes. Its pointers (verbatim, validated):
- `go/internal/swarm/dispatcher.go:62` — `Dispatch` 72%; needs a stub `WorkerProvisioner` to inject failures without real git
- `go/internal/swarm/kill.go:35` — `Kill` 75%; needs `ExecSessionKiller.KillTmux` func injection instead of real tmux
- `go/internal/phases/swarmrunner/swarmrunner.go:132` — `enforce` writer branch 0%; needs fake `MergeTrainDeps.Merger` returning `AllMerged:false`

"Write more unit tests" cannot reach ≥98% on these packages. The next cycle goal must authorize
**small DI-seam refactors** (production code) as part of coverage tasks — a different contract
than test-only cycles.

### RC5 — codex-tmux daily quota wall (OPEN — burned 283's wall-clock and turn budget)
`escalation-report.json` (00:28:07Z): `cli=codex-tmux, pattern_name=rate_limit, reason=escalate`.
Every codex-routed phase in 283 exited 85 (`ExitUnknownPrompt` family, `bridge/exitcodes.go:32`)
and fell back to claude — but each one first burned its boot + stall-detection window
(api-contract-design, architecture-design, test-amplification attempt 1; amplification attempt 2
then stalled 900s, `artifact_timeout`). The dispatch chain *worked* (claude completed each phase)
but the batch paid ~15 min/phase in dead codex time.

Fix direction: batch-scoped per-CLI circuit breaker — after N (=2?) `rate_limit` escalations from
one CLI family, demote that family for the remainder of the batch (or until a probe succeeds);
route directly to the fallback. The codex update-menu is NOT implicated (pinned via
`dismissed_version: 999.999.999`).

## Secondary findings

- **S1 — Phase agents run with cwd = main repo**, writing into worktrees only by prompt discipline.
  This is the structural root of the whole leak class (RC1 was its worst case; 283's builder leak
  its routine case). Counter-consideration: codex requires per-directory trust
  (`pretrustCodexProjects` exists for this). Structural fix: launch phase tmux sessions with
  `workdir = phase worktree` + pre-trust worktree paths at provisioning. Kills leak-recover churn.
- **S2 — Spine fail-open**: 283 build proceeded despite a missing mandatory predecessor handoff
  artifact (`[orchestrator] WARN spine not satisfied for next=build ... proceeding fail-open`).
  Known open defect class (dead spine gate); becomes dangerous once handoff artifacts gate evals.
- **S3 — tmux "no server running" in two escalation `final_pane`s** — RESOLVED (benign race):
  escalation mtimes (23:52Z, 00:43Z) fall mid-cycle while later phases ran fine; tmux's server
  auto-exits when its last session closes, so the watcher captured `final_pane` AFTER the bridge
  tore the session down. Not a server-stability problem, but the post-kill capture has zero
  diagnostic value — watchers should capture before teardown (rider on the workdir inbox item).
- **S4 — Test residue pollution**: a full nested repo copy at
  `go/internal/phases/swarmrunner/.evolve/worktrees/cycle-1-integration/` (gitignore-invisible).
  Some integration test created `.evolve/worktrees` relative to package cwd. Tests must use `t.TempDir()`.
- **S5 — gofmt-gate fix (`gofmt -s`) still unlanded after 4 attempts** (277 lost, 280 lost,
  282 starved, 283 not selected). Trivial; do interactively with the next binary rebuild.
- **S6 — Banked salvage**: 282's full output reviewed (code-simplifier: clean, 8/8 packages PASS,
  `gofmt -s` + `go vet -tags acs` clean) and parked at `.evolve/operator-salvage/cycle-282-main-tree/`
  (+ operator copies in `~/ai/claude/evolve-salvage/`). 283 partials in `evolve-salvage/cycle-283-final-*`.
  Re-landing banks ~8 package floors without spending any LLM budget.

## Recommended execution order

1. **Bank the wins (no LLM cost)**: land 282 salvage + 283 swarm partials + gofmt-gate fix
   interactively (tests + dual-review already done for 282) → one `/commit`-class ship + binary rebuild.
2. **Bound the planner**: keep `coverage-floor-overpacking` inbox HIGH; next batch goal text should
   set per-cycle scope explicitly ("one coverage task per cycle").
3. **Authorize seams**: next batch goal explicitly permits DI-seam refactors for RC4's three pointers.
4. **codex breaker**: implement batch-scoped CLI demotion after rate_limit escalation (RC5) —
   medium Go task, good single-cycle candidate.
5. **Structural workdir fix (S1)**: worktree-workdir + codex pretrust — design-review first
   (touches bridge launch contract for every driver).
6. **Hygiene**: delete S4 residue + file t.TempDir defect; verify S3 tmux-server timeline.

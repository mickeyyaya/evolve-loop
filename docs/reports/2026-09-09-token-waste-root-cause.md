# Token waste investigation after operator pause

The operator stopped the live batch on 2026-09-09 at approximately 06:50 UTC.
Cycles 1619–1621 were attempted; zero useful changes landed on main and zero
verification waves completed. The parent returned `stop_reason: signal` and
`cycles: null`. Cycle 1620 made a local candidate commit and rebased it to
`31082b6836b491d994de7223ff3975147800e661`; that commit is not contained in main.
Preserved candidates are salvage inputs, not successful deliveries.

No further live cycles or reviewer agents were launched for this investigation.
The local investigation used existing artifacts, source inspection and one Go
regression. Production fixes below remain pending; the loop must stay paused.

## Issue / gap / solution

### 1. Cancellation does not reliably stop provider sessions

**Issue:** after parent 83716 and all three cycle children exited, their dedicated
tmux server still contained three phase sessions and three readiness-probe
sessions. Stopping the orchestrator was insufficient to stop all providers.
The operator explicitly killed only this batch's server, `evolve-bridge-p83716`.
Subsequent process and tmux checks confirmed it was gone. Worktrees and reports
were preserved.

**Gap:** `go/internal/bridge/driver_tmux_repl.go:179` defers `tmuxCleanup` with
the original context. `go/internal/bridge/tmux_session.go:106` uses that context
for both liveness and kill commands. A canceled context prevents those commands
from executing. This is a confirmed cleanup defect, not a complete explanation
of every surviving readiness session or abrupt process-termination path.

**Evidence:** `/tmp/recovery-pause-red.log` records the focused Go overlay test:
the live-context control passes; canceled-context cleanup fails with
`owned provider session survives cleanup after cancellation`. No provider was
launched by this test. Existing cancellation tests check verdict/log retention,
which does not establish provider-process termination.

**Required correction:** use a separately bounded cleanup context; ensure batch
termination reaps exactly its registered provider sessions even when children
exit abruptly. Test real subprocess termination and isolation from other runs.

### 2. Known missing optional phases trigger expensive learning

**Issue:** cycle 1619 spent 344.692 seconds in retrospective for a missing
`defect-disposition-preflight` persona. Cycle 1620 spent 311.200 seconds for a
missing `pre-audit-evidence-check` persona. Both then correctly skipped the phase.
That is 10.9 combined lane-minutes spent on two deterministically known absences.

**Gap:** `go/internal/core/cyclerun_dispatch.go:286` calls
`recordFailureLearning` before recording the admitted optional skip.
`go/internal/core/failure_learning.go:386` synchronously runs the retrospective
agent. The earlier skip repair fixed the terminal verdict but retained this spend.

**Required correction:** preserve deterministic diagnostics/learning for known
optional configuration absences without invoking an LLM. Prevent unavailable
personas from entering selectable plans. Keep mandatory/floor failures explicit.

### 3. Base disagreement forces another Build/Audit after Ship

**Issue:** fresh cycle 1620 began on `98d85b0e`, while the actual local landing
branch remained `68e305b6`. Continuation lanes used the latter. Ship created a
candidate commit, failed fast-forward, rebased, then dispatched Build again.
The base difference was twelve cycle-dossier files, with no Go source delta.

**Gap:** `go/cmd/evolve/cmd_loop_wavesync.go:72` treats successful
`git merge --ff-only origin/main` as proof HEAD moved. When local HEAD is ahead,
Git can succeed without moving it; line 80 nevertheless reports a fast-forward.
The live transcript contains that misleading message. Separately,
`go/internal/core/ship_recovery.go:97` deliberately routes a rebased explanation
through Build. That preserves binding, but amplifies avoidable base disagreement.
The exact fresh-lane base-selection path needs a composed regression before a fix.

**Required correction:** resolve and verify the actual integration HEAD before
dispatch, and bind fresh lanes, continuations and landing to that authority.
Test local-ahead, remote-ahead and divergence. Do not bypass composed-tree Audit.

### 4. Tests and handoffs do not establish the selected feature

**Issue:** cycle 1619's green tests checked the majority Markdown report, while
its auditor reproduced downstream JSON retaining the last, minority draw.
Cycle 1621 reduced unified planning to an `AdmittedIDs` producer and explicitly
deferred unified execution and stale-plan invalidation. It then spent 1,545
seconds on its first Build correction for missing explanation coverage; another
correction exposed inherited API execution-coverage failures.

Cycle 1620's auditor reproduced a complete declaration falsely rejected after an
unrelated JSON fence, despite nine passing predicates. Its WARN was ship-eligible
under the configured non-strict policy, contrary to the report's prose assertion
that WARN blocks shipping. No policy bypass was needed for the attempted Ship.

**Gap:** helper/parser tests and structural checks were treated as end-to-end
feature evidence. Inherited work enlarged the validation surface. Late handoff
failures restarted expensive general-purpose phases. The operator also kept
retrying full fleet waves after infrastructure repairs without first establishing
one complete, useful landing; that was a poor validation sequence.

**Required correction:** salvage existing work, freeze the original acceptance
criteria, and test the actual downstream consumer with adversarial ordering and
positive controls. Run deterministic handoff checks before declaring Build done.
Distinguish report correction, semantic repair and base reconciliation. Treat
partial implementation as partial, regardless of a green predicate count.

## Measured usage and limits

The 24 available phase-usage files sum to 266,174 output tokens, 32,834,969
cache-read tokens, 736,978 cache-write tokens and 526 input tokens. These are
recorded counters, not verified billing or unique context tokens; cache reads
accumulate across requests. Codex/Agy usage is explicitly unmeasured, and some
interrupted/repeated attempts lack complete usage files. The displayed zero
dollars is not a cost measurement. Model substitution alone cannot fix retries
or incomplete telemetry.

## Model configuration applied while paused

Native `evolve setup detect --json` verifies Claude deep/top = `opus` and Codex
deep/top = `gpt-5.6-sol`. The Codex change uses the existing runtime operator
override `.evolve/bridge-manifests/codex-tmux.json`, including its subscription
fallback default. Fast/balanced tiers are unchanged. The current Codex catalog
entry is detect-sourced and does not override dispatch; automatic refresh is off.
This is local runtime configuration, not a change to the embedded repository
defaults. Evidence: `/tmp/recovery-model-change-detect.json`.

## Recovery order

1. Repair and test cancellation/owned-session cleanup locally.
2. Remove LLM work for known optional absences; align actual integration bases.
3. Salvage one candidate and prove all original acceptance criteria through its
   consumers. Repair its discovered defects before claiming completion.
4. Obtain bounded architecture, Go and security reviews for those fixes; run
   targeted regressions and required repository gates, then land through the
   sanctioned commit/ship path.
5. After the pause is lifted, verify one complete lane before returning to the
   requested two fleet waves. Record actual main landings and complete usage.

No wave restart, production repair, or completion claim is authorized by this
report itself. The current user instruction is to remain paused and investigate.

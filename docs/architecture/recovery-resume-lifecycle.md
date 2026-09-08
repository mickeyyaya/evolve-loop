# Resume lifecycle recovery

## Issue

The documentation review on 2026-09-09 found that `RunCycleFromPhase` could
return a successful result without the terminal lifecycle used by `RunCycle`.
Resumed cycles skipped the verdict-coherence and lost-landing checks, durable
cycle dossiers, continuation snapshots, and carryover accounting. Its separate
32-iteration loop could exhaust its bound and still report success.

Resume also lost difficulty-based build timeouts and recurrence-based model
escalation. After a second interruption the existing quota checkpoint still
pointed to the original phase. The CLI treated quota exhaustion during resume
as a terminal error instead of returning its existing resumable exit code 5.

## Gap in the previous verification

The resume tests covered phase dispatch, worktree and explanation identity,
audit binding, and some repair-round behavior. They did not compare terminal
outcomes or durable checkpoint advancement across the fresh and resumed
entrypoints. High-level success assertions accepted a cycle without its dossier.

The ordinary phase-boundary checkpoint writer deliberately yields to an existing
escalation checkpoint. Calling it during resume would therefore leave the old
quota pause intact. The test suite also lacked the real sequence of consuming
a quota checkpoint, completing Build, interrupting Audit, and resuming again.

The host cycle number has two meanings: the last completed cycle and the latest
allocated cycle. A quota pause does not complete a cycle. Singleton resume must
validate against the host allocation cursor as well as the completed cursor;
fleet checkpoints continue to derive their authoritative identity from the
per-run path. A rewritten checkpoint cannot select an arbitrary cycle.

## Solution

Both entrypoints now use `cycleRun.completeCycle`, which delegates to the
existing finalizer and dossier writer. This keeps verdict reconciliation,
continuation production, carryover merging, and closeout recording together.
Resume keeps the original cycle and run identity; it never allocates a new
cycle. The completed and allocated host cursors remain monotonic when an older
fleet lane finishes after a newer lane.

Terminal resumed errors use the existing abnormal-exit epilogue. Quota pauses
retain their typed phase evidence and checkpoint without publishing a terminal
FAIL dossier, on both entrypoints. Interrupted resumed work stays resumable.
Exhausting the configured phase-iteration bound returns an explicit failure.
Successful resume persists an `end` marker; checkpoint discovery and direct
resume both reject a completed run instead of dispatching it again.

The host floor verdict is persisted with each completed phase and restored on
resume. A post-audit Retro or Memo cannot replace an earlier FAIL or WARN with
the default PASS. Legacy checkpoints use recorded host failure reasons or the
last completed authoritative phase's timing verdict. Missing evidence retains
WARN until a new authoritative phase runs, with an explicit diagnostic; agent
report prose cannot supply this fallback.

Fresh cycles persist their original pre-cycle HEAD for closeout; resume restores
that baseline so a pause after Ship does not erase observed throughput. Legacy
checkpoints without that baseline use resume-entry HEAD and warn that earlier
ship throughput cannot be reconstructed. The worktree base is never substituted
for the closeout baseline. Throughput skips a cycle already represented in the persisted window;
carryover continues to use its existing ID-based merge. Existing worktree and
ship integrity checks remain authoritative.

This preserves the existing HEAD-based accounting heuristic; concurrent sibling
commits are not proof of a particular lane's ship. The separate lost-landing
floor consults run-owned ship artifacts, and the live-wave evaluation inspects
each lane's actual commit rather than treating throughput as ship authority.

Fresh and resumed requests share `applyDispatchPolicy`: recurrence escalation,
repair-round escalation, and difficulty-conditioned build budgets use the same
profile envelopes and persisted triage artifacts. Resume restores lane scope
from the existing lane-scope artifact. The host audit dispatch counter travels
as `PhaseRequest.AuditRound` to Audit and Ship.

Fresh cycle state persists the original goal hash and text. Resume restores
that context before generating phase prompts or a dossier, and rejects a
conflicting replacement goal. Legacy checkpoints use their own lane pin and,
where available, a matching prior dossier; a singleton may use its host batch
goal. A legacy checkpoint with no recoverable goal gets an explicitly labeled
descriptive goal and warning, rather than silently borrowing a sibling's task.

The checkpoint package supplies a resume-specific writer through the existing
hook pattern. After validating the checkpoint and acquiring the normal storage
lock, resume writes a current-phase checkpoint before each dispatch. This
consumes the old escalation under the same sidecar lock, preserves the integrity
chain, and leaves a discoverable `operator-requested` recovery checkpoint.
Fresh leases exclude a running fleet lane from discovery. Quota exhaustion
replaces that checkpoint with the canonical quota reason and reset-time hint.
The CLI shares its quota-pause reporting path and returns 5 on either entrypoint.

Quota classification is an explicit retry hook for fresh and resumed serial
dispatch, whose callers own the durable pause. Parallel evaluation retains its
existing optional-warning and mandatory-error behavior; it cannot advertise a
quota pause without a partial-batch recovery checkpoint. Legacy singleton goal
resolution also checks its persisted batch hash before accepting caller input,
so a known original task cannot be replaced through a missing checkpoint field.

## Verification

`go/internal/core/resume_lifecycle_test.go` exercises durable closeout, original
identity and goal preservation, goal conflict rejection, explicit iteration
failure, lost landing, failed-work continuation, carryover and throughput
idempotency, host cursor monotonicity, persisted lane scope, request budgets,
audit-round propagation, and the nonterminal quota path.

`go/internal/checkpoint/resume_progress_test.go` uses the real filesystem storage
and checkpoint producer. It pauses at Build, completes Build, interrupts Audit,
loads the new Audit checkpoint, resumes successfully without repeating Build,
and verifies that the completed checkpoint cannot be resurrected.

`go/cmd/evolve/cmd_loop_resume_quota_test.go` runs the actual CLI orchestration
with real state/checkpoint storage and external-agent substitutes. It verifies
exit code 5, the quota-pause result, checkpoint retention, and absence of a false
terminal dossier. The tmux process boundary is stubbed to isolate the host.

The existing rebase recovery test now materializes its memory storage's
checkpoint file, as the production caller already does; its original Build and
base-identity assertions remain unchanged.

Run the recovery and preservation suite from `go/`:

```sh
go test -tags integration ./internal/core ./internal/checkpoint ./internal/adapters/storage ./internal/cyclestate ./cmd/evolve -run 'TestResume|TestRunCycleFromPhase|TestLoadResume|TestActivateResume|TestFinalize|TestPersistCycleEndState|TestRunCycle_AllFamilies|TestRetry|TestPhaseBoundary|TestCycleState|TestAbnormalEpilogue|TestRunLoop_(Resume|QuotaPause)' -count=1
```

## Boundaries

Checkpoint recovery provides durable phase boundaries, not exactly-once external
agent execution. An interruption inside a phase can repeat that phase; its ship
and audit bindings still govern acceptance. Dossier persistence retains the
existing best-effort warning behavior after terminal state accounting succeeds.
A malformed or unwritable resume checkpoint fails before launching the next
phase. Legacy checkpoints without a persisted original human goal cannot
reconstruct text that was never recorded.

These tests establish lifecycle and policy behavior with deterministic agent
substitutes. They do not establish a real-provider ship-rate improvement; that
requires the separately authorized bounded verification waves.

# Comment history: `internal/checkpoint`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## cycle-1853 secondary defects (pane loss, quota evidence)

### `go/internal/checkpoint/quota_wakeat_test.go:3` — above `import (`

```text
// quota_wakeat_test.go — RED contract for the inbox defect
// `quota-pause-wakeat-unpopulated`.
//
// The all-families-exhausted checkpointer (core.QuotaBoundaryCheckpointer, the
// cycle-656 seam) wrote ReasonQuotaLikely with QuotaResetAt/QuotaResetSource left
// EMPTY. cmd_loop then printed `QUOTA-PAUSE: … wake-at= source=unknown`, and
// skills/loop/SKILL.md instructs the operator model to parse wake-at=ISO8601 and
// compute its ScheduleWakeup delay from it — arithmetic that could never run on
// the Go path. Auto-resume was structurally dead for every quota wall.
//
// FIX CONTRACT: the checkpointer populates both fields at write time from
// quotareset.Compute — the package that already owns the source-priority chain
// (operator override > workspace hint file > now + default hours) — so the
// wake-at is NEVER empty and the source always says where it came from.
//
// ADVERSARIAL DIVERSITY: one test per source arm (override / parsed hint /
// estimate fallback) plus a negative that the sibling phase-complete
// checkpointer does NOT grow a fabricated wake-at — only the quota wall has a
// reset instant, and stamping one on every phase boundary would invent data.
```

## quota-resume-cap-and-boot-wall

### `go/internal/checkpoint/checkpoint.go:203` — above `core.QuotaBoundaryCheckpointer = func(cs core.CycleState, projectRoot string, now time.Time) error {`

```text
// Cycle-656: the mid-cycle all-families-exhausted detector needs to write
// a quota-likely checkpoint from inside core's dispatch seam. quota-likely
// IS an escalation reason, so unlike the phase-complete hook below it
// writes unconditionally (still carrying the recorded integrity chain and
// holding the same sidecar lock). detectQuotaPause / `evolve loop --resume`
// consume it — including the wake-at withQuotaReset stamps on.
```

### `go/internal/checkpoint/checkpoint.go:224` — above `return flock.WithPathLock(cycleStatePath, func() error {`

```text
// ADR-0049 N17: check-and-act under ONE lock. The escalation guard read
// used to be UNLOCKED and the phase-complete write took the cycle-state
// lock SEPARATELY — a TOCTOU. Under fleet mode concurrent cycles share the
// host-global cycle-state.json, so a peer could write an escalation
// checkpoint in the window between our read and our write, which this
// lowest-priority phase-complete write then clobbered (lost escalation).
// Folding the read + write into a single flock.WithPathLock critical
// section closes the window: both serialized orders converge on the
// escalation. applyWithHooks (the lock-free inner of ApplyToStateFile) is
// called DIRECTLY — flock is non-reentrant, so re-calling ApplyToStateFile
// here would self-deadlock on the same sidecar lock.
```

### `go/internal/checkpoint/checkpoint.go:247` — above `existing := readExistingIntegrity(cycleStatePath)`

```text
// ADR-0065: CARRY the recorded per-phase integrity chain through the
// phase-complete write (ComposeWithIntegrity), else this Compose would
// drop it (omitempty nil clobbers the on-disk phaseIntegrity). Read +
// write under the one lock — no TOCTOU with a peer's append.
```

## comment-zero h2

### `go/internal/checkpoint/checkpoint.go:68` — above `PhaseIntegrity []phaseblock.Digest 'json:"phaseIntegrity,omitempty"'`

```text
// PhaseIntegrity is the per-phase agent-block digest chain (ADR-0065).
// Additive + omitempty: absent on pre-field checkpoints, so old state
// round-trips byte-clean and plain Compose stays integrity-free.
```

### `go/internal/checkpoint/checkpoint.go:101` — above `func ComposeWithIntegrity(cs core.CycleState, reason Reason, cost float64, gitHead string, now time.Time, integrity []ph…`

```text
// ComposeWithIntegrity is Compose plus the per-phase integrity chain (ADR-0065),
// so a phase-complete checkpoint write carries (never clobbers) the recorded
// chain. Pure; no I/O. Plain Compose stays integrity-free for back-compat.
```

### `go/internal/checkpoint/checkpoint.go:142` — above `func ApplyToStateFile(path string, cp Checkpoint) error {`

```text
// ApplyToStateFile reads a cycle-state.json, splices the checkpoint
// block into it (preserving every existing field), and atomically
// writes back. Mirrors bash cycle_state_checkpoint at cycle-state.sh:479.
//
// ADR-0049 G7: the read-modify-write holds the SAME cycle-state.json sidecar
// lock storage.WriteCycleState holds, so a concurrent fleet cycle's phase write
// (which owns "phase" and preserves "checkpoint") and this checkpoint write
// (which owns "checkpoint" and preserves "phase") serialize instead of clobbering
// each other's key. flock.WithPathLock is the single home for the "<file>.lock"
// convention shared across packages.
```

### `go/internal/checkpoint/checkpoint.go:294` — above `func hasEscalationCheckpoint(path string) (bool, error) {`

```text
// hasEscalationCheckpoint reports whether path already holds a checkpoint
// block with a canonical reason other than phase-complete. It reads WITHOUT
// taking the cycle-state lock, so the caller MUST already hold
// flock.WithPathLock(path) (ADR-0049 N17) — otherwise the check and the
// caller's subsequent write straddle the lock and a peer escalation is lost.
```

### `go/internal/checkpoint/checkpoint_test.go:148` — above `missing := filepath.Join(t.TempDir(), core.CycleStateFile)`

```text
// A missing cycle-state.json inside an existing (writable) dir — the realistic
// precondition. ADR-0049 G7 wraps the apply in the cycle-state sidecar lock,
// which creates "<file>.lock" in the (present) dir, then the read of the absent
// data file surfaces ErrNotExist. (Production always calls under .evolve/, so a
// nonexistent parent dir is not a real path.)
```

### `go/internal/checkpoint/cyclestate_concurrency_test.go:17` — above `func TestCycleState_ConcurrentWriters_BothUpdatesSurvive(t *testing.T) {`

```text
// TestCycleState_ConcurrentWriters_BothUpdatesSurvive is the G7 (ADR-0049)
// regression. Two writers read-modify-write the SAME cycle-state.json:
//   - the phase writer (storage.WriteCycleState) owns "phase" and preserves the
//     "checkpoint" block it reads;
//   - the checkpoint writer (checkpoint.ApplyToStateFile) owns "checkpoint" and
//     preserves the "phase" it reads.
//
// Under fleet mode the whole-cycle project lock is skipped, so the two run
// concurrently. Without a SHARED per-file lock, one writer renames over a stale
// read and the peer's update is silently reverted (lost update). Both serialized
// orders yield the SAME outcome — phase=audit AND checkpoint.gitHead=CP1 (both
// updates applied) — so any other result is a lost update. A start barrier makes
// the two RMWs overlap, so the bug trips reliably across the iterations.
```

### `go/internal/checkpoint/integrity.go:107` — above `func decodeIntegrity(h hooks, v any) []phaseblock.Digest {`

```text
// decodeIntegrity reads the existing phaseIntegrity (an untyped map slice) back
// into typed digests. A missing/garbled value yields an empty chain rather than
// an error — capture is best-effort and never blocks the cycle. The
// tamper-evident record of record is the append-only hash-chained LEDGER anchor
// (the second SSOT, ADR-0065); a security consumer verifies chain continuity
// there, so a reset of this resume-facing copy cannot silently erase the truth.
```

### `go/internal/checkpoint/phaseboundary_concurrency_test.go:12` — above `func TestPhaseBoundaryCheckpointer_ConcurrentEscalation_NotClobbered(t *testing.T) {`

```text
// TestPhaseBoundaryCheckpointer_ConcurrentEscalation_NotClobbered is the N17
// (ADR-0049) regression. PhaseBoundaryCheckpointer used to read the escalation
// guard UNLOCKED (hasEscalationCheckpoint) and then write phase-complete under a
// SEPARATE lock (ApplyToStateFile) — a check-and-act straddling the lock
// boundary. In fleet mode concurrent cycles share the host-global
// cycle-state.json, so in the TOCTOU window a peer could write an escalation
// checkpoint (quota-likely / batch-cap-near / operator-requested / stall) that
// this lowest-priority phase-complete write then clobbered — a lost escalation
// the consumer (e.g. detectQuotaPause after RunCycle) never sees.
//
// Folding the check+act under ONE flock.WithPathLock makes BOTH serialized
// orders converge on the escalation:
//   - peer writes escalation first -> our guard reads it under the lock -> yields
//   - we write phase-complete first -> peer's escalation lands after -> survives
//
// so the final reason is ALWAYS the escalation. A start barrier forces the two
// to overlap, so the pre-fix lost-update trips across the iterations.
```

### `go/internal/checkpoint/phaseboundary_test.go:1` — above `package checkpoint`

```text
// phaseboundary_test.go — cycle-234 task `phase-boundary-checkpoint` (RED).
//
// AC1: a new ReasonPhaseComplete constant joins the canonical reason set so
// the orchestrator can write a checkpoint block at EVERY phase boundary
// (campaign retro Invariant 3 — three failed `--resume` attempts in the
// 215-231 campaign because checkpoints existed only at the quota wall).
//
// These tests are authored by the TDD-Engineer BEFORE the constant exists:
// the compile error "undefined: ReasonPhaseComplete" is the intended RED
// signal. Builder adds the constant + IsValid arm; do NOT modify the tests.
```

### `go/internal/checkpoint/phaseboundary_test.go:69` — above `func TestPhaseBoundaryCheckpointer_YieldsToEscalation(t *testing.T) {`

```text
// TestPhaseBoundaryCheckpointer_YieldsToEscalation pins the overwrite policy
// (found landing cycle-234/236 work: the production wiring of the hook broke
// TestRunLoop_QuotaPause_Rc5 because a routine phase-complete checkpoint
// clobbered the seeded quota-likely block before detectQuotaPause read it).
// phase-complete is the LOWEST-priority reason: the hook must never overwrite
// an existing escalation checkpoint (quota-likely, batch-cap-near,
// operator-requested, stall-inactivity) — those must survive until consumed.
```

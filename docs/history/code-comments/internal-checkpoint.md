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

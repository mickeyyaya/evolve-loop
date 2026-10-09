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

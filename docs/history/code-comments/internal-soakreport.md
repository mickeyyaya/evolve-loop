# Comment history: `internal/soakreport`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/soakreport/soakreport.go:1` — above `package soakreport`

```text
// Package soakreport is the R8.3 soak evidence reporter — the read-only
// instrument the EVOLVE_PHASE_RECOVERY enforce flip (plan R8.5) is gated on.
//
// Every ADR-0044/0045 component already leaves DURABLE shadow records:
// interaction ledgers carry C2 fatal_pane_shadow, I2 salvage-ladder, I3
// kernel_answer, and I4 rule_shadow_fire outcomes; the phase observer's
// events ndjson carries the C4/C3 INCIDENT envelopes with their
// policy-decided action/action_reason. This package aggregates them per
// batch and renders the per-component evidence table against the plan §6
// bars. It never mutates anything — the sweep (cmd_loop) and the flip
// (operator) act; the reporter only accounts.
```

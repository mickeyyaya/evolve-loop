# Comment history: `acs/cycle1112`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1112/predicates_test.go:3` — above `package cycle1112`

```text
// Package cycle1112 materialises the cycle-1112 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - exhaustion-regex-drift-failloud → arm the drift alarm for codex-tmux and
//     agy-tmux by adding controls.usage.drift_probe_regex to their manifests,
//     plus per-CLI regression coverage in exhaustion_drift_test.go.
//
// Why this is a real gap, not a cosmetic one. warnExhaustionRegexDrift
// (go/internal/bridge/exhaustion_drift.go) is generic and fail-OPEN: it keys
// entirely off manifestDriftProbePattern(cli) and returns silently when that is
// empty. claude-tmux carries a broadened drift_probe_regex; codex-tmux and
// agy-tmux do not, so a wording drift in THEIR exhausted_regex degrades to "not
// exhausted" with no diagnostic at all — the exact 8-cycle silent burn the
// watcher exists to prevent (exhaustion_drift.go header; the
// audit_quota_wording_drift incident family).
//
// Predicate strategy — each predicate COMPILES and EVALUATES the shipped regexes
// against wall/benign pane corpora, or shells the real unit suite; none is a
// source-grep of production code (the cycle-85 degenerate-predicate ban). The
// cheapest gaming fake — copy-pasting exhausted_regex verbatim into
// drift_probe_regex — satisfies "field is non-empty" but is killed by 001 (no
// drift-detectable gap) and by 004 (verbatim-copy check).
//
// RED now: neither manifest has the field, so manifest lookup returns "" and
// 001/004 fail at the presence gate; 002's per-CLI unit cases do not exist yet.
```

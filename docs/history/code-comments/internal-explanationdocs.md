# Comment history: `internal/explanationdocs`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/explanationdocs/activation_belt.go:10` — above `func CrossCheckActivation(typed CycleBinding) (bool, error) {`

```text
// CrossCheckActivation is the single home of the activation belt (architecture
// review 2026-09-01, HIGH): CycleBinding.ContractVersion==0 encodes BOTH
// "legacy cycle" and "a caller dropped the field", and only the host
// activation marker can tell them apart. Ship carried this cross-check inline
// while audit had none — a zero version silently disabled audit's explanation
// gate. Both gates now consult the host here.
//
// Host active: every typed field must equal the host's; any mismatch —
// including a zero ContractVersion against an active host — errors loudly.
// Host inactive: (false, nil) ONLY when typed.ContractVersion==0 — genuine
// legacy, and the host agrees. A non-zero typed version with no matching
// host activation is a stale or foreign identity and errors.
//
// The marker cycle prefers the one derived from a run workspace path
// (.evolve/runs/cycle-N) over the typed field, exactly as ship always did —
// the workspace is host-provisioned and harder to mistype than an int field.
```

### `go/internal/explanationdocs/activation_belt.go:81` — above `func sameWorkspacePath(a, b string) bool {`

```text
// sameWorkspacePath is the ONE workspace-equality belief, shared by the belt
// and resolveActivation (the 2026-09-01 re-review found the two comparing
// differently: Clean vs raw — a trailing separator passed one and failed the
// other 30 lines later).
```

### `go/internal/explanationdocs/explanationdocs_test.go:980` — above `func TestMaterialPaths_RuntimeStateIsNeverMaterial(t *testing.T) {`

```text
// Adversarial takeover finding (PR #517 e2e diagnosis): changedSince counts
// UNTRACKED files, and the orchestrator symlinks .evolve/cycle-state.json into
// every cycle worktree. On a host whose repo lacks the production .gitignore's
// `.evolve/*` rule (the e2e temp projects; any fresh operator checkout), that
// runtime-state symlink read as a MATERIAL change — forcing Status: REQUIRED
// with a full cycle document for builds that changed no code at all, and
// hanging every e2e pipeline cycle in the correction ladder until timeout.
// Runtime state is not a material code change; the classifier must say so
// itself rather than relying on every host's ignore hygiene.
```

### `go/internal/explanationdocs/gitio.go:552` — above `func validRelative(path string) bool {`

```text
// validRelative projects the single-sourced path-safety belief from reportdoc
// (architecture review 2026-09-01: this package's local validator accepted
// uncleaned traversal, colons, and NUL bytes that the citation validator
// rejects — the same string was judged safe here and unsafe there).
```

### `go/internal/explanationdocs/pathvalidity_ssot_test.go:17` — above `{"a/../../b", false},`

```text
// The three divergence cases caught by architecture review 2026-09-01:
```

### `go/internal/explanationdocs/rebind.go:23` — above `func RebindIdenticalRebase(ctx context.Context, binding CycleBinding, newBaseSHA string, persist func() error) (bool, er…`

```text
// RebindIdenticalRebase moves an approved Build contract to newBaseSHA without a new Build when the host
// proves the change it explains is byte-identical on that base (ADR-0105). It returns (false, nil) and
// writes nothing when the change is not provably identical, and (true, nil) once the result snapshot,
// the commit point, is written.
```

### `go/internal/explanationdocs/reviewcontract.go:12` — above `type Review struct {`

```text
// Review is what a reviewed explanation handoff yields. Since 2026-09-13
// (ADR-0102, operator decision) the reviewer's REASONING is the gate and the
// section's SHAPE is advisory: Status is the judgment as written (VERIFIED or
// NEEDS_CORRECTION; "" when the reviewer used another word), Advisories are
// the shape findings — the status enum, the build-status echo, the
// required/not_applicable document match, the path:line citations — which
// the caller reports on the phase record and which never touch a verdict.
```

### `go/internal/explanationdocs/reviewcontract.go:41` — above `func ValidateReviewedHandoff(ctx context.Context, fields map[string]string, view *phaseio.ExplanationView, worktree, bas…`

```text
// ValidateReviewedHandoff checks the phase-agnostic core of an explanation
// documentation review section against the Build handoff view and returns
// the Review; each caller applies its own phase policy to it. This is the
// single home of the contract — the audit and retro gates previously each
// restated it and had nothing binding the copies (architecture review
// 2026-09-01). The error is reserved for host-side defects no reviewer wrote
// — a missing view, an unknown handoff status — which must fail loudly here
// rather than fall through both phase gates silently.
```

### `go/internal/explanationdocs/reviewcontract_test.go:11` — above `func TestValidateReviewedHandoff_ContractCore(t *testing.T) {`

```text
// TestValidateReviewedHandoff_ContractCore names the single home of the
// review contract the audit and retro gates both delegate to (architecture
// review 2026-09-01). Since 2026-09-13 (ADR-0102, operator decision) the
// reviewer's REASONING is the gate and the section's SHAPE is advisory: the
// status enum, the build-status echo, the required/not_applicable document
// match and the path:line citations are reported as advisories on the phase
// record, never as a blocking error. The reviewer's own NEEDS_CORRECTION is a
// judgment, not a shape finding. The one loud error left is a host-side
// defect — an unknown handoff status, a missing view — which no reviewer
// wrote and which must not pass through both phase gates silently.
```

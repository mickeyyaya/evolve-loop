# Comment history: `internal/core/failurediag`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/core/failurediag/delivery_test.go:3` — above `import (`

```text
// delivery_test.go — unit 02 (ADR-0103, design decomposition/02-failure-diagnostics.md):
// the delivery-failure classifier, pinned in the unit with an injected
// timeout predicate (the unit never imports core's sentinel). RED first.
```

### `go/internal/core/failurediag/diag.go:1` — above `package failurediag`

```text
// Package failurediag is unit 02 of the component breakdown (ADR-0103): the
// phase failure diagnostic — the <phase>-failure-diag.json sidecar written
// when a mandatory phase aborts — and the delivery-failure classifier the
// sidecar and the retro relaunch key on. The orchestrator injects its clock
// and its artifact-timeout gate (a predicate, so the unit never imports the
// core sentinel); the unit's own failure mode is a failurediag.warning signal
// under module failurediag. Design:
// docs/architecture/decomposition/02-failure-diagnostics.md.
```

### `go/internal/core/failurediag/diag.go:100` — above `data, _ := json.Marshal(w.diagnose(phase, cycle, phaseErr, attempts))`

```text
// Strings and ints only: encoding/json has no error path for this shape
// (it fails on unsupported types, non-finite floats, cycles and Marshaler
// errors), so the pre-extraction marshal branch could never fire and is
// gone rather than excused (ADR-0103 item 3). A float field added later
// must bring the branch back with a code (unit 01's precedent).
```

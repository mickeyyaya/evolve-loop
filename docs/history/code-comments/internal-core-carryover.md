# Comment history: `internal/core/carryover`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/internal/core/carryover/carryover.go:1` — above `package carryover`

```text
// Package carryover is unit 03 of the component breakdown (ADR-0103): the
// lifecycle of state.json's carryoverTodos[] — the memory the next cycle's
// planner reads. One Lifecycle owns mint admission (fingerprint twin →
// refresh, id twin → skip, else append), the cycle-closeout readers (the memo
// and prescription documents, the triage-dropped retirement) in their
// load-bearing order, the ship-time retirement rule and the state.json persist
// behind two ports at the point of use. Every stamp arrives computed (the
// producers own their TTLs); the unit holds only the Signal Center accessor
// and reports its failure modes as carryover.warning under module carryover.
// Design: docs/architecture/decomposition/03-carryover-lifecycle.md.
```

### `go/internal/core/carryover/carryover.go:39` — above `PrescriptionPrefix = defectledger.PrescriptionPrefix`

```text
// PrescriptionPrefix tags the defect-ledger rows an audit's WARN
// prescriptions carry — the LEDGER's belief, projected from its home
// (internal/core/defectledger, ADR-0103 unit 09).
```

### `go/internal/core/carryover/closeout.go:9` — above `func (l *Lifecycle) Closeout(state *cyclestate.State, workspace string, cycle int, now time.Time) {`

```text
// Closeout is the cycle-terminal order, encoded once: the memo merge, then
// the prescription merge, then the triage-dropped retirement — retiring
// BEFORE the merges would let a merge resurrect the very id triage just
// dropped (the cycle-1538 reproduction). The caller persists afterwards.
```

### `go/internal/core/carryover/defectledger_projection_test.go:12` — above `func TestCarryover_ProjectsTheLedgerVocabulary(t *testing.T) {`

```text
// Test 47 (ADR-0103 unit 09) — the consumer pin of the ledger's vocabulary:
// PrescriptionPrefix projects the leaf's; MergePrescriptions over a ledger the
// leaf wrote keeps exactly the OPEN + prefixed rows, skips a FIXED
// prescription and an ordinary defect, and STILL tolerates a padded " OPEN"
// (preserved quirk Q3 — the lifecycle's TrimSpace compare).
```

### `go/internal/core/carryover/identity.go:10` — above `var cycleTokenRE = regexp.MustCompile('(?i)\bcycle[ -]\d+')`

```text
// cycleTokenRE folds the cycle number out of a todo's action so the same
// failure class re-minted in a later cycle fingerprints identically (the
// 2026-08-10 flood: 124 of 254 entries were per-cycle re-mints).
```

### `go/internal/core/carryover/mint.go:12` — above `func CapRunes(s string, maxRunes int) string { return textcap.CapRunes(s, maxRunes) }`

```text
// CapRunes truncates s to maxRunes with a trailing ellipsis rune — a
// projection of textcap.CapRunes, the rule's ONE home since ADR-0103 unit 04
// (the advisor prompt reads the same rule there instead of importing the
// todo lifecycle for a string helper).
```

### `go/internal/core/carryover/mint.go:18` — above `func TruncateRunes(s string, max int) string { return textcap.TruncateRunes(s, max) }`

```text
// TruncateRunes trims surrounding whitespace and caps s at max runes, marking
// truncation with " …[truncated]" — the remediation title's rule (ADR-0103
// unit 03b), the third cap beside CapRunes ("…") and Summary
// (" ...[truncated]"); since unit 04 a projection of textcap.TruncateRunes.
// Three rules, one file enumerates them.
```

### `go/internal/core/carryover/rules_test.go:3` — above `import (`

```text
// rules_test.go — unit 03 (ADR-0103, design decomposition/03-carryover-lifecycle.md):
// the pure rules — identity, unions, retirement, expiry, caps, the summary and
// the priority vocabulary. RED first; every rule is byte-for-byte the
// pre-extraction behaviour.
```

### `go/internal/core/carryover/rules_test.go:187` — above `func TestTruncateRunes_TrimsCapsAndMarks(t *testing.T) {`

```text
// ADR-0103 unit 03b: the THIRD rune cap — the advisor prompt's and the
// remediation title's — moved beside CapRunes and Summary so the three rules
// are enumerable in one file. Distinct marker from both (the F5 twin).
```

### `go/internal/core/carryover/textcap_projection_test.go:10` — above `func TestTruncateRunes_ProjectsTextcap(t *testing.T) {`

```text
// ADR-0103 unit 04: the lifecycle's two rune caps are projections of textcap
// — the ONE home the advisor prompt also reads — so neither consumer can
// silently re-implement the rule. Unit 03's own rule tests stay beside them.
```

### `go/internal/core/carryover/workspace.go:59` — above `var doc defectledger.Doc`

```text
// the ledger's own wire shape (ADR-0103 unit 09); decoded through THIS reader so a fault stays a carryover code
```

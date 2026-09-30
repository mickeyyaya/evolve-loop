# Comment history: `acs/regression/cycle49`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle49/predicates_test.go:3` — above `package cycle49`

```text
// Package cycle49 ports the cycle-49 ACS predicates (6 bash files).
// Source-presence ports of the task-fingerprint + research-cache + CLAUDE.md schema acceptance criteria.
```

### `go/acs/regression/cycle49/predicates_test.go:16` — above `func TestC49_001_TaskFingerprintExists(t *testing.T) {`

```text
// TestC49_001_TaskFingerprintExists ports cycle-49/001.
```

### `go/acs/regression/cycle49/predicates_test.go:29` — above `func TestC49_002_FingerprintDeterminism(t *testing.T) {`

```text
// TestC49_002_FingerprintDeterminism ports cycle-49/002.
// Behavioral: whitespace-equivalent inputs produce identical fingerprints.
```

### `go/acs/regression/cycle49/predicates_test.go:56` — above `func TestC49_003_ResearchCacheExists(t *testing.T) {`

```text
// TestC49_003_ResearchCacheExists ports cycle-49/003.
```

### `go/acs/regression/cycle49/predicates_test.go:69` — above `func TestC49_004_PromoteResearchCacheExists(t *testing.T) {`

```text
// TestC49_004_PromoteResearchCacheExists ports cycle-49/004.
```

### `go/acs/regression/cycle49/predicates_test.go:82` — above `func TestC49_005_ScoutProfileTools(t *testing.T) {`

```text
// TestC49_005_ScoutProfileTools ports cycle-49/005.
// Verifies scout.json tools list includes WebFetch/WebSearch.
```

### `go/acs/regression/cycle49/predicates_test.go:95` — above `func TestC49_006_ClaudeMdSchema(t *testing.T) {`

```text
// TestC49_006_ClaudeMdSchema ports cycle-49/006.
// CLAUDE.md must contain the researchCache schema reference.
```

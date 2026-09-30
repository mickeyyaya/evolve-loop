# Comment history: `acs/cycle1`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1/predicates_test.go:3` — above `package cycle1`

```text
// Package cycle1 materializes the cycle-1 acceptance criteria for two tasks:
//
//   - codex-pretrust-concurrent-regression: regression test for concurrent
//     pretrust of distinct worktree paths against a shared
//     EVOLVE_CODEX_CONFIG_PATH file (Slice 2, concurrency-arch-slices campaign).
//
//   - cycle-audit-cycle-scoped-ci-gap: audit gate verification ensuring the
//     gofmt CI-parity gate and SKILL.md drift gate are wired in NewDefault
//     (prevents recurrence of cycles 339-341 CI-red regressions).
```

### `go/acs/cycle1/predicates_test.go:24` — above `func TestC1_001_ConcurrentTestFileExistsAndTracked(t *testing.T) {`

```text
// TestC1_001_ConcurrentTestFileExistsAndTracked asserts that
// go/internal/bridge/codex_pretrust_concurrent_test.go was created in the
// worktree and is git-tracked. Disk presence alone is insufficient — a
// gitignored file is silently dropped at ship (cycle-93 lesson).
```

### `go/acs/cycle1/predicates_test.go:35` — above `if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {`

```text
// git-tracking check (cycle-93 pattern): untracked files are dropped at ship.
```

### `go/acs/cycle1/predicates_test.go:122` — above `func TestC1_005_GofmtGateWiredInAuditNewDefault(t *testing.T) {`

```text
// TestC1_005_GofmtGateWiredInAuditNewDefault verifies the gofmt CI-parity
// gate is active in the deployed NewDefault configuration. Behavioral: runs the
// existing TestNewDefault_WiresGofmtCheck test as a subprocess against the
// worktree's audit package. A dirty go file with a green EGPS suite MUST cause
// Verdict=FAIL — if NewDefault no longer wires the gate, this test exits
// non-zero, and this predicate reports RED.
//
// Prevents recurrence of cycles 339-341: generated go/acs/cycle<N>/*.go files
// that were not gofmt-clean shipped CI-red because the cycle-scoped audit
// never ran gofmt before ship.
```

### `go/acs/cycle1/predicates_test.go:148` — above `func TestC1_006_SkillsDriftGateWiredInAuditNewDefault(t *testing.T) {`

```text
// TestC1_006_SkillsDriftGateWiredInAuditNewDefault verifies the SKILL.md
// phase-facts drift gate is active in the deployed NewDefault configuration.
// Behavioral: runs the existing TestNewDefault_WiresSkillsDriftCheck test as a
// subprocess. A cycle that edits .evolve/profiles/*.json without regenerating
// SKILL.md must FAIL audit — this predicate confirms the gate is armed.
//
// Prevents recurrence of cycle 339's SKILL.md drift that shipped CI-red on
// TestSkills_NoDrift.
```

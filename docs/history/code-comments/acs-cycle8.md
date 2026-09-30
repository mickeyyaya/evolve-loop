# Comment history: `acs/cycle8`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle8/predicates_test.go:3` — above `package cycle8`

```text
// Package cycle8 materializes the cycle-1 acceptance criteria for two committed tasks:
//
//   - campaign-cmd-driver — add go/cmd/evolve/cmd_campaign.go implementing
//     `evolve campaign` with three subcommands (study, replan, run) and wire
//     it into registry.go.
//
//   - campaign-adr-and-citations — write ADR-0056 and the companion
//     docs/architecture/campaign-planning-citations.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	campaign-cmd-driver:
//	  AC1  "campaign" entry in registry.go / lookupCommand returns non-nil → C1_001
//	  AC2  `evolve campaign` (no args) prints usage and exits non-zero         → C1_002
//	  AC3  `study` subcommand accessible (not "unknown command")               → C1_003
//	  AC4  `replan --feedback` subcommand accessible                           → C1_004
//	  AC5  `run --plan <path> --simulate` exits 0 with valid plan              → C1_005
//	  [adversarial] run with invalid plan exits non-zero                       → C1_005neg
//	  AC6  go build ./cmd/evolve/... and go vet pass                           → C1_006 (pre-existing GREEN)
//
//	campaign-adr-and-citations:
//	  AC1  ADR-0056 file exists and is git-tracked                             → C2_001
//	  AC2  ADR has ## Status / ## Context / ## Decision / ## Consequences      → C2_002
//	  AC3  ADR describes all four slices S1–S4                                 → C2_003
//	  AC4  campaign-planning-citations.md exists, non-empty (≥5 lines)         → C2_004
//	  AC5  No placeholder URLs (example.com / TODO) in ADR                    → C2_005
//
// Floor binding (R9.3): predicates only for committed top_n tasks.
```

### `go/acs/cycle8/predicates_test.go:207` — above `repo := acsassert.RepoRoot(t)`

```text
// The simulate walk runs against a SCRATCH project root, never the checkout:
// a plumbing check that wrote runs/ledger/dossiers into the repo and committed
// closeouts onto the dev branch was the 2026-09-14 suite-litter incident
// (docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md). The
// checkout's history and tree must be byte-identical before and after.
```

### `go/acs/cycle8/predicates_test.go:284` — above `func TestC2_001_ADRFileExistsAndTracked(t *testing.T) {`

```text
// TestC2_001_ADRFileExistsAndTracked verifies that the ADR-0056 file exists on
// disk AND is tracked by git (not gitignored or untracked).
//
// BEHAVIORAL: disk presence + git tracking check prevents a gitignored worktree
// file from being silently dropped at ship (cycle-92 defect mode).
//
// RED: file does not exist yet.
```

### `go/acs/cycle8/predicates_test.go:307` — above `func TestC2_002_ADRHasRequiredSections(t *testing.T) {`

```text
// TestC2_002_ADRHasRequiredSections verifies that ADR-0056 contains the four
// standard ADR section headers.
//
// // acs-predicate: config-check
// Structural doc sections are inherently a config-presence check; the waiver
// applies. Each section name is the unambiguous identity of the required ADR
// structure per the project's ADR convention.
//
// RED: file does not exist (FileContains fails on read error).
```

### `go/acs/cycle8/predicates_test.go:328` — above `func TestC2_003_ADRDescribesAllFourSlices(t *testing.T) {`

```text
// TestC2_003_ADRDescribesAllFourSlices verifies that ADR-0056 describes all four
// implementation slices: S1 (wave engine / dag.Levels), S2 (preliminary-study
// phase), S3 (CLI driver / cmd_campaign), and S4 (documentation).
//
// // acs-predicate: config-check
// The slice descriptions are required content in the Decision section;
// checking their presence is a structural doc requirement.
//
// RED: file does not exist (FileContains fails on read error).
```

### `go/acs/cycle8/predicates_test.go:397` — above `func TestC2_005_NoPlaceholderURLsInADR(t *testing.T) {`

```text
// TestC2_005_NoPlaceholderURLsInADR verifies that ADR-0056 does not contain
// placeholder URLs: no "example.com" and no "TODO" in a URL context.
//
// // acs-predicate: config-check
// Absence of placeholder patterns is a structural constraint on doc quality.
//
// RED: file does not exist (FileNotContains returns false on read error).
```

### `go/acs/cycle8/predicates_test.go:434` — above `branches, errOut, code, err := acsassert.SubprocessOutput("git", "-C", repo, "branch", "--list", "cycle-*")`

```text
// Three of the incident's five mutations were out-of-tree: cycle-* branches
// and registered worktrees — both immune to build-artifact noise, both part
// of the fingerprint.
```

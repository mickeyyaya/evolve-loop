# Comment history: `acs/cycle5`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle5/amplified_test.go:3` — above `package cycle5`

```text
// Package cycle5 — Test Amplification (adversarial) tests for cycle 5.
//
// These tests probe dimensions NOT covered by the TDD contract in
// predicates_test.go. The amplifier is a black-box tester: it reads only
// the spec (scout-report, build-report file list, agent-mailbox) and NOT
// the implementation.
//
// Each test targets a distinct failure mode: incomplete ADR, wrong cluster
// attribution, missing lifecycle keyword, ADR number collision, or missing
// table-row descriptions in runtime-reference.md.
```

### `go/acs/cycle5/amplified_test.go:22` — above `func TestC5_AMP_001_ADRHasPriorArtSection(t *testing.T) {`

```text
// TestC5_AMP_001_ADRHasPriorArtSection verifies that ADR-0054 contains a
// Prior Art section. The scout report explicitly requires this section to
// reference concurrent-build systems (GitLab CI_CONCURRENT_ID, Jenkins @N,
// Temporal). The TDD contract checks for "## Status/Layer 1/Layer 2/runscope/
// ADR-0049" only — Prior Art is unchecked.
//
// Failure mode caught: builder wrote the ADR without the Prior Art section.
```

### `go/acs/cycle5/amplified_test.go:35` — above `func TestC5_AMP_002_ADRHasConsequencesSection(t *testing.T) {`

```text
// TestC5_AMP_002_ADRHasConsequencesSection verifies that ADR-0054 includes
// a Consequences section. Standard ADR format requires Context/Decision/
// Consequences; omitting Consequences makes the record incomplete.
//
// Failure mode caught: builder created a minimal ADR covering only the
// technically-required sections, skipping the operational impact.
```

### `go/acs/cycle5/amplified_test.go:47` — above `func TestC5_AMP_003_ADRHasDecisionsSection(t *testing.T) {`

```text
// TestC5_AMP_003_ADRHasDecisionsSection verifies that ADR-0054 includes
// a Decisions section (or Decision section). An ADR without explicit
// decisions records is a design rationale document, not a decision record.
//
// Failure mode caught: builder structured the ADR as a design doc
// (Context + Implementation) without a distinct Decisions heading.
```

### `go/acs/cycle5/amplified_test.go:62` — above `func TestC5_AMP_004_ADRStatusHasLifecycleKeyword(t *testing.T) {`

```text
// TestC5_AMP_004_ADRStatusHasLifecycleKeyword verifies that the ## Status
// section of ADR-0054 contains a standard ADR lifecycle keyword
// (Accepted, Proposed, Deprecated, or Superseded). A builder could satisfy
// TestC5_002 by writing "## Status" with no body (or with "TODO").
//
// Failure mode caught: Status heading is present but body is empty/placeholder.
```

### `go/acs/cycle5/amplified_test.go:76` — above `func TestC5_AMP_005_NoDuplicateADR0054(t *testing.T) {`

```text
// TestC5_AMP_005_NoDuplicateADR0054 verifies that exactly one file in
// docs/architecture/adr/ is numbered 0054. A copy-paste of ADR-0053 can
// inadvertently produce a second 0054-something-else.md file.
//
// Failure mode caught: duplicate ADR number collision from careless copy-paste.
```

### `go/acs/cycle5/amplified_test.go:93` — above `func TestC5_AMP_006_EVOLVELaneNotInSiblingWorktreeCluster(t *testing.T) {`

```text
// TestC5_AMP_006_EVOLVELaneNotInSiblingWorktreeCluster verifies that no
// single line of go/internal/flagregistry/registry_table.go contains BOTH
// "EVOLVE_LANE" AND "Sibling-Worktree". EVOLVE_LANE belongs to the Fleet
// (ADR-0049) cluster; misattributing it to Sibling-Worktree would corrupt
// the registry's cluster semantics.
//
// Note: The scout report confirms EVOLVE_LANE is at registry_table.go:160
// in cluster "Concurrency / Fleet (ADR-0049)". This test guards the opposite.
//
// Failure mode caught: copy-paste of a new Sibling-Worktree struct literal
// accidentally overwrites or moves the EVOLVE_LANE row's cluster field.
```

### `go/acs/cycle5/amplified_test.go:112` — above `func TestC5_AMP_007_ADRHasSliceTable(t *testing.T) {`

```text
// TestC5_AMP_007_ADRHasSliceTable verifies that ADR-0054 documents the
// six-slice campaign delivery. The build spec requires a "Slice-by-Slice
// delivery table". Without it, the ADR is not the design record for the
// campaign.
//
// Failure mode caught: builder wrote a general architecture ADR without
// documenting the phased delivery (slices 1-6).
```

### `go/acs/cycle5/amplified_test.go:142` — above `func TestC5_AMP_010_ADRMentionsCliadmit(t *testing.T) {`

```text
// TestC5_AMP_010_ADRMentionsCliadmit verifies that ADR-0054 mentions
// cliadmit as a Layer 2 component. Per the campaign spec, Layer 2 (shared
// host runtime guards) consists of sessionreaper + cliadmit. The TDD
// contract checks for "Layer 2" and "runscope" but not for the concrete
// Layer 2 package names.
//
// Failure mode caught: ADR abstractly describes Layer 2 without naming
// the two packages introduced by slices 3 and 4.
```

### `go/acs/cycle5/predicates_test.go:3` — above `package cycle5`

```text
// Package cycle5 materializes the cycle-5 acceptance criteria for:
//
//   - concurrent-loop-adr-docs: Slice 6 of the concurrency-arch-slices campaign.
//     Deliverables: ADR-0054 (sibling-worktree architecture doc), runtime-reference
//     flag entries for EVOLVE_LANE/EVOLVE_REAP_ORPHANS/EVOLVE_CLI_MAX_CONCURRENT_<CLI>,
//     and flag registry rows for EVOLVE_REAP_ORPHANS + EVOLVE_CLI_MAX_CONCURRENT_<CLI>.
//
//   - convert-hang-classifier-to-policy: EVOLVE_HANG_CLASSIFIER → ClassifyPolicy
//     struct + ClassifyConfig() accessor in policy.go; os.Getenv removed from
//     classify.go; registry entry → StatusDeprecated; apicover test added.
//
//   - convert-modelcatalog-autorefresh-to-policy: EVOLVE_MODELCATALOG_AUTOREFRESH
//     → CatalogPolicy{AutoRefresh *bool} + CatalogConfig() in policy.go; const +
//     os.Getenv removed from cmd_models_live.go; shouldRefreshCatalog param →
//     bool; registry entry → StatusDeprecated; apicover test added.
//
//   - convert-anthropic-base-url-to-bridge-policy: EVOLVE_ANTHROPIC_BASE_URL →
//     BridgePolicy.AnthropicBaseURL string field; EVOLVE_-prefixed reads removed
//     from driver_claudetmux.go and setup.go; raw ANTHROPIC_BASE_URL reads
//     preserved; registry entry → StatusDeprecated; bridge test extended.
```

### `go/acs/cycle5/predicates_test.go:35` — above `func TestC5_001_ADRFileExistsAndTracked(t *testing.T) {`

```text
// TestC5_001_ADRFileExistsAndTracked asserts that
// docs/architecture/adr/0054-concurrent-evolve-loop-sibling-worktrees.md
// was created in the worktree and is git-tracked. A gitignored file is
// silently dropped at ship (cycle-93 lesson). Also covers AC7 — the ADR
// number must be exactly 0054 (not a renumbered copy of 0053 or 0055).
```

### `go/acs/cycle5/predicates_test.go:52` — above `func TestC5_002_ADRFileHasRequiredSections(t *testing.T) {`

```text
// TestC5_002_ADRFileHasRequiredSections verifies that the ADR file contains
// all five required structural elements: a Status section, Layer 1, Layer 2,
// runscope, and a reference to ADR-0049. Encodes AC1's content requirements.
//
// acs-predicate: config-check — ADR structural assertions are inherently
// doc-section-presence checks; the behavioral anchor is TestC5_001 (git-tracked)
// and TestC5_005 (go build passes with the doc committed).
```

### `go/acs/cycle5/predicates_test.go:85` — above `func TestC5_005_GoBuildPassesAfterFlagRows(t *testing.T) {`

```text
// TestC5_005_GoBuildPassesAfterFlagRows verifies that adding the two flag
// registry rows introduces no compilation regression. AC4.
//
// Pre-existing GREEN expected: the branch is documentation-only (Slice 6).
// HEAD aaf12fc5 passes go build; new flag rows are purely data (no new Go code).
```

### `go/acs/cycle5/predicates_test.go:104` — above `func TestC5_006_FlagRegistryTestsPassAfterNewRows(t *testing.T) {`

```text
// TestC5_006_FlagRegistryTestsPassAfterNewRows verifies that the flagregistry
// unit tests still pass after the two new rows are added. AC5. The flag
// registry package has tests that enforce table invariants; new rows must
// satisfy them.
//
// Pre-existing GREEN expected: flagregistry tests pass on HEAD aaf12fc5.
```

### `go/acs/cycle5/predicates_test.go:140` — above `func TestC5_010_HangClassifierEnvReadRemoved(t *testing.T) {`

```text
// =============================================================================
// Flag-Reduction Campaign Cycle 5: Three EVOLVE_ env reads → policy.json typed
// fields. Tasks: convert-hang-classifier-to-policy (010-013),
// convert-modelcatalog-autorefresh-to-policy (014-018),
// convert-anthropic-base-url-to-bridge-policy (019-024).
// =============================================================================
```

### `go/acs/cycle5/predicates_test.go:176` — above `func TestC5_012_ClassifyConfigTestFileTracked(t *testing.T) {`

```text
// TestC5_012_ClassifyConfigTestFileTracked asserts that the new apicover test
// file for ClassifyPolicy/ClassifyConfig was created and is git-tracked (C5).
// A git-untracked file is silently dropped at ship (cycle-93 lesson).
```

### `go/acs/cycle5/predicates_test.go:244` — above `func TestC5_016_CatalogConfigTestFileTracked(t *testing.T) {`

```text
// TestC5_016_CatalogConfigTestFileTracked asserts that catalog_config_param_test.go
// was created and is git-tracked (C5). Same cycle-93 ship-guard rationale as
// TestC5_012.
```

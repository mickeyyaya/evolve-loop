# Comment history: `acs/cycle12`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle12/predicates_test.go:3` — above `package cycle12`

```text
// Package cycle12 materializes the cycle-12 acceptance criteria for the
// committed top_n task:
//
//	phase-recovery-flag-retire — retire EVOLVE_PHASE_RECOVERY from the flag
//	registry by converting all four bare env-var reads to the policy/config-resolved
//	path (RecoveryConfig in policy.go; Deps.Env overlay for bridge/fatalpane;
//	RecoveryStage field for CoreAdapter; IPC const for phasecmd subprocess), then
//	deleting the registry row (35 → 34 rows).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	phase-recovery-flag-retire:
//	  AC1      flagregistry.Lookup("EVOLVE_PHASE_RECOVERY") returns ok=false  → C12_001 (behavioral)
//	  AC2      registry row count strictly reduced to 34                        → C12_002 (behavioral)
//	  AC3      no bare env reads in core_adapter.go + phase_observer.go         → C12_003 (config-check, waiver)
//	  AC4      flagreaders ACS guard still passes                               → manual+checklist (CI regression lane)
//	  AC5      all affected packages test suite green                           → manual+checklist (CI pipeline)
//	  AC6      stallPolicyFromEnv reads IPC const, not os.Getenv("...")        → C12_004 (config-check, waiver)
//	  AC7      CoreAdapter.RecoveryStage field added                            → C12_005 (config-check, waiver)
//	  EDGE1    control-flags.md has no EVOLVE_PHASE_RECOVERY entry              → C12_006 (config-check, waiver)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C12_001 — Lookup must return false; cannot be satisfied by any
//	           magic-string patch — the row must be deleted from registry_table.go.
//	           This is the primary anti-no-op signal: cycle-10 w2-phaserecovery-ipc
//	           shipped with 35→35 rows because only readers were converted; this test
//	           ensures that mistake is not repeated (flagprogress guard PR #212 added
//	           after that failure also catches this, but at the metric level).
//	Edge/OOD:  C12_002 — exact count == 34 (not just "< 35"), pinning the delta so
//	           accidental additional deletions are caught immediately.
//	Lexical:   Lookup() / len() / FileNotContains / FileContains — four distinct verbs.
//	Semantic:  registry-row-absent, count-exact, env-reads-deleted (2 files),
//	           IPC-const-defined (new protocol), field-added, doc-regenerated.
//
// 1:1 enforcement: predicate=6, manual+checklist=2 → 7 ACs + 1 EDGE = 8 dispositions ✓
//
// Manual+checklist dispositions:
//
//	AC4 (flagreaders ACS guard): `go test -tags acs ./acs/regression/flagreaders/...`
//	    — enforced by the CI regression lane; a cycle predicate would duplicate
//	    an existing durable guard and add no signal beyond what that guard already provides.
//	AC5 (full test suite): `go test ./internal/bridge/... ./internal/adapters/observer/...
//	    ./internal/cli/phasecmd/... ./internal/config/... ./internal/policy/...`
//	    — enforced by CI on every commit; no additional cycle predicate is needed.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (phase-recovery-flag-retire). Deferred tasks get zero predicates.
```

### `go/acs/cycle12/predicates_test.go:61` — above `func TestC12_001_PhaseRecoveryFlagAbsentFromRegistry(t *testing.T) {`

```text
// TestC12_001_PhaseRecoveryFlagAbsentFromRegistry verifies that EVOLVE_PHASE_RECOVERY
// is no longer registered after Builder removes its row from registry_table.go.
//
// AC1: flagregistry.Lookup("EVOLVE_PHASE_RECOVERY") must return ok=false.
//
// BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT function. A source
// edit alone cannot satisfy this; the row must be absent from registry_table.go for
// Lookup to return ok=false. This is the primary anti-no-op signal: cycle-10
// (w2-phaserecovery-ipc) shipped with rows 35→35 because only the readers were
// converted without deleting the registry entry. The flagprogress guard (PR #212)
// catches this at the metric level; this predicate asserts the specific row is gone.
//
// RED: flagregistry.Lookup currently returns (flag, true) for EVOLVE_PHASE_RECOVERY
// (row at registry_table.go:30, StatusActive, "Phase Recovery (ADR-0044, Go-native)").
```

### `go/acs/cycle12/predicates_test.go:87` — above `func TestC12_002_RegistryRowCountDroppedTo34(t *testing.T) {`

```text
// TestC12_002_RegistryRowCountDroppedTo34 verifies that the registry row count
// is exactly 34 after Builder removes the EVOLVE_PHASE_RECOVERY row.
//
// AC2: the flagprogress gate requires len(flagregistry.All) < 35 (HEAD count).
// This predicate pins the exact target (34) so any unintended additional deletion
// or regression is also caught immediately.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production SSOT slice count.
// Unlike a source-file grep, this cannot be satisfied by editing comments or
// adding magic strings.
//
// RED: len(flagregistry.All) is currently 35 (HEAD 52039d82).
```

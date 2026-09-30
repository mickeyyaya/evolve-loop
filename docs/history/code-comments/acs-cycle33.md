# Comment history: `acs/cycle33`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle33/predicates_test.go:3` — above `package cycle33`

```text
// Package cycle33 materializes the cycle-33 acceptance criteria for:
//
//	swarm-config-cluster-33 — migrate 2 live EVOLVE_* swarm flags from
//	req.Env map reads in swarmrunner.go to policy.SwarmPolicy (Configuration Object);
//	dead-sweep 2 comment-only flags. 4 flags total:
//	  - EVOLVE_SWARM_STAGE       → policy.SwarmPolicy.Stage + swarmrunner.Config.Stage
//	  - EVOLVE_SWARM_PORT_BASE   → policy.SwarmPolicy.PortBase + swarmrunner.Config.PortBase
//	  - EVOLVE_PHASE_BUILD_BIN   → dead sweep (comment-only; no Go reader)
//	  - EVOLVE_SHIP_SCRIPT       → dead sweep (comment-only; no Go reader)
//	Lower FlagCeiling 97→93; regenerate docs/architecture/control-flags.md.
//	Delete portBaseFromEnv() helper from swarmrunner.go (zero callers after migration).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	swarm-config-cluster-33:
//	  AC1   4 flags absent from Lookup               → C33_001 (behavioral)
//	  AC2   Registry row count == 93                 → C33_002 (behavioral, count)
//	  AC3   FlagCeiling const == 93                  → C33_003 (config-check, waiver)
//	  AC4   EVOLVE_SWARM_STAGE not in swarmrunner.go → C33_004 (config-check, waiver)
//	  AC5   EVOLVE_SWARM_PORT_BASE not in swarmrunner.go → C33_005 (config-check, waiver)
//	  AC6   SwarmConfig() defaults: Stage="shadow", PortBase=0 → C33_006 (behavioral)
//	  AC7   flagreaders regression guard green        → manual+checklist (see below)
//	  AC8   EVOLVE_WORKTREE_PATH still registered     → C33_008 (behavioral, PRE-EXISTING GREEN)
//	  AC9   control-flags.md has no removed flag rows → C33_009 (config-check, waiver)
//	  NEG1  portBaseFromEnv deleted from swarmrunner.go → C33_NEG1 (config-check, waiver)
//	  NEG2  swarmrunner.Config used at composition root → C33_NEG2 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle33 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_SWARM_STAGE" or "EVOLVE_SWARM_PORT_BASE"
//	        in any non-test, non-registry Go file via env map reads
//	        (grep -rn '"EVOLVE_SWARM_STAGE"\|"EVOLVE_SWARM_PORT_BASE"'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle33' → 0 matches);
//	    (d) EVOLVE_PHASE_BUILD_BIN and EVOLVE_SHIP_SCRIPT had zero Go readers
//	        before dead sweep; verify no new reader was added.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C33_001 — 4 flags must be ABSENT from Lookup (if Builder misses any
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C33_004/C33_005 — env-read literals must be ABSENT from swarmrunner.go
//	           (if Builder removes registry rows without deleting call sites, the
//	           literal strings remain and these tests fail — the cycle-8 split-const
//	           anti-gaming check).
//	           C33_NEG1 — portBaseFromEnv() function itself must be ABSENT (closing
//	           the gaming surface where callers migrate but dead helper stays).
//	Edge/OOD:  C33_002 checks exact count 93; both over-removal (< 93) and
//	           under-removal (> 93) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / direct struct-field
//	           access — distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads-stage,
//	           no-env-reads-portbase, config-defaults, worktree-path-present,
//	           doc-absence, helper-fn-deleted, composition-root-updated — 10 distinct
//	           behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (swarm-config-cluster-33). Deferred tasks (SWARM_PLANNER, REFLECTION_JOURNAL,
// STRICT_AUDIT, IPC cluster-10) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=10, manual+checklist=1, unverifiable-remove=0 → total AC=11 ✓
```

### `go/acs/cycle33/predicates_test.go:82` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 4 flags that cycle-33 removes:
//   - EVOLVE_SWARM_STAGE:     migrated to policy.SwarmPolicy.Stage + swarmrunner.Config.Stage
//   - EVOLVE_SWARM_PORT_BASE: migrated to policy.SwarmPolicy.PortBase + swarmrunner.Config.PortBase
//   - EVOLVE_PHASE_BUILD_BIN: dead sweep (comment-only; no Go reader)
//   - EVOLVE_SHIP_SCRIPT:     dead sweep (comment-only; no Go reader)
```

### `go/acs/cycle33/predicates_test.go:94` — above `func TestC33_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {`

```text
// TestC33_001_RemovedFlagsAbsentFromRegistry verifies that all 4 removed flags
// are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. The 4 flags span two removal patterns:
//   - 2 live flags: SwarmPolicy migration (Configuration Object pattern, cycles 29-32 precedent)
//   - 2 dead flags: dead sweep (comment-only; zero Go readers confirmed by scout)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 4 flags are currently registered (FlagCeiling=97); each Lookup
// returns (flag, true).
```

### `go/acs/cycle33/predicates_test.go:118` — above `func TestC33_004_SwarmStageNotInSwarmrunner(t *testing.T) {`

```text
// TestC33_004_SwarmStageNotInSwarmrunner verifies that the env-map read for
// EVOLVE_SWARM_STAGE has been deleted from swarmrunner.go.
//
// Covers AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot just
// remove the registry row while leaving the env["EVOLVE_SWARM_STAGE"] string
// literal in swarmrunner.go; this predicate catches that gap.
//
// acs-predicate: config-check
//
// RED: swarmrunner.go currently reads env["EVOLVE_SWARM_STAGE"] in swarmStage()
// at line ~212, and the package doc comment also references the flag name.
// Both must be absent after migration (Builder replaces swarmStage with
// parseSwarmStage(d.cfg.Stage) and updates the package doc).
```

### `go/acs/cycle33/predicates_test.go:143` — above `func TestC33_005_SwarmPortBaseNotInSwarmrunner(t *testing.T) {`

```text
// TestC33_005_SwarmPortBaseNotInSwarmrunner verifies that the env-map read for
// EVOLVE_SWARM_PORT_BASE has been deleted from swarmrunner.go.
//
// Covers AC5. Anti-gaming (cycle-8 split-const lesson): removing the registry
// row without deleting portBaseFromEnv(req.Env) from dispatchDeps() is the
// split-const hiding pattern — this predicate closes that gap.
//
// acs-predicate: config-check
//
// RED: swarmrunner.go:125 contains portBaseFromEnv(req.Env) which reads
// env["EVOLVE_SWARM_PORT_BASE"]. After migration, the env-read string must
// not appear in swarmrunner.go.
```

### `go/acs/cycle33/predicates_test.go:204` — above `func TestC33_008_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC33_008_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC8 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```

### `go/acs/cycle33/predicates_test.go:249` — above `func TestC33_NEG1_PortBaseFromEnvDeleted(t *testing.T) {`

```text
// TestC33_NEG1_PortBaseFromEnvDeleted is the anti-gaming predicate that verifies
// the portBaseFromEnv() helper function has been completely deleted from
// swarmrunner.go.
//
// Anti-gaming rationale (cycle-8/cycle-85 lesson): Builder could replace
// the env["EVOLVE_SWARM_PORT_BASE"] read with d.cfg.PortBase in dispatchDeps()
// while leaving the dead portBaseFromEnv() function in place as unused code.
// C33_005 confirms the EVOLVE_SWARM_PORT_BASE string is gone; NEG1 adds a
// second layer by asserting the portBaseFromEnv function itself is gone —
// closing the gaming surface where the function stays as dead code.
//
// acs-predicate: config-check
//
// RED: swarmrunner.go currently defines "func portBaseFromEnv(env map[string]string) int"
// that reads env["EVOLVE_SWARM_PORT_BASE"]. After migration, this function must
// not exist.
```

### `go/acs/cycle33/predicates_test.go:279` — above `func TestC33_NEG2_SwarmConfigUsedAtCompositionRoot(t *testing.T) {`

```text
// TestC33_NEG2_SwarmConfigUsedAtCompositionRoot verifies that the composition
// root (cmd/evolve/cmd_cycle.go) has been updated to wire swarmrunner.Config
// into the New() call rather than relying on the old 3-arg form.
//
// Covers NEG2. This is the mandatory final step of the Configuration Object
// pattern (cycles 29-32 precedent): once SwarmPolicy + SwarmConfig are added
// to policy and swarmrunner.Config is added to the Decorator, the composition
// root must use the 4-arg New(inner, br, mode, swCfg) form.
//
// acs-predicate: config-check
//
// RED: cmd_cycle.go currently calls swarmrunner.New with 3 args; the type
// swarmrunner.Config does not appear in the file yet.
```

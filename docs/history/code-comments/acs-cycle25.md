# Comment history: `acs/cycle25`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle25/predicates_test.go:3` — above `package cycle25`

```text
// Package cycle25 materializes the cycle-25 acceptance criteria for:
//
//	interactive-policy-profiles — remove 3 EVOLVE_INTERACTIVE_POLICY cluster flags
//	(EVOLVE_INTERACTIVE_POLICY, EVOLVE_SCOUT_INTERACTIVE_POLICY,
//	EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY) by deleting os.Getenv tier-2 reads
//	(envchain.Resolve in bridge.go::resolvePolicy) and adding Profile SSOT
//	(Profile.InteractivePolicy field) + typed BridgeRequest.InteractivePolicy field.
//	Lower FlagCeiling 135→132, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	interactive-policy-profiles:
//	  AC1  3 flags absent from Lookup             → C25_001 (behavioral)
//	  AC2  Registry row count == 132              → C25_002 (behavioral, count)
//	  AC3  FlagCeiling const == 132               → C25_003 (config-check, waiver)
//	  AC4  No prod readers + docs_contract pruned → C25_004 (mixed, config-check waiver)
//	  AC5  BridgeRequest.InteractivePolicy field  → C25_005 (behavioral, reflect)
//	  AC6  flagreaders guard green                → manual+checklist (see below)
//	  AC7  WORKTREE_PATH still registered         → C25_007 (behavioral — PRE-EXISTING GREEN)
//	  AC8  control-flags.md has no removed rows   → C25_008 (config-check, waiver)
//	  NEG1 Profile.InteractivePolicy honored      → C25_NEG1 (behavioral, reflect)
//	  NEG2 runtime-reference.md still docs flag   → C25_NEG2 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC6 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle25 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no stale EVOLVE_INTERACTIVE_POLICY / EVOLVE_SCOUT_INTERACTIVE_POLICY /
//	        EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY literal strings remain in
//	        non-test production Go (grep -rn 'EVOLVE_INTERACTIVE_POLICY\|...' go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C25_005 — uses reflect to verify BridgeRequest.InteractivePolicy field
//	           exists AND verifies bridge.go passes req.InteractivePolicy to resolvePolicy.
//	           The field-level behavioral check (reflect.FieldByName) is the strongest
//	           anti-no-op signal: adding a magic comment or grep string cannot satisfy it
//	           — the struct field must exist.
//	           C25_NEG1 — uses reflect to verify Profile.InteractivePolicy field exists
//	           AND checks runner.go reads prof.InteractivePolicy. Both checks are required:
//	           field existence alone does not prove the runner wires it to BridgeRequest.
//	Edge/OOD:  C25_001 tests ALL 3 flags in the cluster; includes EVOLVE_INTERACTIVE_POLICY
//	           (global) and 2 per-agent variants, ensuring no partial removal.
//	           C25_004 covers both the resolvePolicy envchain.Resolve call site (2 calls)
//	           AND the docs_contract_test.go cleanup (5 dead allowedUndocumented entries).
//	Lexical:   Lookup / len / FileContains / FileNotContains / CountInGoFunc /
//	           reflect.FieldByName / reflect.Kind — seven distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-const, structural-reader-absence,
//	           typed-field-reflection, doc-absence, worktree-path-preserved,
//	           profile-field-reflection, runtime-reference-preserved — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (interactive-policy-profiles). Deferred tasks (WORKTREE_PATH, ROUTER_CLI/MODEL,
// Workflow Defaults, BYPASS_* cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
```

### `go/acs/cycle25/predicates_test.go:75` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 3 INTERACTIVE_POLICY cluster flags
// that cycle-25 removes by migrating from envchain.Resolve (os.Getenv tier-2)
// to Profile SSOT + typed BridgeRequest.InteractivePolicy field.
```

### `go/acs/cycle25/predicates_test.go:218` — above `func TestC25_007_WorktreePathStillInRegistry(t *testing.T) {`

```text
// TestC25_007_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 3-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by TestC50_009.
//
// Covers AC7 (WORKTREE_PATH must not be touched). Cycles 17, 18, and 19 all
// failed when Builder over-reached and removed WORKTREE_PATH, breaking TestC50_009.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered and must stay so.
```

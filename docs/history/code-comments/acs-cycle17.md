# Comment history: `acs/cycle17`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle17/predicates_test.go:3` — above `package cycle17`

```text
// Package cycle17 materializes the cycle-17 acceptance criteria for 4 flag-reduction
// tasks, targeting 5 EVOLVE_* flags for elimination (registry 29 → 24 rows):
//
//   - dead-flag-delete: delete EVOLVE_CLI_MAX_CONCURRENT_CODEX registry row (0 literal readers)
//   - catalog-dir-di: convert EVOLVE_MODEL_CATALOG_DIR to fn-var DI via BridgePolicy.CatalogDir
//   - policy-acs-kb: add ACSConfig+PathsConfig to policy.go; wire EVOLVE_ACS_GO_TIMEOUT_S
//     and EVOLVE_KB_SEARCH_PATHS to these new policy structs
//   - phase-roots-policy: convert EVOLVE_PHASE_ROOTS to PathsConfig.PhaseRoots in policy.json
//
// AC map (1:1 with scout-report.md ACs, all in triage top_n):
//
//	dead-flag-delete:
//	  AC1  Lookup("EVOLVE_CLI_MAX_CONCURRENT_CODEX").found == false           → TestLookup_CliMaxConcurrentCodexAbsent
//	  AC2  No literal in registry_table.go                                    → TestC17_101_CliMaxConcurrentCodexNoLiteralInRegistry
//	  AC3- Driver prefix "EVOLVE_CLI_MAX_CONCURRENT_" still in driver source  → PRE-EXISTING GREEN (currently true; regression guard)
//
//	catalog-dir-di:
//	  AC1  No os.Getenv("EVOLVE_MODEL_CATALOG_DIR") in catalog_overlay.go     → TestC17_110_CatalogDirNoOsGetenv
//	  AC2  No os.Setenv("EVOLVE_MODEL_CATALOG_DIR") in cmd_cycle.go           → TestC17_111_CatalogDirNoOsSetenv
//	  AC3  Lookup("EVOLVE_MODEL_CATALOG_DIR").found == false                  → TestLookup_ModelCatalogDirAbsent
//	  AC4- modelCatalogDirFn fn-var seam present in catalog_overlay.go        → TestC17_113neg_CatalogDirFnVarInPlace
//
//	policy-acs-kb:
//	  AC1  No env read for EVOLVE_ACS_GO_TIMEOUT_S in acssuite.go             → TestC17_120_AcsSuiteNoEnvGetenv
//	  AC2  No env read for EVOLVE_KB_SEARCH_PATHS in kb.go                    → TestC17_121_KbNoEnvGetenv
//	  AC3a Lookup("EVOLVE_ACS_GO_TIMEOUT_S").found == false                   → TestLookup_AcsGoTimeoutSAbsent
//	  AC3b Lookup("EVOLVE_KB_SEARCH_PATHS").found == false                    → TestLookup_KbSearchPathsAbsent
//	  AC5- Empty policy → ACSTimeoutConfig.GoTimeoutS==0 (uses DefaultTimeout) → TestC17_124neg_EmptyACSConfigZeroTimeout
//	  AC6- Empty PathsConfig → KB fallback dirs non-empty                     → TestC17_125edge_EmptyPathsConfigKBFallback
//
//	phase-roots-policy:
//	  AC1  No os.Getenv(rootsEnv) in mergedcatalog.go                         → TestC17_130_PhaseRootsNoEnvRead
//	  AC2  phasespec test suite green                                          → PRE-EXISTING GREEN (suite ok now; guard against regression)
//	  AC4  Lookup("EVOLVE_PHASE_ROOTS").found == false                        → TestLookup_PhaseRootsAbsent
//	  AC5- Absent PathsConfig.PhaseRoots → defaultRoot fallback               → TestC17_132neg_AbsentPathsConfigDefaultFallback
//	  AC6- Absolute path in PhaseRoots passes through unchanged                → TestC17_133edge_AbsolutePathPassThrough
//
//	ALL tasks:
//	  count len(flagregistry.All) == 24 (29 − 5)                              → TestC17_999_RegistryCountIs24
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  AC3-NEG (dynamic driver prefix preserved), AC4-NEG (fn-var seam),
//	           AC5-NEG (nil-safe ACSConfig default), AC5-NEG (absent PathsConfig fallback)
//	Edge/OOD:  AC6-EDGE (absent PathsConfig → KB fallback), AC6-EDGE (absolute path pass-through)
//	Lexical:   Lookup, FileNotContains, FileContains, ACSTimeoutConfig, SearchPathsFromEnv,
//	           RootsWithPolicy, len(All) — seven distinct verbs
//	Semantic:  registry-deletion, env-read-removal, env-write-removal, fn-var-injection,
//	           policy-struct-accessor, nil-safety, absolute-path-preservation
//
// RED state: Package fails to compile because policy.PathsConfig, policy.ACSConfig
// (via ACSTimeoutConfig()), phasespec.RootsWithPolicy, and the new
// research.SearchPathsFromEnv(policy.PathsConfig{}) signature do not exist yet.
// Compile failure = RED (per ACS README: "RED = compile failure or t.Errorf/t.Fatalf").
//
// Pre-existing GREEN: AC3-NEG (driver prefix), AC2 phase-roots (phasespec suite currently passes).
//
// Floor binding (R9.3): predicates authored only for tasks in triage top_n.
// No predicates for deferred or dropped tasks.
```

# Comment history: `acs/cycle47`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle47/predicates_test.go:3` — above `package cycle47`

```text
// Package cycle47 materializes the cycle-47 acceptance criteria for two tasks:
//
//	release-ollama-env-aliases-47 — remove 2 env aliases with existing CLI equivalents:
//	  EVOLVE_RELEASE_REQUIRE_PREFLIGHT → alias for --require-preflight in cmd_release_pipeline.go
//	  EVOLVE_OLLAMA_BASE               → alias for --ollama-base in cmd_skills_publish.go
//	  Lower FlagCeiling 59 → 57.
//
//	modelcatalog-classifier-di-47 — DI migration:
//	  EVOLVE_MODELCATALOG_CLASSIFIER_CLI → add overrideCLI string param to pickClassifierCLI()
//	  Lower FlagCeiling 57 → 56.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: release-ollama-env-aliases-47 ===
//	AC1  EVOLVE_RELEASE_REQUIRE_PREFLIGHT absent from registry → C47A_001 (behavioral: Lookup)
//	AC2  EVOLVE_OLLAMA_BASE absent from registry               → C47A_002 (behavioral: Lookup)
//	AC3  No prod env read for RELEASE_REQUIRE_PREFLIGHT        → C47A_003 (config-check, waiver)
//	AC4  No prod env read for OLLAMA_BASE                      → C47A_004 (config-check, waiver)
//	AC5  cmd_release_pipeline_test.go has zero env key refs    → C47A_005 (config-check, waiver)
//	AC6  FlagCeiling == 57                                     → C47A_006 (config-check, waiver)
//	AC7  go test ./cmd/evolve/... PASS                         → manual+checklist (Auditor)
//	AC8  go test ./internal/flagregistry/... PASS              → manual+checklist (Auditor)
//	AC9  flagreaders ACS guard PASS                            → manual+checklist (Auditor)
//	AC10 control-flags.md regenerated with 57 flags            → C47A_010 (config-check, waiver)
//	NEG  row count ≤ 57 after A-task flags removed             → C47A_NEG (behavioral: len)
//
//	=== Task B: modelcatalog-classifier-di-47 ===
//	AC1  EVOLVE_MODELCATALOG_CLASSIFIER_CLI absent from registry → C47B_001 (behavioral: Lookup)
//	AC2  Zero prod reads for MODELCATALOG_CLASSIFIER_CLI         → C47B_002 (behavioral: CountInGoFunc)
//	AC3  pickClassifierCLI has overrideCLI string second param   → C47B_003 (config-check, waiver)
//	AC4  cmd_models_live_test.go has zero env key refs           → C47B_004 (config-check, waiver)
//	AC5  FlagCeiling == 56                                       → C47B_005 (config-check, waiver)
//	AC6  go test ./cmd/evolve/... PASS                           → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard PASS                              → manual+checklist (Auditor)
//	AC8  go test -tags acs ./acs/cycle47/ PASS                   → manual+checklist (self-referential)
//	NEG  exact row count == 56 (final state after both tasks)    → C47B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	AC7 (Task A — cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages in output
//	  (c) docs_contract_test.go entry for EVOLVE_RELEASE_REQUIRE_PREFLIGHT removed
//	      (otherwise TestAllFlagsInRegistryAreDocumented fails here)
//
//	AC8 (Task A — flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 57)
//
//	AC9 (Task A — flagreaders ACS guard):
//	  (a) evolve acs suite (or: go test -tags acs ./acs/regression/flagreaders/...)
//	  (b) Neither EVOLVE_RELEASE_REQUIRE_PREFLIGHT nor EVOLVE_OLLAMA_BASE appears as orphan reader
//
//	AC6 (Task B — cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages in output; TestPickClassifierCLI* and TestShouldRefreshCatalog pass
//
//	AC7 (Task B — flagreaders ACS guard):
//	  (a) evolve acs suite (or: go test -tags acs ./acs/regression/flagreaders/...)
//	  (b) EVOLVE_MODELCATALOG_CLASSIFIER_CLI does not appear as orphan reader
//
//	AC8 (Task B — acs/cycle47 suite passes):
//	  (a) cd go && go test -tags acs -count=1 ./acs/cycle47/
//	  (b) all TestC47A_* and TestC47B_* pass (GREEN after Builder implements both tasks)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C47A_001/002, C47B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C47A_NEG: row count ≤ 57 (catches under-removal from 59).
//	            C47B_NEG: exact count == 56 (catches over- and under-removal; strongest invariant).
//	Edge/OOD:   C47B_NEG catches both <56 (over-removal) and >56 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains / FileMatchesRegex / CountInGoFunc — six distinct verbs.
//	Semantic:   registry-absence (3 flags), exact-row-count / upper-bound-row-count, prod-source-clean (3 files),
//	            test-env-key-clean (2 test files), ceiling-const (2 values), control-flags-doc, DI-signature — 9 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_MODELCATALOG_AUTOREFRESH, EVOLVE_HANG_CLASSIFIER, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate count: 14 funcs (C47A_001–006, C47A_010, C47A_NEG, C47B_001–005, C47B_NEG)
//	manual+checklist: 6 (A_AC7, A_AC8, A_AC9, B_AC6, B_AC7, B_AC8 — checklists addressed to Auditor above)
//	unverifiable-remove: 0
//	Total AC count: 20; every AC has exactly one disposition row.
```

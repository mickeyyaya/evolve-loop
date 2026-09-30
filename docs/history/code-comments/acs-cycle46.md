# Comment history: `acs/cycle46`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle46/predicates_test.go:3` — above `package cycle46`

```text
// Package cycle46 materializes the cycle-46 acceptance criteria for task:
//
//	retro-stdout-config-46 — remove 2 flags from the operator registry:
//	  EVOLVE_RETRO_MODEL   → Bucket 1 (Config Object): retro.Config{Model string}
//	                         replaces req.Env["EVOLVE_RETRO_MODEL"] in Run().
//	  EVOLVE_STDOUT_FILTER → Bucket 1 (Config Object / DI): runner.Options{DisableStdoutFilter bool}
//	                         replaces envchain.Resolve("EVOLVE_STDOUT_FILTER", req.Env, "", "on").
//	Lower FlagCeiling 61 → 59.
//
// AC map (1:1 with triage top_n):
//
//	AC1  EVOLVE_RETRO_MODEL absent from registry         → C46_001 (behavioral: Lookup)
//	AC2  EVOLVE_STDOUT_FILTER absent from registry       → C46_002 (behavioral: Lookup)
//	AC3  No prod env read for EVOLVE_RETRO_MODEL         → C46_003 (config-check, waiver)
//	AC4  No prod envchain read for EVOLVE_STDOUT_FILTER  → C46_004 (config-check, waiver)
//	AC5  FlagCeiling == 59                               → C46_005 (config-check, waiver)
//	AC6  retro.Config.Model string field exists          → C46_006 (behavioral: reflect)
//	AC7  runner.Options.DisableStdoutFilter bool exists  → C46_007 (behavioral: reflect)
//	AC8  retro tests pass                                → manual+checklist (Auditor)
//	AC9  runner tests pass                               → manual+checklist (Auditor)
//	AC10 flagregistry tests pass                         → manual+checklist (Auditor)
//	AC11 flagreaders ACS guard passes                    → manual+checklist (Auditor; standing regression)
//	AC12 acs/cycle46 predicates pass                     → this file (self-referential, no extra func)
//	AC13-NEG No EVOLVE_STDOUT_FILTER env key in test    → C46_009 (config-check, waiver)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	AC8 (retro tests pass):
//	  (a) exit 0: cd go && go test ./internal/phases/retro/...
//	  (b) no FAIL packages in output
//
//	AC9 (runner tests pass):
//	  (a) exit 0: cd go && go test ./internal/phases/runner/...
//	  (b) no FAIL packages in output
//
//	AC10 (flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 59)
//
//	AC11 (flagreaders ACS guard):
//	  (a) evolve acs suite (or: go test -tags acs ./acs/regression/flagreaders/...)
//	  (b) Neither EVOLVE_RETRO_MODEL nor EVOLVE_STDOUT_FILTER appears as an orphan reader
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C46_001/002 — both flags ABSENT from Lookup (any hit = still registered).
//	            C46_NEG_ExactRowCountIs59 — registry EXACTLY 59; over- or under-removal fails.
//	Edge/OOD:   ExactRowCountIs59 catches both <59 (over-removal) and >59 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains / reflect / CountInGoFunc — distinct verbs.
//	Semantic:   registry-absence (2 flags), exact-row-count, prod-source-clean (2 files),
//	            struct-field-existence (2 types), docs-contract, test-env-key, ceiling-const,
//	            control-flags-doc, run-func-scoped AST — 10 distinct dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for retro-stdout-config-46
// (in triage top_n). Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	predicate count: 12 funcs (C46_001-009, C46_010, C46_012, C46_NEG)
//	manual+checklist: 4 (AC8, AC9, AC10, AC11 — checklists addressed to Auditor above)
//	unverifiable-remove: 0
//	Total AC count: 16; every AC has exactly one disposition row.
```

### `go/acs/cycle46/predicates_test.go:78` — above `var removedFlags = []string{`

```text
// removedFlags lists the 2 env flags removed in cycle 46.
```

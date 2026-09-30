# Comment history: `acs/cycle44`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle44/predicates_test.go:3` — above `package cycle44`

```text
// Package cycle44 materializes the cycle-44 acceptance criteria for one task:
//
//	workflow-dead-ipc-44 — remove 3 flags from the operator registry:
//	  EVOLVE_STRATEGY         → Bucket 7 (dead): env write in buildCycleEnv never read;
//	                            Strategy flows via Context["strategy"]
//	  EVOLVE_RESET            → Bucket 7 (dead): cfg.Reset used before buildCycleEnv;
//	                            env write at cmd_loop_args.go:275 has zero readers
//	  EVOLVE_SHIP_RELEASE_NOTES → Bucket 5 (IPC): exec.Command parent→child env
//	                            handoff in releasepipeline → evolve ship subprocess;
//	                            replace string literal with split-const "EVOLVE_"+"SHIP_RELEASE_NOTES"
//	Lower FlagCeiling 68 → 65.
//
// AC map (1:1 with triage top_n):
//
//	AC1  flagregistry.All has 65 entries             → C44_NEG_ExactRowCountIs65 (behavioral)
//	AC2  EVOLVE_STRATEGY absent from prod env writes → C44_002 (config-check, waiver)
//	AC3  EVOLVE_RESET absent from prod env writes    → C44_003 (config-check, waiver)
//	AC4  EVOLVE_SHIP_RELEASE_NOTES literal absent    → C44_004 (config-check, waiver)
//	AC5  No prod os.Getenv reads for STRATEGY/RESET  → covered by AC2/AC3 (dead-write-only flags)
//	AC6  FlagCeiling == 65                           → C44_006 (config-check, waiver)
//	AC7  flagreaders ACS guard PASS                  → manual+checklist (see below)
//	AC8  Full affected-package suite passes          → manual+checklist (see below)
//	AC9  control-flags.md regenerated               → C44_009 (config-check, waiver)
//	AC10 SSOT IPC comment present                   → C44_010 (config-check, waiver)
//	NEG  3 flags absent from Lookup                 → C44_001 (behavioral)
//	NEG  Exact row count == 65                      → C44_NEG_ExactRowCountIs65 (behavioral)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders ACS guard PASS):
//	  Checklist for Auditor:
//	  (a) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`
//	  (b) none of EVOLVE_STRATEGY, EVOLVE_RESET, EVOLVE_SHIP_RELEASE_NOTES appear in
//	      non-test, non-registry Go files:
//	      `grep -rn '"EVOLVE_STRATEGY"\|"EVOLVE_RESET"\|"EVOLVE_SHIP_RELEASE_NOTES"' go/ \
//	       --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'` → 0 matches
//
//	AC8 (full suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test ./cmd/evolve/... ./internal/flagregistry/...
//	      ./internal/releasepipeline/... ./internal/phases/ship/...`
//	  (b) no FAIL packages in output
//	  (c) `go build ./...` exits 0
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C44_001 — 3 flags must be ABSENT from Lookup (any hit = flag still registered).
//	            C44_NEG_ExactRowCountIs65 — registry must be EXACTLY 65; over- or under-removal fails.
//	Edge/OOD:   C44_NEG_ExactRowCountIs65 catches both <65 (over-removal) and >65 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains — distinct assertion verbs.
//	Semantic:   registry-absence (3 flags), exact-row-count (anti-both-directions), dead-env-write
//	            absent from cmd_loop_args.go (2 flags), IPC-literal-absent from 2 prod files,
//	            IPC-channel-preserved (split-const present), ceiling-const updated,
//	            control-flags doc clean — 7 distinct behavioral dimensions.
//
// Floor binding (R9.3): predicates authored only for workflow-dead-ipc-44
// (sole top_n task). Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=8 (C44_001, C44_002, C44_003, C44_004, C44_005, C44_006, C44_009, C44_010,
//	             C44_NEG_ExactRowCountIs65 = 9 funcs including the NEG)
//	manual+checklist=2 (AC7, AC8)
//	unverifiable-remove=0
//	total AC count=10 + 2 NEG; every AC has exactly one disposition row.
```

### `go/acs/cycle44/predicates_test.go:78` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 3 env flags that cycle-44 removes
// from the registry.
```

# Comment history: `acs/cycle16`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle16/predicates_test.go:3` — above `package cycle16`

```text
// Package cycle16 materializes the cycle-16 acceptance criteria for two
// committed top_n tasks:
//
//	phases-cli-flags — convert EVOLVE_PROFILE_DIR and EVOLVE_PERSONA_OVERRIDE
//	os.Getenv reads in phasecmd/phases.go to explicit CLI flags (--profile-dir,
//	--persona-override) parsed via flag.NewFlagSet at the top of RunPhases.
//
//	systemprompt-profile-ssot — remove the os.Getenv tier for EVOLVE_SYSTEM_PROMPT
//	in systemprompt.go by adding envchain.ResolveNoOS (3-tier: reqEnv → profile → def).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	phases-cli-flags:
//	  AC1  --profile-dir flag routes validate to alt profiles dir         → C16_001 (behavioral)
//	  AC2  --persona-override flag wires into check-coherence             → C16_002 (behavioral)
//	  AC3  EVOLVE_PROFILE_DIR env no longer honored without flag (neg)    → C16_003 (behavioral, negative)
//	  AC4  unknown flag exits non-zero                                    → PRE-EXISTING GREEN (default case returns 10)
//	  AC5  os.Getenv calls absent from phases.go                          → C16_005 (config-check, waiver)
//	  AC6  full phasecmd suite                                            → manual+checklist
//	  AC7  ship/commitgate no regression                                  → manual+checklist
//
//	systemprompt-profile-ssot:
//	  AC8  process env alone no longer overrides EVOLVE_SYSTEM_PROMPT     → C16_004 (behavioral)
//	  AC9  reqEnv still wins over profile                                 → manual+checklist (existing test green)
//	  AC10 profile returned when reqEnv absent                            → manual+checklist (existing test green)
//	  AC11 full systemprompt suite                                        → manual+checklist
//	  AC12 envchain no regression                                         → manual+checklist
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C16_003 (env var IGNORED — setting EVOLVE_PROFILE_DIR without
//	           --profile-dir flag no longer redirects validate) +
//	           C16_004 (process env IGNORED — EVOLVE_SYSTEM_PROMPT in process
//	           env no longer overrides profile when nil reqEnv).
//	Edge/OOD:  C16_003 uses default project dir (no alt) while env points elsewhere.
//	Lexical:   RunPhases / Resolve / FileNotContains — three distinct verbs.
//	Semantic:  CLI flag routing (C16_001), persona wiring (C16_002), env rejection
//	           (C16_003), tier removal (C16_004), source absence (C16_005).
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (phases-cli-flags, systemprompt-profile-ssot). No deferred tasks.
```

# Comment history: `acs/cycle413`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle413/predicates_test.go:3` — above `package cycle413`

```text
// Package cycle413 materializes the cycle-413 acceptance criteria for three prompt-optimization tasks:
//   - strip-ondemand-heading-prefix-match (Task A)
//   - compact-prompts-config-enable (Task B)
//   - real-doc-ondemand-strip-guard (Task C)
//
// Goal: activate the doubly-dormant CompactPrompts lever — fix the heading match
// (exact equality → line-anchored prefix) and wire the config knob so ~23 KB of
// on-demand reference tail is stripped from per-cycle agent dispatches.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	strip-ondemand-heading-prefix-match:
//	  AC1 production heading "## Reference Index (Layer 3, on-demand)" triggers strip  → C413_001 (RED)
//	  AC2 inline mention of production heading does NOT trigger strip (negative)        → C413_002 (pre-existing GREEN)
//	  AC3 bare "## Reference Index" heading still stripped after fix (edge/OOD)        → C413_003 (pre-existing GREEN)
//
//	compact-prompts-config-enable:
//	  AC1 RoutingConfig has CompactPrompts bool field                                   → C413_004 (RED)
//	  AC2 config.Load populates CompactPrompts from registry workflow.compact_prompts   → C413_005 (RED)
//	  AC3 no literal CompactPrompts: true in phase constructors (anti-gaming)           → C413_006 (pre-existing GREEN, config-check)
//
//	real-doc-ondemand-strip-guard:
//	  AC1 realdoc_strip_test.go exists and is git-tracked                              → C413_007 (RED)
//	  AC2 StripOnDemandSections on real auditor doc shrinks body ≥ 4096 bytes          → C413_008 (RED)
//	  AC3 tdd-engineer doc returned unchanged (no Reference Index tail, negative)      → C413_009 (pre-existing GREEN)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: production heading as inline mention → C413_002 (no-op must NOT strip);
//	          literal CompactPrompts: true present → C413_006; no-op leaves auditor body unchanged → C413_008.
//	Edge/OOD: bare heading still works after prefix change → C413_003;
//	          tdd-engineer has no heading → C413_009.
//	Semantic:  reflection-based field presence (C413_004) vs. config-load value (C413_005) are distinct behaviors.
//
// Deferred (zero predicates per R9.3): B1 (externalize tdd/triage on-demand content),
// B2 (per-cycle context injection audit).
```

### `go/acs/cycle413/predicates_test.go:199` — above `if reduction < 256 {`

```text
// Recalibrated 2026-08-10 (was 4096, latent-red since #434): that floor
// fossilized the auditor's operational tail below a mid-file marker —
// the persona-strip lobotomy incident. The marker now sits at EOF; the
// predicate keeps pinning the heading-prefix match against the real doc.
```

### `go/acs/cycle413/predicates_test.go:213` — above `fixture := "# Agent\n\nOperational content, no reference-index heading.\n"`

```text
// Re-anchored 2026-08-10 (persona-strip lobotomy incident): this predicate
// pinned tdd-engineer.md as marker-LESS (strip must be a no-op on the real
// doc) — contradicting cycle-415's marker requirement (latent-vs-CI, the
// incident's root-cause shape) and today's EOF marker. The no-op-when-
// heading-absent edge it exercised is preserved on a fixture body.
```

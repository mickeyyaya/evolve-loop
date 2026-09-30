# Comment history: `acs/cycle754`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle754/predicates_test.go:3` — above `package cycle754`

```text
// Package cycle754 materializes the cycle-754 acceptance criteria for the sole
// committed top_n task token-resolver-production-wiring (triage-report.md
// ## top_n; the scout's second selection, retro-bridge-timeout-width10, was
// DEFERRED by triage, so per R9.3 no predicates bind to it).
//
// Task source: inbox id token-resolver-production-wiring (weight 0.96),
// corrected by cycle-754 scout: composition-root wiring already landed, but
// tokenusage.DefaultResolver chains ONLY the transcript tier, so every
// tmux-driven launch (the production majority) records "source":"none" with
// zero tokens — confirmed live across 124 .evolve/runs/*/llm-calls.ndjson.
//
// AC map (1:1), from scout-report.md "Acceptance Criteria Summary" Task 1:
//
//	AC1 DefaultResolver chains all 3 tiers in fidelity order → C754_002
//	    (scrollback tier reachable) + C754_003 (transcript still wins —
//	    anti-reorder pin)
//	AC2 no-transcript launch with events log resolves via
//	    EventsResultCollector                             → C754_001 (resolver
//	    layer) + C754_006 (engine end-to-end)
//	AC3 SourceNone when no tier has data (no fabrication) → C754_004 +
//	    C754_007 (engine-level negative anti-stamp)
//	AC4 malformed events log falls through cleanly        → C754_005
//	AC5 real-cycle `evolve tokens report` shows non-zero  → manual+checklist
//	    (needs a genuinely tmux-driven cycle; see test-report.md)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in the target package, which EXERCISES DefaultResolver /
// Engine.recordTokenUsage against real on-disk fixtures — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
```

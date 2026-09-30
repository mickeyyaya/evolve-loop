# Comment history: `acs/cycle1275`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1275/predicates_test.go:3` — above `package cycle1275`

```text
// Package cycle1275 materializes the cycle-1275 acceptance criteria for this
// fleet lane's sole committed task, mint-carries-select-metadata (triage top_n
// id: mint-carries-select-metadata).
//
// Goal: the advisor's phase MINTER must satisfy the catalog SELECT-metadata
// contract itself instead of being papered over after the fact by
// metadataAllowlist entries. Today router.MintSpec carries no
// Description/WhenToUse channel, so mintConfigsFrom
// (go/internal/core/phase_advisor.go:986) constructs every minted
// phasespec.PhaseSpec with empty SELECT metadata, and
// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata only stays green because
// each concrete minted name is hand-added to the shrinking allowlist (#404,
// #406).
//
// Every predicate below EXERCISES THE SYSTEM UNDER TEST through its PRODUCTION
// caller: core.PhaseAdvisor.Plan — the router.Planner entry point the
// orchestrator calls — which composes the real plan prompt
// (composePlanPrompt → writePlanResponseSchema) and parses the advisor's
// response through the real parsePhasePlan → mintConfigsFrom path. None call
// the minter directly and none grep source text, so none can pass on dead code
// or on a magic string (the cycle-85 degenerate-predicate failure mode).
//
// RED today: 001/003 fail on empty Description/WhenToUse and on a plan prompt
// that never mentions the two keys. The Builder makes them GREEN by adding the
// two omitempty fields to router.MintSpec, threading them through
// mintConfigsFrom into the minted phasespec.PhaseSpec, and extending the
// plan-prompt mint-block documentation + JSON example — WITHOUT modifying this
// file.
//
// SUT CONTRACT the Builder must implement (see test-report.md handoff):
//
//	package router
//	    type MintSpec struct {
//	        …
//	        Description string `json:"description,omitempty"`   // one line: what the phase produces
//	        WhenToUse   string `json:"when_to_use,omitempty"`   // the signal that should trigger SELECTing it
//	    }
//
//	package core
//	    mintConfigsFrom copies e.Mint.Description / e.Mint.WhenToUse into the
//	    constructed phasespec.PhaseSpec (same JSON keys as phasespec's own tags).
//	    writePlanResponseSchema documents both keys and shows them in the
//	    strict-JSON mint example.
```

### `go/acs/cycle1275/predicates_test.go:165` — above `func TestC1275_003_PlanPromptDocumentsMintMetadata(t *testing.T) {`

```text
// TestC1275_003_PlanPromptDocumentsMintMetadata proves the other half of the
// fix: an advisor that is never TOLD about the keys will never emit them, so
// the composed plan prompt must document description/when_to_use and show them
// in its strict-JSON mint example. Asserted on the prompt the bridge actually
// received, for BOTH prompt-assembly paths (persona = production,
// no-persona = legacy inline fallback) — the pair that silently diverged at
// #293.
```

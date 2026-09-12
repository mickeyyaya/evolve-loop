# Build Explanation — Cycle 1604

## Build Binding
- Cycle: 1604
- Base SHA: 91e955ca1d946d9181f2de29c651ab34d83e1dbc

## Summary
The live TDD composer now receives a bounded, deterministic digest of sealed upstream handoffs.

## Rationale
Rendering only the active PhaseIO path preserves legacy prompts while giving live TDD runs compact context. JSON quoting and a separately closed text fence treat agent-authored data as untrusted.

## Changed Areas
- `.evolve/evals/phaseio-handoff-digest-contract.md` — records the behavioral contract for bounded, deterministic handoff projection.
- `.evolve/evals/runner-consumes-handoff-digests.md` — records the production-caller and inactive-path contract.
- `go/acs/cycle1604/predicates_test.go` — supplies TDD-owned acceptance tests for the digest and live composer seam.
- `go/internal/phaseio/digest.go` — adds the deterministic rune-capped, JSON-safe upstream digest.
- `go/internal/phaseio/digest_test.go` — covers boundary, deterministic, safety, and present-versus-absent digest behavior.
- `go/internal/phases/tdd/tdd.go` — renders the digest only for active input inside a closed untrusted-data fence.

## Design Decisions
The digest uses fixed typed-view order and sorted generic keys. Degraded reads precede generic data so a large generic value cannot silently turn a read failure into clean absence.

## Verification
Targeted PhaseIO and TDD package tests, the eight cycle predicates, and the native ACS suite cover the public API and real `ComposePrompt` path.

## Compatibility
Inactive PhaseInput requests retain byte-identical legacy prompt output. The new API defaults only a non-positive caller cap.

## Limitations
The digest is a compact prompt projection, not a replacement for source handoff artifacts.

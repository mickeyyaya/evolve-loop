# Build Explanation — Cycle 1942

## Build Binding
- Cycle: 1942
- Base SHA: df4ae9c1e839dd1060d41d1a4248fc6743bfab3a

## Summary
The build-planner, swarm-plan, debugger and intent prompts now carry the upstream handoff digest of their own edge. Seven more internal/core functions reach 100.0% coverage under behavior tests. A new inbox item carries the rest of the internal/core coverage work.

## Rationale
The build hook already renders `runner.UpstreamDigestBlock(req.Input.Upstream().DigestFor(phase))` when the typed input is active (go/internal/phases/build/build.go:48-50). The four other hooks copy that one guard, so each phase gets only the sections that `phaseio.EdgeUpstreams` declares for it. A shared helper was rejected: it saves one line per hook and adds an API that only these hooks use.

## Changed Areas
- `go/internal/phases/buildplanner/buildplanner.go` — ComposePrompt appends the build-planner edge digest when the input is active. An inactive input keeps the old bytes.
- `go/internal/phases/swarmplan/swarmplan.go` — ComposePrompt appends the swarm-plan edge digest in the same way.
- `go/internal/phases/debugger/debugger.go` — ComposePrompt appends the debugger edge digest (triage and build) after the Ship Failure Envelope.
- `go/internal/phases/intent/intent.go` — ComposePrompt has the same guard. The intent edge is declared empty, so no block renders today; the guard makes a later edge declaration take effect.
- `go/internal/phases/buildplanner/upstream_digest_prompt_test.go` — proves the declared triage section renders, an undeclared build section does not, and an inactive input renders no block.
- `go/internal/phases/swarmplan/upstream_digest_prompt_test.go` — the same proof for swarm-plan.
- `go/internal/phases/debugger/upstream_digest_prompt_test.go` — the same proof for debugger, with an undeclared audit section.
- `go/internal/phases/intent/upstream_digest_prompt_test.go` — proves the empty intent edge renders no upstream data.
- `go/internal/core/core_coverage_batch_diff_test.go` — `TestCoreCoverageBatch_*` behavior tests for the unified-diff parser (mergerung2.go), defectHead and causeHead (failure_digest.go) and stampingLedger.Verify (runid.go).
- `.evolve/inbox/2026-10-18T07-00-00Z-core-package-coverage-batches-after-1942.json` — the remainder item for the other internal/core functions and the .cover-strict registration.

## Design Decisions
The digest goes after all other context lines in each hook, as in build.go. Production code in internal/core is not changed; the batch is tests only. The remainder item links to both parent items through `connects_to`.

## Verification
`go test -tags acs -count=1 ./acs/cycle1942` passes 6/6. The new package tests fail against the base versions of the three hooks and pass with the change.

## Compatibility
An inactive PhaseInput gives a byte-identical prompt for build-planner and swarm-plan (predicates 001 and 002). No exported API changes.

## Limitations
internal/core is not yet at 100.0%, and it is not registered in go/.cover-strict. The queued remainder item owns that work.

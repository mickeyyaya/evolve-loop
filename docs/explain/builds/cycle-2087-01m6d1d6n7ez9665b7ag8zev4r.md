# Build Explanation — Cycle 2087

## Build Binding
- Cycle: 2087
- Base SHA: dedc78592a1b9ccc4c4c65b1636e1f0725cb1abd

## Summary
The chain boundary rebuild now calls the boot refresh build function. One function spawns the make build command for the boot refresh, the chain boundary and the evolve rebuild verb.

## Rationale
The chain boundary adapter held a second copy of the make build call. Acceptance 3 requires one build function. The adapter now captures the output of the shared function in a buffer and keeps its error text.

## Changed Areas
- `go/cmd/evolve/cmd_loop_chain.go` — defaultChainRebuild calls defaultBootRefreshRebuild and wraps the output tail in its error.
- `go/cmd/evolve/cmd_rebuild_singlehome_test.go` — tests that count the make build call sites and compare the verb and boot seams.
- `go/acs/cycle2087/predicates_test.go` — ACS predicates for the acceptance criteria.
- `.evolve/evals/cli-rebuild-console.md` — the eval for this task.

## Design Decisions
The adapter calls the default function, not the seam variable, so a test that swaps the seam does not change the chain behavior.

## Verification
The cmd/evolve unit tests, the integration tests for both adapters and the ACS suite pass.

## Compatibility
The chain error text stays "make -C go build: <err>: <trimmed output>".

## Limitations
The verb itself was shipped in an earlier cycle; this cycle only unifies the build call.

# Build Explanation — Cycle 1938

## Build Binding
- Cycle: 1938
- Base SHA: 2ab5823a7237968e8e5c193e04043f4f3c253441

## Summary
First, every `ComposePrompt` phase edge now has explicit handoff config. `phaseio.EdgeUpstreams` lists the upstream phases of each edge. `Handoffs.DigestFor` renders only those sections under the 1024-rune cap. The triage and build prompts consume that digest. Second, five `internal/core` functions reach 100.0% coverage under the `TestCoreCoverageBatch_*` tests, and `writeBuildSelfCheckArtifact` reaches 81.8%. Third, this branch carries the cycle-1934 salvage, which makes `cover_changed.stage=enforce` the compiled default.

## Rationale
The tdd prompt rendered the full upstream digest, and the triage and build prompts rendered none. A per-edge table makes the flow reviewable in one place, and the audit edge stays empty, so the auditor never reads the builder's self-report. The table lives in `internal/phaseio`. Built-in phases have no tracked phase file, so a table in `phasespec.IO` can split over two sources. The core batch follows the inbox item's rule: one function group per cycle, with one behavior test for each uncovered branch.

## Changed Areas
- `go/internal/phaseio/digest.go` — adds `EdgeUpstreams` and `DigestFor`; splits `UpstreamDigest` into `sectionLines`, `genericLines` and `capDigest`, so both digests share one renderer and one cap. `UpstreamDigest` output stays byte-equal.
- `go/internal/phaseio/edges_test.go` — tests the edge table, the copy semantics, section filtering, the `<upstream>.` key prefix, the cap, and the degraded, unencodable and tiny-cap paths of `UpstreamDigest`.
- `go/internal/phases/runner/handoffdigest.go` — `UpstreamDigestBlock`, the one formatter of the `## Upstream Handoff Digest` block that the tdd, triage and build prompts share.
- `go/internal/phases/runner/handoffdigest_test.go` — tests the empty and non-empty block.
- `go/internal/phases/triage/triage.go` — the active-envelope prompt renders `DigestFor("triage")` (scout only).
- `go/internal/phases/triage/upstream_digest_prompt_test.go` — proves the scout digest renders, the build handoff does not leak, and a landable carry-forward branch is listed (a `gittest` repo).
- `go/internal/phases/build/build.go` — the active-envelope prompt renders `DigestFor("build")` (scout and triage).
- `go/internal/phases/build/upstream_digest_prompt_test.go` — proves the triage digest renders and the audit handoff does not leak.
- `go/internal/phases/tdd/tdd.go` — uses `UpstreamDigestBlock` with the unchanged `UpstreamDigest(0)`; the bytes stay equal, as cycle1632 predicate 010 requires.
- `go/internal/phases/tdd/upstream_digest_prompt_test.go` — pins the full tdd digest block.
- `go/internal/core/core_coverage_batch_test.go` — the `TestCoreCoverageBatch_*` tests for `worktreeBaseTreeSHA`, `recordBuildBinding`, `recordGenericBinding`, `packageNewThisCycle`, `writeBuildSelfCheckArtifact` and `contentionBackoff`, through the `gitRunner` subprocess seam and real temp directories.
- `go/internal/core/parallel_default_test.go` — lists the four batch tests that swap `gitRunner` as serial; the other two run in parallel.
- `go/cmd/evolve/build_handoff_probe_replay_test.go` — the green probe fixture now adds a test for the function it adds. The `enforce` default refused the untested fixture function.
- `docs/architecture/packages/internal-phaseio.md` — documents the per-edge digest and its pinning tests.
- `.evolve/inbox/2026-10-18T06-00-00Z-tokenopt-handoff-digests-remaining-consumers.json` — queues the intent, build-planner, swarm-plan and debugger consumers.
- `.evolve/inbox/2026-10-18T06-00-00Z-core-package-coverage-remaining-batches.json` — queues the remaining core functions and the `.cover-strict` registration.
- `.evolve/evals/tokenopt-handoff-digests-per-edge-remainder.md` — the TDD phase's eval for the digest task, unchanged by the build.
- `.evolve/evals/core-package-coverage-to-100.md` — the TDD phase's eval for the core batch, unchanged by the build.
- `go/acs/cycle1938/predicates_test.go` — the TDD phase's predicates, unchanged by the build.
- `go/internal/policy/budgets_recovery.go` — cycle-1934 salvage: the compiled default of `cover_changed.stage` is `enforce`.
- `go/internal/policy/coverchanged_config_test.go` — cycle-1934 salvage: the default test expects `enforce`.
- `go/internal/policy/coverchanged_config_amplify_test.go` — cycle-1934 salvage: table-tests the `policy.json` stage values and proves that a read of the config does not change the policy.
- `go/internal/core/cover_changed_floor_test.go` — cycle-1934 salvage: tests that relied on the old default now set the stage explicitly.
- `go/internal/core/cover_changed_floor_strict_test.go` — cycle-1934 salvage: the same explicit stage.
- `go/internal/core/cover_changed_floor_tagfilter_test.go` — cycle-1934 salvage: the same explicit stage.
- `docs/architecture/policy-config.md` — cycle-1934 salvage: the stage table shows the `enforce` default.
- `docs/operations/runtime-reference.md` — cycle-1934 salvage: the wiring text shows the `enforce` default.
- `.evolve/evals/cycle-ship-enforces-cover-changed.md` — cycle-1934 salvage: the eval of that task.
- `docs/private/research/archived-2026-10-17/superseded-predicate-packages/cycle1934/predicates_test.go` — the cycle-1934 predicates, archived because their cycle did not ship.
- `docs/private/research/archived-2026-10-17/unshipped-build-explanations/cycle-1934-01m555wgjgcarm7c1d8387c6jv.md` — the cycle-1934 explanation, archived for the same reason.

## Design Decisions
An undeclared or empty edge renders an empty digest, never the full fallback. Thus a new phase gets no upstream data until it has a declared edge. A generic key belongs to a section only when its prefix ends with a dot, so `scoutx.k` never reaches a scout edge.

The debugger edge is triage and build, because it repairs a build; it never reads the audit handoff. Rejected: the edge table in `phasespec.IO` or `.evolve/phases/*.json`, because built-in phases have no tracked file. Rejected: a build digest for audit, because the adversarial auditor must grade without the builder's self-report. Rejected: a test seam for `json.MarshalIndent`. The error is impossible for the type, and a seam adds code only to reach dead code.

Rejected: removal of that dead branch in this cycle. `phase_bindings_selfcheck.go` is on the protected control-plane surface (ADR-0064), so the removal is console work.

## Verification
`go test -tags acs -count=1 ./acs/cycle1938` passes 8/8 predicates. `go test -count=1 -tags integration -coverprofile` over `internal/phaseio`, `internal/phases/{runner,tdd,build,triage}` reports 100.0% for every touched function. The `TestCoreCoverageBatch_` run reports 100.0% for five core functions and 81.8% for `writeBuildSelfCheckArtifact`. Its encode-error branch is unreachable. `go vet ./...` and `go test -count=1 ./...` run over the module.

## Compatibility
Prompts change only for an active `PhaseInput`; the legacy map path stays byte-equal. The tdd digest block is byte-equal. `UpstreamDigest` keeps its output and its signature. With `cover_changed.stage=enforce`, the build floor now refuses a change that leaves a touched function below 100%.

## Limitations
The intent, build-planner, swarm-plan and debugger prompts do not consume their declared digest yet; the first queued remainder covers them. `internal/core` stays below 100.0% and is not in `go/.cover-strict`; the second queued remainder covers that. The dead encode-error branch of `writeBuildSelfCheckArtifact` is in a protected file; the remainder names it as console work.

# Test refactoring: integration record

Date: 2026-09-14. The [design and research dossier](../architecture/test-refactoring-design-2026-09-14.md) was reviewed and merged first in [PR #602](https://github.com/mickeyyaya/evolve-loop/pull/602). Implementation followed in isolated worktrees with separate file ownership and independent cross-review. The active runtime checkout and its user state were not edited or synchronized.

## Delivered changes

| Slice | Change | Evidence |
|---|---|---|
| S1: coverage gates | Preserve failed child/tool status, validate manifest/profile inputs, retain existing strict/API enrollments | [CI implementation](test-gates-ci-implementation-2026-09-14.md) |
| S2: structure checker | Replace fifteen copied scanners with a stdlib helper; traverse `else`; retain all fifteen callers and limits | [Checker implementation](test-structure-checker-implementation-2026-09-14.md) |
| S2: fixtures | Match real storage snapshot isolation; report artifact filesystem errors without losing scripted errors | [Fixture implementation](test-fixtures-implementation-2026-09-14.md) |
| S3: CI parity | Share integration/API recipes and release workflows; retain no-race E2E tags; test landing before build; add supported Go alongside minimum-version checks | [CI implementation](test-gates-ci-implementation-2026-09-14.md) |
| S4: consolidation pilots | Remove one proven cyclecost duplicate; combine two prompt-echo cases while retaining inputs, assertions, API references and ACS selectors | [Fixture implementation](test-fixtures-implementation-2026-09-14.md) |
| S2 follow-on: release test isolation | Build the real current CLI source in a temporary repository, retaining the version-stamp assertion without replacing the source checkout's executable | [Release build isolation](test-release-build-isolation-2026-09-14.md) |

The design's full static inventory remains a dated baseline: 2,902 tracked first-party test files, 14,496 AST test declarations, and 59 identical-body candidate groups. The [subsequent semantic review](test-duplicate-dispositions-2026-09-14.md) covers all 59 groups / 161 declarations: ten retain distinct contracts, two groups have implemented consolidation, and 47 remain conditional candidates with explicit missing proof. This is not an assertion-by-assertion audit of the whole repository. Only the documented, behaviorally verified cases were consolidated.

## TDD and independent review

The checker, fixture and Make-gate defects were reproduced as semantic assertion failures before implementation; their reports record the commands and unchanged-test GREEN results. The release-build isolation change preserves an already-green test while removing an observed checkout side effect, with unchanged coverage and a version-stamp mutation control. For previously correct behavior, isolated mutations establish assertion sensitivity; intentionally broken production code was never committed just to manufacture RED evidence.

Each slice received both a simplification review and a Go/test correctness review from an agent who did not author it, plus the integration owner's review. Review produced additional actionable cases:

- A filesystem failure and scripted bridge error must both remain in the returned error chain, with the scripted response unchanged.
- A readable directory cannot satisfy the strict-coverage manifest contract.
- A negative subprocess test cannot count its own timeout as the expected product failure.
- The landing workflow guard must reject publication from pull requests; a reversed comparison cannot satisfy its test.

Known-good controls accompany negative cases. The fixture's prior empty-artifact/path behavior and successful artifact-with-scripted-error behavior remain. The real filesystem storage adapter establishes the snapshot contract. Existing architecture/API-name tests and acceptance identities remain valid obligations.

## Baseline and preservation

Local baseline toolchain: Go 1.27.1, darwin/arm64. Full runtime baseline command, from `go/`:

```sh
go test -race -count=1 -p 4 -parallel 4 -json \
  -coverprofile=/tmp/evolve-full-baseline-2026-09-14.cover ./...
```

Result: **232 passing packages, 16,224 passing test/subtest events, zero failures**, three package-level no-test skips and fourteen test skips. `go vet ./...` passed. The earlier separately executed landing baseline passed all six packages: 94 top-level tests, 103 passing events, no skips.

The fourteen runtime skips are accounted for below. Preserving a skip is not evidence that its skipped behavior executed.

| Reason | Count |
|---|---:|
| Explicit live-model opt-in absent | 2 |
| Process helper bodies invoked separately by their parent stress tests | 2 |
| Local ledger unavailable in isolated checkout | 2 |
| Linux-only bwrap test on macOS | 1 |
| Local replay directory not supplied | 1 |
| Optional memo pin absent | 2 |
| Legacy Bash script absent / Bash-parity opt-in absent | 2 |
| Permanently disabled live-tree-mutating API reproduction | 1 |
| Existing unsynthesized Git-state case | 1 |

The old disabled API reproduction remains historical; the new gate tests use throwaway modules. Legacy parity and the unsynthesized Git case remain limitations rather than newly executed coverage.

Matched targeted comparisons show all fifteen scanner owner packages retain 100% statement coverage and every previously covered block. The unchanged cyclecost and phasestream production sources retain exactly the same hit-block sets, at 96.2% and 96.5% statement coverage respectively. Fixture coverage rose from 77.4% to 81.0%; because fixture source changed, that percentage is an improvement signal rather than a same-source equivalence proof. The reports preserve exact old-to-new case and selector maps and named mutation outcomes. No repository-wide mutation score or wall-clock speedup is claimed.

An independent comparison of downloaded GitHub coverage artifacts confirms preservation on **both Linux and macOS, Go 1.23**, for PRs #603 and #604 against baseline run `34803400283`. Every one of the four PR/OS comparisons retains the same **2,729 block identities and 2,716 hit blocks** across the fifteen checker owners plus cyclecost and phasestream: zero lost or added hits. Those scoped production sources and both coverage manifests are identical to the baseline. The [machine-readable receipt](test-refactoring-ci-coverage-2026-09-14.json) records per-package results, run/job/head identities and profile digests. Its `/tmp/` paths identify local download locations; the run/artifact identifiers provide the remote provenance.

The full execution comparison retains the same fourteen skipped identities. Three `TestModelsListJSONFlagOrdering` subtest names contain `t.TempDir()` paths and consequently differ between runs; their unchanged parent, three input cases and terminal results remain present. The two intentionally removed pilot identities have explicit replacements in the fixture report. A raw name-set difference alone is not interpreted as lost coverage.

During validation, a full-suite release test temporarily replaced tracked `go/evolve` with a version `9.9.9` binary. A concurrent manual ship staged that temporary file, and the commit gate correctly rejected the changed tree. Build metadata and timestamps tied the staged file to `TestDefaultRebuildBinary_NonDryRun_RealRepo`; its cleanup had restored disk contents but could not restore a concurrently captured index. After the suite finished, only the accidental binary staging was restored and the ordinary gate rerun. The follow-on isolation change removes this test's writes to the source checkout. No guard was bypassed and no generated binary entered the implementation commits.

## Integration and merge receipts

All implementation branches passed full formatting, vet and uncached default-suite race checks locally. Counts below belong to their individual branch snapshots, not an invented sum of independently executed suites. All retain the same fourteen baseline test skips and three package-level no-test skips.

| Branch | Passing packages | Passing test/subtest events | Additional evidence |
|---|---:|---:|---|
| Structure checker | 233 | 16,256 | Includes new helper; all old owner-package cases retained |
| Fixtures and pilots | 232 | 16,239 | Final review added two mixed-error subtests after this run compiled; the final fixture supplement passed all 42 events, followed by CI on the final source |
| Release build isolation | 232 | 16,224 | All 113 release-package identities retained; byte-identical 95.4% package profile |
| Gates and CI | 232 | 16,229 | Integration-tagged ciparity checks separately passed 32 top-level cases and 21 subtests |

The real strict target passed all nineteen 100% floors. Full API enforcement passed **218 measurement packages and 231 inspections**, with **4,077 covered exports and zero uncovered/false-green exports**, using `GOMAXPROCS=4 GOFLAGS=-p=4 make apicover-enforce`. All six landing packages and pinned actionlint 1.7.12 with shellcheck passed.

The first full API attempt failed honestly on four native tmux startup cases. Independent diagnostics ran those exact cases on baseline and candidate with both fresh ambient-shell and controlled-shell servers: 16 passes, no skips. The original failure did not reproduce; its exact cause remains unresolved. The bounded-concurrency full rerun used its own fresh profile and passed. Neither targeted diagnostics nor a later success reclassifies the first failed execution as passing. The [CI report](test-gates-ci-implementation-2026-09-14.md#full-local-integration-checks) records the commands and retained failure digest. No assertions, sandbox controls or timeouts were weakened.

Every published branch went through the normal review attestation and `evolve ship --class manual`; no commit-gate bypass was used. PR merges match the reviewed head and follow successful checks. Remote validation supplies Linux/macOS integration/E2E, strict/API and durable-ACS evidence, while the CI change adds the supported Go and landing lanes.

| Change | Reviewed head | Main merge | Remote checks |
|---|---|---|---|
| [Design #602](https://github.com/mickeyyaya/evolve-loop/pull/602) | `d212a0420266` | `1b4bd9d62238` | Plugin validation and durable ACS passed |
| [Checker #603](https://github.com/mickeyyaya/evolve-loop/pull/603) | `9bdbc46634b0` | `405cdf26fd39` | [Linux/macOS Go 1.23](https://github.com/mickeyyaya/evolve-loop/actions/runs/34807913729), plugin validation and durable ACS passed |
| [Fixtures #604](https://github.com/mickeyyaya/evolve-loop/pull/604) | `63d87f00fbea` | `1098026da882` | [Linux/macOS Go 1.23](https://github.com/mickeyyaya/evolve-loop/actions/runs/34807918081), plugin validation and durable ACS passed |
| [Release build isolation #605](https://github.com/mickeyyaya/evolve-loop/pull/605) | `0543f4906ba9` | `465eb45cfbbf` | [Linux/macOS Go 1.23](https://github.com/mickeyyaya/evolve-loop/actions/runs/34808800104), plugin validation and durable ACS passed |
| [Gates and CI #607](https://github.com/mickeyyaya/evolve-loop/pull/607) | `3efc783bb7eb` | `0016e0b12849` | [Linux/macOS Go 1.23 and Linux Go 1.27](https://github.com/mickeyyaya/evolve-loop/actions/runs/34809439362), landing tests/build, plugin validation and durable ACS passed; PR deployment correctly skipped |

Concurrent [PR #606](https://github.com/mickeyyaya/evolve-loop/pull/606) also changed main during integration. Its triage/refusal implementation is separate work and is not attributed to this refactor. The pinned before/after coverage comparisons above apply to their recorded PR heads; later unrelated production changes are not silently treated as identical baseline source.

Final combined-main validation: commit `0016e0b1284905f9119fabfb930f938cfda883c5` contains all four implementation PRs and concurrent PR #606. Its [Go matrix](https://github.com/mickeyyaya/evolve-loop/actions/runs/34810071738) passed on Linux/macOS Go 1.23 and Linux Go 1.27, including integration/race, E2E and strict/API gates in every job. The [plugin/ACS checks](https://github.com/mickeyyaya/evolve-loop/actions/runs/34810071734) and [landing workflow](https://github.com/mickeyyaya/evolve-loop/actions/runs/34810071746) also passed; the latter tested, built and deployed Pages after the main merge. An independent interaction review found no material conflict among the merged test changes or with PR #606. No release tag was created and no plugin release was published; reusable release topology is tested and schema-validated, rather than demonstrated by publishing a release.

## Remaining design work

S5–S7 remain a roadmap: host-owned TDD execution receipts, the legacy simulator replacement, live failure classification, further owner-by-owner consolidation, broader fuzz/mutation and live-provider calibration. Those involve additional behavior or policy contracts and are not claimed as completed by this coverage-preserving foundation. The design's unfiltered result aggregator and external branch-rule enforcement also remain separate work; repository rules were not changed.

The stricter host TDD receipt design must bind executable tests, helpers, fixtures, configuration, inventory, environment/toolchain and both source trees. The current human/agent review reports and hashed local logs are supporting evidence, not an implementation of that host enforcement. Likewise, `apicover-check` consumes a profile supplied by a successful preceding caller; CI/Make sequencing establishes freshness for these runs, not a persistent SHA-bound profile attestation.

# Test architecture and assertion review — 2026-09-14

## Scope and confidence

Read-only repository review against `80b348e6943b08b3bfc4e2d30f06f42db69c1db9`, in `/Users/danleemh/ai/claude/evolve-loop/dev/test-architecture-design-2026-09-14`. Read `AGENTS.md`, platform compatibility, operating policy, `go/docs/testing.md`, and the installed `evo:golang-test-review` skill. No test/production/workflow edits and no full suite or live model execution. One isolated copied-scanner experiment, below. Initial reads of runtime's `c6bb682c` have identical `go/` code per root's comparison.

Structural inventory is whole-tree; assertion review is a risk-selected sample, not certification of all cases. Prefer the root's AST inventory: **2,902 tracked `_test.go` files in 700 directories, 14,496 `Test` declarations (14,402 runtime + 94 landing), 11 TestMain, 8 Benchmark, 2 Fuzz**. My earlier regex count of 14,566 included embedded Go source and must not be used as a declaration total.

File counts: `go/internal` 2,178, `go/cmd` 223, `go/acs` 475, `go/pkg` 6, `go/test` 8, `landing` 12. Largest directories: core 364, cmd/evolve 214, bridge 198, ship 98, policy 66, audit 64, runner 54. Literal search finds 75 test files importing shared fixtures and 108 `apicover_named_test.go` files. Such search counts describe source occurrences, not runtime execution or coverage. There are 3 files calling `rapid.Check` (ledger and two gc tests) in addition to the 2 native fuzz targets.

## Priority findings

### HIGH — duplicated clean-code scanner misses nesting in every `else` branch

`go/internal/config/limits_test.go:68` traverses `nesting`; at lines 72–76 an `IfStmt` recursively visits `childBody` then returns false from `ast.Inspect`. `childBody` returns only `IfStmt.Body` at lines 85–86, so the `Else` subtree is never scanned. `go/internal/bridge/launchoutcome/limits_test.go:68` is the same implementation. Fourteen test bodies are identical after formatting; `go/internal/signalcenter/limits_test.go:23` is a fifteenth enrollment with an extra `t.Parallel()` and the same scanner algorithm.

**Confirmed experiment:** copied the exact config scanner into an isolated temporary Go module, changed its package name only, and provided a six-deep else control path. Its test passes despite `maxNesting = 4`. See exact reproduction below. This demonstrates an oracle blind spot, not a claim that current production code violates the limit.

**Issue / gap / solution:** a copied checker can pass incorrect code; checker behavior has no independent fixture oracle for this branch; first write failing scanner tests for else/else-if and boundary depths, then extract a small shared scanner and keep every package enrollment and exact threshold. Preserve current intended rules (`function lines <50`, `file lines <800`, `nesting <=4`); do not use deduplication to remove enrolled package checks. Keep correction of the oracle distinguishable from mechanics-only consolidation.

### HIGH — fake artifact writer can report success after failed setup

`go/test/fixtures/fakes.go:254` ignores `MkdirAll` and `WriteFile` errors at 259–260 and then sets `Resp.Stdout` to the requested artifact at 261. Identical local fake behavior appears in `go/internal/phases/build/build_test.go:27` and `go/internal/phases/tdd/tdd_test.go:28`. `TestFakeBridge_MaterializesArtifact` (`go/test/fixtures/fixtures_test.go:128`) tests only successful materialization and checks file contents as a substring.

The helper can therefore claim output even when a regular file blocks a parent directory. Before migrating many callers, establish red tests for mkdir failure, write failure, exact bytes, absent vs empty artifact, and injected bridge error precedence. Returning setup failure would strengthen the harness contract and must be reviewed as a deliberate behavior correction, not silently bundled into deduplication. No claim here that a production bridge currently loses artifacts.

### HIGH — timing-based fast test can assert before startup and uses sleep as synchronization

`go/internal/dashboard/server_test.go:175` sleeps 60ms twice around `s.current()` and only compares sequence numbers. `newTestServer` (`:19–25`) launches `s.Run` asynchronously with a 10ms poll interval. There is no barrier proving the first sample follows initialization or that an unchanged poll occurred between samples. A slow scheduler can yield 0 then 1 (false failure); no useful polling could yield equal samples (weak success). The file is untagged.

Use an observed-initialization barrier and a deterministic poll/tick seam, then assert one explicit unchanged refresh preserves the sequence and one changed refresh increments it. Preserve the real HTTP/SSE integration separately. Do not "fix" by enlarging sleeps. Another concrete timing example is `go/internal/bridge/channel/e2e_test.go:42–45`, whose comment explicitly relies on a 10ms separation letting a 1ms ticker process logs. Filename `e2e_test.go` does not apply an e2e build tag.

### MEDIUM — named API tests sometimes describe more than their oracle checks

`go/internal/phasestream/apicover_named_test.go:25` invokes `Emit` twice and checks kind, severity, data, CLI/cycle/phase, schema, sequence 1/2. Its comments promise a unified counter across line and rule events and stamped trace/source. It does not interleave `Line` and `Emit` or check trace ID, producer, agent, timestamp. Current implementation uses one `newEnvelope` (`classify.go:80–95`), but a future split counter could satisfy this sampled test. Add an independent expected sequence for `Line → Emit → Line`, check complete source/trace, and retain existing assertions. This is a sampled oracle gap, not proof no other suite covers the property.

`go/test/fixtures/fixtures_test.go:157` calls `FixedClock` only twice although the distinguishing documented property is behavior from the third call onward (`clock.go:14–17`). A two-position legacy clock would pass this test. Add the third/fourth-call, zero-step and concurrent-unique-index contracts before standardizing more clock users.

## Eight reviewed consolidation families

| ID | Evidence | Assertion comparison and disposition | Preservation requirements |
|---|---|---|---|
| D1 | `go/internal/cyclecost/coverage_test.go:62` and `cyclecost_test.go:134` | **Confirmed semantic duplicate.** Both create the same empty `cycle-1` directory, call `SummarizeCycle(ws,1)`, and require `errors.Is(err,ErrNoLogs)`. | One canonical behavior with old→new case mapping. Keep missing workspace and workspace-is-file cases (`cyclecost_test.go:146,154`) distinct. Check named-test/ACS selectors before removing either wrapper. |
| D2 | `go/internal/phasestream/apicover_named_test.go:57` and `prompt_echo_c654_test.go:34` | **Near duplicate behavior; inputs differ.** Both configure a reviewer prompt, suppress exact `missing rate limits.`, then require real 429 output. One prompt is longer; trace IDs differ. | Table-consolidate retaining both prompt strings and both positive/negative assertions. Maintain API-symbol evidence and references to old `TestC654_003...`; preserve special empty/non-echo cases elsewhere. |
| D3 | `go/acs/cycle1013/predicates_test.go:46,65,74,84,94` and `cycle1015/predicates_test.go:57,77,89,100,111` | **Confirmed repeated execution witnesses.** Same helper runs `go test -count=1 -v` against the exact same cmd/evolve package, exact same four test names, same exit and PASS checks. | Keep distinct cycle acceptance provenance. Canonical regression owns four real behaviors; cycle IDs retain mappings. Removing wrappers needs ACS selection/manifest history review. Across one historical-cycle invocation there may be no repeated execution; do not promise a speed gain without observed job selection. |
| D4 | `go/internal/config/limits_test.go:24`, `bridge/launchoutcome/limits_test.go:24`, plus 12 identical bodies and signalcenter variant | **Repeated scanner implementation, distinct source scopes.** Each scans its own current directory with the same limits. | Share scanner, retain all **15** source-package enrollments, thresholds and diagnostics; fix verified else blind spot with independent red tests first. Never replace 15 obligations with one package's test. |
| D5 | `go/internal/phases/build/build_test.go:20,216,224,232`; `tdd/tdd_test.go:21,158,166,174`; audit `audit_test.go:551–587`; `go/test/fixtures/fakes.go:244` | **Same fixture implementation, constructor assertions are not redundant.** Missing bridge/prompts and bridge failure hit different phase constructors. Clone fake behavior matches shared FakeBridge except configurable probe behavior/mutex. | Migrate fixtures once its contract is pinned. Keep per-phase constructor calls and error/FAIL assertions; only use a matrix if it retains every phase constructor as a row. Maintain constructor-specific prompt paths/artifact contracts. |
| D6 | `go/internal/ledgerverify/verify_test.go:420,437,450,462,470,482`; `verify_coverage_test.go:24`; `apicover_named_test.go:13` | **Repeated setup, mostly distinct cases.** Per-cycle precedence, fallback, trimmed verdict, all missing, malformed JSON, string-not-bool and missing key cannot be collapsed semantically. Named API case verifies both fields together. | A table can standardize files, roots, expected full struct and case names. Retain different input combinations and the combined-fields case. Do not call all false-return assertions duplicates. |
| D7 | `go/internal/phasestream/produce_coverage_test.go:12,32,52,74`; `produce_branches_test.go:14,44,63` | **Repeated filesystem arrangement, distinct failures.** Stdout open, stderr open, create-temp, rename, encode, empty-input file creation, ENOTDIR stat exercise different layers and cleanup obligations. | Preserve error origin and cleanup assertions, avoid moving branch-specific assertions into a generic non-nil error check. Capture permission cases skipped as root. Any injected-I/O version must retain one real filesystem integration witness. |
| D8 | `go/internal/core/orchestrator_test.go:23–27` vs `go/test/fixtures/fakes.go:10`; `go/test/fixtures/clock.go:14–17` vs local clock helpers | **Intentional non-equivalence / architectural exception.** White-box `package core` imports cannot use fixtures because fixtures imports core. FixedClock increments forever; legacy two-position clocks stop after call two. | Preserve white-box local fakes (or move only exported-API tests to core_test when justified). No blanket import migration. Parameterize fresh clocks appropriately and prove 3+ call semantics. Do not create a large new framework solely to eliminate this exception. |

The underlying D3 command tests were read: `go/cmd/evolve/cmd_tokens_test.go:185` asserts text identity/cycle, no false-positive provider, JSON count and provider; `:237` checks zero-phase render; `:263` rejects false signals including quota-abort; `:289` requires surfaced tripwire and no ESC byte. These four are **distinct** behaviors and must all remain even if their ACS wrappers are consolidated.

## Tier/module architecture and gaps to retain in the design

The existing two axes are useful and should remain: build tags select cost, while package organization describes granularity (`go/docs/testing.md:7–21,48–63`). Do not invent `unit` tags that exclude the default suite. Ordinary co-located white-box/black-box tests dominate; `go/test/component` holds a storage durability test, `go/test/integration` a git workspace test, `go/test/e2e` a binary version smoke, and full-cycle e2e also lives under cmd/evolve. Moving files into central tier directories is not itself stronger coverage.

`go/test/trustkernel/trustkernel_test.go:39–90` constructs a passing/failing minimal ACS package and executes the real `acssuite.Run`. Its empty passing test is intentional controlled harness input, not a vacuous application-behavior oracle: assertions check host verdict computation. This distinction matters to automated "empty test" detectors. Routing/state-machine/profile checks in that file remain separate safety obligations.

ACS consists of 475 tagged files, including 47 `go/acs/regression` and 2 redteam files. Historical cycle packages, durable structural regressions, and current-cycle acceptance selection have different lifecycles. Archive provenance rather than renaming/removing by `cycle*` filename alone. A same-named `go test -run` missing its target can still exit 0: preserve exact execution receipts when refactoring names.

`go/pkg/phaseproto` has five wire benchmarks; adapter ledger two; bridge/panestream one. Native fuzz targets are `clihealth/resetparse_fuzz_test.go:19` (fixed clock, zero-value/future/24h invariants) and `bridge/usageclassify_fuzz_test.go:14` (determinism, malformed family/path and provider seeds). Passing seed runs do not prove CI ran mutation fuzzing. Expand by risk (artifact/NDJSON parser, event-order/correlation, path resolution, ledger append/truncation, phase-state sequences), using independent oracles and persisted failure seeds.

Landing is a separate Go module with 12 test files / 94 AST tests. Samples include local `httptest` image API tests (`landing/cmd/genimage/run_test.go:51`) and source-sensitive CSS contracts (`landing/internal/buildsite/pipeline_demo_stacking_test.go:124,142,162,186`). Preserve request/error/bytes checks; do not replace CSS source contracts with visually similar snapshots alone. Shared `apiEndpoint` mutation and `t.Setenv` (`run_test.go:37–48`) make blanket `t.Parallel` unsafe. Local HTTP test servers do not mean live external-model evaluation.

Fast-tier time.Sleep search produced 31 candidate files, but some occurrences are embedded source fixtures; root AST inventory is authoritative for actual calls. Likewise fixture import count is not a compliance grade: pure tests need no fixture, and dependency cycles forbid some reuse. Avoid bulk `t.Parallel`, build-tag changes or automated dedup purely from syntax.

## Suggested staged work with preservation gates

1. **Baseline and mapping:** pin SHA, Go version/OS/tags/env, enumerate actual test/subtest identities and required selectors, capture per-package/per-tier statement coverage and executed/pass/skip receipts. Record behavior dimensions and assertions for each proposed mapping. Coverage percentage alone is not sufficient.
2. **Harness correctness first:** independent red tests for scanner else branches, FakeBridge write failures, third-call clock semantics, expected command/writer faults; then the smallest fix. Retain case-level red→green evidence. Snapshot aliasing and concurrent calls deserve targeted tests if central fakes are expanded.
3. **Pilot D1 + D2:** consolidate one exact duplicate and one retained-input table; run old and new tests on the same baseline/mutants, preserving known-failure detection, exact outputs/errors and required name selectors. No semantics removed because file coverage appears unchanged.
4. **Fixture migration in touched files:** replace cloned bridge/write helpers while retaining every phase case. Preserve core's documented exception. A fixture builder should expose scenario facts; it should not compute expected outputs using the system under test.
5. **Determinism and granularity:** replace sleeps with observed barriers/injected ticks under strict TDD. Preserve a tagged real subprocess/HTTP integration witness for every extracted seam. If tags change, prove the target job runs those tests and report coverage under both old/new selection.
6. **ACS/runtime receipts:** reduce repeated orchestration only after equivalence and lifecycle mapping. Keep adversarial exit/skip/no-test/timeout checks and cycle provenance; shard only independent resources. Validate CI selectors and tests together.
7. **Broaden only from evidence:** add fuzz/property/model sequence tests from incident and risk inventory; seed provider recordings are versioned, redacted, and deterministic. Live-provider tests remain a separate repeated-trial evaluation with explicit grading criteria and captured failure traces.

Acceptance for each refactor: exact old→new mapping, unchanged or stronger assertion dimensions, no newly skipped required tests, per-package/per-tag coverage baseline preserved, named API/ACS selector receipts remain valid, selected mutant/known-bug detection preserved, race/shuffle checks appropriate to affected state, and measured latency under matched conditions. No coverage or performance improvement has been claimed from this review.

## Exact scanner reproduction

Temporary directory (outside repository):

`/var/folders/11/n1_42bt961s29wjcr9qxj45m0000gn/T/evolve-test-review-nesting-vn3nn3wy`

`limits_test.go` was byte-for-byte `go/internal/config/limits_test.go` except first package declaration replaced by `package nestingprobe`. `go.mod`:

```go
module nestingprobe

go 1.23
```

`nested.go`:

```go
package nestingprobe
func SixDeepElsePath() {
 if false {
  return
 } else {
  for {
   for {
    for {
     for {
      for { return }
     }
    }
   }
  }
 }
}
```

Executed in that directory:

```text
go test -run '^TestLimits_' -count=1 -v ./...
```

Observed output and process exit:

```text
exit 0
=== RUN   TestLimits_FunctionsFilesAndNestingStayWithinTheBar
--- PASS: TestLimits_FunctionsFilesAndNestingStayWithinTheBar (0.00s)
PASS
ok  nestingprobe 10.975s
```

Only this isolated copied scanner was run. Repository tests, live models, production gate verdicts and current production nesting compliance were not evaluated. Review verdict: **CHANGES REQUESTED for future broad test refactoring until these preservation/harness gates are established**; documentation-first work can proceed with these open findings stated accurately.

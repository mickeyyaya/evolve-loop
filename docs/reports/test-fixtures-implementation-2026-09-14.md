# Test fixture correctness and consolidation pilot

Date: 2026-09-14. Baseline: `80b348e6943b08b3bfc4e2d30f06f42db69c1db9`. Worktree: `dev/test-fixtures-2026-09-14`; branch: `refactor/test-fixtures-2026-09-14`. Local toolchain: `go1.27.1 darwin/arm64`.

This implements the fixture portion of S2 and the two S4 pilots from the test architecture design. The fixture fixes were witnessed failing before implementation. The pilot refactors leave production `cyclecost` and `phasestream` code unchanged. Independent review, combined repository validation and the gated merge are handled by the parent integration task; this worktree has not committed or published changes.

## Result and scope

- `FakeBridge.Launch` propagates artifact directory/write errors. It preserves scripted responses/errors when no artifact or path is supplied, and preserves successful artifact writing alongside an injected bridge error. A filesystem failure does not overwrite scripted stdout with undelivered artifact text; any injected error remains in the error chain.
- `FakeStorage` reads, writes and write-history entries own their mutable slices, including nested failure defects. The same read/write isolation tests run against the existing filesystem adapter and the fake. The clones preserve the fake's nil/empty slice shape and avoid JSON normalization. The real adapter and production state types are unchanged.
- `FixedClock`'s existing test now asserts calls one through four against explicit offsets. Production clock behavior is unchanged.
- One exact `cyclecost` duplicate was removed. Two prompt-echo tests became two named rows in the existing package, retaining both prompts, trace values, positive/negative assertions, and the direct `SetInjectedPrompt` API reference.
- Two ACS selectors were migrated after notifying the integration owner. Their predicate names, cycle identities and regression purpose remain unchanged.

No changes were made to Makefiles, workflows, coverage thresholds, real storage adapters, locks, scheduling, or live-model execution.

## RED and preservation evidence

Tests were added first, compiled successfully, and run on the old fixture implementation:

```sh
cd go
go test -race -count=1 -json ./test/fixtures > /tmp/test-fixtures-red.json
```

This exited 1 with these intended assertion failures:

| Regression | Baseline result |
|---|---|
| `TestFakeBridge_ArtifactWriteFailureIsReported/parent_is_file` | `artifact materialization error = <nil>, want filesystem error` |
| `TestFakeBridge_ArtifactWriteFailureIsReported/target_is_directory` | Same failure: the fake swallowed the write error |
| `TestStorage_StateSnapshotsDoNotAliasCallers/fake/write_input` | Mutating caller state/cycle slices changed the value subsequently read |
| `TestStorage_StateSnapshotsDoNotAliasCallers/fake/read_result` | Mutating a returned value changed the next read |
| `TestFakeStorage_WriteHistoryIsIndependent` | Mutating the stored value changed recorded write history |

The filesystem variants of both storage cases passed in the same RED run. This establishes the real adapter's existing behavior rather than inventing expected fake semantics. The shared test covers `State.FailedAt` and its nested `Defects`, `CarryoverTodos`, `TriageThroughput`, and `CycleState.CompletedPhases`, `AuditFailReasons`, `ShipFailReasons`, and `FailedAt`/`Defects`.

These preservation cases also passed before the fix: existing successful artifact materialization, empty-artifact no-write, empty-path no-write, and exact artifact bytes plus a scripted response/error. After the minimal fixture implementation, the same fixture test files passed with `-race -count=1`: 40 passing test/subtest events, no failures or skips.

The test files were unchanged between the semantic RED and GREEN runs:

| Test file | SHA-256 |
|---|---|
| `go/test/fixtures/fixtures_test.go` | `584f7ab4a39e5d6b177d1ccfd89e82e99bc8d354a3a63c8a04abea85e282890c` |
| `go/test/fixtures/storage_contract_test.go` | `6d8428b5645c65fd5a4d3afff5cb6e5e48b9449f3986fbdb4df394c1200fcb8a` |

An initial syntax error in the new storage test was corrected before the semantic RED run; that compile error is not counted as RED evidence.

Independent review then identified a gap in the combined-error contract: the filesystem-failure cases had no scripted error, while the scripted-error case wrote successfully. Both filesystem shapes now also run with a scripted error, require `errors.As` to find `*os.PathError` and `errors.Is` to find the scripted error, and assert the returned and stored responses exactly equal the original scripted response. No production code changed for this review supplement. The initial RED/GREEN hashes above identify the earlier test version; the final `fixtures_test.go` SHA-256 is `f2ca4bf4975dd7c996ef02f8d3ee26646409b8c1f678d5901b104fba0c6fab13`.

## Old-to-new assertion and selector map

| Old identity | Replacement | Preserved inputs and observables |
|---|---|---|
| `cyclecost.TestSummarizeCycle_GlobReturnsEmpty` | Existing `TestSummarizeCycle_Empty` | Fresh temp `cycle-1` directory; cycle 1; no logs; `errors.Is(err, ErrNoLogs)`; parallel-safe isolation. The two original bodies were identical. Missing workspace and malformed/log-specific cases remain separate. |
| `phasestream.TestClassifier_SetInjectedPrompt` | `TestClassifier_SetInjectedPrompt/api_contract` | Original short reviewer prompt, `trace-cover`, normalizer/adversarial-review source; echoed `missing rate limits.` emits no infra failure; genuine `Error: 429 Too Many Requests (rate limit hit)` does emit one. |
| `phasestream.TestC654_003_PromptEchoNotEmittedGenuineEmitted` | `TestClassifier_SetInjectedPrompt/cycle654_incident` | Original longer prompt with allocation/recursion and `Report exploits only.`, `trace-654`, same source and both original assertions. Cycle-641/642 and cycle-654 incident context remains with the canonical test and ACS predicates. |

Search found no external source selector for the removed cyclecost duplicate. The two prompt-echo source selector consumers were:

- `go/acs/cycle654/predicates_test.go`, `TestC654_003_NormalizerPromptEchoGate`.
- `go/acs/cycle672/predicates_test.go`, `TestC672_004_GenuineSignalsSurvive`.

Both now select the canonical parent `TestClassifier_SetInjectedPrompt`, which executes both preserved cases. Their existing helper wraps the selector in `^(...)$`; embedding a subtest slash inside that group would produce unmatched expressions when Go splits `-run` on `/`. Selecting the parent avoids changing the historical helper contract. The near-by cycle-672 wiring-test comment was also updated; the shared `hasInfraFailure` helper and real producer wiring test remain.

The final Go JSON stream contains terminal PASS events for both `TestClassifier_SetInjectedPrompt/api_contract` and `/cycle654_incident`. The selected ACS predicates also executed and passed; validation did not rely solely on compilation or exit zero with no selected tests.

## Coverage and fault sensitivity

Matched baseline and replacement command:

```sh
go test -race -count=1 -coverprofile=<profile> ./test/fixtures ./internal/cyclecost ./internal/phasestream
```

| Package | Before | After | Preservation detail |
|---|---:|---:|---|
| `test/fixtures` | 77.4% | 81.0% | Fixture implementation gained explicit error handling and snapshot clones; percentages are improvement signals, not a same-source block equivalence claim |
| `internal/cyclecost` | 96.2% | 96.2% | Same 62 coverage-block identities; same 58 covered blocks; zero lost blocks |
| `internal/phasestream` | 96.5% | 96.5% | Same 226 coverage-block identities; same 218 covered blocks; zero lost blocks |

The pilot block comparison used exact file/range block identities and hit/non-hit state, on unchanged production sources and the same local OS/toolchain/tag tuple. It did not compare raw execution counts, which properly decrease when redundant tests are removed.

Named mutants were generated outside the worktree and applied only through `go test -overlay`. The first four compare the old and strengthened/consolidated tests; the last compares the tests before and after the independent-review supplement:

| Mutant | Old witness | Replacement witness |
|---|---|---|
| Change the no-log return from `ErrNoLogs` to `nil` | Both original cyclecost tests fail | Canonical `TestSummarizeCycle_Empty` fails |
| Drop stored prompt in `Classifier.SetInjectedPrompt` | Both original prompt tests fail on echoed text | Both replacement subtests fail on echoed text |
| Make every nonblank line count as a prompt echo | Both original prompt tests fail on suppressed genuine 429 | Both replacement subtests fail on suppressed genuine 429 |
| Saturate `FixedClock` after the second position | Original two-call test passes | Strengthened test fails at call three |
| Return only the filesystem error instead of `errors.Join(err, f.Err)` at both artifact failure sites | Pre-review filesystem-only failure cases pass | Both new mixed-error cases fail because the scripted error is missing; the filesystem-only cases still pass |

These are five explicit fault experiments, not a measured repository-wide mutation score. No authoritative source file was modified for a mutant. Wall-clock timings differed under concurrent host workload, so no latency improvement is claimed.

## Completed verification

| Check | Result |
|---|---|
| Fixture/cyclecost/phasestream suite before the review supplement, `-race -count=1 -json -coverprofile=...` | 3/3 packages PASS; 148 passing test/subtest events; no skips |
| Final affected fixture suite after the review supplement, `go test -race -count=1 -json ./test/fixtures` | PASS; 42 passing test/subtest events; no failures or skips |
| `go test -race -count=1 -json ./internal/routingtest ./test/component` | 2/2 caller packages PASS; 45 passing test/subtest events; no skips |
| `go test -tags acs -count=1 -json -run 'TestC654_003_NormalizerPromptEchoGate\|TestC672_004_GenuineSignalsSurvive' ./acs/cycle654 ./acs/cycle672` | Both affected ACS packages compile; 2/2 selected predicates PASS |
| `go tool cover -func=<after.cover>` followed by `go run ./cmd/apicover -cover <after.func> -enforce ./internal/phasestream` | 36 exported, 36 covered, 0 uncovered, 0 false-green |
| `gofmt` on edited Go files; `git diff --check` | PASS |

The ACS command above uses shell single quotes around the regular expression; run it with a literal `|` (the Markdown table escapes the character). Full repository and cross-OS checks are integration-stage obligations, not claimed by this slice.

A first local apicover invocation mistakenly supplied the raw coverage profile instead of `go tool cover -func` output and reported false greens. The corrected documented command passed; the malformed invocation is not treated as a product regression or gate success.

Raw local evidence is in `/tmp/test-fixtures-{red,green,after,acs,callers,review-green}.json`, `/tmp/test-fixtures-{baseline,after}.cover`, `/tmp/test-fixtures-apicover.txt`, and `/tmp/evolve-fixture-mutants-2026-09-14/`. The mixed-error mutant used `go test -count=1 -json -overlay=<overlay.json> -run '^TestFakeBridge_ArtifactWriteFailureIsReported$' ./test/fixtures`; its baseline overlay also substitutes the pre-review test file. The following digests identify the retained local files; the commands and assertions above make the evidence reproducible after temporary-file cleanup:

| Evidence | SHA-256 |
|---|---|
| RED JSON | `1f8090bad03f02e93e731e425e1345ad177e9e482fad95f68e959bb6fef67bab` |
| GREEN fixture JSON | `08653549ac12cac6b74c033314d04c883f3b973a46addefed53daad182405d03` |
| Baseline coverage | `50a4493297ed7876315cacf785761e1b24887dbf86e30ea1efdb06efcb381ec2` |
| Replacement coverage | `0fdd8f4fb88f72ded1f49ca79d3394860752dfb135ff6efb291f12b317902150` |
| ACS JSON | `c24c281dde49067dcfb265a9d5b3ca97f9b2a8726ae3085ff7a94b47d7286f3c` |
| Caller JSON | `ab2b4c3b352387b2475badc87105ce18b05a565d958e75bf062b03bee7480703` |
| Final review GREEN fixture JSON | `c438d83318275e8eceaa593d60a4c32d270496926c11de39f713630392888779` |
| Mixed-error mutant, pre-review test JSON | `45d89f6017221699e4db1953780c2dca5b7421f5f5f91ae2043fbb49773c0441` |
| Mixed-error mutant, final test JSON | `a773fb924535b404987dfac42a02b9428dbf4d5caaf6bb649e4c5e1d04719fce` |

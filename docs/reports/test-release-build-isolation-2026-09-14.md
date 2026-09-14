# Release build test isolation

Date: 2026-09-14. Baseline: `1b4bd9d62238796d3a83a484c0bda42b5fafb2b5`. Worktree: `dev/test-release-build-isolation-2026-09-14`; branch: `refactor/test-release-build-isolation-2026-09-14`. Local toolchain: `go1.27.1 darwin/arm64`.

`TestDefaultRebuildBinary_NonDryRun_RealRepo` now builds the real CLI from a temporary copy of the current Go module. The build output never targets the source worktree's tracked `go/evolve`. Production code, test identity, tool-availability skips, default build tags, output-existence check, and executed `--version` assertion remain unchanged.

## Observed defect

During the preceding checker ship, the default release build integration test was running concurrently in the same worktree. Its old body called `defaultRebuildBinary` with the source checkout root, then restored the original binary in `t.Cleanup`. Between the rebuild and cleanup, manual ship's `git add -A` staged the temporary binary. The commit-gate correctly rejected the changed tree with `COMMIT_GATE_STALE`; cleanup then restored only the working file, leaving `MM go/evolve`.

This was verified against both logs and the staged binary, rather than inferred from a dirty status alone:

- The checker gate passed at `04:51:09Z`.
- The test ran at `04:51:36.817Z` through `04:51:41.282Z`.
- The rejected ship log ended at `04:51:41.321Z`.
- The staged binary's build information contained `version=9.9.9` and `builtAt=2026-09-14T04:51:36Z`, identifying the test's release build.

The relevant existing boundaries are `rebuild_binary_default_test.go`'s old real-root build/cleanup, `releasepipeline.go:458`'s `go build -o evolve`, and `phases/ship/verify.go:229`'s manual staging before attestation verification. The ship command did not copy `go/bin/evolve`; the build test produced the transient artifact. No shipping gate is weakened by this change.

## Minimal refactor and preservation map

| Existing contract | Final behavior |
|---|---|
| `TestDefaultRebuildBinary_NonDryRun_RealRepo` identity | Name retained; source search found no executable external selector to migrate |
| Actual current CLI source | `os.CopyFS` copies the current worktree's entire `go` module, including uncommitted edits, embedded assets, module metadata and vendored dependencies |
| Real Git commit lookup in `defaultRebuildBinary` | Existing `initTempRepoWithTag` supplies a temporary repository with a real commit, retaining the successful `git rev-parse` branch; source bytes are copied separately and are not replaced by a HEAD checkout |
| Actual `go build -o evolve ./cmd/evolve`, target `9.9.9`, `dryRun=false` | Same production function, command and target; only the destination repository changes |
| Built executable exists | Same `os.Stat` assertion; copied `go/evolve` is removed first, with an absent file accepted, so old output cannot satisfy the assertion |
| CLI reports target version | Same executable invocation with `--version` and original `strings.Contains(..., "9.9.9")` assertion |
| Cleanup | Existing fixture's `t.TempDir` owns the copied source and output; no restore writes to the source worktree |
| Cost/tag classification | Existing untagged subprocess test remains untagged; no new integration-tag enforcement or performance improvement is claimed |

The module copy uses the standard library API available at the module's Go 1.23 minimum. The inspected source module contained 4,224 files, approximately 52 MB, and no symbolic links. `os.CopyFS` creates independent file contents; the fixture does not use a clone of committed source, a symlink/hardlink to source, a replacement CLI, or a new shared copy abstraction. Copy errors fail the test. Copying current files assumes writers do not concurrently edit the source module during validation, as the original build did.

## TDD and fault sensitivity evidence

This is a refactor of an already-green behavior test, not a production behavior change. The original package passed before editing. A wrong-version fault was then injected outside the worktree through a Go overlay, before the refactor, to establish the assertion's existing sensitivity. No syntax error or manufactured production RED is claimed as a TDD result.

The mutant changes only the production build function's version linker argument from the requested target to `0.0.0-mutant`. The original and final tests both fail at the same executed CLI version assertion, showing the actual output `evolve 0.0.0-mutant (...)` and the expected target `9.9.9`. The final run reaches this assertion after a successful real build in the temporary repository.

During the original mutant run, an external observer recorded the source binary changing from its committed SHA to rebuilt bytes and back again. During the final unmodified package race run, the same observer saw one stable source-binary inode/mtime/content state throughout. The observer samples at 10 ms, so it is an observed regression witness, not a proof that sampling detects every possible transient write. The final code's independent destination path establishes the isolation.

The observer was a Python `subprocess.Popen` wrapper around each documented Go command, with working directory `/Users/danleemh/ai/claude/evolve-loop/dev/test-release-build-isolation-2026-09-14/go`. It inspected that directory's source `evolve` file, compared `(st_ino, st_mtime_ns, st_size)` on each sample, and computed SHA-256 on state changes until process completion. Its raw records are `/tmp/evolve-release-isolation-sideeffect-before.json` and `/tmp/evolve-release-isolation-sideeffect-after.json`; it did not modify the file or Git index.

Execution order:

1. Original package race suite: PASS.
2. Original wrong-version overlay: intended version assertion FAIL; transient source-binary replacement observed.
3. Isolate test setup; preserve build and assertions.
4. Remove copied output following independent review; final package race suite: PASS.
5. Same wrong-version overlay against final test: intended version assertion FAIL.

No mutant was written to authoritative production source. Original and final test-file SHA-256s are respectively `9d9b6b10f700ef94bb3cd54f06ce5bae6374099bdff60b4c9ce1ffe693e998d3` and `f2e05dc85acf2c05bd88764f5d8a33adc6dcdca1445f0b10cad5acd58ea02e85`.

## Validation

Matched before/after package command:

```sh
cd /Users/danleemh/ai/claude/evolve-loop/dev/test-release-build-isolation-2026-09-14/go
go test -race -count=1 -json -coverprofile=<profile> ./internal/releasepipeline
```

Both runs passed 113 test/subtest events with no failures or skips. Coverage is 95.4% before and after: identical 266 block identities, identical 249 covered blocks, and 413/433 covered statements. The raw coverage profiles are byte-identical. The successful production Git-lookup block is retained.

| Check | Result |
|---|---|
| Full module `gofmt -l` on 3,998 Go files | PASS; no unformatted files |
| `go vet -p=4 ./...` | PASS |
| Final releasepipeline race suite | PASS, as detailed above |
| Wrong-version overlay before and after | Both fail the original version assertion |
| `go test -race -count=1 -p=4 -parallel=4 -json -coverprofile=<profile> ./...` | PASS; 232 packages, 16,224 passing test/subtest events, no failures; 14 test skips and 3 packages without enabled tests |
| `make -C go build` after the full suite | PASS; fresh runtime output is the ignored `go/bin/evolve` |

The full default-tag race run completed at `2026-09-14T05:08:36Z`. Its 14 test skips concern existing platform, local-ledger/history, subprocess-helper, legacy-Bash-parity and disabled-regression conditions; this change adds no skip. The release build test itself executed and passed. The three package-level skips were `acs/cycle1158`, `test/e2e` and `test/integration`. This result does not claim separately tagged integration/e2e or cross-OS execution. The tracked source `go/evolve` still matched `HEAD` byte-for-byte after the full suite.

The mutant command is `go test -count=1 -json -overlay=/tmp/evolve-release-isolation-mutant/overlay.json -run '^TestDefaultRebuildBinary_NonDryRun_RealRepo$' ./internal/releasepipeline`.

Raw local evidence is retained under `/tmp/evolve-release-isolation-*`; the commands and named assertions above are reproducible after temporary-file cleanup. Independent Go-test review and simplification approved the final source and verified matching assertion identities, coverage blocks and mutation failures. The parent task owns the ordinary commit gate and ship. No commit, push, or merge has been performed by this implementation slice.

| Evidence | SHA-256 |
|---|---|
| Original package JSON | `1064af7e2af8c44603cce00c6ea3040765d07b03678cb9d4092d38c703baca83` |
| Final package JSON | `0d0d6968a1ff94903eb819142966868eb641754db146886f94e7f21285c21cf9` |
| Both package coverage profiles | `de6d56eac06f68869aff4f502f1b309fb0aa547940e2012ff1677833386ee8eb` |
| Original wrong-version mutant JSON | `57ca46eb81ae9664d140533e705acb7880b839d569dac165de552b900f6059dd` |
| Final wrong-version mutant JSON | `59a932fd4329d6998caeae9878eab9d9231490725cc7d35c519031cfe74e3c4e` |
| Original source-binary observations | `9b87cb35b4503d347081b49daa66a2728478e53186c8b96944026b3d8245ec0b` |
| Final source-binary observations | `057dcd3ca4d7fd95ebb1c9dc1c14c73edd51a22994f00fc302f61422a57e6df4` |
| Full default-tag race JSON | `b3fbcdbfb3791b5211f8608faae2ff2bd96f231161e4151f65c5252d3d2da63d` |
| Full default-tag coverage profile | `bfef99a81c064b9d99fa78ec8bb1af49f9822d7be4a5bd357cfc50fa958032c4` |

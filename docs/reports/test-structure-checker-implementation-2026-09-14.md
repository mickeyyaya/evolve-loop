# Shared structure checker: implementation and preservation evidence

Date: 2026-09-14. Baseline: `80b348e6943b08b3bfc4e2d30f06f42db69c1db9`. Branch: `refactor/test-structure-checker-2026-09-14`. Implements the scanner portion of S2 in [the test refactoring design](../architecture/test-refactoring-design-2026-09-14.md). This report covers the isolated implementation; final integration checks and gated shipping belong to the parent change.

## Issue / gap / solution

The fifteen source-limit tests copied the same traversal. For `IfStmt`, it visited only `Body`, then stopped `ast.Inspect`, silently excluding `Else`. A six-deep branch could pass a limit of four. The copies had no independent source-fixture tests for this behavior.

Independent nesting tests first failed against the original config scanner. The fix adds traversal of `Else` at the existing if depth; an `else` block adds no separate control-statement level, while an `else if` includes its nested `IfStmt`. After the unchanged regression passed, the checker moved into the stdlib-only [go/test/structure](../../go/test/structure/limits.go). All fifteen package-local `TestLimits_FunctionsFilesAndNestingStayWithinTheBar` entry points now call `structure.CheckLimits(".")`.

The helper has no `core`, `fixtures`, runtime-package, or external dependency. The existing `internal/codequality` helpers were inspected before adding it: they implement formatting/module discovery, not structure limits. Existing import-graph tests scan production files and remain unchanged. No interfaces, runtime policy knobs, package exclusions, or new test runner were introduced.

## Preserved contracts

- Functions remain **fewer than 50 lines**, nesting **at most four**, files **fewer than 800 newline bytes**. The historical newline-byte convention is explicitly retained, including an unterminated final line; this change does not redefine physical line counting.
- Each caller still scans its own current directory, only names ending `.go`, excluding `_test.go`, without recursively entering child directories. Source is checked regardless of build constraints or platform filename.
- Named function declarations retain the existing start/end line calculation and doc-comment exclusion. Bodyless/nonfunction declarations remain excluded. Anonymous functions within a named function retain their existing traversal behavior; they do not become separately measured declarations.
- Every detected size/depth violation is retained in source order. Filesystem/parser errors still fail the test; earlier diagnostics are retained when an error stops scanning. The helper joins diagnostics into an error, which the local test reports at its call site.
- All existing test names and the signalcenter caller's `t.Parallel()` policy are unchanged. No existing runtime behavior or production source was edited. The only added non-test Go source is the shared test checker itself.

## Strict TDD sequence

1. Baseline: all fifteen owner packages passed `-race -count=1`, with 779 passing test/subtest events and no skips.
2. Added `TestNesting_ControlFlowDepth` to the existing config scanner, before changing the scanner. Thirteen independent snippets specify depths, including known-good neighboring cases. The original code failed exactly three subtests:

   ```text
   else_body:       got 1, want 6
   else_if:         got 1, want 3
   deepest_sibling: got 2, want 3
   go test exit 1
   ```

3. Added the minimal `Else` traversal to the original checker. The same regression plus the existing local enrollment passed. The regression function body SHA-256 remained `ff2ad50aa8b25b960b481158da7ad873295fce5dab6093e5f7877d58575375cd` through GREEN and its later move into the shared helper's tests. No test was edited to make the fix pass.
4. Before extraction, copied the original checker into isolated temporary packages and verified eight manually specified boundary decisions: function 49/50 lines, file 799/800 newlines, depth 4/5, doc comments, and unterminated final line. All eight matched. The aggregate process returned one because the 50-line function, 800-newline file and depth-five packages were deliberate negative controls.
5. Wrote permanent source-boundary, scope, complete-diagnostic and I/O/parser tests, then extracted under the green regression. These preserve existing behavior; the new behavioral RED is the `Else` defect, not an invented failure in previously correct thresholds.
6. All fifteen composed callers and the helper passed. No previously unreported production limit violation was found, so no production edits or threshold changes were necessary.

The regression command before extraction was:

```text
cd go
go test -count=1 -json ./internal/config -run '^TestNesting_ControlFlowDepth$'
```

GREEN before extraction added the existing entry point to the selector:

```text
go test -count=1 -json ./internal/config -run '^Test(Nesting_ControlFlowDepth|Limits_FunctionsFilesAndNestingStayWithinTheBar)$'
```

## Exact scope and coverage comparison

Local toolchain: `go1.27.1 darwin/arm64`; `GOFLAGS` empty. Both measurements used the same source revision, machine, package selection, `-race -count=1`, and coverage instrumentation. Only the final command additionally selected the new helper package. The table records covered/total **statements**, not an average of function percentages.

| Owner package under `go/` | Before | After |
|---|---:|---:|
| internal/bridge/launchoutcome | 81/81 | 81/81 |
| internal/config | 287/287 | 287/287 |
| internal/core/advisor | 563/563 | 563/563 |
| internal/core/defectledger | 304/304 | 304/304 |
| internal/core/failurelearning | 127/127 | 127/127 |
| internal/inboxmover/lifecycle | 422/422 | 422/422 |
| internal/loopchain | 253/253 | 253/253 |
| internal/loopwave | 227/227 | 227/227 |
| internal/observerengine | 206/206 | 206/206 |
| internal/phases/audit/ciparitygate | 342/342 | 342/342 |
| internal/phases/runner/verdict | 201/201 | 201/201 |
| internal/phases/ship/landing | 122/122 | 122/122 |
| internal/signalcenter | 347/347 | 347/347 |
| internal/subagent/subagentrun | 364/364 | 364/364 |
| internal/textcap | 8/8 | 8/8 |
| test/structure (new) | — | 48/49 |

All 2,481 original block spans still appear in the final profile; all **2,480 previously hit blocks** remain hit. There are no lost original passing test identities. The fifteen original packages remain at 100% statement coverage. The helper's uncovered statement is the retained defensive default in `childBody`, which the typed control-statement caller cannot reach; no artificial direct-call test was added to inflate its percentage. `CheckLimits`, `checkFunctions`, and `nesting` each have 100% statement coverage.

The old-to-new map is identical for every row: the same package plus `TestLimits_FunctionsFilesAndNestingStayWithinTheBar` now delegates arrangement/scanning to the shared checker; no acceptance identity is removed. Searches found no external callers of the removed private scanner helpers or their constants. The new exported `CheckLimits` is directly named and exercised by meaningful tests; existing API-name/import-graph tests ran in the full owner-package selection.

Exact final selection:

```sh
cd go
go test -race -count=1 -coverprofile=/tmp/evolve-structure-final.cover -json \
  ./internal/bridge/launchoutcome ./internal/config ./internal/core/advisor \
  ./internal/core/defectledger ./internal/core/failurelearning \
  ./internal/inboxmover/lifecycle ./internal/loopchain ./internal/loopwave \
  ./internal/observerengine ./internal/phases/audit/ciparitygate \
  ./internal/phases/runner/verdict ./internal/phases/ship/landing \
  ./internal/signalcenter ./internal/subagent/subagentrun ./internal/textcap \
  ./test/structure
```

Result: **16/16 packages PASS; 811 passing test/subtest events, zero failures, zero skips**. All 779 original events remain; the helper contributes 32 events. The baseline command was identical except it omitted `./test/structure` and wrote `/tmp/evolve-structure-baseline.cover`.

## Fault sensitivity and integration limits

Four isolated Go overlays changed only the checker while retaining its tests. Each mutant failed with a semantic assertion, not a build error:

| Mutant | Rejecting oracle |
|---|---|
| Remove `Else` traversal | All three original RED cases and the composed six-deep source case |
| Allow a 50-line function | `TestCheckLimits_SourceBoundaries/function_50_lines` |
| Allow an 800-newline file | `TestCheckLimits_SourceBoundaries/file_800_newlines` |
| Allow depth five | Depth-five boundary and exact complete-diagnostic assertions |

This is four targeted fault controls, not a repository-wide mutation score. Temporary overlays never modified the authoritative source tree. `gofmt` and `git diff --check` pass. No runtime tag or ACS selection changed; the original fifteen test names remain selectable, and the helper is available in default/integration/e2e builds because it is untagged. The direct owner-package tests establish local wiring. Full repository vet/tests, Linux/Go-1.23 compatibility, final CI and any release gating remain integration checks; this report does not claim they ran here.

No performance gain is claimed from cold/warm timing differences. The maintained checker implementation is shared rather than copied fifteen times; all fifteen obligations still execute.

## Local evidence digests

These temporary JSON logs and profiles are local execution evidence; the commands and outcomes above remain reviewable after temporary files expire.

| File under `/tmp/` | SHA-256 |
|---|---|
| evolve-structure-red.jsonl | d80c98e8f73cf890d74353f8eeb3b50cdf78ff962554de6aab8f1e0eaccb5dae |
| evolve-structure-green-before-extract.jsonl | 37c5ffedd33f0d813a28de2a2eeba8ac5118447d8188f602058ad03a4ced8243 |
| evolve-structure-threshold-baseline.jsonl | cf9ce105abb29594dcb27437e0a2f060044ef404c789029546c11fb3edc79ce2 |
| evolve-structure-baseline.jsonl | 1efdc4124af465d78035cbcba626b6bb67766b732c5c774703e95943723505d3 |
| evolve-structure-final.jsonl | 26451a6f5e97c5591e85d2dac3f4743e41717d7a1fde0814677f5023ce8d91ad |
| evolve-structure-baseline.cover | e58e90355b3505372d6a042e2695fbcadfb9090a259e47176353c05a212b85af |
| evolve-structure-final.cover | df51decb91d0d57de6da6de9102aa6a1f2c296e17cddf351b7376e8575e07d67 |

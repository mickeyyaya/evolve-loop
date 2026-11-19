# Research: tests that cannot fail, tests that pin code text, and fakes that differ from the real thing

- **Status:** research, 2026-11-18, revised after the design review.
- **Plan:** [the test-quality audit](../plans/test-quality-audit-2026-11.md).

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [Local evidence](#3-local-evidence)
4. [Current practice outside this repository](#4-current-practice-outside-this-repository)
5. [Refinements](#5-refinements)

## 1. Request

The operator wrote on 2026-11-18:

> "Explore my test suite and identify tests (or groups of tests) which are tautological, too structure-sensitive, or where the mocks don't match reality. Follow to the end that no tests violated and correct them"

The operator then gave a direction on how to organize the work:

> "Test should be separated from the pipeline core functions that evo loop should be able to handled."

*Implication:* a fix item changes only `*_test.go` files and `testdata`. The evo loop can then build each item. A finding that needs a production change is a separate item (F3.7 gives one case).

## 2. Terms used throughout

- **Tautological test:** a test that cannot fail, or that asserts what it set up itself.
- **Change-detector test:** a test that pins the text or the shape of the code, not its behavior. It fails on a refactor that keeps the behavior.
- **Guard:** a test that enforces one policy on the whole tree, for example an import direction or a banned call. A guard is not a change-detector.
- **Fake:** a test double with working behavior in place of a real dependency.
- **Verified fake:** a fake that passes the same contract tests as the real dependency.
- **Predicate:** a test in `go/acs/cycleNNNN/` that proves the deliverable of one cycle.
- **Named export:** an export that a test mentions. **Called export:** an export that a test calls or reads in an assertion.

## 3. Local evidence

The census ran on 2026-11-18 over `go/` at `origin/main` (commit `df3393b57`). Each number has a command, or it says "estimate".

| Figure | Value | Command |
|---|---|---|
| Test files | 6,140 (4,428 in `internal/`, 923 in `acs/`, 754 in `cmd/`) | `git ls-tree -r --name-only origin/main -- go \| grep -c '_test\.go$'`, with a path filter for each directory |
| Test functions | 33,528 (33,572 with `Benchmark`, `Fuzz` and `Example`) | `git grep -hE '^func Test[A-Z_0-9]' origin/main -- 'go/*_test.go' \| wc -l` |
| `acs` test functions | 5,951 | the same command, with `'go/acs/*_test.go'` |
| Files named `apicover_named_test.go` | 150 (155 when the name `*_apicover_named_test.go` counts) | `git ls-tree -r --name-only origin/main -- go \| grep -c '/apicover_named_test\.go$'` |
| `var _ = x` or `_ = x` lines in those files | 45 (29 of them typed, `var _ T = x`) | `git grep -hE '^[[:space:]]*(var _ \|_ = )' origin/main -- 'go/*apicover_named*_test.go' \| wc -l` |
| Same lines in all test files | 1,015, an upper bound | the same command, with `'go/*_test.go'` |
| Enforced packages | 367 patterns | `grep -vcE '^\s*(#\|$)' go/.apicover-enforce` |

The first census gave 34,713 functions, 6,226 files and 164 `apicover_named` files. A repeat with the commands above did not reproduce them, so this revision uses the repeated numbers.

The figures F3.1, F3.3, F3.5 and F3.6 below are **estimates** of the first census (a body scan and a file scan). No single command repeats them. The T1 dry run of the plan replaces each estimate with a measured count, and the plan does not use an estimate as a target.

**F3.1** About 65 test functions have no assertion (estimate). A body scan found 103 candidates, and about 30 percent were false hits from test names inside fixture strings. Examples: `internal/core/extra_coverage_test.go:148` only calls `recordRoutingDecision`, and `internal/deliverable/reviewer_breaker_amplify_test.go:113` only calls `writeBreaker("", 42)`. Two tests only log (`internal/looppreflight/freeze_test.go:226`).
*Implication:* these tests pass on any behavior that does not panic. The 30 percent false-hit rate of a text scan is the reason that the rules read `go/ast`.

**F3.2** 150 files named `apicover_named_test.go` exist so that `internal/apicover` counts an export as named. `Classify` (`internal/apicover/classify.go:21`) marks a function as covered when its bare name appears in any `_test.go` file and its line coverage is above zero. `NamesReferencedInTests` (`internal/apicover/refs.go:13-41`) collects each identifier and each selector name, with no type information.
- Some files assert their own setup. `internal/phaseintegrity/apicover_named_test.go:22` builds `RepinResult{NewSHA: "new"}` and then asserts `NewSHA == "new"`.
- Other files only name a symbol. `internal/phaseintegrity/apicover_named_test.go:31` holds the typed form `var _ func(...) (RepinResult, error) = RepinIfDrifted`.
- The gate is hard. `.github/workflows/go.yml:121-127` runs `make apicover-check` over `go/.apicover-enforce`. That file lists 367 package patterns, and `internal/phaseintegrity` is one of them.

*Implication:* the metric rewards a test that names a symbol without calling it. The cause is the metric, not the 150 files. A change of the metric flips each export whose only reference is such a line, in a package that CI gates hard.

**F3.3** About 15 tests skip when a file of this repository fails to load (estimate). `cmd/evolve/routing_order_realtree_test.go` has three hidden paths:
- `:24` drops the error of `config.Load` (`cfg, _ :=`), and `:25-27` skips when the order is empty.
- `:28-31` skips when `phasespec.Load` of the built-in registry fails.
- `:33-36` drops the error of `discoverUserSpecsClamped` (`userSpecs, _ :=`), and skips when the result is empty.

The registry is always present, so each skip can only hide a parse failure.
*Implication:* a corrupt data file turns these tests into skips, not failures. A rule that checks only a skip inside an error branch misses the two dropped-error shapes.

**F3.4** `internal/research/faillearn_contract_test.go:13` compares `NumField()` of `research.lessonYAML` with the const `expectedLessonSchemaFields` (value 9, `:11`). It never reads the `faillearn` struct, which is unexported (`internal/faillearn/faillearn.go:40`).
- The test `TestParseLessonFile_RoundTripsFaillearnRender` (`:21`) already checks the parity through behavior.
*Implication:* the field-count test checks a number, not the parity that its name claims. The fix is to delete it. The round-trip test covers the claim. The compared value is a const, so the rule must resolve a const to its literal.

**F3.5** About 35 tests outside `acs/` read one named `.go` file and match its text (estimate). `internal/core/resume_parity_pin_test.go:12` matches the exact call `phaseOutcomeFrom(next, resp, attempts, ferr.Error(), cs.PhaseStartedAt)`. `internal/bridge/driver_tmux_cursor_order_test.go:13` compares the text positions of two calls to claim a run-time order.

Ten more tests parse one named file with `go/parser` (estimate). About 49 tests walk the whole tree, and they are guards (estimate).
*Implication:* a rename or a move fails the 45 pins with no change of behavior. A guard is a different kind and stays. A walk that filters down to one file is a pin in disguise, so a guard must visit more than one file.

**F3.6** In `acs/`, 614 of 5,939 predicate functions (10.3 percent) only match source text (estimate). About 96 only check that a file exists (estimate). The groups can overlap, so 710 is an upper bound. At least 1,817 predicates run behavior (estimate).
- Example: `acs/cycle4/predicates_test.go:12-19` checks a file with `FileExists` and the absence of one quoted string with `FileNotContains`.
- A string concatenation defeats it.
*Implication:* a source-text predicate proves that a cycle wrote some text, not that the behavior works. A predicate that proves a banned string is absent is a guard shape. It is a tree-wide guard test, and it does not need a cycle.

**F3.7** Fakes differ from the real dependency, and only one fake has a parity test (`internal/treestate/treestate_test.go:152`, `TestSHA_RealGitParity`):
- `internal/faketmux` writes the pane to stdout with no regard to `-p` (`internal/faketmux/server.go:209-224`). It drops `\r` (`internal/faketmux/session.go:199`), and `resize-window` only checks the target (`internal/faketmux/server.go:275-278`).
- The bridge fake git has no failure case: `fakeGit.cmd` (`internal/bridge/git_evidence_test.go:21-35`) never returns a non-nil error. The mismatch is in production code. The adapter `gitCmd` (`internal/bridge/completion.go:122-127`) discards the exit code (`_, err := deps.Runner(...)`). `execRunner` (`internal/bridge/engine.go:580-590`) returns `(code, nil)` for a non-zero exit. So a failed `git rev-parse` gives `err == nil` with empty output, and `poll` (`:132`) survives only because `head == ""`. A failed `rev-list` or `log` that prints to stdout is read as data.
- The rollback git stub is a shell script that `fakeclitest.Install` writes (`internal/rollback/edge_cases_integration_test.go:207-218`). It prints `refs/tags/v9.9.9` for `ls-remote`, where real git prints `<sha>\trefs/tags/v9.9.9`. The production check is `strings.Contains(out, tag)` (`internal/rollback/rollback.go:337`), so the stub also passes on the real format. The effect is low. The same script exits 0 for every other subcommand, so no test sees a revert conflict.
- The orchestrator fake clock runs the callbacks of `AfterFunc` on the caller goroutine (`internal/orchestrator/helpers_test.go:65-75`). The real `time.AfterFunc` runs them on a new goroutine (standard library behavior, not run in this revision).

*Implication:* a test can pass on the fake and fail on the real dependency. The bridge case is a production defect, not a fake defect. If the fake returned a nil error on failure, it pins the swallowed exit code. The production adapter must turn a non-zero code into an error first. Then the fake and a contract suite check one rule.

**F3.8** Existing tools: `internal/evalqualitycheck` rejects shell evals that cannot fail and predicates whose polarity contradicts their wording. `internal/testdup` ratchets one count of duplicated helpers in `.test-dup-baseline` (`internal/testdup/testdup.go:247-256`). `internal/apicover` counts named exports. `internal/testhost` already holds a skip helper that fails, not skips, on a job that declares a tool as required (`internal/testhost/testhost.go:33-44`). The jobs declare `TESTHOST_REQUIRED_TOOLS: tmux` (`.github/workflows/go.yml:47,180,258`).

No tool finds a Go test with no assertion. No tool finds a skip on a load error, a single-file source pin, or a fake with no contract test.
*Implication:* a new analyzer is needed. A baseline ratchet with the same two-sided check as `testdup` keeps the count falling. The skip helper exists, so the plan does not build a second one.

**F3.9** Mutation checks exist as a practice, not as a gate. The reviewer personas run single-idea mutants with `go test -overlay` in a scratch copy, up to ten for each review (`agents/evolve-code-reviewer.md:68`, `agents/evolve-diff-architecture-reviewer.md:57`). A persona `agents/evolve-mutation-gate.md` names `gremlins` and `go-mutesting`, with a manual fallback. The repository has neither tool as a module.
*Implication:* a mutation check for rewritten tests uses the overlay mutants that the reviewers already run. A new outside tool is not needed.

## 4. Current practice outside this repository

**F4.1** Google calls a test that fails on each production change, with the behavior unchanged, a change-detector test. Google advises testing through the public API ([Testing on the Toilet](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html), [Software Engineering at Google, chapter 12](https://abseil.io/resources/swe-book/html/ch12.html)).
*Implication:* a test asserts behavior through the API. Only a guard reads the code text.

**F4.2** Coverage can reach 100 percent with no assertion. Mutation testing finds a test that cannot fail: it changes the code and checks that a test fails ([GoLab talk](https://golab.io/talks/your-tests-pass-but-do-they-work-mutation-testing-in-go), [go-mutesting](https://pkg.go.dev/github.com/jonbaldie/go-mutesting/v2)).
*Implication:* a static detector finds the known shapes. A small mutation check confirms that a rewritten test can fail.

**F4.3** A verified fake passes the same contract tests as the real dependency. One suite runs twice, once on each. So a drift of the real dependency forces the fake to follow ([verified fakes](https://pythonspeed.com/articles/verified-fakes), [test double](https://www.wikipedia.org/wiki/Test_double), [avoiding fake drift](https://4comprehension.com/avoiding-fake-drift/)).
*Implication:* each fake of a real tool gets a contract suite that runs on the real tool where it is present.

**F4.4** Static test-smell detectors exist for Java (tsDetect, JNose, TestLint), with 19 to 26 smell types each ([test smell detector](https://testsmells.github.io/pages/testsmelldetector.html)). No Go detector for an assertion-free test was found.
The same catalogs list other smells that fit this request:
- a test that asserts an exact order of calls on a mock;
- a test that sleeps to wait;
- a golden file that a flag regenerates.
*Implication:* the repository builds its own analyzer on `go/ast`. The plan gives a rule to each smell, or names it as a non-goal.

## 5. Refinements

| Id | Refinement | Because |
|---|---|---|
| R1 | Build `evolve test audit` on `go/ast`, with one rule for each defect shape, and a baseline that only shrinks. | F3.8, F4.4 |
| R2 | Count an export as named only when a test calls it, so a test does not need to name a symbol to pass the metric. Ship the change with a per-package baseline that only shrinks. | F3.2 |
| R3 | Let a test skip only for an absent outside tool, through `testhost`. A failed load of repository data fails the test. | F3.3, F3.8 |
| R4 | Replace each single-file source pin with a behavioral test, or delete it when a behavioral test exists. Keep the guards. | F3.4, F3.5, F4.1 |
| R5 | Turn each source-text-only predicate into a behavioral one, or retire it when a unit test covers the behavior. Refuse new ones. | F3.6 |
| R6 | Give each fake of a real tool a contract suite that runs on the fake and on the real tool. Fix the production defect of the bridge adapter before its fake. | F3.7, F4.3 |
| R7 | Confirm each rewritten test with one to three overlay mutants of the code under test. | F3.9, F4.2 |
| R8 | Keep a fix item inside `*_test.go` files and `testdata`. A finding that needs a production change is its own item. | the request, section 1 |

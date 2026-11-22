# Plan: find and remove tests that cannot fail, tests that pin code text, and fakes that differ from the real thing

- **Status:** partly built, 2026-11-21 (7 of 11 components on main, 2 partial, 2 not started). Earlier status: proposed, 2026-11-18, revised after the design review.
- **Research:** [tests that cannot fail, tests that pin code text, and fakes that differ from the real thing](../research/test-quality-audit-2026-11.md).
- **Decision:** no new ADR. The rules are data in one package, and section 6 is the spec. T2 changes the hard `apicover` gate of [ADR-0050](../architecture/adr/0050-modularization-and-unified-phase-io.md), so T2 adds an `Amended by` line to that ADR. T5 does the same for the ADR that records the eval gate, if one exists.

## Table of contents

1. [Request](#1-request)
2. [Goals and non-goals](#2-goals-and-non-goals)
3. [Candidate designs](#3-candidate-designs)
4. [Decisions](#4-decisions)
5. [Worked example](#5-worked-example)
6. [The rules](#6-the-rules)
7. [Components](#7-components)
8. [Order](#8-order)
9. [Verification](#9-verification)
10. [Risks](#10-risks)

## 1. Request

On 2026-11-18, the operator asked for two things. First, find each test that is tautological, too sensitive to the code structure, or built on a mock that does not match reality. Second, "follow to the end that no tests violated and correct them". The quote is in section 1 of the research.

The operator then gave a direction (2026-11-18):

> "Test should be separated from the pipeline core functions that evo loop should be able to handled."

This plan makes four decisions from that direction:

- Each fix item changes only files in its scope. The scope is data in `internal/testaudit/scope.json`: `*_test.go` files, `testdata`, and the test-support packages that `fakes.json` lists as fake locations (for example `internal/faketmux`). A fix item never changes production code, and T10 refuses a diff outside the scope (section 7).
- A finding that needs a production change becomes its own item. The first one is P1 (section 7): `internal/bridge/completion.go` drops the exit code of the runner.
- Each component of section 7 is a lane item that the evo loop builds. The console builds an item only when its path is a protected surface (`docs/operations/operating-policy.md:154,174`).
- A fix item never edits the analyzer, the metric or the gates. Those are the tooling items T1, T2, T3 and T5.

## 2. Goals and non-goals

Goals:

1. A tool finds each known defect shape on the whole tree, and a person can repeat the result. (F3.8)
2. The count of findings only falls, and a new finding fails CI. (F3.8)
3. The count reaches zero for the rules of section 6. Each finding is fixed, or it is a guard by a row that a reviewer can read. (request)
4. Each fake of a real tool passes a contract suite. For a tool with a real-side CI job, the suite also runs on the real tool. For a tool with real side `none`, the row of `fakes.json` states the reason, and a reviewer reads it. (F3.7, F4.3)

Non-goals, each with a reason:

- No rewrite of a guard that enforces a policy on the whole tree (F3.5).
- No mutation run on the whole tree. A small mutation check confirms the rewritten tests only (F4.2).
- No rule for `t.Parallel` with shared state. The CI test step runs with `-race` (`.github/workflows/go.yml:42`), and the race detector decides this better than a syntax rule.
- No rule for an assertion of `err != nil` only. Syntax cannot tell if the identity of the error matters. The rule `error-text-match` covers the case where a sentinel exists.
- No rule for a test that asserts the return value of its own mock. The rule `asserts-own-setup` covers the direct case, and it states its bound in section 6.

*Implication:* "zero findings" means zero for the rules of section 6, and the non-goals above name what that zero does not prove.

## 3. Candidate designs

Scoring key: 0 = fails the criterion, 1 = meets it in part, 2 = meets it.

| Design | What it does |
|---|---|
| A. A manual sweep | Read and fix the tests by hand, once. |
| B. Mutation testing only | Run a mutation tool on each package, and fix each test that lets a mutant live. |
| C. An analyzer, a ratchet and batched fixes | `evolve test audit` finds each shape on `go/ast`. A baseline only shrinks. Lane items fix the findings, one package each. Overlay mutants confirm the rewrites. |
| D. A model review of each test | A seat reads each of the 33,528 tests and judges it. |

| Criterion | A | B | C | D |
|---|---|---|---|---|
| Repeatable result (goal 1) | 0 | 1: slow, and the result depends on the mutants | 2 | 0: not deterministic |
| No new defect lands (goal 2) | 0 | 1 | 2: the ratchet | 0 |
| Cost on 33,528 tests | 0 | 0: hours for each package | 2 | 0: thousands of seat calls |
| Finds a fake that differs (goal 4) | 1 | 0 | 1: finds the fakes with no contract suite | 1 |
| **Total** | 1 | 2 | 7 | 1 |

C wins. B confirms the rewrites inside C (R7). A model reads only the findings that a rule cannot judge, for example a fake whose drift is in behavior.

## 4. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | One package, `internal/testaudit`, holds the rules of section 6 as data, each with an id, a class and a check. A rule that needs types uses the standard `go/types`, as `internal/adapters/codeindex/gotypes/build.go:7-10` does. `golang.org/x/tools` is not in `go/go.mod`, so the plan does not use `go/packages`. The verb `evolve test audit [--json]` prints each finding with its file, line, rule and reason. | F3.8, R1 | A grep: test names in fixture strings give false hits (F3.1). |
| D2 | The baseline is a set of shards: one `testdata/testaudit-baseline.json` in each package directory. A row has three keys: package, test function (or type) and rule. A row has no line number. A test in `internal/testaudit` fails on a finding with no row, and on a row with no finding. So the baseline only shrinks. | F3.8, review H4 | One file of rows keyed by file and line: an edit moves the line. All items share one file, so lanes conflict. One count like `testdup`: a count cannot say which finding is fixed. |
| D3 | A guard is exempt by row, never by shape. The data file `internal/testaudit/guards.json` lists a package or a test function, with a reason for each row. A new file in `internal/guards/` is not exempt until a row names it, because the row decides what a guard is. A test fails when the row count is above `max_rows` in the same file. A diff that adds a row also edits `max_rows`, and the pre-commit review fleet reads it. The reviewer checks the walk. | F3.5 | Exempt by location or by shape: a single-file pin put in a guard directory, or wrapped in a trivial walk, passes. Exempt each AST test: the 10 single-file pins stay. |
| D4 | `internal/apicover` counts an export as **called**, not named. A bare mention does not count. The typed form `var _ T = Symbol` does not count. The false-green class stays: a test calls the export, and its line coverage is zero (`internal/apicover/classify.go:21-36`). Only "named" changes to "called". The list below the table gives the counted reference for each kind. | F3.2, R2 | Fix the 150 files and keep the metric: new files of the same kind appear. Warn-only for several waves, then a hard switch: a warning stops no new weak export, and the flip still needs a baseline at the switch. |
| D4a | T2 ships with a per-package baseline of the flipped exports. The file `testdata/apicover-flipped.txt` in each enforced package lists the exports that were named but not called. `apicover` treats a listed export as covered. It fails on a listed export that a test now calls, so the baseline only shrinks. T9 empties each file. So `make apicover-check` stays green at each commit. | F3.2, review C1 | Merge T2 and T9 per package: one item changes the metric and fixes its own package. It makes T2 a very large item, and it blocks the metric on 367 packages of rewrites. |
| D5 | A test skips only through `testhost.Tool` or `testhost.Unavailable` (`internal/testhost/testhost.go:24-62`). `Tool` fails the test, and does not skip, on a job that declares the tool in `TESTHOST_REQUIRED_TOOLS` (`.github/workflows/go.yml:47,180,258`, today `tmux` only). `Unavailable` fails only on darwin with `CI=true` (`internal/testhost/testhost.go:24-31`). A failed load of a file of this repository fails the test. The rule `skip-hides-error` finds each other skip in an error branch. The plan does not add a second helper. | F3.3, F3.8, R3 | A new `testenv.RequireTool`: it copies `testhost`. Keep each skip: a corrupt registry stays green. |
| D6 | A single-file source pin becomes a behavioral test of the same claim. When a behavioral test of the claim exists already, the pin is deleted. A field-count test is deleted when a round-trip test covers the parity (F3.4). It compares two structs only when a test can reach both. | F3.4, F3.5, R4 | Keep the pins as documentation: they fail on a rename and teach nothing. A new export of `faillearn` for a reflect test: it adds production surface for one pin. |
| D7 | A source-text-only predicate in `acs/` is one of three classes, listed below the table. `evalqualitycheck` refuses a new predicate for which the T1 rule `single-file-source-pin` gives a finding. The refusal is an AST shape, not a name. | F3.6, R5 | Retire all 614: some guard a claim that no unit test holds. Refuse by a list of names: it drifts from the rule. |
| D8 | Each fake of a real tool has a contract suite in its package: one table of cases that runs on the fake. It also runs on the real tool through `testhost.Tool`, on the jobs that declare the tool. The set of tools is data: `internal/testaudit/fakes.json` lists rows (tool, fake location, contract suite location). A contract suite registers a `fakecontract.Suite` value that names its tool, and the rule reads that value, not a name. Each row also has a real-side field: a CI job that declares the tool, or `none` with a reason. A mismatch of F3.7 gets a case that fails on the old fake, and the bridge case needs P1 first. | F3.7, F4.3, R6 | Delete the fakes: the real tools are slow and need a host. Record a transcript of the real tool as the golden side: a transcript is a copy that drifts. It needs an update flag, which the rule `golden-update-flag` flags. |
| D9 | Each fix item records its mutation check. For each rewritten test, the item runs one to three single-idea mutants with `go test -overlay` in a scratch copy, as the reviewer personas do (`agents/evolve-code-reviewer.md:68`). The item writes `mutation.json` in its workspace, with the mutant diff, the test name and the result for each mutant. The auditor re-runs each recorded mutant, and a live mutant makes the verdict FAIL. An item of pure deletion records the mutant of the covering test (D7). A rewrite with no mutable statement records that fact, and the auditor judges it. | F3.9, F4.2, R7 | No mutation check: a rewrite can be a new tautology. A check by the code-reviewer of the pre-commit fleet: a lane item does not go through that fleet. The `gremlins` or `go-mutesting` tool of `agents/evolve-mutation-gate.md`: the repository has neither as a module, and no gate reads their output for a lane item. |

Detail of the decisions:

- D2 key: a finding in a helper or a method uses the enclosing function name in place of the test. A finding at file level, such as `golden-update-flag`, uses the file path. A row also holds a finding count, and a count that differs from the real count fails the test in both directions.
- D8 real side: a row with `none` is a fake that no run checks against the real tool. A reviewer reads each reason, and `max_rows` bounds the count of `none` rows. Today the fakes of `gh`, `claude` and `codex` have no CI job that declares the real tool.
- D9 owed file: T10 adds `mutation.json` to `AgentOwedFiles` of the item contract. The orchestrator gate then rejects a missing or non-JSON file like a missing deliverable (`internal/phasecontract/render.go:47-48`).
- D2 re-key: a move or a rename re-keys through `evolve test audit --rebaseline <package>`. The verb refuses a shard that grows.
- D2 relation: this is the two-sided check of `internal/testdup` (`internal/testdup/testdup.go:247-256`), with rows in place of one count.
- D4 function or method: a call that `go/types` resolves to that object. So `x.Run()` on another type does not count.
- D4 type: a field read or a method call on a test expression of that type, used in a condition or a call argument. The value must come from a call, a parameter or a field, not from a composite literal or a conversion in the same test function. A test that builds a literal and asserts its own field is the rule `asserts-own-setup`, and it does not count as a use.
- D4 const or var: a read in a condition or in a call argument.
- D7 class 1, a behavior claim: the predicate runs the code. A predicate that only checks that a file exists runs the loader of the file.
- D7 class 2, a claim that a unit test covers: the predicate retires. The unit test must fail under one overlay mutant that reverses the claim, and the item records that mutant (D9).
- D7 class 3, an absence claim such as a banned string: it moves to a tree-wide guard test with a guard row, or it retires.

Consequences:

- Good: a new tautology, a new source pin or a new unverified fake fails CI.
- Good: the metric of D4 stops the incentive that made 150 weak files, and D4a keeps CI green on each commit.
- Good: a fix item is a test-only item, so the evo loop builds it and no protected surface is in its scope.
- Bad: the first baseline is large. The fixes take several waves, one package for each lane item.
- Bad: a contract suite that needs the real tool runs only on a job that has it. Git is present on each job. The jobs declare only `tmux` today, so T3 adds `git` to each declared list.
- Bad: each mutant run builds one package. The cost is in the risk table.

What changes our mind: the false-positive gate of T1 (section 7). If it fails, the rules change before the fixes start.

## 5. Worked example

Package `internal/phaseintegrity` and package `internal/treefence`, in `internal/*/apicover_named_test.go`:

1. The audit reports `TestNamePublicAPI_Repin` (`internal/phaseintegrity/apicover_named_test.go:21-28`) under the rule `asserts-own-setup`. It builds `RepinResult{NewSHA: "new"}` and asserts `NewSHA == "new"`. It calls no function of the package.
2. `repin_ifdrifted_test.go` already calls `RepinIfDrifted` in `TestRepinIfDrifted_ProvenanceVerifiedRebuild_Repins` (`:52-57`). So the claim has a behavioral test. That test reads `res.Repinned` and `res.NewSHA` from the call result (`:62,66`). So D4 counts `RepinResult` as used after the deletion.
3. The fix item deletes `TestNamePublicAPI_Repin`, and deletes its baseline row. It records the mutant of the covering test: a mutation that skips the write fails `repin_ifdrifted_test.go`.

The second case has no behavioral test. `internal/treefence/apicover_named_test.go:37` holds `var _ string = RestoredMarker`. The const `RestoredMarker` (`internal/treefence/fence.go:548`) is used in the diagnostic text (`:568`). No other test mentions it.

1. After D4, `apicover` reports `RestoredMarker` as not called, and the flipped file of D4a lists it. CI stays green.
2. The fix item replaces the `var _` line with `strings.Contains(diags[0].Message, RestoredMarker)` on the `diags` value that the test builds.
3. The item deletes the row from `testdata/apicover-flipped.txt`. A mutant that changes the text at `fence.go:568` fails the test.

The self-asserting test builds `RepinResult{...}` itself (`apicover_named_test.go:24`), so D4 does not count it as a use. The typed form `var _ func(...) (RepinResult, error) = RepinIfDrifted` (`apicover_named_test.go:31`) does not count under D4 either. That test is deleted with the first.

## 6. The rules

Each rule has a red fixture and a green fixture in `internal/testaudit/testdata`. A test name inside a raw string is not a test function, because the rules read `go/ast`, not text. Each rule states its bound and its known false negatives, so that "zero findings" has a clear meaning.

| Rule id | Class | Finds | Bound and known false negatives |
|---|---|---|---|
| `no-assertion` | tautological | a `Test` function with no failure call on `t`, no call that passes `t` or a `testing.TB`, and no subtest that does either | `Fuzz*`, `Example*`, `Benchmark*` and `TestMain` are out of scope. A test whose only check is a `defer` with `recover` is also a finding, because the test runner already fails on an unrecovered panic. A real smoke test takes a guard row with a reason, or asserts an observable effect. For `extra_coverage_test.go:148`, assert that the ledger failure was swallowed and that the state is unchanged. A helper that takes `t` and never asserts passes. |
| `log-only` | tautological | a test whose only calls on `t` are `t.Log` or `t.Logf` | none known |
| `asserts-own-setup` | tautological | each compared operand comes from a literal that the test set, and the test calls no function of the package under test | one step of copy is followed. An alias through a function call is not. Such a test passes the rule. |
| `both-branches-pass` | tautological | an `if` with an `else`, or a `switch` with a `default`, where each branch ends with no failure call | `select` and an early `return` are not traced. A branch that holds `t.Skip` is the case of `skip-hides-error`. |
| `skip-hides-error` | tautological | `t.Skip*` in a branch on an error value, or guarded by a value whose producing call had its error assigned to `_` (`routing_order_realtree_test.go:24-27,33-36`). A skip through `testhost.Tool` is exempt. A skip through `testhost.Unavailable` is exempt only when its reason is a missing tool. | a skip guarded by a value that a helper produced is not traced |
| `literal-field-count` | structure | `NumField()` or `NumMethod()` compared with an integer literal, or with a const that resolves to an integer literal (`faillearn_contract_test.go:13`) | a count held in a var is not resolved |
| `single-file-source-pin` | structure | a read of one named non-test `.go` file, then a text or regexp match, outside the guard list. The path is traced through a literal, a const and `filepath.Join` of literals. The rule also finds a walk whose callback selects one path by a literal, and the `acsassert.FileContains` family. | a path that a helper computes at run time is not traced |
| `single-file-ast-pin` | structure | a parse of one named file with `go/parser`, with no walk of the tree, outside the guard list | the same bound as above |
| `unverified-fake` | mock fidelity | a row of `fakes.json` with no registered `fakecontract.Suite` for its tool, or a `fakeclitest.Install` target (its base name) that no row lists. The rule traces the target with the rule of `single-file-source-pin`: a literal, a const, or `filepath.Join` of literals. | A target computed at run time, for example a bare variable such as `path`, `shim` or `bin`, is not traced. The rule counts a registered Suite, not a run: the real-side field of the row states which tools run on the real tool in CI. An in-process interface fake, such as `fakeLedger` (`extra_coverage_test.go:150`), stands in for no outside tool and is not listed. A fake that is not listed and not installed through `fakeclitest` is not found. |
| `mock-call-order` | structure | `reflect.DeepEqual`, `slices.Equal` or `cmp.Diff` on the call log of a fake and a literal slice of two or more calls | A fake is a type that a test declares with a `fake`, `stub`, `mock` or `spy` prefix. A test whose claim is the order takes a row in the guard list. A loop compare is not found. |
| `golden-update-flag` | review | a test that writes a golden file with no `update` flag, or a golden file that a script regenerates without a reviewed diff. The finding is at file level. | A golden test with an `update` flag fails when the output drifts, so the flag alone is not a finding. A regeneration by a script is not found. |
| `error-text-match` | structure | `err.Error()` matched with a literal, when the producer package exports an `Err*` sentinel or an error type and the test has no `errors.Is` | an error from a package with no sentinel is not a finding |
| `sleep-sync` | structure | `time.Sleep` in a `_test.go` file, outside a fake clock. A call in a helper or a fake method has the function name as its key (D2). | a test that waits for a real timer by design takes a row in the guard list |

Two decisions about the guard exemption:

- A rule that reads the guard list reads data only. It never reads the name of a package.
- The rule `single-file-source-pin` runs on `acs/` as on `internal/`, so that T5 and T1 use one check (D7).

## 7. Components

- A *fix item* changes only files in the scope of `scope.json` (section 1).
- A *tooling item* changes the analyzer, the metric or a gate.
- The group key of an item is one package directory. An item has at most 25 test files. A package above the cap splits by file list. T8 is the one exception. An item owns a range of 50 `acs/cycleN` shards, and two items never share a range.
- An item that changes a protected path goes to the console, by the routing of the operating policy.
- The console changes the guard list and `max_rows` in the same item that needs it.

| Id | Component | Kind | Red test first | Owner |
|---|---|---|---|---|
| T1 | `internal/testaudit`: the rules of section 6, `guards.json`, `fakes.json`, `scope.json`, the shard baselines, `evolve test audit` and the ratchet (D1, D2, D3). T1 also adds `internal/fakecontract` with the `Suite` type only. The file `fakes.json` starts with the known rows, so the rule reports them at once. T1 ends with the false-positive gate, described below the table. | tooling | each rule's red fixture gives one finding, and its green fixture gives none. Red fixtures include a const compared with `NumField()`, a skip after `cfg, _ :=`, a walk that filters to one file, and a `fakeclitest.Install` script named `git`. A finding with no row fails the ratchet. | lane |
| T2 | The `apicover` metric of D4 and the per-package baseline of D4a, and the `Amended by` line of ADR-0050. T2 uses `go/types`. | tooling | a package whose test has only `var _ T = Exported` reports `Exported` as not called, unless the flipped file lists it. A listed export that a test now calls fails as stale. | console when `internal/apicover` is protected, else lane |
| T3 | The fixes of the `skip-hides-error` findings through `testhost` (D5), and `git` added to the three `TESTHOST_REQUIRED_TOOLS` lines of `go.yml`. | fix, plus one CI line | a test with a corrupt registry fixture fails, and does not skip. | lane for the tests, console for `go.yml` |
| P1 | A production fix in `internal/bridge/completion.go:122-127`: the adapter turns a non-zero exit code of `deps.Runner` into an error. The item is recorded through the issue-record skill, with its logs, root cause, fix and regression test. It is the only item of this plan that changes production code. | production | a runner that returns `(128, nil)` for `rev-parse` gives an error from `gitCmd`, and `poll` reports no completion. A failed `rev-list` that prints to stdout is not read as data. | console when `internal/bridge` is protected, else lane |
| T4 | The fakes of F3.7 and their contract suites for git, tmux, the clocks and the rollback stub (D8). The `Suite` type is in T1. Its fakes are test-support code, and `fakes.json` puts them in the fix scope. The bridge item starts after P1, and its suite asserts one contract: a non-zero exit gives an error. | fix | each mismatch of F3.7 has a contract case that fails on the old fake. | lane, one fake for each item |
| T5 | `evalqualitycheck` calls the one exported check of the T1 rule `single-file-source-pin`, and refuses a new predicate for which it gives a finding (D7). | tooling | a new predicate that reads a file with `os.ReadFile` and matches it with `strings.Contains` is refused. A second case with only `FileContains` is refused. | console when the eval gate is protected, else lane |
| T6 | Fix batches for the tautological findings outside `acs/` (D9). | fix | each rewritten test fails on a recorded mutant of its code. | lane |
| T7 | Fix batches for the structure findings outside `acs/` (D6, D9). | fix | each replacement test fails on a behavior mutant, and passes on a rename. | lane |
| T8 | Fix batches for the `acs/` predicates: convert or retire, one item for 50 cycles of predicates (D7, D9). | fix | each converted predicate fails on a behavior mutant. Each retired one names the unit test that covers its claim and records the mutant of that test. | lane |
| T10 | The admission checks of a fix item (D9, section 1). First, the item contract owes `mutation.json` through `AgentOwedFiles`. Second, `agents/evolve-auditor.md` re-runs each recorded mutant and gives FAIL for a live one. Third, a check refuses a fix item whose diff has a file outside `scope.json`. | tooling | a fix item with no `mutation.json` is rejected by the gate. A fix item with a recorded live mutant gets FAIL. A diff with a production `.go` file is refused, and a diff in a `fakes.json` location is accepted. | console when the contract and the auditor persona are protected, else lane |
| T9 | The 150 `apicover_named_test.go` files (155 with the variant names) after T2. Each item takes one package: it adds a call or an assertion for each listed export, and empties the flipped file of the package (D4a). | fix | `apicover -enforce` passes with an empty flipped file for the package. | lane |

The false-positive gate of T1:

- The console reads a sample of 8 findings for each rule, or all findings when a rule has fewer. It records yes or no for each in the item report.
- A rule with 2 or more false findings in its sample changes before any fix starts.
- More than 10 false findings in the whole sample also stops the fixes.

## 8. Order

1. T1 first. Its first run writes the shards and gives the real count. The false-positive gate decides if the rules change before anything else starts.
2. T2 with its baseline, T5 and T3. T2 is green at once because the flipped files hold each export that flips (D4a). P1 can run at the same time, as it does not depend on T1.
3. T10, after T1 and before any fix item. T4. The bridge item of T4 needs P1. The other fakes do not.
4. T6, T7, T9 and then T8 (each item starts after T10), until each shard and each flipped file is empty. The items of different packages run in parallel, because each item owns the shard of its package.

## 9. Verification

| Measure | Before (2026-11-18) | Target | Command |
|---|---|---|---|
| Findings in the shards | not measured. The first census gives estimates of about 65, 2, 15, 45 and up to 710 for the families of F3.1 to F3.6. The rules of M5 add families with no count. | 0 | `evolve test audit --json` |
| Exports that flip under D4 | not measured. The upper bound is 1,015 lines of the form `var _ .. =` or `_ = ` in all test files, and 45 in the `apicover_named` files (research, section 3). | 0 in each flipped file | the dry run below |
| Listed tools with no registered contract suite | not measured | 0 | the rule `unverified-fake` |
| Listed tools whose contract suite ran on the real tool in CI | not measured. Today only `git` and `tmux` can, after T3 (`gh`, `claude` and `codex` have no job). | every row has a CI job, or `none` with a reason that a reviewer accepted. Rows with `none` are at or below `max_rows`, and falling. | the real-side field of `fakes.json`, and the CI logs |
| Fix items accepted with no `mutation.json` | not guarded | 0 | the gate of T10 |
| New findings that land | not guarded | 0 | the ratchet of T1 in CI |
| Rows in the guard list | not measured | at or below `max_rows`, and falling | the test of `guards.json` |

The dry run of D4 has three steps. The console runs them once at T2 with no flipped files, and the result writes the files:

1. Build `apicover` with the new metric.
2. Run `bin/apicover -cover coverage.func.txt` over each directory of `go/.apicover-enforce` (367 patterns), as `make apicover-check` does.
3. Count, for each package, the symbols that flip: Covered by the old metric, Uncovered by the new one.

The date and the owner of goal 3 follow the count. The console sets the date in the wave plan after the T1 dry run, because the count is not measured today.

## 10. Risks

| Risk | Control |
|---|---|
| A rule gives false findings, and people learn to ignore it. | Red and green fixtures for each rule, and the false-positive gate of T1 before the fixes start. |
| A rewrite deletes a test that guarded a real claim. | Each deletion names the test that covers the claim, with its recorded mutant (D7, D9). The review of the item checks it. |
| A batch item touches many files, and lanes conflict. | One package for each item, at most 25 files, and one shard for each package (D2). |
| The mutation check costs too much. | One to three mutants for each rewritten test, and each runs `go test -overlay` on one package. The cap for an item follows the reviewer cap of ten mutants. A rewrite with no mutable statement records that fact. |
| An item needs a protected path. | The routing of the operating policy sends it to the console. The console changes the guard list and `max_rows` there. |
| The guard list grows to hide findings. | `max_rows` in the same file, and the pre-commit review fleet reads each diff that raises it. |
| A contract suite skips on a job that has the tool. | `testhost.Tool` fails on a job that declares the tool in `TESTHOST_REQUIRED_TOOLS`. T3 declares `git`. |
| A fix item records a mutant that nobody re-runs. | T10: the item owes `mutation.json`, and the auditor re-runs each mutant (D9). |
| The stale rows of a rename block unrelated lanes. | Rows have no line number, and `--rebaseline <package>` re-keys a moved test. It refuses a shard that grows. |

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| T1 analyzer, rules, config, ratchet | done | cycle 2489, [`0f9537a`](https://github.com/mickeyyaya/evolve-loop/commit/0f9537a5612b0c8c13acaede92bbff592b6e2a9d); cycle 2523, [`0718e2d`](https://github.com/mickeyyaya/evolve-loop/commit/0718e2d6d1499d0832641953bb9a15258bbc6c5b); cycle 2518, [`f84667e`](https://github.com/mickeyyaya/evolve-loop/commit/f84667e9234cc40eb8b4431ef8bcd530aa9ea505); `go/internal/testaudit/rules.go` | none |
| T2 `apicover` counts calls | done | cycle 2500, [`c87e787`](https://github.com/mickeyyaya/evolve-loop/commit/c87e7879638550417df391d4d24afb567aad31a3) | none |
| T3 skip fixes and `git` in `go.yml` | done | cycle 2539, [`6e12e60`](https://github.com/mickeyyaya/evolve-loop/commit/6e12e6032ae6f889e9a59b279593bd2d7d8478dc); cycle 2541, [`228bf32`](https://github.com/mickeyyaya/evolve-loop/commit/228bf32abef0dd1f36e78bf38c2668b8d31aa430) | none |
| P1 bridge non-zero exit gives an error | done | cycle 2490, [`d195ed6`](https://github.com/mickeyyaya/evolve-loop/commit/d195ed6e358f5289942ce111e48e8127b927904d); `go/internal/bridge/completion.go:124` | none |
| T4 fake contract suites (rollback git, bridge git, clock, tmux) | done | cycle 2512, [`650f180`](https://github.com/mickeyyaya/evolve-loop/commit/650f180dce5a7259de054e473f9cf6a876e558b3); cycle 2529, [`33b8548`](https://github.com/mickeyyaya/evolve-loop/commit/33b8548637944c7e385d7fad7f7eda5569812009); cycle 2511, [`45b6487`](https://github.com/mickeyyaya/evolve-loop/commit/45b648774476acb79760c7ce99719462e3f5ca7b); cycle 2509, [`b2486e6`](https://github.com/mickeyyaya/evolve-loop/commit/b2486e62bed06b3b57a1792ad65e0b99566f277a) | none |
| T5 `evalqualitycheck` refuses source pins | done | cycle 2499, [`b27e13c`](https://github.com/mickeyyaya/evolve-loop/commit/b27e13c68c27e597197d10e5254a751284b51eff) | none |
| T6 tautology fix batches | partial: core, bridge and rest done; `go/cmd/evolve` open | cycle 2506, [`77b47d7`](https://github.com/mickeyyaya/evolve-loop/commit/77b47d7b30c21d1c86c2f50515327a3fb9c732a3); cycle 2504, [`e876b05`](https://github.com/mickeyyaya/evolve-loop/commit/e876b054b6d7906db65bd7dedbb1706fd5817306); cycle 2501, [`ebf677e`](https://github.com/mickeyyaya/evolve-loop/commit/ebf677e65020027f6ed3b4fab1ecf1a39301a104) | `test-audit-t6c-tautology-cmd`, `test-audit-t6d-tautology-rest-ac-1b-console` |
| T7 structure-pin fix batches | done | cycle 2552, [`5cd11df`](https://github.com/mickeyyaya/evolve-loop/commit/5cd11df69984e97814e3b75d3cab6e356ae912d7) | none |
| T8 `acs/` predicate batches | not started | none | `test-audit-t8a-acs-predicates-early`, `test-audit-t8b-acs-predicates-mid`, `test-audit-t8c-acs-predicates-late` |
| T10 fix-item admission checks | not started | none | `test-audit-t10-fix-item-checks` |
| T9 `apicover_named_test.go` files | dropped (operator, 2026-11-22) | cycle 2514, [`c6c8eae`](https://github.com/mickeyyaya/evolve-loop/commit/c6c8eaedb81f2a83f2219aa1206948e4583c058d); cycle 2564, [`ca8b6f0`](https://github.com/mickeyyaya/evolve-loop/commit/ca8b6f025676462ddc5ff46dccc1007e2cf7caae) | none: coverage hygiene with no failed cycle cited; the 165 remaining files stay a background task. Old status: partial; the first batches landed, and 165 files remain on main |

The tooling and the first fix batches are on main. T6 for `go/cmd/evolve`, T8 and T10 have open items. The build floor now runs the test-audit ratchet ([#1597](https://github.com/mickeyyaya/evolve-loop/pull/1597)). The ship backstop does not run it yet, and the item `ship-backstop-runs-the-test-audit-ratchet` tracks that. The operator dropped T9 on 2026-11-22.

The follow-up draft `followups/test-audit-t9-apicover-named-tests-remaining.json` stays unfiled.

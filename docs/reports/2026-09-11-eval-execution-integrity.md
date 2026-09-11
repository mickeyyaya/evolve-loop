# Eval execution integrity recovery

**Date:** 2026-09-11

**Scope:** `go/internal/verifyeval`

**Trigger:** post-#560 diagnostic evo-loop wave

## Finding

The diagnostic wave generated two acceptance commands with `go test -run`
selectors that matched no tests. Go printed `[no tests to run]` and returned exit
code `0`, so command success did not prove that either acceptance criterion ran.

The host verifier had two broader execution gaps. It enforced an exit code only when
the eval contained a machine-readable `## Expected` section. The tracked corpus
contains 314 eval files with bash command blocks and none with that section.
Consequently, a command returning a normal nonzero test failure still received a
verifier PASS unless another optional output predicate happened to reject it.
This contradicted the verifier's role as the independent pre-ship trust boundary.

The old parser also treated every non-comment physical line as an independent
shell. That model cannot execute the tracked corpus: fences use assignments,
`cd`, loops, quoted arguments, and backslash continuations whose state and syntax
span lines. Ignoring exit codes had hidden those invalid fragment executions.

The corpus audit also found prose-only negative expectations. Prose cannot safely
tell the verifier whether exit `1` is the expected rejection or a broken grader.
Three prose-only negative graders were corrected. Two absence checks now require
`grep` exit `1` and reject operational errors. A brittle source grep for the
extend path now runs the existing named behavioral test instead.

## Contract

`evolve eval verify` now applies these rules to every parsed bash fence:

1. One fence is one Bash script and one result. Variables, the current directory,
   control flow, and continuations persist inside it; separate fences use separate
   shells.
2. Bash pipeline failure propagation is enabled. Normal Bash status handling is
   preserved, including `cmd; rc=$?` and conditional negative probes.
3. The expected final script exit code is `0` by default.
4. An explicit `exit_code` under `## Expected` overrides that default, including
   intentional nonzero probes.
5. A script that invokes `go test` with `-run` or `--run` fails when its captured
   output reports no matching tests or no test files and contains no evidence
   that another selected package or test ran.
6. A wide Go test command remains a valid compile or empty-package check.
7. A recursive narrowed command remains valid when at least one package records
   a normal `ok` result or the output contains a text or JSON test-run event.
8. A file with no executable bash fence fails rather than passing vacuously.

The machine-readable section is the only expectation override. Narrative text
such as `Expected: no match` does not change exit semantics. A negative grader
normally checks the rejected command itself and exits `0` when the rejection is
proved; the retained global `## Expected` format supports the uncommon
single-exit override.

The verifier performs the new check on stdout and stderr it already captured.
It does not rerun a command, rewrite the selector, or add provider/model work.
Runtime cost is linear in the command output size and no new dependency is
introduced.

The runner does not inject `set -e`. Existing evals deliberately inspect
nonzero statuses, and forcing fail-fast execution would exit before their
assertions. A fence that needs fail-fast behavior declares it in the script.
Without that declaration, normal Bash semantics allow a later successful
command to become the fence result; detecting whether that was deliberate would
require interpreting arbitrary shell intent. Authoring checks, rather than the
execution runner, own that advisory concern.

## TDD evidence

The first test supplied exit `0` plus the real Go zero-match marker. Before the
fix, `TestVerify_NarrowedGoTestWithNoMatches_FAIL` failed because the verifier
returned PASS. The second test supplied a nonzero `go test` result in an eval
with no Expected section. Before the default was added,
`TestVerify_NoExpectedSectionNonzeroExit_FAIL` also failed because the verifier
returned PASS.

The first architecture/Go/simplifier review found the line-fragment model and
two additional native Go output shapes: a local-package zero match prints a
warning plus an ordinary `ok`, and a selected package without test files prints
`[no test files]`. Those findings were treated as blocking and added as RED
contracts before the implementation was revised.

The GREEN matrix covers the defects and their preservation boundaries:

- zero-match narrowed test: FAIL with `matched no tests` context;
- local-package zero match and selected package with no test files: FAIL;
- one matching package plus one empty package: PASS;
- wide test over a package with no test files: PASS;
- missing Expected section plus nonzero exit: FAIL with expected/observed code;
- explicit expected nonzero exit: PASS;
- no executable bash fence: error instead of a vacuous PASS;
- one multiline fence preserves variables, `cd`, and continuations;
- a script can inspect and assert a prior nonzero status;
- a failed final pipeline is preserved through `pipefail`;
- separate fences do not share shell state;
- quoted shell punctuation does not hide a later `-run` selector, while a
  selector in a separate shell command is not attributed to the Go test;
- redirections before `-run` and ordinary `$(go test ...)` output capture do not
  bypass selector detection;
- migrated absence graders reject both forbidden matches and `grep` execution
  errors, and the extend-path grader now executes behavior instead of filtering
  source text.

Package tests for `internal/verifyeval` and its CLI adapter run after the focused
matrix. The repository-wide format, vet, test, commit-gate, and review results
are recorded in the pull request that promotes this slice.

## Boundary and remaining work

This repair verifies commands parsed from bash fences. Evals that use only the
`score_cap`/`evidence` form remain under the ACS suite; sending one to this
bash-fence verifier now returns an explicit error. The verifier does not claim
that arbitrary shell output is impossible to spoof; it closes the observed
native Go success-with-zero-execution shape and makes ordinary shell failure fail
closed.

The diagnostic wave also exposed independent router-artifact and provider-usage
collection defects. They are separate modules and must use separate TDD and PR
cycles before the two accepted live validation waves run.

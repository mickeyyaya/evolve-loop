# Build Explanation — Cycle 1783

## Build Binding
- Cycle: 1783
- Base SHA: 49a2e66a1759df1547c7f2d45f0d2367897ae553

## Summary
Three silent failures in `internal/evalgate` are closed. Gate C (floor-binding) now blocks and names `triage-decision.json` when that companion is present but unreadable or malformed, instead of discarding the read and parse errors. Gate A (evals-materialized) now reads the workspace eval before the project root's and its remediation for a graderless eval names the writable workspace path, never the deny-write project root. `LintMonotonicBinaryTarget`, which had only test callers, now runs in `evolve inbox add` as an advisory warning.

## Rationale
A gate that ignores a malformed declaration silently falls back to prose and can approve a floor predicate triage deferred; failing loud names the one file to fix. Gate A's scout runs under a sandbox where the project root is deny-write, so a remediation naming a root path cannot be applied; reading the workspace first makes the named path the one the gate checks. Deleting the monotonic lint was rejected because `go/acs/cycle1190` predicates call it and the builder may not edit ACS predicates; the inbox intake is the one place where an item's class and acceptance criteria are both at hand and the author can still rewrite them.

## Changed Areas
- `go/internal/evalgate/floorbinding.go` — propagates `ReadDeferredFloors`/`ReadDeclaredFloors` errors into a blocking reason naming the companion; the stale "fails open on every ambiguity" doc comment is removed.
- `go/internal/evalgate/floorbinding_test.go` — table test for invalid JSON and wrong field types blocking, plus a test that a malformed companion is irrelevant without floor predicates.
- `go/internal/evalgate/materialization.go` — `evalFilePath` checks the workspace before the project root; remediation names workspace paths for missing and graderless evals through one `workspaceEvalPaths` helper.
- `go/internal/evalgate/materialization_graders_test.go` — the existing grader test now expects the workspace path; a new test proves a graded workspace eval satisfies the gate over a graderless root eval.
- `go/cmd/evolve/cmd_inbox_add.go` — after a successful filing, decodes the item and prints one `WARN` line per `LintMonotonicBinaryTarget` finding; filing is unaffected.
- `go/cmd/evolve/cmd_inbox_add_test.go` — covers a monotonic item warned exactly once and a one-shot class not warned.
- `docs/architecture/packages/internal-evalgate.md` — design notes record the new lookup order, the Gate C exception to fail-open, and the lint's production caller.

## Design Decisions
The malformed-companion block applies only once floor predicates exist, so a cycle with no floor predicate is never blocked by an unrelated triage file. An absent companion stays a prose fallback. The lint is advisory at intake, not blocking, because it is deliberately narrow and an absolute target can be legitimate.

## Verification
`go test -count=1 ./internal/evalgate/ ./cmd/evolve/` and `go test -tags acs ./acs/cycle1783/ ./acs/cycle1190/` pass; TestC1783_001 and TestC1783_004 turned from RED to GREEN.

## Compatibility
No exported identifier, flag or file format changes. A cycle with a malformed `triage-decision.json` and floor predicates now blocks at tdd where it used to pass.

## Limitations
Items filed by paths other than `evolve inbox add` (for example autofile) are not linted. A malformed companion is still silently tolerated by triagecap's other readers, such as `CommittedFloorCount`.

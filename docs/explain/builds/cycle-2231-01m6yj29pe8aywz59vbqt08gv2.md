# Build Explanation — Cycle 2231

## Build Binding
- Cycle: 2231
- Base SHA: 4497ad2e7497395de9a5cb5c3b0efc46abd6f317

## Summary
A duplicate ratchet now counts the extra copies of equal test helper bodies. The baseline file holds 334 copies. Two of three copies of `min` and all three list-contains helpers moved to the standard library. The count went down from 338 to 334.

## Rationale
The plan (test-system section 6.4) asks for a count that only goes down. A scanner in a new package, plus one structure test in `internal/repocontract`, is the smallest design that does this. The age of an `acs/` directory comes from one `git log` call, not from file mtimes, because a fresh checkout resets each mtime.

## Changed Areas
- `go/internal/testdup/testdup.go` — the scanner, `Check`, and the baseline parser and formatter.
- `go/internal/testdup/testdup_test.go` — tests of the scanner, the ratchet and the baseline format.
- `go/internal/testdup/apicover_named_test.go` — names each exported symbol of the package in a real assertion.
- `go/internal/repocontract/testdup_ratchet_test.go` — the structure test that runs the ratchet on the real tree.
- `go/.test-dup-baseline` — holds `count=334` and `date=2026-11-09`.
- `go/.apicover-enforce` — enrolls `./internal/testdup` in the repo-wide apicover gate.
- `go/acs/cycle2231/predicates_test.go` — the predicates of this cycle.
- `.evolve/evals/test-dup-ratchet.md` — the eval graders of this task.
- `docs/architecture/packages/internal-testdup.md` — the package page.
- `go/internal/testdup/testdup_edges_test.go` — tests of the error paths and the ordering of groups.
- `go/internal/bedrock/bedrock_test.go` — removes a copy of `min`.
- `go/internal/subagent/subagent_test.go` — removes a copy of `min`.
- `go/cmd/evolve/cmd_ci_gate_integration_test.go` — `slices.Contains` replaces `slicesHas`.
- `go/cmd/evolve/cmd_code_test.go` — `slices.Contains` replaces `containsString`.
- `go/internal/skillcheck/instructions_test.go` — `slices.Contains` replaces `containsString`.

## Design Decisions
The body key is the token stream of the function body. A formatter or a comment cannot hide a copy. Bodies of one statement are not counted, because no replacement is worth the work. `Check` also fails below the baseline, which forces the baseline down with the count.

## Verification
`go test` of `internal/testdup` covers the fixture trees, the date rule and the baseline format. The structure test passes on the real tree at 334. The acs predicates of cycle 2231 pass.

## Compatibility
No production code changes. The five edited test files keep their tests.

## Limitations
The scanner does not find near-equal bodies or equal bodies with renamed locals. 334 copies remain; later cycles lower the count.

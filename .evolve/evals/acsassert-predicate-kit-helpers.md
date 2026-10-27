---
score_cap:
  - criterion: "ModuleDir, OutputTail, FuncCoverage and WalkModule each have a table test and a failure case that pass"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestModuleDir_|TestOutputTail_|TestFuncCoverage_|TestWalkModule_' ./pkg/acsassert/"
  - criterion: "Every exported helper of pkg/acsassert is named by a test that runs it (apicover reports 0 uncovered, 0 false-green)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -coverprofile=/dev/null ./pkg/acsassert/ && go run ./cmd/evolve apicover -enforce pkg/acsassert"
  - criterion: "The tdd persona and go/acs/README.md name all four new helpers"
    max_if_missing: 6
    evidence: "for n in ModuleDir OutputTail FuncCoverage WalkModule; do grep -q \"\\`$n\\`\" agents/evolve-tdd-engineer.md && grep -q \"\\`$n\\`\" go/acs/README.md || exit 1; done"
  - criterion: "Every function in the kit file is covered at 100.0%"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -coverprofile=/tmp/acsassert-kit.out ./pkg/acsassert/ && go tool cover -func=/tmp/acsassert-kit.out | awk '$1 !~ /assertions.go|go_tests.go|^total:/ && $NF != \"100.0%\" {bad=1} END {exit bad}'"
---

# Eval: pkg/acsassert predicate kit (test-system plan T6)

> Pins the four helpers that component T6 adds to `go/pkg/acsassert`: the module
> directory, the tail of an output, the coverage of one function and a walk of the
> module. Cycles wrote 597 copies of these helpers (research F4.2). The kit stops
> the copies only when each helper has a failure case, a naming test and a place in
> the predicate personas. Source: cycle 2044, inbox item
> `test-system-efficiency-t6-predicate-kit`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| table-and-failure | Each helper has a table test and a failure case | 4/10 | `go test -run 'TestModuleDir_\|TestOutputTail_\|TestFuncCoverage_\|TestWalkModule_' ./pkg/acsassert/` |
| apicover | Each exported helper is named by a test that runs it | 6/10 | `evolve apicover -enforce pkg/acsassert` |
| persona-names | Persona and README name the helpers | 6/10 | `grep` of the four names in both files |
| kit-coverage | Each kit function stays at 100.0% | 7/10 | `go tool cover -func` filtered to the kit file |

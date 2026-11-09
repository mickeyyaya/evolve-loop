---
score_cap:
  - criterion: "Compress keeps every failure line byte-exact and halves the tokens of bulky go test, build and diff fixtures"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2248_00[1-3]' ./acs/cycle2248"
  - criterion: "Never-compress regions survive byte-exact inside bulky logs"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2248_007' ./acs/cycle2248"
  - criterion: "Policy decoding refuses an absent or invalid log_compression block and the package holds no numeric literal"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2248_011|TestC2248_012|TestC2248_014' ./acs/cycle2248"
  - criterion: "logcompress is enrolled in apicover, names every exported symbol, and passes vet and the race detector"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2248_015|TestC2248_016' ./acs/cycle2248"
---

# Eval: Deterministic log compression before prompt injection

> Pins the contract of `logcompress.Compress`, the pure filter that collapses passing-test runs, repeated stack frames, build download noise and large diff bodies while keeping every failure line byte-exact. Thresholds, ratios and pattern lists come from the `log_compression` block of policy.json. Wiring at the injection boundary and the token soak (acceptance 3) are deferred by triage in cycle 2248.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| golden-fixtures | Failures exact, tokens halved on go test, build and diff fixtures | 4/10 | `go test -tags acs -run 'TestC2248_00[1-3]' ./acs/cycle2248` |
| never-compress | Contract regions untouched | 5/10 | `go test -tags acs -run 'TestC2248_007' ./acs/cycle2248` |
| policy-only-thresholds | No literal threshold in Go | 6/10 | `go test -tags acs -run 'TestC2248_011\|TestC2248_012\|TestC2248_014' ./acs/cycle2248` |
| apicover-vet-race | Enrolled, named, vet and race green | 6/10 | `go test -tags acs -run 'TestC2248_015\|TestC2248_016' ./acs/cycle2248` |

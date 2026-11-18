---
score_cap:
  - criterion: "No apicover_named_test.go outside the protected surface holds a `var _ = ...` discard line"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_001' ./acs/cycle2514"
  - criterion: "No asserts-own-setup finding remains for a test of an unprotected apicover_named_test.go, and none sits in a baseline shard"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_002' ./acs/cycle2514"
  - criterion: "The discard detector flags a discard, spares a call and a quoted string, and testaudit still flags a self-asserting test"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_003' ./acs/cycle2514"
  - criterion: "Every exported symbol of an apicover-enforced package with a named test file stays named by a test"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_004' ./acs/cycle2514"
  - criterion: "The protected-file carve-out (4 files, 8 discard lines, 1 asserts-own-setup finding) never grows"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_005' ./acs/cycle2514"
  - criterion: "Every package that starts a real tmux server in its tests calls tmuxtest.Main from a TestMain (faketmux included)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_006' ./acs/cycle2514"
  - criterion: "The faketmux test package compiles and vets with its guard, with no import cycle against tmuxtest"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2514_007' ./acs/cycle2514"
---

# Eval: Replace the apicover_named_test.go discards with calls (T9)

> Pins the T9 test-quality work. An `apicover_named_test.go` names an export with `var _ = X{}` or `var _ T = f`,
> which satisfies apicover without running the export. Each such line becomes a call whose result is asserted.
> The rewrite must not add an asserts-own-setup finding, and must keep every export named.
> Source incident: cycle 2514, inbox item test-audit-t9-apicover-named-tests. Four files sit on a protected
> surface (commitgate, adapters/ledger, phaseintegrity, guards); the console follow-up owns them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| zero-discards | no `var _ = ...` line in an unprotected named file | 3/10 | `go test -tags acs -run TestC2514_001 ./acs/cycle2514` |
| zero-own-setup | no asserts-own-setup finding or baseline row | 4/10 | `go test -tags acs -run TestC2514_002 ./acs/cycle2514` |
| detector-sanity | detectors separate a discard from a call | 5/10 | `go test -tags acs -run TestC2514_003 ./acs/cycle2514` |
| exports-named | apicover naming survives the rewrite | 4/10 | `go test -tags acs -run TestC2514_004 ./acs/cycle2514` |
| carve-out-ceiling | protected carve-out does not grow | 5/10 | `go test -tags acs -run TestC2514_005 ./acs/cycle2514` |
| tmux-orphan-guard | Every tmux test package calls tmuxtest.Main (audit round 3 H1, faketmux) | 6/10 | `go test -tags acs -run TestC2514_006 ./acs/cycle2514` |
| faketmux-compiles | faketmux test package vets with the guard | 4/10 | `go test -tags acs -run TestC2514_007 ./acs/cycle2514` |

---
score_cap:
  - criterion: "No tautological testaudit finding remains in internal/bridge, and no baseline shard under it carries one"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_001' ./acs/cycle2504"
  - criterion: "Every flagged bridge test is rewritten in place, none is deleted or renamed"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_002' ./acs/cycle2504"
  - criterion: "The concurrent-launch test fails on a mutant of Engine.Launch that swallows the launch outcome"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_003' ./acs/cycle2504"
  - criterion: "The doctor-report test fails on a mutant deep probe and on a mutant verdict"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_004' ./acs/cycle2504"
  - criterion: "The joined-pane test fails on a mutant that reads a plain pane"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_005' ./acs/cycle2504"
  - criterion: "The golden-rows test fails on a duplicated or a dropped golden row"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_006' ./acs/cycle2504"
  - criterion: "A key built from production rows is not flagged asserts-own-setup, so the detector is fixed and no caller changes syntax to dodge it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_007' ./acs/cycle2504"
  - criterion: "A recorder that the test hands to the code under test is not flagged asserts-own-setup"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_008' ./acs/cycle2504"
  - criterion: "A pointer literal or a key built only from literals is still flagged asserts-own-setup"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_009' ./acs/cycle2504"
  - criterion: "Every exported symbol of the apicover-enforced ./internal/bridge stays named by a test after the rewrites"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2504_010' ./acs/cycle2504"
---

# Eval: Fix the tautological test findings in internal/bridge (T6b)

> Pins the internal/bridge half of the T6 test-quality work. The testaudit analyzer reports 4 tautology-rule
> findings in internal/bridge: `TestEngine_ConcurrentLaunch_OnBootRaceFree` (no-assertion),
> `TestDoctorReport_BoundByDoctor`, `TestLaunchOutcome_GoldenRowsCoverEveryCodeCtxAndFixture` and
> `TestClaudeCloudDriver_ReadsTheConfirmationFromTheJoinedPaneWhenTheControllerOffersIt` (asserts-own-setup).
> Two are weak tests: the concurrent-launch test asserts nothing, and the doctor-report test asserts a struct it
> builds itself to name the DeepProbe type. Two are detector false positives: a key built from decoded rows, and a
> recorder pointer that the test hands to the code under test. The mutant predicates compile the bridge test
> binary with a `go test -overlay` copy of one production file, so the rewritten test runs against the mutant.
> Source incident: cycle 2501 (inbox item test-audit-t6d-tautology-rest), whose first build dodged the
> detector with syntax changes. Predicates 007 and 008 keep the same dodge out of this lane.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| zero-findings | no tautology finding remains in internal/bridge | 3/10 | `go test -tags acs -run TestC2504_001 ./acs/cycle2504` |
| rewrite-not-delete | every flagged test still exists | 4/10 | `go test -tags acs -run TestC2504_002 ./acs/cycle2504` |
| mutant-launch | no-assertion test kills a swallowed launch outcome | 3/10 | `go test -tags acs -run TestC2504_003 ./acs/cycle2504` |
| mutant-doctor | doctor test kills a broken probe and verdict | 4/10 | `go test -tags acs -run TestC2504_004 ./acs/cycle2504` |
| mutant-pane | joined-pane test kills a plain-pane read | 4/10 | `go test -tags acs -run TestC2504_005 ./acs/cycle2504` |
| mutant-golden | golden test kills a duplicated or dropped row | 4/10 | `go test -tags acs -run TestC2504_006 ./acs/cycle2504` |
| detector-production-key | production-built key is not own setup | 4/10 | `go test -tags acs -run TestC2504_007 ./acs/cycle2504` |
| detector-handed-out | recorder handed to production is not own setup | 4/10 | `go test -tags acs -run TestC2504_008 ./acs/cycle2504` |
| detector-true-positive | literal-only values are still own setup | 5/10 | `go test -tags acs -run TestC2504_009 ./acs/cycle2504` |
| apicover-named | no enforced bridge export is left unnamed | 4/10 | `go test -tags acs -run TestC2504_010 ./acs/cycle2504` |

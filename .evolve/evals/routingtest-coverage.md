---
score_cap:
  - criterion: "internal/routingtest statement coverage is >= 70%"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/routingtest/... -cover 2>&1 | grep -qE 'coverage: (7[0-9]|[89][0-9]|100)(\\.[0-9]+)?% of statements'"
  - criterion: "at least 7 TestInvariant_* tests PASS (one per invariantChecks key)"
    max_if_missing: 7
    evidence: "cd go && [ \"$(go test ./internal/routingtest/... -v -run TestInvariant 2>&1 | grep -c '^--- PASS')\" -ge 7 ]"
  - criterion: "at least 5 TestBrick* tests PASS"
    max_if_missing: 5
    evidence: "cd go && [ \"$(go test ./internal/routingtest/... -v -run TestBrick 2>&1 | grep -c '^--- PASS')\" -ge 5 ]"
  - criterion: "at least 1 TestEngine* test PASS (RunAll/runPure pipeline exercised)"
    max_if_missing: 5
    evidence: "cd go && go test ./internal/routingtest/... -v -run TestEngine 2>&1 | grep -q '^--- PASS: TestEngine'"
  - criterion: "negative case TestInvariant_DuplicatePhaseRejected runs and PASSes"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/routingtest/... -v -run TestInvariant_DuplicatePhaseRejected 2>&1 | grep -q '^--- PASS: TestInvariant_DuplicatePhaseRejected'"
  - criterion: "no regression — dual-rendering keystone stays green"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/routingtest/... -run TestSignalSpec_DualRenderingAgree 2>&1 | grep -q '^ok'"
---

# Eval: routingtest-coverage

> Pins the `internal/routingtest` package's test depth at the level reached in
> cycle 265: the 8 kernel-floor invariants in `invariants.go`, the
> `RunAll`/`runPure`/`buildConfig` engine pipeline in `engine.go`, and the Brick
> catalog in `bricks.go` must stay exercised (>=70% statement coverage). The
> routing kernel is the "model proposes, kernel disposes" integrity floor; if its
> test harness rots back toward the 23% baseline, adversarial-proposal regressions
> can land unseen. Source incident: the `routingtest-coverage` task FAILed cycles
> 263 and 264 (builder under-delivered test count both times); cycle 265 scoped the
> criteria with exact PASS-count thresholds and exact test-name prefixes to enforce
> delivery. This eval migrates the cycle-263/264 draft to the canonical `score_cap`
> frontmatter so the audit score-cap mechanism actually reads it (the prior draft
> used a non-enforced `## Acceptance Criteria` body and was never git-tracked — the
> cycle-92/93 drop-at-ship defect mode).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | routingtest statement coverage >= 70% | 6/10 | `go test ./internal/routingtest/... -cover` reports >=70% |
| invariant-breadth | >=7 TestInvariant_* PASS (one per `invariantChecks` key) | 7/10 | `go test -v -run TestInvariant \| grep -c '^--- PASS'` >= 7 |
| brick-breadth | >=5 TestBrick* PASS | 5/10 | `go test -v -run TestBrick \| grep -c '^--- PASS'` >= 5 |
| engine-pipeline | >=1 TestEngine* PASS (RunAll path) | 5/10 | `go test -v -run TestEngine` shows a `--- PASS: TestEngine` |
| negative-duplicate | TestInvariant_DuplicatePhaseRejected runs+PASSes | 7/10 | `--- PASS: TestInvariant_DuplicatePhaseRejected` present |
| no-regression | dual-rendering keystone stays green | 8/10 | `go test -run TestSignalSpec_DualRenderingAgree` → `ok` |

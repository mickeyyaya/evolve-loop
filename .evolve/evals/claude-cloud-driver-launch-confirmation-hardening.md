---
score_cap:
  - criterion: "A forged View row or a Resume row of another id confirms no cloud session"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2575_00[123]_' ./acs/cycle2575"
  - criterion: "The cloud prompt holds no challenge token and no absolute local path"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2575_(005|006|01[4-9])_' ./acs/cycle2575"
  - criterion: "A remote head that differs from the local head refuses the launch before any session"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2575_0(07|08|09|10)_' ./acs/cycle2575"
  - criterion: "Each function of the cloud driver files is fully covered and the changed files hold no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2575_01[23]_' ./acs/cycle2575"
---

# Eval: claude-cloud driver launch confirmation hardening

> Pins the three hardening fixes F2, F3 and F5 of the adversarial review of the claude-cloud bridge driver (PR 1272). A launch is confirmed only by a Resume row with the id of the View row. The prompt that goes to the cloud holds no challenge token, in any position of a line or in the shape the phase runner writes, and no absolute path. The remote head, read with git ls-remote, must equal the local head.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| forged-row | A forged View row or a Resume row of another id confirms nothing | 4/10 | `TestC2575_001`, `002`, `003` |
| prompt-leak | No challenge token and no absolute path in the cloud prompt | 5/10 | `TestC2575_005`, `006`, `014`, `015`, `016`, `017` (the runner section shape of preparation.go:118, cycle 2575 audit H1; paths outside the five roots, M1) |
| stale-head | A remote head that differs refuses the launch | 5/10 | `TestC2575_007`, `009`, `010` |
| coverage | Full function coverage and zero comments | 6/10 | `TestC2575_012`, `013` |

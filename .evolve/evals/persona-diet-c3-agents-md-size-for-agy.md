---
score_cap:
  - criterion: "AGENTS.md, a rule file that agy loads, fits the 24,000-byte cut"
    max_if_missing: 6
    evidence: "test \"$(wc -c < AGENTS.md)\" -le 24000"
  - criterion: "GEMINI.md, a rule file that agy loads, fits the 24,000-byte cut"
    max_if_missing: 6
    evidence: "test \"$(wc -c < GEMINI.md)\" -le 24000"
  - criterion: "evolve skills check passes after the change"
    max_if_missing: 7
    evidence: "cd go && go run ./cmd/evolve skills check"
  - criterion: "a skills check rejects an AGENTS.md over the cut and accepts one at the cut"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2674_00[12]_' ./acs/cycle2674"
---

# Eval: Each rule file that agy loads fits under its 24,000-byte cut

> Pins the size cut of the rule files that agy loads. agy walks from the working directory to the repository root, loads each `GEMINI.md` and `AGENTS.md`, and cuts a file at 24,000 bytes on a line boundary. `AGENTS.md` was 53,652 bytes, so an agy seat lost the end of its rules. Source: inbox item persona-diet-c3-agents-md-size-for-agy, cycle 2674, docs/plans/persona-diet-2026-11.md row C3.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| loaded-agents | AGENTS.md at most 24,000 bytes | 6/10 | `wc -c < AGENTS.md` |
| loaded-gemini | GEMINI.md at most 24,000 bytes | 6/10 | `wc -c < GEMINI.md` |
| skills-check | `evolve skills check` exits 0 | 7/10 | `go run ./cmd/evolve skills check` |
| gate-behavior | the check rejects over the cut, accepts at the cut | 7/10 | `go test -tags acs -run TestC2674_00[12]_` |

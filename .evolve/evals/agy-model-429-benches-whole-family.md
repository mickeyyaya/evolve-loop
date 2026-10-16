---
score_cap:
  - criterion: "An agy compact reset hint (Resets in 148h8m53s) keeps the 429 line as Evidence and caps BenchedUntil at now+24h"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1911_00[12]_' ./acs/cycle1911"
  - criterion: "An Argon-scoped bench leaves fast and balanced agy-tmux seats first and walks deep and top seats on"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1911_003_' ./acs/cycle1911"
  - criterion: "clihealth.Entry carries a scope finer than Family; legacy family-wide entries still bench every agy tier, and scoped and family entries coexist"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1911_00[45]_' ./acs/cycle1911"
  - criterion: "A deep agy-tmux quota wall through the bridge walk writes one Argon-scoped bench, not a family-wide one"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1911_006_' ./acs/cycle1911"
  - criterion: "The clihealth and bridgechain unit suites stay green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/clihealth/ ./internal/bridgechain/"
---

# Eval: A quota wall on one agy model benches only that model group

> On 2026-10-17 at 02:17 a Gemini 4 Argon 429 (`Individual quota reached ... Resets in 148h8m53s`) benched the whole agy family for 30 minutes while Gemini 3.8 Flash still answered. Cycles 1907 and 1908 then ran every phase on Claude Code. The bench also kept the pane's first line as evidence, because no reset hint matched agy's compact duration. This eval pins both fixes. A bench is scoped to the walled model group, and the compact reset is parsed and capped at 24h. Source: inbox `agy-model-429-benches-whole-family`, cycle 1911.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| compact-reset | 429 line is Evidence; BenchedUntil = now+24h; garbage and zero durations rejected | 7/10 | `go test -tags acs -run 'TestC1911_00[12]_' ./acs/cycle1911` |
| model-scoped-bench | Argon bench spares fast/balanced Flash seats, walks deep/top seats | 8/10 | `go test -tags acs -run 'TestC1911_003_' ./acs/cycle1911` |
| store-compat | legacy family entries read and bench; scoped + family coexist | 7/10 | `go test -tags acs -run 'TestC1911_00[45]_' ./acs/cycle1911` |
| wall-wiring | the walk's deep wall writes an Argon-scoped bench | 8/10 | `go test -tags acs -run 'TestC1911_006_' ./acs/cycle1911` |
| no-regression | clihealth and bridgechain suites green | 6/10 | `go test ./internal/clihealth/ ./internal/bridgechain/` |

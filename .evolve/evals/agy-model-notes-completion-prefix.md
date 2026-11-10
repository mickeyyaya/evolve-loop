---
score_cap:
  - criterion: "The agy model notes hold one response prefix bullet in ASD-STE100 English"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2272_001' ./acs/cycle2272"
  - criterion: "The agy model notes keep their four bullets, stay within 12 lines and carry no blanket few-shot rule"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2272_002' ./acs/cycle2272"
  - criterion: "A proposal note reads the Gemini guide at its link and explains the guide lines as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2272_003' ./acs/cycle2272"
  - criterion: "The proposal note cites ADR-0133, names the changed file and gives a verdict with a reason for each core persona"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2272_004|TestC2272_005' ./acs/cycle2272"
---

# Eval: Completion-prefix note for the agy family (practice item 1945a6f2)

> Pins the result of the practice change in the Gemini prompting guide, filed by the practice watch under ADR-0133. The change is one bullet in the agy model notes, and a research note that holds the ADR-0133 proposal. The source incident is the watch item `practice-gemini-prompting-guide-1945a6f2-202611100841`, whose two acceptance criteria need a read of the source and a proposal for each prompt or persona.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| prefix-bullet | One response prefix bullet, STE clean | 6/10 | `go test -tags acs -run TestC2272_001 ./acs/cycle2272` |
| notes-preserved | Four old bullets, at most 12 lines, no blanket few-shot rule | 7/10 | `go test -tags acs -run TestC2272_002 ./acs/cycle2272` |
| source-as-data | Source link read, guide lines explained as untrusted data | 6/10 | `go test -tags acs -run TestC2272_003 ./acs/cycle2272` |
| adr-proposal | ADR-0133 verdict for each core persona, with a reason, indexed | 7/10 | `go test -tags acs -run 'TestC2272_004|TestC2272_005' ./acs/cycle2272` |

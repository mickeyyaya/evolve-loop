---
score_cap:
  - criterion: "The console skill tells the console to ask the operator on an exit 3 refusal of evolve inbox add, with the ids, the withdraw option and the --allow-overlap --reason option"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2288_00[12]' ./acs/cycle2288"
  - criterion: "internal-inboxrank.md and internal-inboxbatch.md describe the origin field and the exit 3 overlap refusal"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2288_00[345]' ./acs/cycle2288"
  - criterion: "The added doc lines pass evolve docs ste-lint --strict and the docs are changed"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2288_006' ./acs/cycle2288"
---

# Eval: Console instruction for the add-time overlap refusal (inbox plan P6)

> Pins the console instruction and the package notes for the add-time intake that P3 shipped in its revised form: `evolve inbox add` refuses an overlap with exit 3 and the ids, unless `--allow-overlap --reason`. The pending intake and `evolve inbox resolve` of the first plan do not exist (inbox-curator.md D2 rejected them), so no doc may claim them. Source incident: cycle 2273 (the card carried the superseded premise).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| console-instruction | skills/loop/SKILL.md exit 3 paragraph | 6/10 | `go test -tags acs -run 'TestC2288_00[12]' ./acs/cycle2288` |
| package-notes | origin and exit 3 in two package notes | 6/10 | `go test -tags acs -run 'TestC2288_00[345]' ./acs/cycle2288` |
| ste-lint | added lines pass strict lint | 5/10 | `go test -tags acs -run 'TestC2288_006' ./acs/cycle2288` |

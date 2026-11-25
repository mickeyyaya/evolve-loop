---
score_cap:
  - criterion: "ScoreRecall computes recall@k and MRR with the cut at k, one count for a repeated id, and an error for an unscorable input"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2632 -run 'TestC2632_00[1-4]'"
  - criterion: "The checked-in gold set holds at least 30 labelled queries from past cycle plans, and each relevant id exists in the checked-in corpus snapshot"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2632 -run 'TestC2632_00[56]'"
  - criterion: "evolve kb recall score prints recall@5 and MRR for FileKB.Lookup and refuses a set that cannot be scored"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2632 -run 'TestC2632_00[78]'"
  - criterion: "The recorded FileKB.Lookup baseline equals the live score of the default command, and the gold data is not git-ignored"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2632 -run 'TestC2632_(009|010)'"
  - criterion: "Each gold query is quoted from its plan dossier, is not copied from its relevant lesson's own pattern or id, has judged multi-entry relevance, and records a labeller and rationale"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2632 -run 'TestC2632_01[2-5]'"
---

# Eval: knowledge-base recall gold set and score

> Pins the measurement set of approach D, step 0, in docs/research/qmd-knowledge-base-2026-11.md. Each later ranker (BM25, RRF, qmd) is judged against the FileKB.Lookup baseline that this task records. Source: cycle 2632.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| scorer-math | recall@k and MRR values, cut at k, duplicate once, refusals | 5/10 | `go test -tags acs ./acs/cycle2632 -run 'TestC2632_00[1-4]'` |
| gold-data | at least 30 queries from past plans, ids resolve, distractors exist | 6/10 | `go test -tags acs ./acs/cycle2632 -run 'TestC2632_00[56]'` |
| command | the verb prints both figures and refuses bad input | 6/10 | `go test -tags acs ./acs/cycle2632 -run 'TestC2632_00[78]'` |
| baseline | the recorded baseline reproduces from the default command | 7/10 | `go test -tags acs ./acs/cycle2632 -run 'TestC2632_(009|010)'` |
| provenance | queries come from plan text, never from the answer; labels are judged and justified (audit H1, cycle 2632 round 1) | 3/10 | `go test -tags acs ./acs/cycle2632 -run 'TestC2632_01[2-5]'` |

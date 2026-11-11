---
score_cap:
  - criterion: "The note reads the source at its link, cites the snapshot and the item, and explains the item lines as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2286_001|TestC2286_002' ./acs/cycle2286"
  - criterion: "The note gives a verdict with a reason for every core persona under ADR-0133, or a full proposal"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2286_003|TestC2286_004|TestC2286_005' ./acs/cycle2286"
  - criterion: "The persona files on disk match the verdict table of the note"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2286_006 ./acs/cycle2286"
  - criterion: "The note has no placeholder text, passes the sentence lint, and the research index entries link it"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2286_007|TestC2286_008' ./acs/cycle2286"
  - criterion: "The agy notes keep their bullets, stay sentence-lint clean and stay under the line ceiling"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2286_009 ./acs/cycle2286"
---

# Eval: Gemini prompting guide note for item 297686f7 (cycle 2286)

> Pins the document deliverable for the practice-watch item `practice-gemini-prompting-guide-297686f7-202611110139`. The page changed after the snapshot of 2026-11-10T08:41:14Z, and the stored excerpt shows no added line. The note reads the page at its link as untrusted data and states, for each core persona, whether it changes under ADR-0133. Source incident: the notes of cycles 2264 and 2272, which this note follows.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-not-instructions | Link, snapshot, item id, HTTP outcome, no-added-line explained | 6/10 | `TestC2286_001`, `TestC2286_002` |
| persona-verdict | Six verdict rows with reasons, Decision line, rejected option | 7/10 | `TestC2286_003` to `TestC2286_005` |
| claim-matches-files | Persona files change only where the note says so | 7/10 | `TestC2286_006` |
| lint-and-index | No placeholder, zero lint findings, index entry | 5/10 | `TestC2286_007`, `TestC2286_008` |
| notes-preserved | Agy notes keep bullets and ceiling | 5/10 | `TestC2286_009` |

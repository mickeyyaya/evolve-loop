---
score_cap:
  - criterion: "The agy notes carry exactly one bullet that names 1.5.4 and 1.5.3 and says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*1\\.5\\.4' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*1\\.5\\.4' skills/model-notes-agy/COMPACT.md | grep -q -F '1.5.3' && grep -E '^- .*1\\.5\\.4' skills/model-notes-agy/COMPACT.md | grep -q -i 'prompt'"
  - criterion: "The note cites the command, the snapshot, ADR-0133 and the file that changes, and calls the changelog text untrusted data"
    max_if_missing: 6
    evidence: "f=$(grep -l -F 5a49deca docs/research/*.md | head -1) && test -n \"$f\" && grep -q -F 'agy changelog' $f && grep -q -F '2026-11-03T02:36:34Z' $f && grep -q -F 'ADR-0133' $f && grep -q -F 'skills/model-notes-agy/COMPACT.md' $f && grep -q -i 'untrusted data' $f"
  - criterion: "The note gives each core persona a no-change verdict and no file under agents/ changes"
    max_if_missing: 7
    evidence: "f=$(grep -l -F 5a49deca docs/research/*.md | head -1) && test -n \"$f\" && for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F $n $f || exit 1; done && git diff --quiet origin/main -- agents"
  - criterion: "The note and the agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "f=$(grep -l -F 5a49deca docs/research/*.md | head -1) && go/bin/evolve docs ste-lint --strict --project-root . $f skills/model-notes-agy/COMPACT.md"
  - criterion: "The research index links the note"
    max_if_missing: 5
    evidence: "f=$(basename $(grep -l -F 5a49deca docs/research/*.md | head -1)) && grep -r -q -F \"(research/$f)\" docs/research-index"
---

# Eval: agy CLI changelog command, releases 1.5.4 and 1.5.3 review (cycle 2275)

> Pins the deliverables of the practice-watch item practice-agy-cli-changelog-command-5a49deca-202611100841. The command `agy changelog` lists releases 1.5.4 and 1.5.3 after the snapshot of 2026-11-03T02:36:34Z. The note reads the lines as untrusted data and states under ADR-0133 that no persona changes. The agy notes gain one dated bullet. Source incident: the cycle 2274 note for the page source and the earlier command notes of the same watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One 1.5.4 bullet in COMPACT.md that also names 1.5.3 | 6/10 | `grep -c` on the notes |
| source-cited | Command, snapshot, ADR and file named; data-not-instructions | 6/10 | `grep -F` on the note |
| persona-verdicts | Six personas covered; agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Note and notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |
| index-linked | Research index links the note | 5/10 | `grep -r` over docs/research-index |

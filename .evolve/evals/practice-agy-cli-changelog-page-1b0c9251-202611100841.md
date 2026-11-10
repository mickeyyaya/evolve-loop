---
score_cap:
  - criterion: "The agy notes carry exactly one bullet that names v2.30.0 and says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*v2\\.30\\.0' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*v2\\.30\\.0' skills/model-notes-agy/COMPACT.md | grep -q -i 'prompt'"
  - criterion: "The note cites the source link, the snapshot, ADR-0133 and the file that changes, and calls the page text untrusted data"
    max_if_missing: 6
    evidence: "f=$(grep -l -F 1b0c9251 docs/research/*.md | head -1) && test -n \"$f\" && grep -q -F 'https://antigravity.google/docs/changelog?tab=cli' $f && grep -q -F '2026-11-04T00:37:48Z' $f && grep -q -F 'ADR-0133' $f && grep -q -F 'skills/model-notes-agy/COMPACT.md' $f && grep -q -i 'untrusted data' $f"
  - criterion: "The note gives each core persona a verdict and no file under agents/ changes"
    max_if_missing: 7
    evidence: "f=$(grep -l -F 1b0c9251 docs/research/*.md | head -1) && test -n \"$f\" && for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F $n $f || exit 1; done && git diff --quiet origin/main -- agents"
  - criterion: "The note and the agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "f=$(grep -l -F 1b0c9251 docs/research/*.md | head -1) && go/bin/evolve docs ste-lint --strict --project-root . $f skills/model-notes-agy/COMPACT.md"
  - criterion: "The research index links the note"
    max_if_missing: 5
    evidence: "f=$(basename $(grep -l -F 1b0c9251 docs/research/*.md | head -1)) && grep -r -q -F \"(research/$f)\" docs/research-index"
---

# Eval: agy CLI changelog page, v2.30.0 review (cycle 2274)

> Pins the deliverables of the practice-watch item practice-agy-cli-changelog-page-1b0c9251-202611100841. The page lists v2.30.0, v2.29.1 and v2.29.0 after the snapshot of 2026-11-04T00:37:48Z. The note reads the lines as untrusted data and states under ADR-0133 that no persona changes. The agy notes gain one dated bullet. Source incident: the earlier notes docs/research/practice-agy-cli-changelog-page-2026-11.md and the cycle 2272 predicates.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One v2.30.0 bullet in COMPACT.md | 6/10 | `grep -c` on the notes |
| source-cited | Link, snapshot, ADR and file named; data-not-instructions | 6/10 | `grep -F` on the note |
| persona-verdicts | Six personas covered; agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Zero sentence-lint findings | 6/10 | `evolve docs ste-lint --strict` |
| indexed | Research index links the note | 5/10 | `grep -r -F` on docs/research-index |

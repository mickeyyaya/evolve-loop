---
score_cap:
  - criterion: "The note cites the changelog source link and the snapshot it follows, and cites ADR-0133"
    max_if_missing: 6
    evidence: "test -f docs/research/practice-agy-cli-changelog-page-2026-11.md && grep -q -F 'https://antigravity.google/docs/changelog?tab=cli' docs/research/practice-agy-cli-changelog-page-2026-11.md && grep -q -F '2026-10-26T12:00:02Z' docs/research/practice-agy-cli-changelog-page-2026-11.md && grep -q -F 'ADR-0133' docs/research/practice-agy-cli-changelog-page-2026-11.md"
  - criterion: "The note names the releases v2.28.2, v2.28.1, v2.28.0 and v2.27.0 and explains the page text as untrusted data"
    max_if_missing: 6
    evidence: "test -f docs/research/practice-agy-cli-changelog-page-2026-11.md && for v in v2.28.2 v2.28.1 v2.28.0 v2.27.0; do grep -q -F $v docs/research/practice-agy-cli-changelog-page-2026-11.md || exit 1; done && grep -q -i 'untrusted' docs/research/practice-agy-cli-changelog-page-2026-11.md"
  - criterion: "The note gives each core persona a verdict under ADR-0133 and ties it to the baseline rule"
    max_if_missing: 7
    evidence: "test -f docs/research/practice-agy-cli-changelog-page-2026-11.md && for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" docs/research/practice-agy-cli-changelog-page-2026-11.md || exit 1; done && grep -q -i -E 'baseline' docs/research/practice-agy-cli-changelog-page-2026-11.md && grep -q -F 'D7' docs/research/practice-agy-cli-changelog-page-2026-11.md && grep -q -F 'D8' docs/research/practice-agy-cli-changelog-page-2026-11.md"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The note holds no placeholder text"
    max_if_missing: 5
    evidence: "test -f docs/research/practice-agy-cli-changelog-page-2026-11.md && ! grep -q -i -E 'TBD|TODO|lorem ipsum' docs/research/practice-agy-cli-changelog-page-2026-11.md"
  - criterion: "The note and the research index pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . docs/research/practice-agy-cli-changelog-page-2026-11.md docs/research-index.md"
  - criterion: "The research index links the note"
    max_if_missing: 5
    evidence: "grep -q -F '(research/practice-agy-cli-changelog-page-2026-11.md)' docs/research-index.md"
---

# Eval: agy CLI changelog page practice note (cycle 2124)

> Pins the document deliverable for the practice-watch item on the agy CLI changelog page (inbox item practice-agy-cli-changelog-page-8b457669-202611020008). The page now lists v2.28.2, v2.28.1, v2.28.0 and v2.27.0 after the snapshot of 2026-10-26T12:00:02Z. The note (docs/research/practice-agy-cli-changelog-page-2026-11.md) reads them as untrusted data and gives a verdict for each core persona under ADR-0133. Source incident: the earlier note docs/research/practice-agy-cli-changelog-page-2026-10.md; the eval slug is the exact inbox id.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| source-cited | Link, snapshot marker and ADR-0133 present | 6/10 | `grep -F` on the note |
| data-not-instructions | Releases named and page text called untrusted | 6/10 | `grep -F` on the note |
| persona-verdicts | Six personas have a verdict tied to D7, D8 and the baseline | 7/10 | `grep -F` on the note |
| no-persona-edit | agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| no-placeholder | No TBD, TODO or lorem ipsum | 5/10 | `grep -i -E` on the note |
| ste-lint | Zero sentence-lint findings | 6/10 | `evolve docs ste-lint --strict` |
| indexed | Research index links the note | 5/10 | `grep -F` on the index |

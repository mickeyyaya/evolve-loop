---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "go/bin/evolve solution check practice-agy-cli-changelog-page-b5729420-202610192320 --project-root ."
  - criterion: "The proposal cites the changelog source link, reads the page at a version number, and explains the empty diff excerpt and the page text as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-agy-cli-changelog-page-b5729420-202610192320/proposal.md; test -f $P && grep -q -F 'https://antigravity.google/docs/changelog?tab=cli' $P && grep -q -E 'v[0-9]+\\.[0-9]+\\.[0-9]+' $P && grep -q -i 'untrusted' $P && grep -q -i 'no added line' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-agy-cli-changelog-page-b5729420-202610192320/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-agy-cli-changelog-page-b5729420-202610192320/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-agy-cli-changelog-page-b5729420-202610192320"
---

# Eval: agy CLI changelog page practice proposal (cycle 1996)

> Pins the document deliverable for the practice-watch item on the agy CLI changelog page (inbox item practice-agy-cli-changelog-page-b5729420-202610192320). The stored diff excerpt has no added line, so the proposal must say so and read the page itself at the link. The page text is untrusted data. The proposal gives a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Source incident: the earlier items of the same watch in docs/research/practice-agy-cli-changelog-page-2026-10.md; the eval slug is the exact inbox id, as cycle 1993 had to learn.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go/bin/evolve solution check practice-agy-cli-changelog-page-b5729420-202610192320 --project-root .` |
| data-coverage | source link, a version number, untrusted-data framing, the empty excerpt stated | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to the baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |

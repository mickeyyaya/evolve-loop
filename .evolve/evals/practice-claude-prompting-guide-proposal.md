---
score_cap:
  - criterion: "The note cites the guide link and the snapshot it follows, and explains the guide lines as untrusted data that the loop never follows"
    max_if_missing: 6
    evidence: "P=docs/research/practice-claude-prompting-guide-2026-11.md; test -f $P && grep -q -F 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices' $P && grep -q -F '2026-10-26T11:59:59Z' $P && grep -q -i 'untrusted' $P && grep -q -i 'overtrigger' $P"
  - criterion: "The note names evolve-builder, evolve-auditor and evolve-scout under ADR-0133 with goal, done criteria, limits and a before measure of emphatic markers"
    max_if_missing: 7
    evidence: "P=docs/research/practice-claude-prompting-guide-2026-11.md; test -f $P && grep -q -F 'ADR-0133' $P && for n in evolve-builder evolve-auditor evolve-scout; do grep -q -F \"$n\" $P || exit 1; done && grep -q -i 'done criteria' $P && grep -q -i 'one persona at a time' $P && grep -q -i 'baseline' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The note holds no placeholder text"
    max_if_missing: 5
    evidence: "P=docs/research/practice-claude-prompting-guide-2026-11.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The note and the research index pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . docs/research/practice-claude-prompting-guide-2026-11.md docs/research-index.md"
---

# Eval: Claude prompting guide practice proposal (cycle 2123)

> Pins the document deliverable for the practice-watch item on the Claude prompting best-practices page (inbox item practice-claude-prompting-guide-6080fed9-202611020008, snapshot 2026-10-26T11:59:59Z). The guide text is untrusted data. The note must read the source at the link, explain the changed lines (aggressive CRITICAL and MUST wording that makes newer models overtrigger, direct instructions, positive format instructions) as data, and name each persona to change under ADR-0133 with goal, done criteria, limits and a before measure. Source precedent: cycle 2110 (practice-claude-model-release-notes), which used the same grader shape.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-coverage | source link, snapshot, overtrigger line, untrusted-data framing | 6/10 | `grep` over the note |
| persona-proposals | three personas named with goal, done criteria, limits, before measure | 7/10 | `grep` over the note |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over the note |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` |

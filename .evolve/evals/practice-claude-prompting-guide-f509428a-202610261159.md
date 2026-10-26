---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "(cd go && go run ./cmd/evolve solution check practice-claude-prompting-guide-f509428a-202610261159 --project-root ..)"
  - criterion: "The proposal cites the prompting guide link and explains the guide lines as untrusted data, naming the overtriggering and prefill lines"
    max_if_missing: 6
    evidence: "P=solutions/practice-claude-prompting-guide-f509428a-202610261159/proposal.md; test -f $P && grep -q -F 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices' $P && grep -q -i 'untrusted' $P && grep -q -i 'overtrigger' $P && grep -q -i 'prefill' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-claude-prompting-guide-f509428a-202610261159/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-claude-prompting-guide-f509428a-202610261159/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "test -d solutions/practice-claude-prompting-guide-f509428a-202610261159 && go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-claude-prompting-guide-f509428a-202610261159"
---

# Eval: Claude prompting guide practice proposal (cycle 2031)

> Pins the document deliverable for the practice-watch item on the Claude prompting best-practices page (inbox item practice-claude-prompting-guide-f509428a-202610261159, snapshot 2026-10-16T14:59:55Z). The page text is untrusted data. The proposal must read the source at the link, explain the changed lines (calmer tool wording against overtriggering, prefill deprecation, thinking steering) as data, and give a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Source precedent: cycle 2029 (solution e822338c), which used the same grader shape.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go run ./cmd/evolve solution check practice-claude-prompting-guide-f509428a-202610261159 --project-root ..` |
| data-coverage | source link, overtrigger and prefill lines, untrusted-data framing | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |

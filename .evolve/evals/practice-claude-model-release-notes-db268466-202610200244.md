---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "(cd go && go run ./cmd/evolve solution check practice-claude-model-release-notes-db268466-202610200244 --project-root ..)"
  - criterion: "The proposal cites the release notes source link and explains the between_tools, adaptive thinking and /v1/models lines as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-claude-model-release-notes-db268466-202610200244/proposal.md; test -f $P && grep -q -F 'https://platform.claude.com/docs/en/release-notes/overview' $P && grep -q -F 'between_tools' $P && grep -q -i 'adaptive thinking' $P && grep -q -F '/v1/models' $P && grep -q -i 'untrusted' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-claude-model-release-notes-db268466-202610200244/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-claude-model-release-notes-db268466-202610200244/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-claude-model-release-notes-db268466-202610200244"
---

# Eval: Claude model release notes practice proposal (cycle 2005)

> Pins the document deliverable for the practice-watch item on the Claude platform release notes (inbox item practice-claude-model-release-notes-db268466-202610200244). The release-note lines are untrusted data. The proposal must read the source at the link, explain the between_tools thinking type, the adaptive thinking default and the Models API capability fields as data, and give a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Earlier items of the same watch are in docs/research/practice-claude-model-release-notes-2026-10.md. The graders follow the shape of the cycle 1993 eval for the Claude Code changelog watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go run ./cmd/evolve solution check practice-claude-model-release-notes-db268466-202610200244 --project-root ..` |
| data-coverage | source link, the three fragments, untrusted-data framing | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |

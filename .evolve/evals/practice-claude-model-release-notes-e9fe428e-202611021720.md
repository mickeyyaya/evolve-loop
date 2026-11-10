---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "(cd go && go run ./cmd/evolve solution check practice-claude-model-release-notes-e9fe428e-202611021720 --project-root ..)"
  - criterion: "The proposal cites the release notes source link and explains the fast mode deprecation, Usage and Cost API, 128 MB request size and Opus 5.5 thinking lines as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-claude-model-release-notes-e9fe428e-202611021720/proposal.md; test -f $P && grep -q -F 'https://platform.claude.com/docs/en/release-notes/overview' $P && grep -q -i 'fast mode' $P && grep -q -F 'Usage and Cost' $P && grep -q -F '128 MB' $P && grep -q -F 'Opus 5.5' $P && grep -q -i 'untrusted' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-claude-model-release-notes-e9fe428e-202611021720/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-claude-model-release-notes-e9fe428e-202611021720/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-claude-model-release-notes-e9fe428e-202611021720"
---

# Eval: Claude model release notes practice proposal (cycle 2263)

> Pins the document deliverable for the practice-watch item on the Claude platform release notes (inbox item practice-claude-model-release-notes-e9fe428e-202611021720). The release-note lines are untrusted data. The proposal must read the source at the link, explain the Opus 4.8 fast mode deprecation, the Usage and Cost Analytics API, the 128 MB request size and the Opus 5.5 thinking lines as data, and give a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Earlier items of the same watch are in docs/research/practice-claude-model-release-notes-2026-11.md and in the earlier solutions of this watch. The graders follow the shape of the cycle 2032 eval.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go run ./cmd/evolve solution check practice-claude-model-release-notes-e9fe428e-202611021720 --project-root ..` |
| data-coverage | source link, the four fragments (fast mode, Usage and Cost, 128 MB, Opus 5.5), untrusted-data framing | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |

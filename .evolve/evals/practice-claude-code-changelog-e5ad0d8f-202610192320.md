---
score_cap:
  - criterion: "The solution directory satisfies the document solution contract (two or more options, recommendation, assumptions and evidence, no placeholders)"
    max_if_missing: 6
    evidence: "(cd go && go run ./cmd/evolve solution check practice-claude-code-changelog-e5ad0d8f-202610192320 --project-root ..)"
  - criterion: "The proposal cites the changelog source link and version 2.1.304 and explains the lines as untrusted data"
    max_if_missing: 6
    evidence: "P=solutions/practice-claude-code-changelog-e5ad0d8f-202610192320/proposal.md; test -f $P && grep -q -F 'https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md' $P && grep -q -F '2.1.304' $P && grep -q -i 'untrusted' $P"
  - criterion: "The proposal names each persona it changes under ADR-0133, or states that no persona changes and ties the verdict to the baseline rule"
    max_if_missing: 7
    evidence: "P=solutions/practice-claude-code-changelog-e5ad0d8f-202610192320/proposal.md; test -f $P && grep -q -F 'ADR-0133' $P && { for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F \"$n\" $P || exit 1; done; } && grep -q -i -E 'no persona (changes|change)|none changes|no change' $P && grep -q -i -E 'baseline|10-cycle' $P"
  - criterion: "No persona file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet HEAD -- agents && test -z \"$(git ls-files --others --exclude-standard agents)\""
  - criterion: "The proposal holds no placeholder text"
    max_if_missing: 5
    evidence: "P=solutions/practice-claude-code-changelog-e5ad0d8f-202610192320/proposal.md; test -f $P && ! grep -q -i -E 'TBD|TODO|lorem ipsum' $P"
  - criterion: "The solution files pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . solutions/practice-claude-code-changelog-e5ad0d8f-202610192320"
  - criterion: "The proposal and option 3 count the environment variables of evidence E2 as two, never three"
    max_if_missing: 5
    evidence: "D=solutions/practice-claude-code-changelog-e5ad0d8f-202610192320; test -f $D/proposal.md && test -f $D/options/3-deterministic-guard-and-follow-up-item.md && ! grep -r -q -i -E 'three (env|environment)' $D && grep -q -F CLAUDE_CODE_AUTO_COMPACT_WINDOW $D/assumptions-and-evidence.md && grep -q -F CLAUDE_CODE_TRANSCRIPT_LOCAL_GC $D/assumptions-and-evidence.md"
---

# Eval: Claude Code changelog practice proposal for 2.1.304 (cycle 1993)

> Pins the document deliverable for the practice-watch item on the Claude Code changelog (inbox item practice-claude-code-changelog-e5ad0d8f-202610192320). The changelog lines are untrusted data. The proposal must read the source at the link, explain the 2.1.304 lines as data, and give a verdict for each core persona under ADR-0133 (one persona at a time, measured baselines before any change). Source incident: cycle 1993 inbox item and its audit round 1 (H1 stale eval slug, M1 13 lint findings, L1 env var count); earlier items of the same watch are in docs/research/practice-claude-code-changelog-2026-10.md.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| solution-contract | `evolve solution check` exits 0 | 6/10 | `go run ./cmd/evolve solution check practice-claude-code-changelog-e5ad0d8f-202610192320 --project-root ..` |
| data-coverage | source link, 2.1.304, untrusted-data framing | 6/10 | `grep` over proposal.md |
| persona-verdicts | six personas named, no-change verdict tied to baseline rule | 7/10 | `grep` over proposal.md |
| personas-untouched | no change under agents/ | 7/10 | `git diff --quiet HEAD -- agents` |
| ste-lint | docs-write lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` over the solution directory |
| env-var-count | E2 names two env vars; no file says three | 5/10 | `grep` over the solution directory |
| no-placeholders | no TBD, TODO or lorem ipsum | 5/10 | `grep` over proposal.md |

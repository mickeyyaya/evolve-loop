---
score_cap:
  - criterion: "One research note names the item marker 18278f9c, the source link and the words untrusted data"
    max_if_missing: 6
    evidence: "test $(grep -l '18278f9c' docs/research/*.md | wc -l) -eq 1 && grep -l '18278f9c' docs/research/*.md | xargs grep -q 'https://platform.claude.com/docs/en/release-notes/overview' && grep -l '18278f9c' docs/research/*.md | xargs grep -qi 'untrusted data'"
  - criterion: "The note cites ADR-0133 and gives each core persona a no-change verdict"
    max_if_missing: 7
    evidence: "grep -l '18278f9c' docs/research/*.md | xargs grep -q 'ADR-0133' && test $(grep -l '18278f9c' docs/research/*.md | xargs grep -c -E '^\\| .*evolve-(scout|triage|tdd-engineer|builder|auditor|retrospective).*no change') -ge 6"
  - criterion: "No persona under agents/ and not the Claude model notes change"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents skills/model-notes-claude/COMPACT.md"
  - criterion: "The note passes the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . $(grep -l '18278f9c' docs/research/*.md)"
---

# Eval: Claude model release notes, Managed Agents webhooks and filters (cycle 2371)

> Pins the deliverable of the practice-watch item practice-claude-model-release-notes-18278f9c-202611131947. The release notes list Managed Agents webhooks, session and event filters and a cut-off Dreams line. The pipeline runs each seat through evolve agent run and does not use Managed Agents, so the note states under ADR-0133 that no persona changes. Source incident: the cycle 2370 note of the agy practice watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| note-as-data | One note names the item, the link and untrusted data | 6/10 | `grep -l 18278f9c` |
| persona-verdicts | ADR-0133 cited, six no-change rows | 7/10 | `grep -c` on the table rows |
| no-persona-diff | agents/ and the Claude notes unchanged | 7/10 | `git diff --quiet origin/main` |
| ste-lint | Note passes the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

```bash
test $(grep -l '18278f9c' docs/research/*.md | wc -l) -eq 1
```

```bash
git diff --quiet origin/main -- agents skills/model-notes-claude/COMPACT.md
```

```bash
go/bin/evolve docs ste-lint --strict --project-root . $(grep -l '18278f9c' docs/research/*.md)
```

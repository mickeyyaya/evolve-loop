---
score_cap:
  - criterion: "One research note names the item marker 5cd04824, the source link and the words untrusted data"
    max_if_missing: 6
    evidence: "test $(grep -l '5cd04824' docs/research/*.md | wc -l) -eq 1 && grep -l '5cd04824' docs/research/*.md | xargs grep -q 'https://platform.claude.com/docs/en/release-notes/overview' && grep -l '5cd04824' docs/research/*.md | xargs grep -qi 'untrusted data'"
  - criterion: "The note cites ADR-0133 and gives each core persona a no-change verdict"
    max_if_missing: 7
    evidence: "grep -l '5cd04824' docs/research/*.md | xargs grep -q 'ADR-0133' && test $(grep -l '5cd04824' docs/research/*.md | xargs grep -c -E '^\\| .*evolve-(scout|triage|tdd-engineer|builder|auditor|retrospective).*[Nn]o change') -ge 6"
  - criterion: "No persona and no Claude model notes file change"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents skills/model-notes-claude"
  - criterion: "The note passes the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/evolve docs ste-lint --strict --project-root . $(grep -l '5cd04824' docs/research/*.md)"
---

# Eval: Claude release notes, linked billing (cycle 2485)

> Pins the research-note deliverable of the practice-watch item practice-claude-model-release-notes-5cd04824-202611171847. The release notes add linked billing (a beta header and a billing_policy request field) and a cut-off line about a managed Claude API launch. Both are API billing and hosting facts. No persona sets billing_policy, so the note states under ADR-0133 that no persona and no Claude model notes file change. Source incident: the cycle 2446 note of the same practice watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| note-as-data | One note names the item, the link and untrusted data | 6/10 | `grep -l 5cd04824` |
| persona-verdicts | ADR-0133 cited, six no-change rows | 7/10 | `grep -c` on the table rows |
| no-persona-diff | agents/ and the Claude notes unchanged | 7/10 | `git diff --quiet origin/main -- agents skills/model-notes-claude` |
| ste-lint | Note passes the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

```bash
test $(grep -l '5cd04824' docs/research/*.md | wc -l) -eq 1
```

```bash
git diff --quiet origin/main -- agents skills/model-notes-claude
```

```bash
go/evolve docs ste-lint --strict --project-root . $(grep -l '5cd04824' docs/research/*.md)
```

---
score_cap:
  - criterion: "One research note names the item marker 7ad7926a, the source link and the words untrusted data"
    max_if_missing: 6
    evidence: "test $(grep -l '7ad7926a' docs/research/*.md | wc -l) -eq 1 && grep -l '7ad7926a' docs/research/*.md | xargs grep -q 'https://platform.claude.com/docs/en/release-notes/overview' && grep -l '7ad7926a' docs/research/*.md | xargs grep -qi 'untrusted data'"
  - criterion: "The note cites ADR-0133 and gives each core persona a no-change verdict"
    max_if_missing: 7
    evidence: "grep -l '7ad7926a' docs/research/*.md | xargs grep -q 'ADR-0133' && test $(grep -l '7ad7926a' docs/research/*.md | xargs grep -c -E '^\\| .*evolve-(scout|triage|tdd-engineer|builder|auditor|retrospective).*[Nn]o change') -ge 6"
  - criterion: "No persona under agents/ changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
  - criterion: "The note passes the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/evolve docs ste-lint --strict --project-root . $(grep -l '7ad7926a' docs/research/*.md)"
---

# Eval: Claude release notes, text watermark and sampling parameters (cycle 2446)

> Pins the research-note deliverable of the practice-watch item practice-claude-model-release-notes-7ad7926a-202611162241. The release notes say that text from Claude Opus 4.6, Opus 4.5, Sonnet 4.5 and Haiku 4.5 carries a watermark, and that a non-default temperature, top_p or top_k can remove it. No persona sets a sampling value, so the note states under ADR-0133 that no persona changes. Source incident: the cycle 2412 note of the same practice watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| note-as-data | One note names the item, the link and untrusted data | 6/10 | `grep -l 7ad7926a` |
| persona-verdicts | ADR-0133 cited, six no-change rows | 7/10 | `grep -c` on the table rows |
| no-persona-diff | agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Note passes the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

```bash
test $(grep -l '7ad7926a' docs/research/*.md | wc -l) -eq 1
```

```bash
git diff --quiet origin/main -- agents
```

```bash
go/evolve docs ste-lint --strict --project-root . $(grep -l '7ad7926a' docs/research/*.md)
```

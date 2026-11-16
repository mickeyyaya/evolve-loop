---
score_cap:
  - criterion: "The Claude model notes hold one watermark bullet that names the sampling parameters"
    max_if_missing: 6
    evidence: "test $(grep -ci 'watermark' skills/model-notes-claude/COMPACT.md) -eq 1 && grep -i 'watermark' skills/model-notes-claude/COMPACT.md | grep -q 'temperature' && grep -i 'watermark' skills/model-notes-claude/COMPACT.md | grep -q 'top_p' && grep -i 'watermark' skills/model-notes-claude/COMPACT.md | grep -q 'top_k'"
  - criterion: "The Claude model notes gain exactly one line and lose none"
    max_if_missing: 7
    evidence: "test \"$(git diff --numstat origin/main -- skills/model-notes-claude/COMPACT.md | cut -f1,2)\" = \"$(printf '1\\t0')\""
  - criterion: "The Claude model notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/evolve docs ste-lint --strict --project-root . skills/model-notes-claude/COMPACT.md"
---

# Eval: Claude model notes, watermark bullet (cycle 2446)

> Pins the one-bullet change in skills/model-notes-claude/COMPACT.md for the practice-watch item practice-claude-model-release-notes-7ad7926a-202611162241. The bullet warns that a non-default temperature, top_p or top_k on Claude Opus 4.6, Opus 4.5, Sonnet 4.5 or Haiku 4.5 can remove the text watermark. Source incident: the cycle 2446 scout selection practice-claude-watermark-note.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-bullet | Exactly one watermark line that names the three sampling parameters | 6/10 | `grep -ci watermark` |
| diff-shape | One added line, none removed | 7/10 | `git diff --numstat` |
| ste-lint | The notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

```bash
test $(grep -ci 'watermark' skills/model-notes-claude/COMPACT.md) -eq 1
```

```bash
go/evolve docs ste-lint --strict --project-root . skills/model-notes-claude/COMPACT.md
```

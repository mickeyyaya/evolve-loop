---
score_cap:
  - criterion: "The agy notes carry exactly one bullet that names 1.5.8 and says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*1\\.5\\.8' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*1\\.5\\.8' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt changes'"
  - criterion: "The 1.5.8 bullet tells the reader to read each changelog line as data"
    max_if_missing: 6
    evidence: "grep -E '^- .*1\\.5\\.8' skills/model-notes-agy/COMPACT.md | grep -q -i 'as data'"
  - criterion: "No persona under agents/ changes, which is the ADR-0133 verdict that none changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
  - criterion: "The agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md"
---

# Eval: agy CLI changelog command, release 1.5.8 review (cycle 2369)

> Pins the deliverable of the practice-watch item practice-agy-cli-changelog-command-99ca21a0-202611131947. The command `agy changelog` lists release 1.5.8, which changes the CLI product (models, UI, ACP context usage, admin settings, sandbox and permission fixes). The agy notes gain one dated bullet that reads the lines as data and states under ADR-0133 that no prompt or persona changes. Source incident: the cycle 2275 and cycle 2326 notes of the same watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One 1.5.8 bullet in COMPACT.md that says no prompt changes | 6/10 | `grep -c` on the notes |
| data-not-instructions | The bullet says to read each line as data | 6/10 | `grep -i 'as data'` |
| persona-verdicts | agents/ unchanged (none changes under ADR-0133) | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

```bash
test $(grep -c -E '^- .*1\.5\.8' skills/model-notes-agy/COMPACT.md) -eq 1
```

```bash
grep -E '^- .*1\.5\.8' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt changes'
```

```bash
grep -E '^- .*1\.5\.8' skills/model-notes-agy/COMPACT.md | grep -q -i 'as data'
```

```bash
git diff --quiet origin/main -- agents
```

```bash
go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md
```

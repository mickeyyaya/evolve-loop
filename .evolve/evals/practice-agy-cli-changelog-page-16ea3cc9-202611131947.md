---
score_cap:
  - criterion: "The agy notes carry exactly one Cycle 2370 bullet that says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*Cycle 2370' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2370' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'"
  - criterion: "The Cycle 2370 bullet keeps /run-workflow unused by the rule that no LLM starts or directs another"
    max_if_missing: 6
    evidence: "grep -E '^- .*Cycle 2370' skills/model-notes-agy/COMPACT.md | grep -q -i -E '/run-workflow.*(no LLM|directs another)'"
  - criterion: "One research note names the item, the source link and the snapshot, and says the lines are untrusted data"
    max_if_missing: 7
    evidence: "test $(grep -l '16ea3cc9' docs/research/*.md | wc -l) -eq 1 && grep -l '16ea3cc9' docs/research/*.md | xargs grep -q -i 'untrusted data'"
  - criterion: "No persona under agents/ changes, which is the ADR-0133 verdict that none changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
  - criterion: "The agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md"
---

# Eval: agy CLI changelog page, /run-workflow and rewind lines review (cycle 2370)

> Pins the deliverable of the practice-watch item practice-agy-cli-changelog-page-16ea3cc9-202611131947. The page `https://antigravity.google/docs/changelog?tab=cli` changed after the snapshot of 2026-11-13T00:01:25Z. The lines name /run-workflow, release 1.5.8 and a rewind topic. The agy notes gain one dated bullet. A research note explains each line as untrusted data and gives each core persona a verdict under ADR-0133. Source incident: the cycle 2326 and cycle 2369 notes of the same watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One Cycle 2370 bullet in COMPACT.md that says no prompt changes | 6/10 | `grep -c` on the notes |
| workflow-barred | The bullet ties /run-workflow to the no-LLM-directs-another rule | 6/10 | `grep -i` on the bullet |
| research-note | One note names the item and reads the lines as untrusted data | 7/10 | `grep -l` on docs/research |
| persona-verdicts | agents/ unchanged (none changes under ADR-0133) | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

```bash
test $(grep -c -E '^- .*Cycle 2370' skills/model-notes-agy/COMPACT.md) -eq 1
```

```bash
grep -E '^- .*Cycle 2370' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'
```

```bash
test $(grep -l '16ea3cc9' docs/research/*.md | wc -l) -eq 1
```

```bash
git diff --quiet origin/main -- agents
```

```bash
go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md
```

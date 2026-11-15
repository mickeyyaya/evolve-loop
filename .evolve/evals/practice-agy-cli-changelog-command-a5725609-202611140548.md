---
score_cap:
  - criterion: "The agy notes carry exactly one Cycle 2395 bullet that says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'"
  - criterion: "The Cycle 2395 bullet keeps run_workflow unused by the rule that no LLM starts or directs another"
    max_if_missing: 6
    evidence: "grep -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md | grep -q -i -E 'run_workflow.*(no LLM|directs another)'"
  - criterion: "One research note names the item, the agy changelog source and the snapshot, and says the lines are untrusted data"
    max_if_missing: 7
    evidence: "test $(grep -l 'a5725609' docs/research/*.md | wc -l) -eq 1 && grep -l 'a5725609' docs/research/*.md | xargs grep -q 'agy changelog' && grep -l 'a5725609' docs/research/*.md | xargs grep -q -i 'untrusted data'"
  - criterion: "No persona under agents/ changes, which is the ADR-0133 verdict that none changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
  - criterion: "The agy notes and new research note pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md"
  - criterion: "The new research note is linked from a docs/research-index entry file and from the generated index"
    max_if_missing: 5
    evidence: "grep -l 'practice-agy-cli-changelog-command-1-5-9-2026-11.md' docs/research-index/*.md docs/research-index.md | wc -l | grep -q -v '^0$'"
---

# Eval: agy CLI changelog command, v1.5.9 review (cycle 2395)

> Pins the deliverable of the practice-watch item `practice-agy-cli-changelog-command-a5725609-202611140548`. The command `agy changelog` changed after the snapshot of `2026-11-13T19:47:32Z`, adding CLI `1.5.9` (`run_workflow` tool, `pluginControls` admin settings, Auto mode sandbox review improvements, `/resume` improvements, `/browser` `.mp4` recordings, and MCP/ACP/BOM/TUI/Windows/macOS fixes). The agy notes gain one dated bullet. A research note explains the changelog lines as untrusted data and gives the ADR-0133 verdict that no persona changes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One Cycle 2395 bullet in COMPACT.md that says no prompt changes | 6/10 | `grep -c` on the notes |
| workflow-barred | The bullet ties `run_workflow` to the no-LLM-directs-another rule | 6/10 | `grep -i` on the bullet |
| research-note | One note names the item and reads the lines as untrusted data | 7/10 | `grep -l` on docs/research |
| persona-verdicts | agents/ unchanged (none changes under ADR-0133) | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Notes and research doc pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

## Graders

1. `[code]` Exactly one Cycle 2395 bullet exists in `skills/model-notes-agy/COMPACT.md`:
```bash
test $(grep -c -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md) -eq 1
```

2. `[code]` The Cycle 2395 bullet states no prompt changes and keeps `run_workflow` unused under the no-LLM-directs-another rule:
```bash
grep -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change' && grep -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md | grep -q -i -E 'run_workflow.*(no LLM|directs another)'
```

3. `[code]` Negative case — the Cycle 2395 bullet does not instruct agents to enable or invoke `run_workflow` (fails if `enable run_workflow` or `use run_workflow` appears):
```bash
! grep -E '^- .*Cycle 2395' skills/model-notes-agy/COMPACT.md | grep -q -i -E '(enable|use) run_workflow'
```

4. `[code]` Exactly one research note under `docs/research/*.md` references `a5725609`, names `agy changelog`, and marks the embedded lines as untrusted data:
```bash
test $(grep -l 'a5725609' docs/research/*.md | wc -l) -eq 1 && grep -l 'a5725609' docs/research/*.md | xargs grep -q 'agy changelog' && grep -l 'a5725609' docs/research/*.md | xargs grep -q -i 'untrusted data'
```

5. `[code]` Edge case — no persona under `agents/` is modified (ADR-0133 verdict: none changes):
```bash
git diff --quiet origin/main -- agents
```

6. `[code]` `skills/model-notes-agy/COMPACT.md` and the new `docs/research/*.md` note pass strict STE lint:
```bash
go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md && grep -l 'a5725609' docs/research/*.md | xargs go/bin/evolve docs ste-lint --strict --project-root .
```

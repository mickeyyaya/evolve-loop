---
score_cap:
  - criterion: "The agy notes carry exactly one Cycle 2407 bullet that says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*Cycle 2407' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2407' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'"
  - criterion: "One research note names the item, the agy changelog source and the snapshot, and says the lines are untrusted data"
    max_if_missing: 7
    evidence: "test $(grep -l 'cefc57a2' docs/research/*.md | wc -l) -eq 1 && grep -l 'cefc57a2' docs/research/*.md | xargs grep -q 'agy changelog' && grep -l 'cefc57a2' docs/research/*.md | xargs grep -q '2026-11-14T05:48:36Z' && grep -l 'cefc57a2' docs/research/*.md | xargs grep -q -i 'untrusted data'"
  - criterion: "The note gives each core persona a no-change verdict under ADR-0133 and no file under agents/ changes"
    max_if_missing: 7
    evidence: "f=$(grep -l 'cefc57a2' docs/research/*.md | head -1) && test -n \"$f\" && for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F $n $f || exit 1; done && git diff --quiet origin/main -- agents"
  - criterion: "The agy notes and new research note pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md && grep -l 'cefc57a2' docs/research/*.md | xargs go/bin/evolve docs ste-lint --strict --project-root ."
  - criterion: "The new research note is linked from docs/research-index"
    max_if_missing: 5
    evidence: "f=$(basename $(grep -l 'cefc57a2' docs/research/*.md | head -1)) && grep -r -q -F \"research/$f\" docs/research-index"
---

# Eval: agy CLI changelog command, snapshot 2026-11-14T05:48:36Z review (cycle 2407)

> Pins the deliverable of the practice-watch item `practice-agy-cli-changelog-command-cefc57a2-202611151110`. The command `agy changelog` changed after the snapshot of `2026-11-14T05:48:36Z`, surfacing lines from CLI `1.5.9` (`/browser` `.mp4` recordings) and `1.4.4` (`/diff` whole-file toggle and hunk folding, MCP `root_conversation_id`, Auto mode sandbox review). The agy notes gain one dated Cycle 2407 bullet. A research note explains the changelog lines as untrusted data and gives the ADR-0133 verdict that no persona changes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One Cycle 2407 bullet in COMPACT.md that says no prompt changes | 6/10 | `grep -c` on the notes |
| research-note | One note names the item, snapshot, and untrusted data | 7/10 | `grep -l` on docs/research |
| persona-verdicts | Six personas covered; agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Notes and research doc pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |
| index-linked | Research index links the note | 5/10 | `grep -r` over docs/research-index |

## Graders

1. `[code]` Exactly one Cycle 2407 bullet exists in `skills/model-notes-agy/COMPACT.md` and states no prompt changes:
```bash
test $(grep -c -E '^- .*Cycle 2407' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2407' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'
```

2. `[code]` Negative case — the Cycle 2407 bullet does not instruct agents to enable MCP or follow changelog text as instructions:
```bash
! grep -E '^- .*Cycle 2407' skills/model-notes-agy/COMPACT.md | grep -q -i -E '(enable mcp|use mcp|follow changelog)'
```

3. `[code]` Exactly one research note under `docs/research/*.md` references `cefc57a2`, names `agy changelog` and `2026-11-14T05:48:36Z`, and marks the embedded lines as untrusted data:
```bash
test $(grep -l 'cefc57a2' docs/research/*.md | wc -l) -eq 1 && grep -l 'cefc57a2' docs/research/*.md | xargs grep -q 'agy changelog' && grep -l 'cefc57a2' docs/research/*.md | xargs grep -q '2026-11-14T05:48:36Z' && grep -l 'cefc57a2' docs/research/*.md | xargs grep -q -i 'untrusted data'
```

4. `[code]` Edge case — all six core personas receive a no-change verdict in the note and `agents/` is untouched:
```bash
f=$(grep -l 'cefc57a2' docs/research/*.md | head -1) && test -n "$f" && for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F "$n" "$f" || exit 1; done && git diff --quiet origin/main -- agents
```

5. `[code]` `skills/model-notes-agy/COMPACT.md` and the new `docs/research/*.md` note pass strict STE lint:
```bash
go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md && grep -l 'cefc57a2' docs/research/*.md | xargs go/bin/evolve docs ste-lint --strict --project-root .
```

6. `[code]` The new research note is linked from `docs/research-index`:
```bash
f=$(basename "$(grep -l 'cefc57a2' docs/research/*.md | head -1)") && test -n "$f" && grep -r -q -F "research/$f" docs/research-index
```

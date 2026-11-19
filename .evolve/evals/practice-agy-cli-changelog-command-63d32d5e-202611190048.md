---
score_cap:
  - criterion: "The agy notes carry exactly one Cycle 2525 bullet that names agy 1.5.12 and says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*Cycle 2525' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2525' skills/model-notes-agy/COMPACT.md | grep -q 'agy 1.5.12' && grep -E '^- .*Cycle 2525' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'"
  - criterion: "The Cycle 2525 bullet reads each changelog line as data and states no persona change under ADR-0133"
    max_if_missing: 7
    evidence: "grep -E '^- .*Cycle 2525' skills/model-notes-agy/COMPACT.md | grep -q -i 'read each changelog line as data' && grep -E '^- .*Cycle 2525' skills/model-notes-agy/COMPACT.md | grep -q -i 'no persona change.* under ADR-0133'"
  - criterion: "The agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md"
  - criterion: "No file under agents/ changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
---

# Eval: agy CLI changelog command, release 1.5.12 review (cycle 2525)

> Pins the deliverable of the practice-watch item `practice-agy-cli-changelog-command-63d32d5e-202611190048`. The command `agy changelog` changed after the snapshot of `2026-11-17T10:16:34Z` and now lists release 1.5.12. The agy notes gain one dated Cycle 2525 bullet that reads the lines as data and states that no persona changes under ADR-0133. Source pattern: cycle 2454 for item `c17a8478`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One Cycle 2525 bullet names agy 1.5.12 and says no prompt changes | 6/10 | `grep -c` on the notes |
| data-and-verdict | The bullet reads lines as data and cites ADR-0133 with no persona change | 7/10 | `grep -q` on the bullet |
| ste-lint | The notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |
| agents-untouched | No persona file changes | 7/10 | `git diff --quiet origin/main -- agents` |

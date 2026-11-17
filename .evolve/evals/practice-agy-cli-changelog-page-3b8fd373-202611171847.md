---
score_cap:
  - criterion: "The agy notes carry exactly one Cycle 2484 bullet that says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*Cycle 2484' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2484' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'"
  - criterion: "The Cycle 2484 bullet names App v2.33.0 and CLI 1.5.9, and reads the lines as data"
    max_if_missing: 6
    evidence: "grep -E '^- .*Cycle 2484' skills/model-notes-agy/COMPACT.md | grep -q 'v2.33.0' && grep -E '^- .*Cycle 2484' skills/model-notes-agy/COMPACT.md | grep -q '1.5.9' && grep -E '^- .*Cycle 2484' skills/model-notes-agy/COMPACT.md | grep -q -i 'as data'"
  - criterion: "The Cycle 2484 bullet states no persona changes under ADR-0133 with a reason"
    max_if_missing: 7
    evidence: "grep -E '^- .*Cycle 2484' skills/model-notes-agy/COMPACT.md | grep -q -i 'no persona change.* under ADR-0133'"
  - criterion: "No persona under agents/ changes, which is the ADR-0133 verdict that none changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
  - criterion: "The agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md"
---

# Eval: agy CLI changelog page, App v2.33.0 review (cycle 2484)

> Pins the deliverable of the practice-watch item `practice-agy-cli-changelog-page-3b8fd373-202611171847`. The page `https://antigravity.google/docs/changelog?tab=cli` changed after the snapshot of `2026-11-17T10:16:34Z`. The page lists App v2.33.0 (2026-11-17: SSH host connections, GitHub pull request pills, the /swarm command for business accounts) and CLI v1.5.9 as the newest CLI release. The changed lines are a multiline prompt cursor fix. The agy notes gain one dated bullet that reads the lines as data and gives the ADR-0133 verdict that no persona changes. Source incident: cycles 2393, 2395, 2454 and 2458 of the same watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One Cycle 2484 bullet in COMPACT.md that says no prompt changes | 6/10 | `grep -c` on the notes |
| bullet-content | The bullet names the App and CLI releases and reads them as data | 6/10 | `grep` on the bullet |
| persona-verdict | The bullet states no persona change under ADR-0133 | 7/10 | `grep -i` on the bullet |
| agents-unchanged | agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

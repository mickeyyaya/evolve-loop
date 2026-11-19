---
score_cap:
  - criterion: "The agy notes carry exactly one Cycle 2522 bullet that says no prompt changes"
    max_if_missing: 6
    evidence: "test $(grep -c -E '^- .*Cycle 2522' skills/model-notes-agy/COMPACT.md) -eq 1 && grep -E '^- .*Cycle 2522' skills/model-notes-agy/COMPACT.md | grep -q -i 'no prompt change'"
  - criterion: "The Cycle 2522 bullet names the source link, Gemini 4 Neon, CLI 1.5.11, the Orchestrator and SSH, and reads the lines as data"
    max_if_missing: 6
    evidence: "b=$(grep -E '^- .*Cycle 2522' skills/model-notes-agy/COMPACT.md) && for k in 'https://antigravity.google/docs/changelog?tab=cli' 'Gemini 4 Neon' '1.5.11' 'Orchestrator' 'SSH'; do printf '%s' \"$b\" | grep -q -F \"$k\" || exit 1; done; printf '%s' \"$b\" | grep -q -i 'line as data'"
  - criterion: "The Cycle 2522 bullet states no persona changes under ADR-0133 with a reason"
    max_if_missing: 7
    evidence: "grep -E '^- .*Cycle 2522' skills/model-notes-agy/COMPACT.md | grep -q -i 'no persona change.* under ADR-0133'"
  - criterion: "The Orchestrator model stays unused, tied to the rule that no LLM starts or directs another"
    max_if_missing: 6
    evidence: "grep -E '^- .*Cycle 2522' skills/model-notes-agy/COMPACT.md | grep -q -i 'orchestrator.*stays unused.*because.*no LLM starts or directs another'"
  - criterion: "No persona under agents/ changes, which is the ADR-0133 verdict that none changes"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents"
  - criterion: "The agy notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . skills/model-notes-agy/COMPACT.md"
---

# Eval: agy CLI changelog page, Gemini 4 Neon review (cycle 2522)

> Pins the deliverable of the practice-watch item `practice-agy-cli-changelog-page-3319a318-202611190048`. The page `https://antigravity.google/docs/changelog?tab=cli` changed after the snapshot of `2026-11-18T05:23:53Z`. The lines show Gemini 4 Neon as the default model, Orchestrator (Neon), SSH connections and CLI v1.5.11. The agy notes gain one dated bullet that reads the lines as data and gives the ADR-0133 verdict that no persona changes. Source incident: cycles 2454, 2458, 2484 and 2497 of the same watch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One Cycle 2522 bullet in COMPACT.md that says no prompt changes | 6/10 | `grep -c` on the notes |
| bullet-content | The bullet names the link, the model, the release, the Orchestrator and SSH, and reads them as data | 6/10 | `grep` on the bullet |
| persona-verdict | The bullet states no persona change under ADR-0133 | 7/10 | `grep -i` on the bullet |
| orchestrator-unused | The Orchestrator stays unused because no LLM starts or directs another | 6/10 | `grep -i` on the bullet |
| agents-unchanged | agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |

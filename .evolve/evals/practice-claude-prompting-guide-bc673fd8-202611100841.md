---
score_cap:
  - criterion: "The claude notes carry exactly one bullet that names effort, Fable 5.2, Mythos 5.2, thinking and lowering the level"
    max_if_missing: 6
    evidence: "test $(grep -c -i -E '^- .*effort' skills/model-notes-claude/COMPACT.md) -eq 1 && grep -i -E '^- .*effort' skills/model-notes-claude/COMPACT.md | grep -q -F 'Mythos 5.2' && grep -i -E '^- .*effort' skills/model-notes-claude/COMPACT.md | grep -q -F 'Fable 5.2' && grep -i -E '^- .*effort' skills/model-notes-claude/COMPACT.md | grep -q -i 'thinking' && grep -i -E '^- .*effort' skills/model-notes-claude/COMPACT.md | grep -q -i 'lower'"
  - criterion: "The note cites the source link, the snapshot, ADR-0133 and the file that changes, and calls the guide text untrusted data"
    max_if_missing: 6
    evidence: "f=$(grep -l -F bc673fd8 docs/research/*.md | head -1) && test -n \"$f\" && grep -q -F 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices' $f && grep -q -F '2026-11-06T02:47:04Z' $f && grep -q -F 'ADR-0133' $f && grep -q -F 'skills/model-notes-claude/COMPACT.md' $f && grep -q -i 'untrusted data' $f"
  - criterion: "The note gives each core persona a no-change verdict and no file under agents/ changes"
    max_if_missing: 7
    evidence: "f=$(grep -l -F bc673fd8 docs/research/*.md | head -1) && test -n \"$f\" && for n in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q -F $n $f || exit 1; done && git diff --quiet origin/main -- agents"
  - criterion: "The note and the claude notes pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "f=$(grep -l -F bc673fd8 docs/research/*.md | head -1) && go/bin/evolve docs ste-lint --strict --project-root . $f skills/model-notes-claude/COMPACT.md"
  - criterion: "The research index links the note"
    max_if_missing: 5
    evidence: "f=$(basename $(grep -l -F bc673fd8 docs/research/*.md | head -1)) && grep -r -q -F \"(research/$f)\" docs/research-index"
---

# Eval: Claude prompting guide, Fable 5.2 effort calibration (cycle 2278)

> Pins the deliverables of the practice-watch item practice-claude-prompting-guide-bc673fd8-202611100841. The source https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices changed after the snapshot of 2026-11-06T02:47:04Z. The note reads the lines as untrusted data and states under ADR-0133 that no persona changes. The claude notes gain one bullet on effort calibration for Claude Fable 5.2 and Claude Mythos 5.2. Source incident: cycle 2276, which uses the same deliverable shape for the release-notes source.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| notes-bullet | One effort bullet in COMPACT.md that names Fable 5.2, Mythos 5.2, thinking and lowering | 6/10 | `grep -c` on the notes |
| source-cited | Source link, snapshot, ADR and file named; data-not-instructions | 6/10 | `grep -F` on the note |
| persona-verdicts | Six personas covered; agents/ unchanged | 7/10 | `git diff --quiet origin/main -- agents` |
| ste-lint | Note and notes pass the sentence lint | 6/10 | `evolve docs ste-lint --strict` |
| index-linked | Research index links the note | 5/10 | `grep -r` over docs/research-index |

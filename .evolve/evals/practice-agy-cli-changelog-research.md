---
score_cap:
  - criterion: "The research note exists and cites the source command agy changelog and the snapshot 2026-10-26T18:27:01Z"
    max_if_missing: 3
    evidence: "f=docs/research/practice-agy-cli-changelog-2026-11.md; test -f $f && grep -q 'agy changelog' $f && grep -q '2026-10-26T18:27:01Z' $f"
  - criterion: "The note frames the changelog lines as untrusted data"
    max_if_missing: 4
    evidence: "grep -qi 'untrusted' docs/research/practice-agy-cli-changelog-2026-11.md"
  - criterion: "The note gives an ADR-0133 verdict for each of the six core personas tied to the M2 baseline"
    max_if_missing: 5
    evidence: "f=docs/research/practice-agy-cli-changelog-2026-11.md; for p in evolve-scout evolve-triage evolve-tdd-engineer evolve-builder evolve-auditor evolve-retrospective; do grep -q \"$p\" $f || exit 1; done; grep -q 'ADR-0133' $f && grep -q 'M2' $f"
  - criterion: "The research index lists the new note and keeps the October note"
    max_if_missing: 5
    evidence: "grep -q 'practice-agy-cli-changelog-2026-11.md' docs/research-index.md && grep -q 'practice-agy-cli-changelog-2026-10.md' docs/research-index.md"
  - criterion: "No persona file under agents/ changes"
    max_if_missing: 6
    evidence: "test -z \"$(git diff --name-only origin/main...HEAD -- agents/)\""
---

# Eval: Practice change in agy changelog, November 2026 note

> Pins the document deliverable of inbox item practice-agy-cli-changelog-command-772be890-202610270728 (cycle 2120). The cycle is a document cycle, so the graders are shell commands and not Go predicates. The note reads the agy changelog as data and records that no persona changes before the M2 baseline of ADR-0133.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| note-present | Note exists, cites source and snapshot | 3/10 | grep of note |
| untrusted-framing | Lines framed as untrusted data | 4/10 | grep of note |
| persona-verdicts | Six personas, ADR-0133, M2 | 5/10 | grep loop |
| index-entry | New row in research-index.md, October row kept | 5/10 | grep of index |
| no-persona-change | agents/ unchanged | 6/10 | git diff |

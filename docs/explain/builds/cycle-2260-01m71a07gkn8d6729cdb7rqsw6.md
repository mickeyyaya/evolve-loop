# Build Explanation — Cycle 2260

## Build Binding
- Cycle: 2260
- Base SHA: c1647a17574b2d597c8f816af18f6b4858e17b10

## Summary
A new research note reviews release 2.1.317 of the Claude Code changelog and states that no persona changes.

## Rationale
The practice watch filed the item. The acceptance asks for the lines to be explained as data and for a proposal under ADR-0133. A note that follows the 2.1.316 review keeps one format for all releases.

## Changed Areas
- `docs/research/claude-code-changelog-2-1-317.md` — explains the lines of 2.1.317 that touch the loop, names two watch items, and gives each core persona a no-change verdict.
- `solutions/practice-claude-code-changelog-a1065345-202611030236/proposal.md` — answers the request first: no persona changes, with the two watch items.
- `solutions/practice-claude-code-changelog-a1065345-202611030236/assumptions-and-evidence.md` — lists the assumptions A1 to A4 and the evidence E1 to E7 that the options cite.
- `solutions/practice-claude-code-changelog-a1065345-202611030236/options/1-no-persona-change-record-verdict.md` — the chosen strategy: record a verdict and change nothing.
- `solutions/practice-claude-code-changelog-a1065345-202611030236/options/2-edit-personas-now.md` — the rejected strategy: edit personas without a baseline.
- `solutions/practice-claude-code-changelog-a1065345-202611030236/options/3-probe-hardening-follow-up-item.md` — the runner-up: harden the `claude -p` probes in a later cycle.
- `solutions/practice-claude-code-changelog-a1065345-202611030236/recommendation.md` — compares the three options and names Option 1 with the flip conditions.

## Design Decisions
The note reads the section at the link, not the cut text of the inbox item. Each persona gets a verdict because ADR-0133 changes one persona at a time and only after a baseline measure.

## Verification
The strict STE lint passes on the note. The graders of the eval check the file, the link, the untrusted label, the watch items and the ADR-0133 statement.

## Compatibility
No code, persona or schema changes.

## Limitations
The note does not review releases 2.1.318 to 2.1.321. The next watch change files them.

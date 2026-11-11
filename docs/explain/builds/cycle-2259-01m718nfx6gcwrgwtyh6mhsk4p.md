# Build Explanation — Cycle 2259

## Build Binding
- Cycle: 2259
- Base SHA: 82dff1afa210209c7c30c37b3f15649997e1cbb5

## Summary
The research note on the agy changelog now has an addendum that dispositions the re-detected practice item.

## Rationale
The note already covers the releases 1.4.3 to 1.4.12 and already finds no persona change before the M2 baseline. An addendum records the snapshot and the decision, so the watch does not raise the item again. A new document can duplicate the note.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-2026-11.md` — adds section 5. It records the item id, the command, the snapshot and the newest release. It states the untrusted-data rule and the ADR-0133 no-change decision with its reason.

## Design Decisions
The addendum is a new top-level section, so the earlier sections and persona verdicts stay unchanged. No agent, skill or Go file changes.

## Verification
The six ACS predicates of cycle 2259 pass, and ste-lint strict reports 0 findings for the note.

## Compatibility
Sections 1 to 4 of the note keep their headings and text.

## Limitations
The two candidates for milestone M3 stay open and need the M2 baseline data.

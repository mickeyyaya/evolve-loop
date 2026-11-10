# Build Explanation — Cycle 2267

## Build Binding
- Cycle: 2267
- Base SHA: eaea4edd8f5d28ea3491fff77a35cefc27d3b303

## Summary
The package note of internal/gc now has an Amended by line that links the gc plan. Its branch invariant and its hook invariant now state the rules of decision D9.

## Rationale
Plan decision D9 says the spec of record must not contradict the code. Without the change, the note keeps the old rule "an unmerged branch is never deleted". The edit is a text change only, so it is the smallest correct change.

## Changed Areas
- `docs/architecture/packages/internal-gc.md` — adds the Amended by line. It replaces the unmerged-branch rule with the D2, D4 and D6 rule. It replaces the batch hook rule with the D2 and D7 rule at `gc.wave_end.scope: full`.
- `go/internal/gc/internal_gc_note_test.go` — checks the note: the link, the five decision ids, both invariants and the removal of the old wording.
- `go/acs/cycle2267/predicates_test.go` — pins the note, the STE lint of the amended lines and the decision ids against the plan.
- `.evolve/evals/gc-legacy-gap-x9-internal-gc-amended-invariant.md` — grades the three acceptance criteria.

## Design Decisions
The check lives in the test file of the package and reads the note. The note stays the single home of the rules, and the test holds no copy of its prose beyond the fixture.

## Verification
The note test and the four ACS predicates pass. The fixture cases remove each amendment in turn, and each removal fails the check with a named reason.

## Compatibility
No production code changes. The old pinned test names in the hook invariant stay valid.

## Limitations
The key `gc.wave_end.scope` is not yet in the policy code. Other components of the plan build it, and this note states the rule they must meet.

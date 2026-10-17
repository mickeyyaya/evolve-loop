# Build Explanation — Cycle 1923

## Build Binding
- Cycle: 1923
- Base SHA: 6b938e05d1f7ccb9f9570504c848c6017f70ff1a

## Summary
The follow-up list of the unit-06 design document has its specifications back. Each 06-Fn bullet now holds short sentences with its targets and `file:line` anchors.

## Rationale
Cycle 1922 cut each follow-up to one line and deleted the targets and anchors (audit finding M1). The restore takes the text from commit 6e477f7b8 and splits it into sentences of 25 words or fewer.

## Changed Areas
- `docs/architecture/decomposition/06-inboxmover.md` — restores the 06-F1 to 06-F11 specifications as indented sub-bullets under each bullet, so a reader can act on each follow-up again.
- `.evolve/evals/inboxmover-doc-spec-restore-06-fn.md` — the eval of the TDD phase that pins the restore; the build did not change it.
- `go/acs/cycle1923/predicates_test.go` — the ACS predicates of the TDD phase that check anchors, the 06-F2 record and the STE limits; the build did not change them.

## Design Decisions
The one-line bullet stays as the title of each follow-up, and the detail goes into sub-bullets. Thus the short list of cycle 1922 stays readable, and the detail returns. The 06-F2 bullet keeps its retirement record of cycle 1864. The rejected option was a verbatim restore of the old paragraph. It has sentences of more than 80 words. It also brings back the present-tense text about the retired writer.

## Verification
`go test -tags acs -count=1 ./acs/cycle1923/` passes all six predicates. `evolve docs ste-lint --strict` reports no finding on lines 365 to 405 of the design document.

## Compatibility
The change is documentation only. No code, gate or test behavior changes.

## Limitations
The `file:line` anchors cite the unit-06 tree of 2026-09-14. Some lines moved since then. The block gives the current place for 06-F8, 06-F9 and 06-F11 only.

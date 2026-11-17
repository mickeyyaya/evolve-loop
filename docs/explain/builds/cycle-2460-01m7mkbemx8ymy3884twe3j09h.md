# Build Explanation — Cycle 2460

## Build Binding
- Cycle: 2460
- Base SHA: 27353b988f88a4db05a4cfeb3c42a139ba647deb

## Summary
The build adds a design that scores three groupings of `go/cmd/evolve`. It names `inbox` as the first group to move. It gives the move order and a red test for each move.

## Rationale
A document cycle moves no code. The design reuses the existing `internal/cli/<group>cmd` pattern, so the plan needs no new mechanism.

## Changed Areas
- `docs/plans/split-cmd-evolve-2026-11.md` — the plan: census, three scored groupings, decision, move order, red tests, risks.
- `solutions/edit-layout-c6-split-cmd-evolve-design/assumptions-and-evidence.md` — the numbered assumptions and evidence that the options cite.
- `solutions/edit-layout-c6-split-cmd-evolve-design/options/1-one-package-per-verb.md` — option 1, score 2.00.
- `solutions/edit-layout-c6-split-cmd-evolve-design/options/2-co-change-clusters.md` — option 2, score 4.20.
- `solutions/edit-layout-c6-split-cmd-evolve-design/options/3-domain-groups.md` — option 3, score 3.60.
- `solutions/edit-layout-c6-split-cmd-evolve-design/recommendation.md` — the comparison and the choice of option 2.

## Design Decisions
Co-change clusters score highest because the files of one usual item stay in one cell. The spine stays in `main` until a separate design.

## Verification
`evolve docs ste-lint --strict` reports 0 findings on the plan, the solution set and this document. The six build items are filed as the requests `inbox-add-1` to `inbox-add-6`. `evolve solution check` reports OK. `git status --porcelain -- go/` is empty.

## Compatibility
No Go file, verb or registry entry changes.

## Limitations
The inbound counts come from a name-match script and can over-count. The docs-review run is requested, not done.

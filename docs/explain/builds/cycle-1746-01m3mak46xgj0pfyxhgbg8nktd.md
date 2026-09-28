# Build Explanation — Cycle 1746

## Build Binding
- Cycle: 1746
- Base SHA: cc16fb890eeefe1fe7d906574638d8c581e2e388

## Summary
The logic-first delivery design document's component table now states what the repository proves: W8, G2 and G3 read shipped with the cycle that shipped them, P3 reads merged with its PR, and the two duplicated bullets are gone.

## Rationale
§7's legend defines shipped as a commit on a branch with its PR open or merged. Three rows contradicted their consumed inbox records: W8 (cycle 1724, `7889cc5b`), G2 (cycle 1726, `5cbc3e56`) and G3 (cycle 1729, `2f7c8683`), and each of those commits is an ancestor of the base. P3 still read "PR pending" although #661 merged as `abc9ced0`. Correcting the status cells in place is the smallest change that makes the document true again; rows whose inbox records are still pending keep their designed status.

## Changed Areas
- `docs/architecture/logic-first-delivery-design.md` — the P3, W8, G2 and G3 status cells corrected from the merge commit and the PASS dossiers; the header's stale 14:14 **Status:** bullet and §12's verbatim-repeated walled-dispatch bullet removed, one copy of each kept; the header's last-updated stamp and a §14 history row record the pass.
- `docs/explain/builds/cycle-1746-01m3mak46xgj0pfyxhgbg8nktd.md` — this explanation record.

## Design Decisions
Each corrected status cites the shipping cycle and its commit, so a reader can check the claim without the inbox. G2 keeps its §5.13 ceiling note, and W8 and G3 keep their §5.10 references; only the status word and its evidence change. Component, files, headings and every other row are untouched.

## Verification
`go test -tags acs -count=1 ./acs/cycle1746/...` passes all seven predicates, which derive each expected status from the consumed and pending inbox records, the PASS dossiers and the P3 merge commit, and compare headings, rows and bullet labels against the base.

## Compatibility
Documentation only; no code, configuration or schema changes.

## Limitations
Rows without an inbox citation (for example P2, H3 or the F rows) were not re-audited against history in this pass.

# internal/researchindex

> Plan: [pr-merge-latency-2026-10](../../plans/pr-merge-latency-2026-10.md) (P3, D9). Commands: `evolve docs index <generate|check>` (`go/cmd/evolve/cmd_derived_sync.go`) and `evolve pr refresh` (`go/cmd/evolve/cmd_pr_refresh.go`). Catalog entry: `research-index` in [internal-derived.md](internal-derived.md).

## Purpose

`internal/researchindex` writes the dated entries of `docs/research-index.md` from one file for each entry in `docs/research-index/`. Before P3, each docs PR appended a section to the end of the index. Thus two docs PRs changed the same lines, and the second PR needed a rebase and a hand merge (plan F4).

## Design

- **One file for each entry.** An entry is a Markdown file in `docs/research-index/`. It starts with a `## ` heading line, which is its index line. The name sets the order: `<yyyy-mm-dd>-<nn>-<slug>.md`. Links in an entry are relative to `docs/`, because the text renders into `docs/research-index.md`.
- **One generated region.** `Generate` writes each entry, in name order, into the region between `Begin` and `End` of `docs/research-index.md`. The text above the region is hand-written and does not change. `Generate` writes only when the text changes.
- **The check.** `Check` returns `ErrStale` when the region differs from its entries. The error names each entry whose heading line is not in the index. Else it says that the region differs. A fault (no entry, an entry with no heading, no index, a broken region) is an error that is not `ErrStale`.
- **A worked example.** PR A adds `2026-10-21-01-a.md`, and PR B adds `2026-10-21-01-b.md`. Each PR runs `evolve docs index generate`. After A merges, B conflicts inside the region. `evolve pr refresh` in the merge of B regenerates the region from both files and stages it. The result is the same in each merge order.

## Invariants

- **Name order, and the head stays.** Pinned by `TestGenerate_WritesEachEntryInNameOrderInsideTheRegion`.
- **A missing index line is named, and `Generate` fixes it.** Pinned by `TestCheck_NamesAnEntryWithNoIndexLineAndPassesAfterGenerate`.
- **A hand edit in the region is stale.** Pinned by `TestCheck_AHandEditInsideTheRegionIsStale`.
- **Two entry PRs merge in either order to one index with no hand merge.** The conflict lies inside the region, so `derived.IsDerivedConflict` is true. Pinned by `TestTwoEntryPRs_MergeInEitherOrderToOneIndexWithNoHandMerge`.

## Limits

- GitHub does not apply a merge driver of `.gitattributes` to a PR merge ([rubocop #3594](https://github.com/rubocop-hq/rubocop/pull/3594), 2016). Thus the second PR still shows a conflict on GitHub. `evolve pr refresh` resolves it with no hand merge. The operator accepted this form of the acceptance on 2026-10-21 (plan D12).

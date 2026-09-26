# Build Explanation — Cycle 1707

## Build Binding
- Cycle: 1707
- Base SHA: a9d8b543cdc9e6493a88a1bb19f5d38fbdcc7411

## Summary
`LineageKey` now additionally strips a calendar-year-anchored date-shaped run
(`YYYY[-MM[-DD]]`) so same-line dated snapshot ids (`gpt-4o-2024-08-06` /
`gpt-4o-2024-11-20`) share one lineage bucket. `NewestInLineage` compares the
numeric version first, read with the date removed, and uses the calendar date
only to order equal versions that are both dated. `PromoteLatest` now puts the
classifier's selection first in its bucket, so any tie keeps the selection.
Dated snapshots of one line now promote to the newer one instead of being a
silent no-op, and a date can never outrank a version.

## Rationale
The existing `LineageKey` stripped only the first dotted-numeric run
(`versionToken`), which left trailing date fragments on dated snapshot ids —
so two snapshots of the same underlying model line never bucketed together
and promotion across them never fired. The conservative behavior was
deliberately pinned by `TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation`
as a fail-safe (an uncertain identity must never substitute), with an
explicit migration note describing exactly how to lift the limitation once a
date-run strip could be added without colliding capability-bearing digit
suffixes (`:8b`, `:70b`, `32b`). Anchoring the new regex on a 4-digit year
prefixed with `19` or `20` is the smallest change that satisfies the
migration note: no live capability suffix or short dotted version number
(`4.6`, `2.5`, `3.1`) contains a calendar-year-shaped run, so the existing
capability-class separation tests continue to pass unmodified.

Stripping the date also widens the bucket: an undated id and a dated id of a
different version (`gpt-5` and `gpt-4-2024-04-09`, both key `gpt`) now share
a bucket. Letting the date alone decide, as the first attempt did, would
promote `gpt-5` down to `gpt-4-2024-04-09`, so the version must decide first.
A date is not comparable with a missing date, so a dated/undated pair at
equal versions is a tie, and a tie must keep the classifier's pick — the
fail-safe rule the removed known-limitation pin stated ("an uncertain
identity must never substitute").

## Changed Areas
- `go/internal/modelquery/lineage.go` — adds the `dateRun` regex and strips it (in addition to the existing `versionToken` strip) before computing a model id's lineage key. It also adds `withoutDate`, the single date strip shared by `LineageKey` and the comparator so the bucket key and the compared version always agree.
- `go/internal/modelquery/newestwins.go` — adds `dateVersion`/`parseDate` and a `newer` comparator: version first (read from the id with the date removed), then calendar date only when both ids are dated and versions are equal. `NewestInLineage`'s loop calls `newer`, and its contract comment now states the date rules. `compareParts` and `newerVersion` are unchanged.
- `go/internal/modelquery/latest.go` — adds `incumbentFirst`, and `PromoteLatest` passes the bucket with the selection moved to the front, so only a strictly newer member replaces it. This stops a dated/undated tie from swapping `gpt-4o` for `gpt-4o-2024-08-06` just because the CLI listed the snapshot first.
- `go/internal/modelquery/latest_test.go` — adds `TestPromoteLatest_MixedDatedBucketsNeverDowngrade`, which covers mixed dated/undated buckets through `PromoteLatest` in both listing orders and checks that the candidate slice is not reordered.
- `go/internal/modelquery/newestwins_test.go` — adds `TestNewestInLineage_VersionDecidesBeforeDate`, pinning the comparator contract directly: the version decides before the date, a date alone is never a version, and a dated/undated tie keeps the first-listed id.
- `go/internal/modelquery/lineage_test.go` — removes `TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation` and moves its case into `TestLineageKey_SeparatesCapabilityClasses`'s `mustMatch` table per the test's own migration note; adds two `mustDiffer` rows (`llama3.1:8b`/`llama3.1:70b`, `qwen2.5-coder:32b`/`qwen2.5-coder:70b`) guarding against an over-broad date strip collapsing size/tag suffixes.
- `go/internal/modelquery/fingerprint.go` — bumps `decisionVersion` from `"v1"` to `"v2"` since this is a real classification/promotion semantics change (per the constant's own doc comment: this is what makes the reuse gate correct rather than merely fast).
- `go/internal/modelquery/decisionversion_pin_test.go` — adds `newestwins.go` to `decisionSurfaceFiles` (the promotion comparator is decision surface but was not covered by the ratchet) and updates `decisionSurfacePin` to the sha256 over the changed decision-surface sources, keeping the hand-bump ratchet in sync with the fingerprint constant's version bump.
- `go/internal/modelquery/repro_cycle1707_test.go` — adds a permanent regression test that reproduces the bug through the production entry point `PromoteLatest` (not `LineageKey`/`NewestInLineage` directly), matching how `latest.go`'s `liveTiers` actually calls it.
- `go/acs/cycle1707/predicates_test.go` — pre-existing TDD-authored acceptance predicates for this cycle; committed alongside the fix unmodified (Builder does not author or edit ACS predicates).
- `docs/architecture/model-discovery-and-catalog.md` — replaces the "Known limitation (deliberate, pinned)" paragraph describing the old date-snapshot behavior with a description of the resolved behavior.
- `.evolve/inbox/consumed/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json` — the harness's own consumption marker for this cycle's bound inbox item, renamed from `.evolve/inbox/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json`: the record is moved from the inbox root into `consumed/` once its task is delivered, so it is not re-queued into a future triage cycle. File content is unchanged; only its inbox-tracked location moves.

## Design Decisions
The date regex is anchored on a `(?:19|20)\d{2}` calendar-year prefix rather
than a bare 4-digit run, so it cannot fire on any digit sequence that isn't
plausibly a year — this is what keeps `:8b`/`:70b`/`32b` and short dotted
version numbers untouched without a suffix/length allowlist. The date-strip
in `LineageKey` runs before the existing `versionToken` strip (not after),
so a date fragment is fully removed before the version-token regex gets a
chance to partially consume a leftover piece of it.

The comparator (`newer`) orders ids lexicographically: the numeric version
first, read with the date run removed, and then the calendar date, only when
both ids are dated. `compareParts`/`newerVersion` are reused unchanged. This
is a strict partial order, so the winner of a scan is always strictly newer
than the id the scan started from. `PromoteLatest` starts the scan from the
selection (`incumbentFirst`), so promotion can never replace the selection
with something that is not provably newer.

The alternative "an undated id beats a dated one at equal versions (treat it
as a moving alias)" was rejected. It would swap a pinned snapshot the
classifier chose for an alias whose resolution Go cannot see, which is an
uncertain substitution.

`decisionVersion` is bumped to `"v2"` because the promotion decision's
observable behavior changed (dated snapshots that previously never promoted
now do); this invalidates any cached `CandidatesHash` computed under the old
semantics so a CLI's prior tier map is reclassified instead of silently
reused across the fix.

## Verification
- `cd go && go test -race -count=1 -v ./internal/modelquery/...` — all
  existing and new tests pass, including the migrated
  `TestLineageKey_SeparatesCapabilityClasses` mustMatch/mustDiffer rows, the
  new `TestPromoteLatest_DatedSnapshotPromotion_Repro`, and
  `TestDecisionVersion_PinnedToAlgorithmSurface` (against the updated pin).
- `cd go && go test -tags acs -v ./acs/cycle1707/...` — all 9 ACS
  predicates (`TestC1707_001`..`TestC1707_009`) pass, including the
  mixed-bucket no-downgrade predicates 007–009.
- Mutation check: with `incumbentFirst` removed from `PromoteLatest`,
  `TestPromoteLatest_MixedDatedBucketsNeverDowngrade` fails on 5 of its 9
  cases.
- `cd go && gofmt -l ./internal/modelquery/ ./acs/cycle1707/` — clean.
- `cd go && go vet ./internal/modelquery/... ./acs/...` — clean.
- `cd go && go run ./cmd/apicover -enforce` — clean (exit 0, no new
  exported identifiers were added; `newer`, `parseDate`, `withoutDate`,
  `incumbentFirst`, and `dateVersion` are all unexported).
- `cd go && go test -count=1 ./internal/setup/... ./cmd/evolve/...` and the
  `acs/cycle442`/`499`/`503`/`504` predicates that import `modelquery` pass.

## Compatibility
Exported signatures are unchanged. Output changes in exactly two ways:

1. A bucket that holds a date-carrying id can now promote. A dated id moves
   to a strictly newer member: a higher version, or the same version with a
   later date. An undated id moves only to a strictly higher version, so it
   is never replaced by an older or equal-version dated id.
2. In `PromoteLatest`, a tie (equal versions, for example `opus-4` and
   `opus-4.0`) now keeps the selection. Before, it took whichever tied
   candidate the CLI listed first. This change is fail-safe only: it never
   swaps in a newer id that the old code would have kept.

Undated lineages with distinct versions keep their exact prior ordering, and
no live CLI id contains a calendar-year-shaped run. Every existing
`mustDiffer`/`mustMatch`/`RealCatalogIDs` case is asserted unchanged, and
`decisionVersion` `v2` invalidates cached tier maps from the old semantics.

## Limitations
A same-version dated/undated pair (`gpt-4o` and `gpt-4o-2024-08-06`) is a
deliberate tie: neither promotes to the other, because Go cannot tell which
snapshot an undated alias resolves to. When the version numbers differ, the
version decides, whether or not a date is present.

Compact `YYYYMMDD` snapshot dates (`claude-3-5-sonnet-20241022`) are not
matched by `dateRun`. They keep distinct keys and never cross-promote, which
is the old fail-safe behavior. Only the `YYYY[-MM[-DD]]` form is normalized.

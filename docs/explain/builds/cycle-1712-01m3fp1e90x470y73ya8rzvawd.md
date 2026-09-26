# Build Explanation — Cycle 1712

## Build Binding
- Cycle: 1712
- Base SHA: 875ba448d31f47152844739265c8a60a2838168a

## Summary
`LineageKey` now also strips a date run anchored on a calendar year
(`YYYY[-MM[-DD]]`). That lets dated snapshot ids from the same model line
(`gpt-4o-2024-08-06` / `gpt-4o-2024-11-20`) share one lineage bucket, so
`PromoteLatest` moves a selection of the older snapshot to the newer one.
Before, it silently did nothing for them. `NewestInLineage` compares the
numeric version first, read with the date removed. It uses the calendar date
only to order equal versions when both ids are dated. `PromoteLatest` now puts
the classifier's selection first in its bucket, so a tie always keeps the
selection. The reuse-gate `decisionVersion` goes from `v1` to `v2`, and its
source-hash pin now also covers `newestwins.go`.

This cycle continues salvage snapshot `aeea2c4f` (cycle 1708). That cycle's
build correction died on a transient rate-limit escalation before it could
ship. The code carried over unchanged. This cycle re-verified it against the
cycle-1712 acceptance predicates and adds the missing sixth criterion:
vet, `-race` and apicover green on `internal/modelquery`.

## Rationale
The old `LineageKey` stripped only the first dotted numeric run
(`versionToken`). Trailing date fragments stayed on dated snapshot ids, so two
snapshots of one line never shared a bucket and promotion between them never
fired. The known-limitation pin test documented this as a deliberate
fail-safe and described how to lift it: strip dates without colliding with
digit suffixes that carry capability (`:8b`, `:70b`, `32b`). The new regex
requires a 4-digit year starting with `19` or `20`. That is the smallest change
that meets the note. No capability suffix and no short dotted version (`4.6`,
`2.5`) contains such a run, so the existing capability-class separation rows
still pass unmodified.

Removing the date also widens buckets: `gpt-5` and `gpt-4-2024-04-09` both key
`gpt`. If the date decided alone, `gpt-5` could be demoted, so the version must
decide first. A date cannot be compared with a missing date, so a dated and an
undated id at the same version are a tie. A tie keeps the classifier's pick,
matching the removed pin's rule that an uncertain identity must never
substitute.

## Changed Areas
- `go/internal/modelquery/lineage.go` — adds `dateRun` and `withoutDate`. `LineageKey` now removes the date before the version token, so dated snapshots of one line share a key.
- `go/internal/modelquery/newestwins.go` — `NewestInLineage` delegates to a `newer` comparator: version first (read by `withoutDate`, so a date is never a version), then calendar date only when both ids are dated. `parseDate` turns the date run into a comparable `YYYYMMDD` integer.
- `go/internal/modelquery/latest.go` — `PromoteLatest` passes the bucket through `incumbentFirst`, so only a strictly newer member replaces the selection. The widened buckets need this so a date or listing order never causes a downgrade.
- `go/internal/modelquery/fingerprint.go` — `decisionVersion` `v1` → `v2`, so a tier map cached under the old lineage semantics is reclassified rather than reused.
- `go/internal/modelquery/decisionversion_pin_test.go` — adds `newestwins.go` to the decision-surface files and re-pins `decisionSurfacePin` for `v2` (re-pinned again after a comment-only correction to `newer` in `newestwins.go`, with no change in behavior).
- `go/internal/modelquery/lineage_test.go` — removes `TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation`. Its dated pair moves into the `mustMatch` table, with `:8b`/`:70b` and `:32b`/`:70b` rows added to `mustDiffer` for collision review.
- `go/internal/modelquery/newestwins_test.go` — adds `TestNewestInLineage_VersionDecidesBeforeDate`, covering version-before-date and both-dated ordering in both listing orders.
- `go/internal/modelquery/latest_test.go` — adds `TestPromoteLatest_MixedDatedBucketsNeverDowngrade`, covering mixed dated and undated buckets and checking that candidates are never reordered in place.
- `go/internal/modelquery/repro_cycle1707_test.go` — reproduces the bug through the production entry point `PromoteLatest`.
- `docs/architecture/model-discovery-and-catalog.md` — replaces the "Known limitation" paragraph with the resolved date-normalization contract. It records the fail-safe cases: compact `YYYYMMDD` dates and dated/undated ties.
- `go/acs/cycle1712/predicates_test.go` — this cycle's TDD-authored acceptance predicates (001–012), all green against this tree.
- `go/acs/cycle1707/predicates_test.go` — the prior attempt's acceptance predicates for the same item, carried from the salvage snapshot as regression coverage.
- `go/acs/cycle1708/predicates_test.go` — the second prior attempt's predicates for the same item, carried from the salvage snapshot as regression coverage.
- `.evolve/evals/lineage-datestamp-normalization.md` — eval rebound to the cycle-1712 predicates, with the vet/`-race`/apicover criterion added.
- `.evolve/inbox/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json` — removed from the pending inbox (the source side of the rename below) because this build implements the item, so it must not be triaged again.
- `.evolve/inbox/consumed/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json` — the same inbox item, moved to `consumed/` by an earlier attempt's ship-time inbox consume step so the record of the request stays alongside the build that delivers it. That step also rewrote the record: it adds a `consumed` provenance stamp (`at: 2026-09-26T13:20:27Z`, `via: ship`, empty `cycle`) and re-serializes the JSON with sorted keys (`weight` moves after `title`) and `>` escaped as `\u003e`, so git scores the rename at 93% similarity. The request fields (`acceptance`, `fix`, `problem`, `title`, `weight`) keep their values.
- `docs/private/research/archived-2026-09-26/unshipped-build-explanations/cycle-1707-01m3entpj6bj7t60tdfhqnx3q9.md` — cycle 1707's explanation of an unshipped build, archived rather than published.
- `docs/private/research/archived-2026-09-26/unshipped-build-explanations/cycle-1708-01m3f1yh2mf2np7bvgy0389fnt.md` — cycle 1708's explanation of an unshipped build, moved out of `docs/explain/builds/`. That directory is reserved for the explanation of a build that ships, and this document is that explanation.

## Design Decisions
One shared `withoutDate` helper serves both the bucket key and the version
comparator. They therefore always agree on which digits are a date. Two regex
call sites could drift apart.

The date comparator sits inside `NewestInLineage` behind a strict order:
version compared first, then date. The order is irreflexive and transitive, and
the scan replaces its current pick only with a member that this order ranks
above it. The winner is therefore either the scan's starting id itself (when nothing later is
strictly newer, ties included) or an id strictly newer than it. Putting the
incumbent first in `PromoteLatest` then makes "a tie keeps the selection" hold
by construction. `FreshnessPolicy`'s alias path checks membership only, so the
reordering cannot affect it.

Rejected alternative: stripping every digit run. That would merge `:8b` with
`:70b`, the exact collision the removed pin warned about. The `mustDiffer`
rows and predicate 002 guard against it.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1712/`: 12/12 predicates PASS (011–012 pin the explanation's own accuracy after audit round 1).
- `go test -count=1 -race ./internal/modelquery/` and `go vet ./internal/modelquery/` are green. apicover `-enforce` on the package passes through predicate 010.
- `evolve acs suite --cycle 1712`: `verdict=PASS green=179 red=0 skip=53 total=232`.

## Compatibility
Exported signatures are unchanged. Behavior changes are limited to dated ids
and to tie handling in `PromoteLatest`. Undated lines promote exactly as before,
guarded by predicate 009. The `decisionVersion` bump costs one reclassification
per CLI on the next refresh. No live CLI reports dated ids today, so current
tier maps are unaffected apart from that one-time refresh.

## Limitations
Compact `YYYYMMDD` dates (`claude-3-5-sonnet-20241022`) are not normalized.
They keep separate keys and never cross-promote, which is the fail-safe choice.
A dated and an undated id at the same version do not replace each other.
Whichever the classifier picked stays.

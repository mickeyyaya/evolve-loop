---
score_cap:
  - criterion: "LineageKey unifies same-line dated snapshots into one bucket and PromoteLatest moves to the newer date"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_001_DatedSnapshotsShareLineageAndPromote$' ./acs/cycle1712/"
  - criterion: "Size suffixes (:8b/:70b, 32b/7b) and capability words (mini, flash/pro) stay distinct after the date strip"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_002_CapabilityClassesStayDistinct$' ./acs/cycle1712/"
  - criterion: "NewestInLineage picks the later calendar date within one lineage in any listing order"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_003_NewestInLineageOrdersDatedSnapshots$' ./acs/cycle1712/"
  - criterion: "The known-limitation pin test is gone and its dated pair lives in the collision-reviewed mustMatch table"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_004_KnownLimitationTestMigratedToMustMatch$' ./acs/cycle1712/"
  - criterion: "decisionVersion is bumped so a cached pre-fix fingerprint is never reused"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_005_DecisionVersionBumped$' ./acs/cycle1712/"
  - criterion: "decisionSurfacePin stays in sync with the changed decision-surface files"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_006_DecisionSurfacePinInSync$' ./acs/cycle1712/"
  - criterion: "PromoteLatest never downgrades an undated/higher-version selection to a dated/lower-version one"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_007_PromoteLatestNeverDowngradesAcrossDateStamps$' ./acs/cycle1712/"
  - criterion: "NewestInLineage compares version before date in every documented ordering case"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_008_NewestInLineageComparesVersionBeforeDate$' ./acs/cycle1712/"
  - criterion: "Undated lines keep promoting exactly as before the date-aware comparator"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_009_UndatedPromotionUnchanged$' ./acs/cycle1712/"
  - criterion: "go vet, -race and apicover -enforce are green on internal/modelquery"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_010_ModelqueryVetRaceApicoverGreen$' ./acs/cycle1712/"
  - criterion: "The build explanation never calls a rename unchanged when git scores it below 100%, and the consumed inbox entry discloses the consume stamp and re-serialization"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_011_ExplanationRenameEntryMatchesTheDiff$' ./acs/cycle1712/"
  - criterion: "The build explanation's scan guarantee admits that NewestInLineage can end at its start"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1712_012_ExplanationScanGuaranteeAdmitsTheStart$' ./acs/cycle1712/"
---

# Eval: Lineage datestamp normalization

> Pins the acceptance criteria for `lineage-datestamp-normalization`
> (inbox record `2026-08-05T15-30-00Z-lineage-datestamp-normalization.json`):
> `LineageKey` strips a calendar-year-shaped date run in addition to its
> version-token strip, so same-line dated snapshots (`gpt-4o-2024-08-06` vs
> `gpt-4o-2024-11-20`) share one lineage bucket and `PromoteLatest` moves to
> the later one, while capability-bearing digits (`llama3.1:8b` vs
> `llama3.1:70b`) and capability words (`gpt-4o-mini` vs `gpt-4o`) never
> collapse. `NewestInLineage` reads the version with the date removed and
> lets a date break only a tie between two dated ids of equal version. The
> reuse-gate `decisionVersion` ratchet and `decisionSurfacePin` move together.
>
> Source: cycles 1707 and 1708 built and salvaged this fix (salvage snapshot
> `aeea2c4f`) but neither shipped — 1708's build correction died on a
> transient rate-limit escalation. Cycle 1712 re-materialises the criteria as
> `go/acs/cycle1712/predicates_test.go` and adds the inbox record's sixth
> criterion (vet/-race/apicover on `internal/modelquery`), which the earlier
> eval did not gate. Against main's pre-fix tree predicates 001/003/004/005/008
> are RED; the remaining five are guards proven RED by mutation (a partial fix
> that edits only `lineage.go`, and an over-broad digit strip).
>
> Cycle 1712's first audit failed the build on its explanation document alone
> (audit round 1, M1). Changed Areas called the consumed inbox item "content
> unchanged", although the ship's consume step stamps and re-serializes it
> (git rename similarity 93%). Design Decisions also said a `NewestInLineage`
> scan always ends strictly newer than its start. Predicates 011 and 012 check
> both claims against git's rename detection and against `NewestInLineage`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| lineage-unification | Dated snapshots share a key and promote | 8/10 | `TestC1712_001` |
| capability-negative | Size suffixes and capability words stay distinct | 8/10 | `TestC1712_002` |
| date-comparator | Later date wins within one lineage | 7/10 | `TestC1712_003` |
| pin-migration | Known-limitation test replaced by mustMatch row | 6/10 | `TestC1712_004` |
| ratchet-bump | decisionVersion bumped for the semantics change | 7/10 | `TestC1712_005` |
| ratchet-sync | decisionSurfacePin tracks the changed surface | 7/10 | `TestC1712_006` |
| no-downgrade | PromoteLatest never downgrades across date stamps | 8/10 | `TestC1712_007` |
| version-before-date | NewestInLineage compares version before date | 7/10 | `TestC1712_008` |
| no-regression | Undated promotion unchanged | 6/10 | `TestC1712_009` |
| package-hygiene | vet, -race, apicover -enforce green | 6/10 | `TestC1712_010` |
| explanation-rename-accuracy | No "unchanged" claim for a sub-100% rename; the consume stamp and re-serialization are disclosed | 5/10 | `TestC1712_011` |
| explanation-guarantee-accuracy | The scan guarantee admits the start | 5/10 | `TestC1712_012` |

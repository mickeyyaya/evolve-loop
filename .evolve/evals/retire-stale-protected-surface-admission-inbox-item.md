---
score_cap:
  - criterion: "The retired triage-protected-surface-admission inbox item stays retired — no file bearing its slug returns under .evolve/inbox/"
    max_if_missing: 6
    evidence: "test -z \"$(find .evolve/inbox -type f -name '*triage-protected-surface-admission*')\""
  - criterion: "The cycle-1312 protected-surface admission check (Classify -> protectedTopNViolation) stays wired and green"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'TestTriageClassify_(RejectsProtectedSurfaceTopNCard_BraceSyntax|RejectsProtectedSurfaceTopNCard_BareSyntax|RejectsAmongMultipleCards_NamesOffendingIdOnly|AllowsNonProtectedTopNCard|NoFilesSegmentIsUnaffected)$' ./internal/phases/triage"
  - criterion: "Retiring one backlog entry never sweeps the rest of .evolve/inbox/"
    max_if_missing: 7
    evidence: "test \"$(find .evolve/inbox -maxdepth 1 -name '*.json' | wc -l | tr -d ' ')\" -ge 60"
---

# Eval: Retire the stale triage-protected-surface-admission inbox item

> Cycle 1383 retired `.evolve/inbox/2026-08-04T05-04-00Z-triage-protected-surface-admission.json`.
> The fix that item requested — a second, commit-time admission check in
> `go/internal/phases/triage/triage.go`'s `Classify`, refusing any `## top_n`
> card whose `files=` segment names a `guards.IsProtectedSurface` path —
> shipped in cycle-1312 (commit `0d07b200`), is covered by five tests in
> `go/internal/phases/triage/protected_surface_admission_test.go`, and is
> documented in `docs/operations/batch-integrity-review-2026-08-04.md` §F4.
> The item was nonetheless never consumed, so it kept re-entering fleet_scope
> as live backlog and burned cycles re-verifying already-shipped work — the
> same "landed fix, inbox item never retired" waste cycles 1314/1323/1340/1343
> hit on a neighbouring item.
>
> This eval pins both halves of that closeout permanently. The retirement must
> not be undone (a re-minted or renamed copy of the item resurrects the waste),
> and — far more important — the admission check itself must stay wired. The
> retirement's whole justification is "the fix already exists"; if the fix ever
> silently regresses, the retirement becomes a lie and a protected-surface card
> can be committed to a cycle unchecked. Hence the highest cap sits on the
> behavioral suite, not on the bookkeeping. The third cap guards the retirement
> mechanism itself: a wildcard sweep of `.evolve/inbox/` would satisfy the
> first criterion while destroying the rest of the backlog.
>
> Source incident: cycle 1383 (scout finding "already-implemented"; triage
> top_n `retire-stale-protected-surface-admission-inbox-item`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stays-retired | No file bearing the item's slug exists under `.evolve/inbox/` | 6/10 | `find .evolve/inbox -type f -name '*triage-protected-surface-admission*'` is empty |
| fix-stays-wired | All 5 cycle-1312 `TestTriageClassify_*` admission cases pass | 9/10 | `go test -run '<the five cases>' ./internal/phases/triage` |
| no-collateral-sweep | ≥60 queued inbox items remain (77 at authoring, 76 after the single retirement) | 7/10 | `find .evolve/inbox -maxdepth 1 -name '*.json' \| wc -l` ≥ 60 |

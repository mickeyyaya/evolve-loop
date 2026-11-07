# Build Explanation — Cycle 2210

## Build Binding
- Cycle: 2210
- Base SHA: 577f1b88708154769bdcf3231bd2ae8b194f15a9

## Summary
A minor or major release now needs a committed content-review record. Preflight refuses the release without it. `evolve release --skip-content-review <reason>` bypasses the gate and writes the reason to the release journal.

## Rationale
The public site stated an old version on 2026-11-06, because no code checked that a content review ran. Two architecture reviews refused the first gate design. The gate therefore binds the record to the release, the seat, the verdict and the previous tag, and it reads the committed file.

## Changed Areas
- `go/internal/releasepreflight/content_review.go` — adds the gate. It reads the committed record and checks the release, the launch record, the seat and the verdict. The run must be later than the previous tag.
- `go/internal/releasepreflight/releasepreflight.go` — adds `Options.SkipContentReview` and runs the gate before the no-WIP gate.
- `go/internal/releasepreflight/preflight_run.go` — carries the bypass reason into the resolved options.
- `go/internal/releasepipeline/releasepipeline.go` — adds `Options.SkipContentReview` to the pipeline options.
- `go/internal/releasepipeline/release_run.go` — journals the step `content-review-bypass` with the reason, and passes the reason to preflight.
- `go/internal/releasepipeline/bridges.go` — adds `runPreflightWithReviewBypass`; `runPreflightLib` keeps its signature and calls it with no reason.
- `go/internal/cli/opscmd/release_pipeline.go` — parses `--skip-content-review` and refuses an empty reason.
- `go/internal/cli/opscmd/release_preflight.go` — states the gate in the usage text.
- `docs/architecture/packages/internal-releasepreflight.md` — documents the gate.
- `docs/architecture/packages/internal-releasepipeline.md` — documents the bypass journal step.
- `docs/plans/release-keeps-site-current-2026-11.md` — marks section 4.3 as built.

## Design Decisions
No `Head` field is added to `launchrecord.Record`, because that package is a protected surface. The run `ts` against the commit date of the previous tag detects a replay instead. The record is a separate file, so the version bump never rewrites it. The standalone `release-preflight` has no bypass, because the plan puts the bypass on `evolve release`. A new function carries the bypass, and the old five-argument seam stays, so no existing caller or test changes.

## Verification
The ten predicates of `go/acs/cycle2210` pass. They drive `RunReleasePipeline` and `RunReleasePreflight` in a two-release journey. Unit tests cover the patch case, the refusal text and the bypass log in preflight, and the journal step in the pipeline.

## Compatibility
A patch release is not gated. `runPreflightLib` and `Steps.Preflight` keep their signatures. A release without the new flag behaves as before, except that a minor or major release needs a record.

## Limitations
The gate cannot prove that the review read the right pages. A run whose clock is wrong can pass the replay check. The `Head` binding stays a console follow-up.

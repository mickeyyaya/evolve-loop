# Build Explanation — Cycle 2660

## Build Binding
- Cycle: 2660
- Base SHA: 83e996a69ddbd334da16cb1c4278ba09d8a6556b

## Summary
`ciwatch` now classifies `go test` timeout panics as `timeout` (`LabelTimeout`). `ghClassifySource.MainRunsBefore` also reads earlier tries (`1 .. Attempt-1`) of main runs, so a prior red try counts toward `recurred_on_main`.

## Rationale
`evolve ci classify` previously returned `unknown` for `panic: test timed out after <duration>` because `scanLog` ignored panic lines and `classify` had no timeout rule. Also, `MainRunsBefore` read only the latest try from `gh run list`. That hid an earlier red try on `main` after a rerun passed. Updating `failureParse` and `MainRunsBefore` fixes both bugs without changing `Evidence` fields or existing rule precedence.

## Changed Areas
- `go/internal/ciwatch/classify.go` — adds `LabelTimeout` (`"timeout"`), `FailingTest.Timeout`, and `classifyFailure` so timeouts get `timeout` while `base-red` and `touched` keep their precedence.
- `go/internal/ciwatch/classify_source.go` — updates `failureParse` to detect timeout panics and `running tests:` names, and updates `MainRunsBefore` with `runViewAttempt` to fetch earlier tries.

## Design Decisions
- `classifyRules` and the four-field `Evidence` struct stay unchanged. `classifyFailure` checks `FailingTest.Timeout` after `classify(e)`, so `base-red` and `touched` keep precedence and recurred timeouts keep `timeout`.
- Each name in `running tests:` becomes one `FailingTest` row, with package or step fallback when empty. That matches `--- FAIL:` rows in `failureKey`.
- Only `MainRunsBefore` expands earlier tries (`LatestRunOn` stays latest-only). Thus `base_red` still reflects the latest result on the base commit.

## Verification
`go test -count=1 ./internal/ciwatch/...`, `landcmd`, `sizeratchet`, `apicover`, and `evolve acs suite --cycle 2660` (`green=272 red=0 skip=53 total=325`) pass. Every changed function in `internal/ciwatch` has 100.0% statement coverage. `gofmt -l` and `go vet` are clean.

## Compatibility
`FailingTest.Timeout` uses `omitempty` so non-timeout JSON rows and the 8-column text output stay unchanged. `runView` calls `runViewAttempt(ctx, id, 0)` so existing `gh run view` flags stay identical.

## Limitations
Timeout detection reads failed-log text (`panic: test timed out after ...`) rather than `go test -json` streams. `MainRunsBefore` fetches earlier tries only when `Attempt > 1`.

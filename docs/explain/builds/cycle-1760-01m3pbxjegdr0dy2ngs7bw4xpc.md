# Build Explanation — Cycle 1760

## Build Binding
- Cycle: 1760
- Base SHA: b401e73d542cbdfd639bf3f44940ed8e9fae3160

## Summary
Shrank the five live size-ratchet offender functions — `releasepreflight.resolve`,
`releasepreflight.checkRecentAudit`, `releaseconsistency.Run`,
`releasetargets.ParseConfig`, and `versionbump.Run` — to at or below the
50-line ratchet limit by extracting named steps at one level of abstraction.
Behavior is unchanged; `offenders.json` was left untouched.

## Rationale
Each offender was one long procedure mixing several concerns (defaulting,
candidate selection, verdict-scoping policy, logging, per-check dispatch,
marker resolution). Extracting each concern into its own small,
descriptively-named function is the minimal change that satisfies the
ratchet without altering control flow, error wording, or return values —
no new abstraction layer, interface, or generic framework was introduced.
Where a function carried a long explanatory comment about non-obvious
policy (e.g. the cycle-1571 H4 scoped-audit rationale), the comment now
sits as a floating comment directly above the extracted function (separated
from the `func` line by a blank line, so it is not a Go doc comment), which
also keeps it outside the ratchet's line count (the scanner measures from
the `func` keyword to the closing brace, not any comment above it).

## Changed Areas
- `go/internal/releasepreflight/preflight_run.go` — `resolve` (54→25 lines,
  measured by `sizeratchet.Walk`) now delegates default-filling to a new
  `(*resolved).applyDefaults` method; behavior identical, same seams, same
  defaulting order.
- `go/internal/releasepreflight/releasepreflight.go` — `checkRecentAudit`
  (114→45 lines) now delegates to three new helpers: `selectAuditCandidate`
  (candidate/phantom-count selection), `scopedOutOrFail` (the scoped-out vs.
  hard-fail policy; the cycle-1571 H4 comment is kept verbatim as a floating
  comment directly above scopedOutOrFail, not a doc comment), and
  `auditAge` (the ledger-entry age check). Every branch, error string, and
  field assignment is preserved exactly.
- `go/internal/releaseconsistency/releaseconsistency.go` — `Run` (97→38
  lines) now delegates to `logChecklistHeader` (banner logging),
  `buildChecks` (the per-marker check table, now typed as `checkSpec`), and
  `runChecks` (dispatch + per-status logging + error counting). Same output
  ordering and same check set.
- `go/internal/releasetargets/releasetargets.go` — `ParseConfig` (65→32
  lines) now delegates to `resolveChecksumsName` (checksum-name default),
  `collectBuildTargets` (build-target dedup/parse), `appendUniversalDarwin`
  (universal-binary macOS target logic), and a small `hasTarget` predicate
  replacing the inline `seen` map lookup for the universal-binary path.
- `go/internal/versionbump/versionbump.go` — `Run` (52→39 lines) now
  delegates the Codex-mirror bump (stat-tolerant, error-on-real-stat-failure)
  to a new `bumpCodexPlugin` helper returning the modified-label string.

## Design Decisions
No new exported API surface, no new dependencies, no new types beyond one
package-private `checkSpec` struct (replacing an anonymous struct literal
type in `releaseconsistency`, needed because the check table is now built in
a separate function). All new helpers are unexported and package-private;
none is a "new exported identifier" requiring the caller-proof floor. Every
extracted helper's call site is the same original call site — the function
that always drove it — so there is no new production entry point to wire.

## Verification
- `cd go && go test -count=1 ./internal/releasepreflight/... ./internal/releaseconsistency/... ./internal/releasetargets/... ./internal/versionbump/...` — all four packages pass unmodified (their `_test.go` files were not touched).
- `cd go && go vet ./internal/releasepreflight/... ./internal/releaseconsistency/... ./internal/releasetargets/... ./internal/versionbump/...` — clean.
- `gofmt -l` over the four packages — clean.
- `cd go && go test -tags acs -count=1 ./acs/cycle1760` — 9/9 PASS (all five functions now measure ≤50 lines via `sizeratchet.Walk`; `offenders.json` byte-unchanged; module-wide `sizeratchet.Check` reports zero problems; no comment lines added or lost per `commentaudit`; no protected surface touched).
- `./go/bin/evolve acs suite --cycle 1760` — `verdict=PASS green=176 red=0 skip=53 total=229`.

## Compatibility
All five functions keep their original signatures, exported names, and
observable behavior (return values, log output, error wording). No public
API changed.

## Limitations
This is a pure hygiene/refactor lane: it does not change `offenders.json`
allowances (a boundary tighten is deliberately deferred to a later cycle,
per the 2026-09-28 ceiling-only rule) and does not add new tests beyond the
harness-owned `go/acs/cycle1760` predicates, since the existing suites for
all four packages already characterize the shrunk functions' behavior.

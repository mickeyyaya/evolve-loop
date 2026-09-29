# Build Explanation — Cycle 1751

## Build Binding
- Cycle: 1751
- Base SHA: 7d7d8655355b558dd79d55f84a4fc06fb88e4f31

## Summary
Three `cmd/evolve` functions that exceeded the size-ratchet ceiling
(`parseLoopArgs` at 146 lines, `defaultMatrixDeps` at 64, `detectQuotaPause`
at 52) are shrunk to ≤50 lines apiece via behavior-preserving extraction, and
their `go/internal/sizeratchet/offenders.json` entries are removed.

## Rationale
Each function mixed several independent concerns in one body (flag
registration + resolution + config assembly; four unrelated dependency
closures; checkpoint-file loading + field extraction). Splitting each concern
into its own private function is the smallest change that satisfies the
ratchet without altering observable behavior — no new exported surface, no
new package, no control-flow change beyond moving code across function
boundaries.

## Changed Areas
- `go/cmd/evolve/cmd_loop_args.go` — `parseLoopArgs` (146→~33 lines) is split
  into `registerLoopArgFlags` (flag registration onto a new `loopArgFlags`
  struct), `registerPerAgentOverrides` (the `--cli`/`--model` repeatable
  flags), `absOrWarn` (was an inline closure), `resolveCycleCount`,
  `resolveStrategy`, `resolveGoal`, and `buildLoopConfig` (the final
  `loopConfig{}` literal). `parseLoopArgs` itself now only wires these calls
  together in the original order; every precedence rule and warning message
  is preserved verbatim in its new home.
- `go/cmd/evolve/cmd_release_verify_clis.go` — `defaultMatrixDeps` (64→9
  lines) is split into four package-level functions —
  `installClaudeDep`, `projectTargetDryRunDep`, `assertGeminiPayloadPresentDep`,
  `binaryAnswersSubcommandDep` — one per `matrixDeps` field; the field wiring
  in `defaultMatrixDeps` is now a plain struct literal referencing them.
- `go/cmd/evolve/cmd_loop_control.go` — `detectQuotaPause` (52→7 lines) is
  split into `loadActiveQuotaLikelyCheckpoint` (file read, JSON unmarshal,
  checkpoint-map lookup, `enabled`/`reason` gate) and
  `quotaPauseFromCheckpoint` (field-by-field `quotaPause` construction,
  including the cycle_id/cycle fallback and the absent-vs-empty-source
  "unknown" normalization); `detectQuotaPause` now composes the two.
- `go/internal/sizeratchet/offenders.json` — removes the
  `cmd/evolve.parseLoopArgs`, `cmd/evolve.defaultMatrixDeps`, and
  `cmd/evolve.detectQuotaPause` entries (all three functions are now within
  the ratchet's allowance; no other entry is touched).
- `go/acs/cycle1751/predicates_test.go`, `.evolve/evals/shrink-*.md` — TDD
  phase's pre-existing test contract and eval score-cap files (not authored
  by this build; carried into this diff's commit since they define the
  acceptance the build satisfies).

## Design Decisions
- Extracted helpers are unexported, package-private, and called only from
  the shrunk function they were pulled out of — no new exported identifier,
  CLI flag, or gate is introduced, so no caller-proof/reachability obligation
  applies (per this cycle's TDD house rules).
- Per this cycle's explicit house rule, extracted helpers carry no
  descriptive comments — only names — even where the original code had
  WHY-comments (e.g. the quota-pause "absent vs empty source" note, the
  binAnswers exit-code-vs-stderr-signal note). The behavior those comments
  described is unchanged; only the comment text was dropped, per the
  builder's task-specific house rules for this cycle.
- `loopArgFlags` is a plain data struct (not a richer type) — the minimal
  vehicle needed to pass ~17 flag destinations between
  `registerLoopArgFlags` and the resolution/assembly helpers without
  reintroducing a long parameter list.

## Verification
- `go build ./...` and `go vet ./cmd/evolve/...` — clean (`gofmt -l .` also
  reports nothing to format).
- `go test -count=1 ./cmd/evolve/...` — full package (and `cmdutil`) passes,
  including all pinned `TestParseLoopArgs_*` (19), `TestVerifyReleaseCLIMatrix_*`
  (4), and `TestDetectQuotaPause_*` (5) subtests.
- `go test -tags acs -count=1 -v ./acs/cycle1751/...` — 10/10 predicates
  PASS (line-count ceilings, offenders.json membership, named subtest
  families, build/vet).
- `evolve acs suite --cycle 1751` — `verdict=PASS green=176 red=0 skip=54
  total=230`.

## Compatibility
No public API, CLI flag, config schema, or observable runtime behavior
changes. `evolve loop`'s argument precedence (`--goal-hash` > `--goal-text` >
positional goal; `--cycles`/`--max-cycles` > positional cycles; `--strategy`
> positional strategy), the release-verify-clis matrix's install/projection/
gemini-layout/binary checks, and quota-pause detection's field semantics are
all bit-for-bit preserved.

## Limitations
This build only reshapes existing logic to fit the size ratchet; it does not
address any of scout's larger deferred offenders (`wireOrchestratorDeps`,
`runBridge`) or the pipeline-integrity carryoverTodos noted in
triage-report.md, which remain out of scope for this cycle per the triage
decision.

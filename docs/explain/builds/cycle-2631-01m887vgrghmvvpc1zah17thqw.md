# Build Explanation — Cycle 2631

## Build Binding
- Cycle: 2631
- Base SHA: e144bfba9d3fbb79d29d399556aec7b277e1c18f

## Summary
A temp shape registry (`tempShapes`) in `go/internal/gc/tempdir.go` replaces the hardcoded `pipelineTempRe` regexp. `UnregisteredTempPatterns` parses Go source files and rejects any `os.MkdirTemp` call whose leading string literal has no registered shape. `classifyTempEntry` now checks candidate directories for a live unix socket before removing entries past the age horizon.

## Rationale
`pipelineTempRe` missed newer pipeline temp directories such as `evcodexhome`, `evcodeusage`, `evtmux`, and `buildfloor-cover-*`. Directory liveness also relied only on `lsof` open-path matching (`LiveHolderProbe`). Replacing the regexp with a shape registry and a static AST guard keeps every `os.MkdirTemp` caller registered. Probing unix sockets with `net.DialTimeout` keeps directories with live listeners even when `lsof` reports no holder.

## Changed Areas
- `go/internal/gc/tempdir.go` — replaces `pipelineTempRe` with `tempShapes`, exports `UnregisteredTempPatterns`, and adds `hasLiveSocket` to `classifyTempEntry`. Candidate directories with a live unix socket listener stay, while stale socket files go.
- `go/cmd/evolve/cmd_integration_l2.go` — renames the L2 checkout scratch prefix from `"tip-"` to `"evolve-l2-tip-"`. This keeps the scratch prefix inside the registered `evolve-` shape.
- `go/cmd/evolve/cmd_integration_regen.go` — renames the derived-regen scratch prefix from `"regen-"` to `"evolve-regen-"` so it matches the registered `evolve-` shape.
- `go/cmd/evolve/cmd_models_live.go` — renames the model-classifier scratch prefix from `"probe-"` to `"evolve-modelprobe-"` so it matches the registered `evolve-` shape.

## Design Decisions
- `tempShapes` is an internal slice of `{id, creator, pattern}` entries compiled at startup. `pipelineTempRe` stays as a zero-byte `tempShapeMatcher` delegating to `matchTempShape`, so frozen cycle-2607 tests in `go/internal/gc/openpaths_test.go` compile unchanged.
- `UnregisteredTempPatterns` resolves `"os"` imports, inspects `os.MkdirTemp` pattern arguments with `leadingStringLiteral`, and tests synthetic names against `matchTempShape`.
- Three private-directory `os.MkdirTemp` callers (`cmd_integration_l2.go`, `cmd_integration_regen.go`, `cmd_models_live.go`) moved to `evolve-*` prefixes instead of adding generic prefixes (`tip-`, `regen-`, `probe-`) to the shared temp reaper.
- `hasLiveSocket` walks candidate directories only after the age cutoff and `heldFn` checks. It dials only `fs.ModeSocket` entries with a 200ms `unix` timeout.

## Verification
Targeted `go/internal/gc` tests, `go/cmd/evolve` tests, cycle 2631 ACS predicates (`8/8 PASS`), and cycle 2607 ACS predicates pass against the updated files. `evolve docs ste-lint --strict` checks documentation compliance.

## Compatibility
`ReapPipelineTemp`, `TempReapReport`, `TempOption`, and `SparingHeld` preserve their signatures and behavior for existing callers. Existing `tempdir_test.go`, `tempdir_suite_dirs_repro_test.go`, and `openpaths_test.go` remain unmodified as required by cycle 2607.

## Limitations
`UnregisteredTempPatterns` inspects only `os.MkdirTemp` calls whose pattern argument starts with a string literal. Dynamic patterns passed with variables or `fmt.Sprintf` sit outside the static AST guard.

# Comment history: `internal/consensusdispatch`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d3

### `go/internal/consensusdispatch/consensusdispatch.go:1` — above `package consensusdispatch`

```text
// Package consensusdispatch ports legacy/scripts/dispatch/consensus-dispatch.sh.
//
// Cross-CLI consensus auditor dispatch (v8.54.0+). Reads a profile's
// consensus block and dispatches N parallel audit invocations, each under
// a DIFFERENT CLI. Aggregates results via aggregator.sh's cross-cli-vote
// merge mode (MAJORITY-PASS with FAIL-VETO).
//
// Until Phase 3b lands fanout-dispatch + aggregator + capability-check in Go,
// this package handles the deterministic prep (env validation, profile
// parsing, voter filtering, quorum adjustment, TSV build) and shells out to
// the bash dependents for the orchestration steps.
```

### `go/internal/consensusdispatch/extra_coverage_test.go:163` — above `func TestResolveNativeDispatch_RequiresNativeBin(t *testing.T) {`

```text
// TestResolveNativeDispatch_RequiresNativeBin covers the v12 contract: with no
// native binary resolvable, dispatch errors — the legacy bash fallback
// (<dir>/<subcmd>.sh) was removed with legacy/scripts/dispatch/ (ADR-0062/T1.7),
// so there is nothing to fall back to.
```

### `go/internal/consensusdispatch/resolve.go:13` — above `func resolveNativeDispatch(dispatchDir, subcmd string, subArgs []string) (*exec.Cmd, error) {`

```text
// resolveNativeDispatch returns an exec.Cmd that runs `evolve <subcmd> <args...>`
// via the native binary.
//
// Resolution order:
//
//  1. <EVOLVE_GO_BIN> if set + executable
//  2. <repo>/go/bin/evolve relative to dispatchDir's ancestor
//  3. `evolve` on PATH
//
// The legacy bash fallback (bash <dispatchDir>/<subcmd>.sh) was removed in
// v12.0.0 together with legacy/scripts/dispatch/ (ADR-0062/T1.7): there is no
// script to fall back to, so a missing native binary is a hard error rather than
// a doomed `bash <deleted>.sh` invocation.
```

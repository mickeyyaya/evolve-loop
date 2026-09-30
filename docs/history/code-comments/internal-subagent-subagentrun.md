# Comment history: `internal/subagent/subagentrun`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/subagent/subagentrun/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under internal/subagent
// (ADR-0103 unit 16 §2): stdlib plus the three named internal packages, never
// internal/subagent, internal/core, internal/bridge, capability or resolvellm
// (the compiler is the cycle guard; this is the leaf-ness declaration —
// signalcenter/importgraph_test.go idiom).
```

### `go/internal/subagent/subagentrun/ledger.go:155` — above `func QualityTier(budgetNative, permissionScoping bool) string {`

```text
// QualityTier maps the capability manifest's support flags to the v8.51.0
// quality_tier label ledger entries carry. full = both supports true;
// degraded = both false; hybrid = one of each.
```

### `go/internal/subagent/subagentrun/ledger_test.go:14` — above `func fixedNowFn() func() time.Time {`

```text
// ledger_test.go — the host's ten direct ledger-writer tests moved (ADR-0103
// unit 16 D5) with their callee adapted from writeSubprocessLedger(path, e,
// now) error to the dispatcher-owned appendLedger(path, e) (op, err): the
// clock is the dispatcher's, the op is asserted.
```

### `go/internal/subagent/subagentrun/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 16 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/subagent/subagentrun/subagentrun.go:1` — above `package subagentrun`

```text
// Package subagentrun is unit 16 of the component breakdown (ADR-0103): the
// `evolve subagent run` execution path. One Dispatcher owns request admission,
// cli/tier/capability resolution, artifact placement, the challenge token and
// git provenance, prompt composition and staging, the bridge exec port,
// artifact verification (the one verdict ladder every dispatch path shares)
// and the agent_subprocess ledger record. Its collaborators are explicit at
// construction: the ten host-backed ports in Deps (the profile grammar, the
// LLM router, the capability inspector, the tier resolver, the role
// allow-list, the recursion cap, the driver check, the bridge adapter, git and
// the run-id resolver) and six stdlib-backed options (clock, entropy, the
// filesystem trio, the prompt stager, the ledger opener, the Signal Center
// accessor). The leaf never writes stderr, never reads the environment, never
// shells git or tmux, and reports its eleven failure modes as bridge.warning
// under module bridge with the BRIDGE_SUBAGENT_ sub-prefix. Design:
// docs/architecture/decomposition/16-subagentrun.md.
```

### `go/internal/subagent/subagentrun/verify_test.go:14` — above `var verifyNow = time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)`

```text
// verify_test.go — the host's contract_test.go moved (ADR-0103 unit 16 D5):
// the same cases over cyclestate.Diagnostic (the type core.Diagnostic
// aliases), with the typed rung asserted per case; the read-error case
// provokes the read seam instead of a chmod (no root skip).
```

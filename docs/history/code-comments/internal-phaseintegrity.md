# Comment history: `internal/phaseintegrity`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d5

### `go/internal/phaseintegrity/apicover_named_test.go:37` — above `func TestNamePublicAPI_RepinIfDrifted(t *testing.T) {`

```text
// TestNamePublicAPI_RepinIfDrifted names the shared detect-drift-and-provenance-
// gated-repin primitive (cycle 636) for the apicover public-API gate; behavior is
// exercised in repin_ifdrifted_test.go. RepinIfDrifted is the single path invoked
// by BOTH boot recovery and the post-build repin, so the two never diverge.
```

### `go/internal/phaseintegrity/repin.go:25` — above `func RepinShipSHA(statePath, runningSHA, runningCommit, pluginVer string, prov ProvenanceVerified, operatorAuthorized bo…`

```text
// RepinShipSHA re-pins state.json:expected_ship_sha to the running binary —
// the sanctioned successor to hand-editing the file (ADR-0065). It re-pins
// ONLY when the binary is trustworthy:
//
//   - provenance: runningCommit is non-empty and prov(runningCommit) is true
//     (a legitimate rebuild from committed source), OR
//   - operatorAuthorized: the operator explicitly passed --reset-sha.
//
// Otherwise it REFUSES (Repinned=false + an actionable error), preserving the
// anti-tamper guarantee. The write is atomic (temp+rename) under the shared
// state.json sidecar lock, so it is safe against concurrent resume/fleet
// writers and never clobbers an unrelated state key.
```

### `go/internal/phaseintegrity/repin.go:66` — above `res := RepinResult{NewSHA: runningSHA, Authorized: authorized}`

```text
// Route the read-modify-write through the single-source statemap adapter
// (cycle-659): it holds the shared "<state.json>.lock" sidecar across the
// whole RMW, so this is safe against concurrent resume/fleet writers and
// preserves every unmodelled key. A malformed file aborts before the write.
```

### `go/internal/phaseintegrity/repin_ifdrifted_test.go:3` — above `import (`

```text
// repin_ifdrifted_test.go — RED tests (cycle 636, task ship-sha-repin-after-build).
//
// Context: SELF_SHA_TAMPERED denied the terminal ship gate on 8 consecutive
// cycles (625->634). The pin was FROZEN (byte-identical expected_ship_sha vs
// on-disk go/bin/evolve every time, both WITHIN plugin version 22.0.1). Root
// cause: the cycle-514 provenance-gated auto-repin runs ONLY at boot; a
// legitimate within-version rebuild that replaces go/bin/evolve without
// repinning leaves every subsequent cycle inheriting a doomed pin.
//
// The fix "extends/duplicates the cycle-514 boot healer's repin path" so it also
// fires after a successful build. To avoid duplicating the detect+provenance-gate+
// repin logic across the boot path (cmd/evolve) and the new post-build path
// (internal/core) — the "never duplicate, centralize" invariant — the Builder
// factors the shared logic into ONE new primitive here:
//
//	// RepinIfDrifted re-pins state.json:expected_ship_sha to the on-disk binary at
//	// binPath WHEN (and only when) its sha256 has drifted from the pin AND the
//	// running binary's build-commit is provenance-verified (the same gate the boot
//	// healer uses via RepinShipSHA). The single shared repin path invoked BOTH at
//	// boot recovery AND immediately after a successful build phase.
//	//   - no pin / binary absent / sha == pin  -> RepinResult{Repinned:false}, nil (no write, prov NOT consulted)
//	//   - drift + provenance-verified           -> RepinShipSHA fires -> RepinResult{Repinned:true, NewSHA: <sha>}
//	//   - drift + provenance-UNVERIFIED          -> RepinResult{Repinned:false} + error; pin left UNTOUCHED
//	func RepinIfDrifted(statePath, binPath, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error)
//
// RED now: RepinIfDrifted is undefined -> package phaseintegrity test build fails.
// Do NOT modify this file — implement the primitive so both repin paths share it.
```

### `go/internal/phaseintegrity/source.go:1` — above `package phaseintegrity`

```text
// Package phaseintegrity provides the production DigestSource for the per-phase
// integrity chain (ADR-0065): it reads the running binary, the binary's
// build-commit, the phase's agent profile/prompt, the phase's report artifact,
// and the worktree tree, and feeds them to phaseblock.Compute. It is the
// non-leaf glue (it shells git for the tree sha), kept separate from the
// git-free phaseblock leaf.
```

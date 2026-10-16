# Comment history: `internal/phases/buildplanner`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/phases/buildplanner/buildplanner.go:1` — above `package buildplanner`

```text
// Package buildplanner implements the optional build-planner phase (Opt C).
// The phase externalises Builder's Step 3 (chain-of-thought design) into a
// separate Opus session so the Builder can spend its 25-turn budget on
// implementation rather than planning.
//
// Rollout (3 cycles):
//   - Cycle 1 (this): shadow mode — EVOLVE_BUILD_PLANNER=0 default; phase wired
//     but always skipped; Go constant, state-machine edges, and profile exist.
//   - Cycle 2: advisory — flip default to 1; build-plan.md produced and read
//     by Builder's Step 2.8.
//   - Cycle 3: enforce — Builder's internal Step 3 removed; Auditor checks
//     build-plan.md adherence.
//
// Shadow-mode invariant: ShouldSkip returns (true, SKIPPED, "build", nil)
// whenever EVOLVE_BUILD_PLANNER != "1", so the bridge is never called and
// Bridge / Prompts may be nil in Config.
```

### `go/internal/phases/buildplanner/buildplanner.go:34` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (one verifier for gate and engine, research F22).
```

### `go/internal/phases/buildplanner/buildplanner.go:58` — above `return runner.New(runner.Options{Hooks: p, Bridge: p.bridge, Prompts: p.prompts, Optional: true, ContractVerifier: p.con…`

```text
// Optional: build-planner is advisory (opt-in via EVOLVE_BUILD_PLANNER). If
// its artifact never appears (ExitArtifactTimeout — e.g. cycle-120 quota
// exhaustion), it degrades to WARN and the cycle advances to build instead
// of aborting. See Workstream D.
```

### `go/internal/phases/buildplanner/buildplanner.go:102` — above `func (p *Phase) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string…`

```text
// Classify implements runner.Hooks. Verifies the required section headings
// in build-plan.md are present. Cycle 1 (advisory): a missing heading is
// WARN rather than FAIL; enforcement lands in Cycle 3.
```

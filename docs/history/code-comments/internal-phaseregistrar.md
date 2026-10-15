# Comment history: `internal/phaseregistrar`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/phaseregistrar/registrar.go:73` — above `if cfg.Description == "" && cfg.WhenToUse == "" {`

```text
// Mint-time SELECT-metadata default (docs/chronicle/2026-08-minted-stub-class.md).
// A minted phase is forced Optional just above, so a config carrying neither
// Description nor WhenToUse persists a phase.json that trips phasespec's
// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata the moment ship's
// whole-tree bind sweeps the stub into git tracking — a repo-wide, CI-only
// failure invisible to per-cycle changed-scope, hand-fixed four times (#399,
// #404, #406, #407). Defaulting here, at the one seam every mint funnels
// through, makes the artifact contract-safe by construction rather than by
// advisor diligence. The condition mirrors the guard's own AND over both
// fields: an advisor that supplied EITHER one keeps its text verbatim.
```

### `go/internal/phaseregistrar/registrar.go:88` — above `if cfg.Catalog == "" {`

```text
// Mint-time persona-resolvability default (2026-08-26, instance #5 of the
// persona-less-phase class — the cycle-1567/1568 halt): the mint's OWN
// dispatch runs on the inline PromptBody, but the persisted phase.json
// outlives the process, and a stub whose derived persona
// (agents/evolve-<name>.md) exists nowhere both reds the tracked-menu
// guard the moment ship's bind sweeps it into tracking (the lane-only
// internal/core regression) and schedules a next-cycle dispatch that dies
// at load-agent. Stamp catalog:"on-demand" at the same construction seam
// as the SELECT-metadata default above — off the menu until someone writes
// the persona, exactly the tracked-phase remedy (#493). A mint whose
// persona DOES resolve, or that carries an explicit catalog word, keeps it
// verbatim.
```

### `go/internal/phaseregistrar/registrar.go:116` — above `if cfg.Dispatch.CLI != "" {`

```text
// Bare CLI family name resolution (cycle 1325, mint-profile-driver-suffix):
// bridge.DriverFor already projects a bare family name ("claude") onto its
// concrete driver ("claude-tmux") for every other dispatch call site
// (subagent, consensusdispatch) — Register was the one mint choke point
// that skipped it, so an advisor-minted bare name reached disk unresolved
// and preflight's exact-match LookupDriver rejected it hours later at the
// next loop launch (the batch-15 incident this closes). Resolved BEFORE
// ToProfile() so the persisted profile carries the resolved name, and
// rejected outright (not silently persisted) when the resolved name is
// not itself a registered driver — an unresolvable family name must fail
// mint loudly, never at the next launch's preflight halt.
```

### `go/internal/phaseregistrar/registrar.go:144` — above `if r.ProfilesDir != "" && cfg.Dispatch.CLI == "" {`

```text
// A PERSISTED profile with no driver is the fourth instance of the same
// class (#406): once tracked it fails the repo-contract suites
// (profiles.TestSmoke_RealProfiles, TestRepoPersonaProfilePairing) and, at
// dispatch preflight, resolves to no CLI at all. "No known driver" is not a
// coherence question the auditor can weigh — it is a hard launch failure
// discovered hours later, so reject the mint instead of writing the stub.
//
// Scoped to ProfilesDir != "" deliberately: with persistence off, Register
// is a pure factory that cannot leak an artifact into the tree, and an
// absent dispatch block is the legitimate, checked-in shape of every
// .evolve/phases/*/phase.json — which the campaign study path registers
// persistence-free (cmd/evolve/cmd_campaign.go:403). Guarding the artifact
// where the artifact is created keeps that path working.
```

### `go/internal/phaseregistrar/registrar.go:175` — above `if r.RegistryPath != "" {`

```text
// Register-before-persist (cycle-967 Variant A2): the minted name must be
// discoverable by every lane's tree-diff guard BEFORE its files can appear
// in a shared-tree diff — the reverse order recreates the cross-lane
// false-abort race. An unregistered mint is an abort landmine for every
// concurrent lane, so a registry failure rejects the mint loudly.
```

### `go/internal/phaseregistrar/registrar.go:230` — above `const (`

```text
// mintedStubDescriptionFmt / mintedStubWhenToUse are the honest reserved-intent
// SELECT metadata a runtime-minted phase carries when the advisor supplied
// none. They follow the #404 precedent — state what the phase actually is and
// steer the router AWAY from selecting an undefined stub — rather than padding
// phasespec's metadataAllowlist, whose stated remedy is "add metadata, do not
// pad the allowlist" and which is only allowed to shrink.
```

### `go/internal/phaseregistrar/registrar_clamp_binding_test.go:3` — above `import (`

```text
// registrar_clamp_binding_test.go — cross-binding pin for the two enforcement
// styles of `writes_source ⟹ sandboxed dispatch profile` (ADR-0073): the
// registrar enforces it by CONSTRUCTION (Register forces Sandbox.Enabled into
// the profile it persists), the discovery clamp by VERIFICATION
// (phasespec.SandboxedProfilePredicate over the same on-disk profile). This
// test drives a real writes_source mint through Register and asserts the
// persisted profile passes the discovery-side predicate — so the two paths
// can never drift into a state where a legitimately-minted writer is
// stripped (or a strip-evading profile shape passes).
```

### `go/internal/phaseregistrar/registrar_mint_ondemand_test.go:3` — above `import (`

```text
// registrar_mint_ondemand_test.go — instance #5 of the persona-less-phase
// class (cycle-1567/1568 halt): a minted phase.json persisted WITHOUT a
// resolvable persona and without catalog:"on-demand" reds the tracked-menu
// guard in every lane worktree that stages it, and its next-cycle scheduled
// dispatch dies at load-agent. The registrar now stamps on-demand at mint
// time — the #493 architect's deferred suggestion (4b), landed after the
// prediction came true.
```

### `go/internal/phaseregistrar/registrar_test.go:299` — above `func TestRegister_AppendsActiveMintRegistry(t *testing.T) {`

```text
// --- cycle-967 (Variant A2): Register records the mint in the shared registry ---
//
// The tree-diff guard exempts a leaked .evolve/phases/<name> path only for a
// REGISTERED mint (core.isActiveMintPhasePath), so Register must append the
// name to the mintregistry — and must do so BEFORE persisting the files: the
// reverse order recreates the cycle-967 race (files visible to a concurrent
// lane's guard before the exemption exists).
```

### `go/internal/phaseregistrar/registrar_test.go:358` — above `func repoRootDir(t *testing.T) string {`

```text
// --- Mint-time catalog-contract safety (docs/chronicle/2026-08-minted-stub-class.md) ---
//
// Register persists a phase.json + profile stub built from the advisor-supplied
// config. Empty Description/WhenToUse mints a stub that fails the repo-wide
// phasespec.TestPhaseCatalog_OptionalPhasesHaveSelectMetadata once ship's
// whole-tree bind tracks it; an empty dispatch.cli mints a driverless profile
// that fails the profiles repo-contract suites and dies at dispatch preflight.
// Four instances (#399, #404, #406, #407) were hand-fixed before the class was
// closed at this seam; these are its permanent regressions.
```

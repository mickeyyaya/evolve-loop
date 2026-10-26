# internal/preflight

## Purpose

`internal/preflight` is the single capability-detection probe. It is a Go port of `preflight-environment.sh`. It probes the host once when the dispatcher starts. It emits a JSON capability profile (schema version 3). The dispatcher reads the profile to configure the settings that it can relax by itself.

## Design

- **Discover, Decide, Log, Verify.** This package does Discover, Decide and Log. Verify happens elsewhere (phase-gate, ledger-SHA, role-gate, ship-gate).
- **One decision with the bridge.** The `sandbox` package is the single source of truth for nested-Claude detection and for the inner-sandbox wrap policy. The bridge launch path shares it. Host readiness and launch thus use the same measured wrapping decision. Session markers describe context, not sandbox capability.
- **Import alias.** This package imports `sandbox` as `sbx`, because a local variable in `Probe` is named `sandbox`.
- **Measured capability is an opt-in seam.** `Options.SandboxCapable` injects the measured sandbox-apply capability (capable, checked). It replaces the nested env-var guess with ground truth. Production wires it to `sandbox.Probe()` at the composition root (`MeasuredSandboxCapability`). A nil seam leaves the result unmeasured and keeps the legacy unmeasured decision.
- **Worktree base order.** `selectWorktreeBase` takes the first writable choice, in this order:
  - the operator `worktree.base` from `policy.json`;
  - the in-project location, when the shell is standalone (easy to inspect);
  - the TMPDIR location, the sandbox-friendly default for nested Claude;
  - the user cache dir;
  - the in-project location as the last resort. Isolation is degraded if a parent sandbox blocks at exec time.
- **An operator base that is not writable falls through.** The bash port does the same. The reason string of the chosen base tells the operator why.
- **`Options.WorktreeBase` replaced the `EVOLVE_WORKTREE_BASE` env read.** This was flag reduction ([ADR-0064](../adr/0064-pipeline-integrity-boundary.md)).
- **The profile names no dead flag.** X18e deleted the `auto_config` key `EVOLVE_SANDBOX_FALLBACK_ON_EPERM`, because no code read it. A reader of `preflight-environment --json` gets no value for that key now. The schema version stays 3, because no reader in the repository used the key.
- **`hostOSType` folds `runtime.GOOS`.** It keeps `darwin` and `linux` and gives `other` for the rest. A test checks each branch on any host.
- **Seams.** `Options` exposes `Env`, `IsNested`, `OSType`, `SandboxCapable` and `WorktreeBase`, so tests control the host.

## Invariants

- **`WriteToFile` is best-effort.** It writes the profile to `<projectRoot>/.evolve/environment.json` by atomic rename. A silent failure is acceptable. The file is a cache and not a source of truth, as in the bash version.
- **`probeWritable` mirrors the bash probe.** It makes the directory, touches a sentinel file and removes the sentinel.
- **No writable base is reported and not hidden.** When every choice fails, `Probe` returns an empty base. Its reasoning string names the operator action: set `worktree.base` in `.evolve/policy.json`.

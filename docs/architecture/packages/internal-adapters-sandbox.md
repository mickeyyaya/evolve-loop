# internal/adapters/sandbox

> **Isolation ([ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md)).** The sandbox holds the read grants, the exec deny and the write fence of the agent cell (I-3, I-5, I-7). The spec of record: [llm-isolation.md](../llm-isolation.md).

## Purpose

`internal/adapters/sandbox` ports the OS-level sandboxing layer. It has two halves. The generators build the macOS `sandbox-exec` profile (SBPL) and the Linux `bwrap` argv from one `Config`. The probe measures if the host's sandbox binary actually applies. `confinement.go` is the single source of the confinement decision that the bridge launch path and the host preflight both consume. The package is ported from `scripts/cli_adapters/claude.sh`.

## Design

- `GenerateSBPL`, `GenerateBwrapArgv` and `BwrapPrefix` are pure functions, so unit tests run on any host. `BwrapPrefix` serves a caller that composes the inner argv later. `GenerateBwrapArgv` is the backward-compatible form.
- `Probe` checks the host for the binary and measures if it applies. It is cached once per process. The bridge's `ShouldWrap` and preflight therefore share one measured value, and the capability exec runs at most once.
- `Sandbox` is the runtime-dispatching `Exec` wrapper. It degrades to a plain exec with a stderr NOTE when the host has no sandbox binary.
- `DetectNested` and `ConfinementSatisfied` are the two halves of the confinement decision. `ShouldWrap` decides if to try the wrapper.
- `getenv` is injected. The detection is tested with a map, and it reads the bridge's request-local env chain in production.

## Invariants

- A nested-session hint is not a measured capability. It is not proof of outer confinement. `DetectNested` reports only a possible nested session. `CLAUDECODE_TYPE=host` marks the top-level host process. That process is not nested under another sandbox, so it can confine its children.
- `ShouldWrap`: a measured capability outranks session hints. An unchecked nested hint stays refused. Success here does not authorize an unwrapped launch, and it does not attest an outer sandbox.
- `ConfinementSatisfied` is fail-closed. Mode `off` is an explicit host opt-out and is never verified confinement. Any other mode is unsatisfied when the wrap did not apply, including unverified nested environments.
- A sampled parent-directory write denial cannot prove the profile's read and write policy. The optional preflight canary therefore does not waive the gate.
- The capability is measured because the env-var guess was wrong. In some nested macOS environments `sandbox_apply()` returns EPERM despite an available binary. The bridge had gated on the binary alone. `EVOLVE_SANDBOX=on` still wrapped under nested macOS and hung the claude REPL boot (exit 80).
- `CapabilityChecked` separates "measured incapable" from "never measured". An unmeasured caller keeps the legacy heuristic, so it behaves as it did before measurement existed.
- The capability exec has a timeout. A trivial confined `/usr/bin/true` returns quickly when the sandbox applies, and it fails fast when it cannot. The timeout keeps a pathological host from reintroducing the 80 s REPL-boot hang. A timeout means not capable. `WaitDelay` bounds the drain, so `Run` returns even when a grandchild holds the pipes.
- SBPL rule order matters, because a later rule overrides an earlier one for overlapping subpaths. The order follows bash. A deny on the repo, followed by per-path write allows, yields a read-only repo with cycle-specific write zones.
- In the bwrap prefix, a broad scratch grant such as `/tmp` must precede a read-only repository beneath it. Otherwise the later parent grant reopens the whole repo.
- Overlays on writable binds use `--ro-bind`, not `--ro-bind-try`. `--ro-bind-try` silently discards a mandatory restriction. Other CLI state dirs are re-bound with `--bind-try`, because not every host has every CLI directory. REPL boot can update auth, cache and config under HOME.
- Network isolation comes from bwrap's namespace, not from allow and deny rules.

## Findings

- The pure generators cannot show that a profile applies on a given host. Only the measured probe can.

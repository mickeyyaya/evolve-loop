# internal/adapters/sandbox

> **Isolation ([ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md)).** The sandbox holds the read grants, the exec deny and the write fence of the agent cell (I-3, I-5, I-7). The spec of record: [llm-isolation.md](../llm-isolation.md).

## Purpose

`internal/adapters/sandbox` ports the OS-level sandboxing layer. It has two halves. The generators build the macOS `sandbox-exec` profile (SBPL) and the Linux `bwrap` argv from one `Config`. The probe measures if the host's sandbox binary actually applies. `confinement.go` is the single source of the confinement decision that the bridge launch path and the host preflight both consume. The package is ported from `scripts/cli_adapters/claude.sh`.

## Design

- `GenerateSBPL`, `GenerateBwrapArgv` and `BwrapPrefix` are pure functions, so unit tests run on any host. `BwrapPrefix` serves a caller that composes the inner argv later. `GenerateBwrapArgv` is the backward-compatible form.
- `Probe` checks the host for the binary and measures if it applies. It is cached once per process. The bridge's `ShouldWrap` and preflight therefore share one measured value, and the capability exec runs at most once.
- `Sandbox` is the runtime-dispatching `Exec` wrapper. It degrades to a plain exec with a stderr NOTE when the host has no sandbox binary.
- `DetectNested` and `ConfinementSatisfied` are the two halves of the confinement decision. `ShouldWrap` decides if to try the wrapper.
- `Config.ExecDenyPaths` and `Config.ExecDenyEnforce` hold the exec deny of I-3 ([llm-isolation.md](../llm-isolation.md) §7.1). At `enforce`, `writeExecDenyRules` denies the exec of each path and of `/usr/bin/sandbox-exec`. An exec by `sandbox-exec` itself stays allowed, so the launch of the cell goes through. When `ExecDenyEnforce` is false, the same rules allow the exec and report it.
- `Config.ExecDenyRoots` gets the same rules with `subpath`, so a deny covers each file under an install root. At `enforce`, `BwrapPrefix` ends with `--ro-bind /dev/null <path>` for each path and `--tmpfs <root>` for each root. The caller gives real paths, because an SBPL `literal` matches the real path of a file and not a symlink.
- `Config.ReadIsolation` holds the read rules of I-4 and I-5 ([llm-isolation.md](../llm-isolation.md) §8.5). `writeReadIsolation` writes them after the write denials and before `DenyReadPaths`, in this order:
  1. the guard of `RunsDir` and the guards of the home secrets;
  2. the grant of the own workspace and of each indexed path;
  3. the read and write guards of the raw output, and the write guards of `<ws>/inputs` and `PinnedFiles`.

   Each guard carries the message `evolve-cell=<tag> rule=<rule> inv=<I4|I5>`. At `enforce` a guard is a `deny`, and otherwise an `allow` with a report. A read guard uses the operation `file-read-data`, and each grant names `file-read*` and `file-read-data`. A grant of `file-read*` alone does not reopen a path that a `file-read-data` deny closed (measured on macOS, 2026-10-26).
- At `enforce`, `bwrapReadIsolation` mounts a `tmpfs` on `RunsDir` and binds the own workspace again. It binds each grant read-only (`--ro-bind-try`) and masks each raw file and secret file with `/dev/null`. It mounts a `tmpfs` on each raw and secret directory, and binds `<ws>/inputs` read-only. At `shadow` it adds nothing, because `bwrap` has no report.
- `Config.ProtectedWriteSubpaths`, `Config.ProtectedWritePrefixes` and `Config.ProtectedWriteEnforce` hold the protected write rule of I-7 ([llm-isolation.md](../llm-isolation.md) §9.1). At `enforce`, `writeProtectedWriteRules` denies each write with `subpath` or `prefix`, after every write grant. When `ProtectedWriteEnforce` is false, the same match allows the write and reports it. At `enforce`, `bwrapProtectedWriteMasks` binds each protected path that exists read-only, before the exec masks. A prefix gets a bind for each file in its directory that starts with it.
- The signal rule `(allow signal (target same-sandbox))` under `(deny default)`, and `--unshare-pid` for `bwrap`, keep a `kill(-1)` of a cell inside the cell. `TestSandboxContract_KillMinusOneFromACellReachesNoHostProcess` pins both and runs no signal.
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

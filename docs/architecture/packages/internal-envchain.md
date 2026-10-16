# internal/envchain

## Purpose

`internal/envchain` is the one place that answers "where does this configuration value come from?". The runner's permission resolution, the bridge policy resolution and every typed environment knob read through it, so the sites cannot drift apart. It also holds the registry of operational `EVOLVE_*` key names and their defaults (`keys.go`).

## Design

- **Four tiers, first non-empty wins.** The order is: the per-request env, the process env, the profile value and the package default. The per-request env (`reqEnv`) is set by the orchestrator on one dispatch. The process env is the shell of the operator. The profile is `.evolve/profiles/<phase>.json`. The pattern is Chain of Responsibility.
- **An empty string is absent.** An empty value in `reqEnv` falls through to the lower tiers. It never masks them. An operator who wants to clear a value pins it to the canonical empty value of the subsystem, never through the chain.
- **`ResolveNoOS` skips the process-env tier.** Use it for a key whose profile is the source of truth. A shell value must never override the profile.
- **`SystemPromptReqEnvKey` is such a key.** The global system-prompt override is looked up in `reqEnv` only. It is an in-process pass-through from the orchestrator, not a shell variable. The key is a concatenation, so the flagreaders AST guard does not count it as an `os.Getenv` reader. It carries the `SSOT IPC-protocol-allowed` waiver, which the envtaint gate reads.
- **`PhaseEnvKey` builds `EVOLVE_<PHASE>_<SUFFIX>`.** Hyphens in the phase name become underscores (`tdd-engineer` gives `EVOLVE_TDD_ENGINEER_MODEL`). One transform prevents drift between `PERMISSION_MODE`, `MODEL`, `PLAN_INPUT`, `PLAN_OUTPUT` and every later per-phase knob.
- **Typed getters read the env tiers only.** `Int`, `IntMin` and `Bool` consult `reqEnv` and then the process env. The profile and default tiers are unused, because the fallback of a typed knob is its typed default argument.
- **`Int` keeps the `strconv.Atoi` behaviour.** It does not trim whitespace, so a value such as `" 3"` is unparseable and gives the default. A call site that moves to `Int` keeps its behaviour.
- **`IntMin` treats a value below the minimum as invalid, not as clamped.** The result is the default, as the phase-latency ceilings behave. If a caller passes a default below the minimum, the helper returns it unchanged. It does not second-guess the caller.
- **`Bool` accepts `1 true yes on` and `0 false no off`,** case-insensitive. Anything else gives the default. For the `1` and `0` values that every older call site used, the result is the same as before.
- **`BoolValue` does no env lookup.** Use it for an already-resolved string, for example a frozen per-cycle env snapshot that must not fall back to the live process env.

## Invariants

- The precedence order never changes: `reqEnv`, process env, profile, default (`TestResolve_PrecedenceOrder`).
- `ResolveNoOS` never reads the process env, even when the profile and `reqEnv` are empty (`TestResolveNoOS_PrecedenceNoOSFallback`).
- `BoolValue` never reads the process env (`TestBoolValue_NoEnvLookup`).
- A request-env value beats a process-env value in every typed getter (`TestTyped_ReqEnvBeatsProcessEnv`).
- A value below the minimum of `IntMin` gives the default, not the minimum (`TestIntMin`).

## Findings

- The registry grows only when a typed getter reads a key. A one-off path override stays inline at its call site.

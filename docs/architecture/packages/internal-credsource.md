# internal/credsource

> Decision: [ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md). Spec of record: [llm-isolation.md](../llm-isolation.md) §7.1.2. Plan: architecture decisions 2026-11, decision D1b, component AD1-7.

## Purpose

`internal/credsource` gives the pipeline one interface to the credential store of the host. The pipeline is the only credential holder (decision D1). Only `git push` and `gh` of the ship path call the source. A phase cell never calls it and never gets a source variable.

## Design

- **The interface.** `Source` has `Name` and `Lookup(ctx, host)`. `Lookup` gives an `Answer`: the name of the store that answered, and the secret. A store with no item gives `ErrNoCredential`.
- **The selection.** `For(goos, deps)` reads a table that maps `runtime.GOOS` to a realization. There is no feature flag. An OS that is not in the table gives an error. `Default` uses `runtime.GOOS`, a real command runner and the passwd home of the uid.
- **macOS.** The realization runs `security find-generic-password -s evolve-loop -a <host> -w`.
- **Linux.** The realization runs `secret-tool lookup service evolve-loop host <host>`. When libsecret does not answer, it reads the file `~/.config/evolve-loop/credentials/<host>`. The file must be a regular file with mode 0600. The check runs before the read, and it refuses a symlink or a wider mode, so a FIFO or a link cannot stop the ship. `~` comes from the passwd entry of the uid, never from `HOME`.
- **Windows.** The realization runs `powershell.exe -NoProfile -NonInteractive -Command` with a script. The script calls `CredReadW` of `advapi32.dll` for the generic target `evolve-loop:<host>`. Windows has no built-in command that prints a stored secret, so the script is necessary.
- **The host check.** Each realization is in a guard. The guard accepts only a host name of letters, digits, dots and hyphens. Thus a host cannot change the arguments of a command or the text of the script.
- **The child variables.** `ChildVariables` lists the variables that carry a token to `gh`. The manifest parser of the bridge refuses each of them in `default_env` and `env_passthrough`. I-2 at `enforce` drops each of them from a cell environment. `TestCredentialSource_ACellEnvironmentHasNoSourceVariable` proves this for each family.
- **The stage.** This package knows no stage. The package `policy` owns the stage names, and `ship.CredentialReport` decides what each stage does.

## The stage

The policy key is `isolation.credential_source.stage`. The values are `off` and `shadow`, and a missing key is `off`. The checked-in policy sets `shadow`.

| Stage | Behavior |
|---|---|
| `off` | The pipeline does not open a store. |
| `shadow` | Before each push and before `gh pr`, the pipeline logs which store answered. The push and `gh` use today's credential path. |
| `enforce` | Not built. The parser refuses it until the operator answers D1-OP1 of the plan. |

## Callers

- `go/internal/phases/ship/landing/push.go`. `WithCredentialProbe` adds a probe. `Push` runs the probe once, before the first send, and writes its line to the push log.
- `go/internal/phases/ship/credential_report.go`. `CredentialReport` reads the stage of the project root. At `shadow` it gives one log line that names the store that answered, or the cause. The lookup has a limit of 5 seconds, so an unlock prompt cannot stop the ship. The line never holds the secret. The landing of ship uses it as the probe.
- `go/cmd/evolve/cmd_ship_pr.go`. `openShipPr` writes the report line to stderr before it calls `gh`.
- `go/internal/bridge/manifest.go`. `isReservedEnvKey` refuses each name of `ChildVariables`.

## Invariants

- The package imports `sysexec` and the standard library only.
- No function sets a process environment variable.
- No log line, error or signal holds the secret.
- A test never runs the real `security`, `secret-tool` or `powershell.exe`. Each table test injects a command runner.

# internal/gotool

> The design: sections 7.5, 7.7 and 7.8 of [gate-contract-g2-hermetic-guard](../gate-contract-g2-hermetic-guard.md), commit 2b (ADR-0165).

## Purpose

`internal/gotool` is the one place where a gate starts the Go toolchain. It allows two verbs: `go list` and `go test`. A gate that needs another verb sends a held proposal.

## Design

- **`List` and `Test`.** Each takes a `Call` with `Dir`, `Env` and `Args`. `Args` holds the words after the verb. Each returns an `*exec.Cmd` that is not started.
- **One `command` function.** It builds the command through `sysexec.Command` with the constant program `go`. So the command has the cancel and the wait delay of the process guard.
- **The caller starts it.** The caller sets `Stderr`, `WaitDelay` and the process group on the result, and then runs it.
- **The verb is data of this package.** A verb is a constant in `List` or in `Test`, so a gate site never names it.

## Invariants

- **The package imports only the standard library and `sysexec`.**
- **The package is a protected surface** (`/go/internal/gotool/` in `guards.ProtectedSurfaceManifest`).
- **The two verbs are pinned** (`TestListAndTest_GiveTheVerbBeforeTheArgsWithTheDirAndEnvOfTheCall`).

## Findings

- **The child process is outside the static reach.** A test that the child runs can read git. The parity harness of G3 controls it, because it runs each entry in three states.

# internal/dispatchcred

> Decision: [ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-3. Spec of record: [llm-isolation.md](../llm-isolation.md) §7.1.1 and [cli-contract.md](../cli-contract.md) §4.4. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X4, item `isolation-dispatch-capability-token`.

## Purpose

`internal/dispatchcred` holds the pipeline dispatch credential. A dispatch verb of `evolve` runs only in a process that can read it. The bridge denies its read to each agent cell, so an agent cannot dispatch, also when it unsets the env markers of X4.

## Design

- **The store.** `Store{Dir}` has two parts. `held/credential` is a 0600 file with 32 random bytes in hex, in a 0700 directory. `stage` is the I-3 stage that the last sandboxed launch wrote. Only `held/` gets the read deny.
- **The location.** `Default` gives `~/.evolve/dispatch`, where `~` comes from the passwd entry of the uid (`user.LookupId`), never from `HOME`. A test binary sets the root at link time: `-ldflags -X github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred.root=DIR`. No environment name can move the store.
- **The writer.** `Ensure(stage)` makes `held/`, writes the credential once and replaces `stage` through a rename. The credential goes to a temporary file first, and a hard link gives it its name. So a concurrent `Hold` sees no part of a token, and the first writer wins. The bridge calls `Ensure` before it wraps a cell.
- **The reader.** `Hold` opens the credential with `O_NOFOLLOW` and checks the open file. It accepts only a regular 0600 file with a token of 64 hex characters, so a link to `/dev/null` or a planted file fails. A missing credential is written first, so the shell of the operator needs no setup. A cell cannot write it, because the profile gives no write grant under `~/.evolve`, and `bwrap` mounts `held/` read-only.
- **The verdict.** `Admit(open)` gives a `Verdict`: `Held`, the `Stage` from the store, and the `Cause` of a failure. `Violation` is true when the process does not hold the credential and the stage is not `off`. `Refused` is true at `enforce`.
- **The stage of a refusal.** A store that does not resolve gives the default stage `shadow`. A missing stage file with a denied credential read gives `enforce`, because only a launch makes a cell, and the launch writes the stage. `Stage` gives `shadow` for a missing stage file and `enforce` for a file that it cannot read or a word that it does not know.

## Invariants

- The package imports `bridge/channel` and the standard library only. It starts no process.
- A failure is an error with its cause. No branch swallows an error of the credential.
- The stage file is outside `held/`, so a cell can read the stage and cannot read the credential.

## Findings

- `env -u EVOLVE_DISPATCH_DEPTH -u EVOLVE_PHASE evolve agent run` passed the marker refusal of X4 (review of #1126, 2026-10-26). Row N4 of the sandbox contract now runs that call in the cell at `enforce`, and the verb exits 3.
- `user.LookupId` returns the passwd home with `HOME` set to another directory, also with `CGO_ENABLED=0` (measured on darwin, 2026-10-26).

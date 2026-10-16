# internal/clihealth

## Purpose

`internal/clihealth` is the durable memory of transient CLI outages. The bridge classifies a quota wall on every dispatch, but nothing remembered it. Each phase therefore paid the walled CLI's boot again before it fell back. This package keeps expiring bench records per CLI family, and per driver for boot timeouts, in `.evolve/cli-health.json`. `llmroute` demotes benched families, the loop canaries check expired ones, and preflight reports the active ones. It is a stdlib-only leaf, so `llmroute`, `runner`, `cmd` and `looppreflight` can import it without cycles.

## Design

- `Entry` is one benched family. `Store` reads and writes the file.
- `NewBenchEntry` composes the record after a classified wall. The strike count continues from the previous record. The bench lasts until the pane's reset hint, when one parses, or else the strike-scaled cooldown. The wall line is kept as evidence.
- The runner's bench writer and the loop canary both compose entries through `NewBenchEntry`. The strike, cooldown and evidence logic therefore cannot drift between them.
- `BenchWall` is the one read-previous, compose, write sequence. `Bench`, `Clear` and `RecordBootStrike` are the other writers.
- `Active` returns the entries that routing must still avoid. `Expired` returns the canary candidates.
- `Benchable` is a deliberately closed set of escalation patterns. Re-dispatching a walled resource is guaranteed waste. Most escalations, such as trust prompts, are situational.

## Invariants

- Every read-change-write runs under the `<file>.lock` sidecar flock, the project-wide convention. The sidecar is locked, not the data file, because the data file is replaced by rename. `withLock` calls `MkdirAll` first, because the directory must exist before the lock file is created.
- The per-family `Strikes` counter is cumulative. Concurrent fleet cycles that wall the same family must each see the prior strike. An unlocked read-change-write loses increments and under-escalates the cooldown.
- Writes use a temp file and a rename. A missing or corrupt file degrades to empty and never fails a dispatch.
- Cooldown: `DefaultCooldown` is 30 minutes. It doubles per strike beyond the first and caps at `MaxCooldown`, 4 hours. Strikes of one or less get the default.
- A login wall (`CredentialPattern`) ignores any reset hint. A stale wall in the same scrollback can otherwise set a time. The family stays benched until a canary probe succeeds after the login. `Active` keeps a credential wall until a probe clears it, because time says nothing about the login.
- An exhausted wall (`ExhaustedPattern`) arrives as `exhausted` from agy and as `rate_limit` from codex. Both must be benched.
- agy's wall text (`Resets in 6h12m56s`) is deliberately not parsed as a reset hint. Doubling per re-bench gives a long bench only after the wall repeats. agy served successful dispatches inside its own claimed window.
- Boot timeouts are keyed by driver, not family, because a REPL boot failure is driver-specific. `RecordBootStrike` benches a driver only at `DefaultBootBenchThreshold` consecutive strikes. Below that, the strike count is kept with `BenchedUntil` set to now, so the driver stays retryable.
- `ClearBootStrike` runs on any other exit, because strike, success, strike must not bench. It removes only an entry whose reason is the boot-timeout pattern, so it cannot clear a rate-limit bench under the same name.
- `truncateRunes` truncates on rune boundaries. Wall text carries multi-byte glyphs, and a byte slice can split one.
- Exit code 80 is an integer literal, held in the constant `replBootTimeoutExitCode`. It equals `ExitREPLBootTimeout` in `bridge/exitcodes.go`. This package cannot import the bridge, because the bridge imports this package.

## Findings

- The bench record is the one place that stores a family's wall. A new routing consumer must read `Active` and must not keep a second store.

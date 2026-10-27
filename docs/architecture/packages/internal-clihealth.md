# internal/clihealth

## Purpose

`internal/clihealth` is the durable memory of transient CLI outages. The bridge classifies a quota wall on every dispatch, but nothing remembered it. Each phase therefore paid the walled CLI's boot again before it fell back. This package keeps expiring bench records per CLI family, and per driver for boot timeouts, in `.evolve/cli-health.json`. `llmroute` demotes benched families, the loop canaries check expired ones, and preflight reports the active ones. It is a stdlib-only leaf, so `llmroute`, `runner`, `cmd` and `looppreflight` can import it without cycles.

## Design

- `Entry` is one benched family, or one model group of a family when `Scope` is set. `Entry.Key` is the store key: the family, or `family@scope`. `Store` reads and writes the file.
- `ModelGroup` strips the effort variant from a model name, so `Gemini 4 Argon (High)` and `Gemini 4 Argon (Medium)` share the group `Gemini 4 Argon`. `BenchScopedWall` benches one group; `BenchWall` benches the family.
- `NewBenchEntry` composes the record after a classified wall. The strike count continues from the previous record. The bench lasts until the pane's reset hint, when one parses, or else the strike-scaled cooldown. The wall line is kept as evidence.
- The runner's bench writer and the loop canary both compose entries through `NewBenchEntry`. The strike, cooldown and evidence logic therefore cannot drift between them.
- `BenchWall` is the one read-previous, compose, write sequence. `Bench`, `Clear` and `RecordBootStrike` are the other writers.
- `Active` returns the family entries that routing must still avoid. `Expired` returns the canary candidates. Both skip model entries.
- Two operator views read `Active` through one sorted list, `activeBenches` in `cmd/evolve`: `evolve clihealth list` and `evolve wave status` (since 2026-10-27). `wave status` prints one `benched:` line for each entry, so the console sees the benches that the pre-wave usage probe wrote at launch.
- A model entry (`BenchModelUntil`, key `family\x00model`) benches one model of a family until its reset. `ActiveModels` returns them. No router or canary reads them yet.
- `Benchable` is a deliberately closed set of escalation patterns. Re-dispatching a walled resource is guaranteed waste. Most escalations, such as trust prompts, are situational.

## Invariants

- Every read-change-write runs under the `<file>.lock` sidecar flock, the project-wide convention. The sidecar is locked, not the data file, because the data file is replaced by rename. `withLock` calls `MkdirAll` first, because the directory must exist before the lock file is created.
- The per-family `Strikes` counter is cumulative. Concurrent fleet cycles that wall the same family must each see the prior strike. An unlocked read-change-write loses increments and under-escalates the cooldown.
- Writes use a temp file and a rename. A missing or corrupt file degrades to empty and never fails a dispatch.
- Cooldown: `DefaultCooldown` is 30 minutes. It doubles per strike beyond the first and caps at `MaxCooldown`, 4 hours. Strikes of one or less get the default.
- A login wall (`CredentialPattern`) ignores any reset hint. A stale wall in the same scrollback can otherwise set a time. The family stays benched until a canary probe succeeds after the login. `Active` keeps a credential wall until a probe clears it, because time says nothing about the login.
- An exhausted wall (`ExhaustedPattern`) arrives as `exhausted` from agy and as `rate_limit` from codex. Both must be benched.
- agy's compact reset (`Resets in 148h8m53s`) is a reset hint for every pattern except `ExhaustedPattern`. The hint is capped at 24 hours. Cycle 1911 added it together with model-group scopes, so a long hint benches one model group, not all of agy.
- An `ExhaustedPattern` wall still ignores the compact hint. Doubling per re-bench gives a long bench only after the wall repeats. agy served successful dispatches inside its own claimed window.
- A legacy file without `scope` reads as family-wide entries. A scoped entry and a family-wide entry of one family coexist under different keys.
- Boot timeouts are keyed by driver, not family, because a REPL boot failure is driver-specific. `RecordBootStrike` benches a driver only at `DefaultBootBenchThreshold` consecutive strikes. Below that, the strike count is kept with `BenchedUntil` set to now, so the driver stays retryable.
- `ClearBootStrike` runs on any other exit, because strike, success, strike must not bench. It removes only an entry whose reason is the boot-timeout pattern, so it cannot clear a rate-limit bench under the same name.
- `truncateRunes` truncates on rune boundaries. Wall text carries multi-byte glyphs, and a byte slice can split one.
- Exit code 80 is an integer literal, held in the constant `replBootTimeoutExitCode`. It equals `ExitREPLBootTimeout` in `bridge/exitcodes.go`. This package cannot import the bridge, because the bridge imports this package.

## Findings

- The bench record is the one place that stores a family's wall. A new routing consumer must read `Active` and must not keep a second store.

## The model_unsupported pattern (2026-10-21)

- `ModelUnsupportedPattern` (`model_unsupported`) names an account that cannot run a model. The exit is deterministic for the pair of the family and the model.
- `NewBenchEntry` benches it for `MaxCooldown`, and `OperatorAction` names the two operator choices: re-pin the tier, or accept the fallback.
- `Benchable` does not hold it, so the canary and the family benches never bench a whole family on it. Only `bridgechain.BenchOnEscalation` uses it, with a model scope.
- Pinned by `TestModelUnsupported_BenchesForTheLongestCooldownWithAnOperatorAction`.
- A campaign retry does not wait for a `model_unsupported` bench (`campaignQuotaCooldown`), because the chain already routes around the model.
- Two per-model keys exist. `Entry.Scope` (`<family>@<model group>`) is the one the router reads, through `scopeCoversSeat`, and this bench uses it. `Entry.Model` (`BenchModelUntil`, `ActiveModels`) holds the usage-probe walls of one model, and no router reads it.
- Known limit: the advisor prompt and the loop preflight print a scoped bench as a family bench ("dispatch chains start at the fallback CLI"). That holds only at the tiers of the benched model.

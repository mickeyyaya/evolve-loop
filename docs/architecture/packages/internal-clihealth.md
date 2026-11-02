# internal/clihealth

## Purpose

`internal/clihealth` is the durable memory of transient CLI outages. The bridge classifies a quota wall on every dispatch, but nothing remembered it. Each phase therefore paid the walled CLI's boot again before it fell back. This package keeps expiring bench records per CLI family, and per driver for boot timeouts, in `.evolve/cli-health.json`. `llmroute` demotes benched families, the loop canaries check expired ones, and preflight reports the active ones. It imports only the standard library and the leaves `internal/log`, `adapters/flock` and `signalcenter`, so `llmroute`, `runner`, `bridge`, `cmd` and `looppreflight` can import it without cycles.

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
- A login wall (`CredentialPattern`) ignores any reset hint. A stale wall in the same scrollback can otherwise set a time. The family stays benched until a probe or a launch of the family succeeds after the login. `Active` keeps a credential wall until a success clears it, because time says nothing about the login.
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

## A success clears the auth bench (2026-11-02)

The request (inbox `a-successful-probe-clears-the-auth-bench`): "A successful probe of the family must clear its auth bench." On 2026-11-02 the operator logged the claude CLI in again. `evolve doctor live claude-tmux` answered LIVE OK, but the `auth_recheck` bench stayed until the console ran `evolve clihealth clear claude`. The bench put claude last, so lanes went to the exhausted agy pools and paused on quota.

**F1** Before this change, only the canary cleared a credential wall, and only after the cooldown expired (`Expired`, up to `MaxCooldown`). `doctor live` and a successful phase launch did not touch the store. The usage probe skipped every benched family. The boundary CLI update skips a walled family without a probe, so it waits for a success on another path.
*Implication:* the text "the bench clears itself once a probe succeeds" in `OperatorAction` was false for hours after a login.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | `Store.ClearAuthBench(family, source, signals)` is the one clearing point. It removes each auth-class entry of the family at any scope. `AuthBench` names the class: `auth_recheck` only. It emits one `BRIDGE_AUTH_BENCH_CLEARED` event for each removed entry. | F1 | A clear in each caller: the reason test and the signal can drift between four callers. |
| D2 | A success clears only the auth benches. A quota bench (`rate_limit`, `exhausted`, `quota_exhausted`, `usage_probe`) and a model bench stay. | A REPL can boot while the quota is spent. | Clear every bench of the family on a success: it sends lanes back into a spent quota. |
| D3 | `AuthBench` is the one home of the auth class. `ClearAuthBench`, `Active`, the usage probe and `credentialWall` in `cmd/evolve` read it. | F1 | A literal `CredentialPattern` test in each reader: a new auth reason then splits clearing, routing and the recheck. |
| D4 | The code belongs to the `bridge` module. | The closed module set has no `clihealth` module, and the store is the bridge's routing memory. | A new `clihealth` module: it changes the closed module set for one code. |

The success paths and their `source` field:

| Success | Caller | `source` |
|---|---|---|
| A phase launch exits 0 | `bridge.Engine.launchModel` | `launch` |
| A live smoke answers its prompt (exit 0): `doctor live`, the loop canary, the `Probe` and `Smoke` seams of the CLI update | `bridge.LiveSmokeTest` | `smoke` |
| The usage probe reads usage windows from the family's own screen | `usageprobe.Prober.act` | `usage_probe` |

`ClearSource` holds the `source` values (`SourceLaunch`, `SourceSmoke`, `SourceUsageProbe`), so the registered meaning and the callers use one list.

- Both bridge callers go through `bridge.clearAuthBench`. It maps the CLI to its family with `llmroute.Family`, opens the store of the project root, and logs one `[bridge]` line for each cleared entry. A failed clear is a `WARN` line, and the launch result does not change.
- A failed probe, a walled probe and a usage screen without windows keep every bench.
- A boot-only smoke (`BootSmokeTest`: the loop preflight and `evolve doctor boot`) keeps every bench. A REPL can boot on an expired login and wall on the first request, so a boot answers no prompt and proves no login.
- The signal goes to the Center that the caller has. The loop wires its root Center into the phase launch, the canary and the usage probe. `doctor live`, the boundary CLI update and the campaign pre-wave probes have no Center. Centers exist only at the process roots. For them, the `[bridge]` log line is the only record.
- `ClearAuthBench` reads the file first and returns without a lock or a write when the family has no auth bench. A success on a project with no bench therefore creates no `.evolve/` directory.
- Pinned by `TestClearAuthBench_ASuccessClearsEveryAuthBenchOfTheFamilyAndSignalsEach`, `TestClearAuthBench_QuotaBenchesStayAndNothingIsWritten`, `TestLiveSmokeTest_ASuccessfulProbeClearsTheAuthBenchOfItsFamily`, `TestLiveSmokeTest_AWalledProbeKeepsTheAuthBench`, `TestEngineLaunch_ASuccessfulLaunchClearsTheAuthBenchAndAFailedOneKeepsIt`, `TestProber_AUsageReadWithWindowsClearsTheAuthBench`, `TestBootSmokeTest_ABootOnlySuccessKeepsTheAuthBench`, `TestClearAuthBench_NoProjectRootNeverClearsTheStoreOfTheWorkingDirectory` and `TestCLIUpdateWiring_AfterASuccessfulLiveProbeTheCredentialWallNoLongerSkipsTheFamily`.
- Limit: no success clears a quota bench before its reset. The usage probe does not yet clear a quota bench when the windows show headroom.
- Limit: the usage probe selects families by the family key of `Active`. It skips a family that has a family-wide quota bench and a scoped auth bench. Only a launch or a smoke pane clears that auth bench.

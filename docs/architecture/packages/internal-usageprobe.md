# internal/usageprobe

> The wiring: `cmd/evolve` `runUsageProbe`, `newUsageProber` and `probeWaveQuota` ([cmd-evolve.md](cmd-evolve.md), *One pre-wave probe protocol*). The manifest vocabulary it reads: [internal-bridge.md](internal-bridge.md), *Provider-aware targets*. The operator's view of agy's `/usage` screen: [agy-runtime.md](../../../skills/loop/reference/agy-runtime.md). The follow-up for per-model windows: [model-level benches](../../plans/model-level-benches-2026-10.md).

## Purpose

`internal/usageprobe` benches every CLI family that is already at a quota cap, before a wave's first phase boots it. It sends each interactive family's usage command over the bridge and reads the pane into typed usage windows. It then writes a bench into the shared `clihealth` store for each exhausted family, and records what it read. The dispatcher's pre-skip then demotes that family for every phase of the wave. The reactive bench, by contrast, only learns of a cap after a phase has spent a boot on it.

The probe is opt-in (`cli_health.proactive_probe`, compiled default off; this project's checked-in policy turns it on) and fails open.

`ProbeQuota` is the measurement path over the same probe seam and the same reader: it turns each pane's windows into `quotastate.QuotaState`s, one per routing family the windows target, which the fleet budget allocator sizes a wave against.

## Design

- **Every seam is injected.** `Prober` holds:
  - `Probe`: send the usage command, return the pane;
  - `Windows`: the typed windows the CLI's manifest reads from the pane;
  - `Classify`: the whole-family regex verdict, the fallback;
  - `Record`: store what was read;
  - the `clihealth.Store` and a log.

  The package holds no tmux and no manifests, so the tests run with neither.
- **One probe per family, concurrently.** `Run` probes every family that has no active bench, in parallel. It returns when all have answered or the context ends. An interrupt wins: no bench and no log line is written after it, and in-flight probes are abandoned.
- **One reader for every CLI** (since 2026-10-06). `quotastate.ReadWindows(spec, pane, now)` reads any usage screen whose layout the CLI's manifest declares in `controls.usage.windows`. A spec names:
  - how a window is found: a label line, optionally inside a section such as agy's group header;
  - how its value is read: used or remaining, with remaining normalized to used;
  - its reset text;
  - what its scope maps to: a routing family, or a family plus one model.

  The result is `[]quotastate.UsageWindow{Scope, Kind, PercentUsed, ResetsText, ResetsAt, Models, Family, Model, Exhausted}`. A window is exhausted at or above the spec's `exhausted_at_used_pct` (default 100). `ResetsAt` (UTC) is parsed from the reset text: a duration (`140h 56m`), a clock time (`12:40am`) or a date (`Oct 11 at 9pm`), in the `(Area/City)` zone the text names when it names one. This one reader replaced claude's compiled-regex `quotastate.Parse` and an agy-only group parser, so the cap probe and the budget probe can no longer read a screen differently.
- **One per-model rule.** `UsageWindow.ExhaustsFamily()` (`Exhausted && Model == ""`) is the one Specification of "this window is the family's quota cause". The probe's bench, the evidence verdict, the budget's `StatesOf` and the `evolve clihealth usage` table all ask it. Before review round 1 (2026-10-06) four places each restated the rule and two disagreed: claude's session at 10% plus a Fable week at 100% gave "verdict=exhausted … verified", which would have blamed an unrelated Opus failure on quota.
- **What the probe does with the windows** (`judgeWindows`):
  - Every parsed window is recorded first. `Record` writes `.evolve/usage-windows.json` (`RecordObservation`, keyed by CLI, under a `<file>.lock` flock and an atomic write). It holds the latest windows per CLI, the fact a router or an operator reads.
  - An exhausted family-scoped window benches that family through `clihealth.Store.BenchWallUntil(family, clihealth.Wall{Pattern, Evidence, Reset})`, with reason `usage_probe` and the windows' evidence.
    - The bench lasts until the latest reset among the family's exhausted windows, plus the store's two-minute margin, capped at 24 h.
    - With no parsed reset, it falls back to the strike cooldown.
    - The newest reading replaces the bench, even when its reset is earlier than the bench in force. The newest screen states when the window reopens, just as `NewBenchEntry` lets the newest wall set the bench (`TestBenchWallUntil_TheNewestReadingsResetReplacesTheBenchEvenWhenItIsEarlier`, decided in the delta re-review, H1). One reading still benches a family until the latest reset among its drained windows.
    - A drained agy Claude group benches `agy-claude` and leaves agy's Gemini models routable. An exhausted claude session or all-models week benches `claude`.
  - An exhausted per-model window (claude's `Current week (Fable)`) benches nothing. Benches are per family, and benching `claude` for one model would sideline its other models. It is a `WARN` line naming the model, and it stays in the recorded windows. [Model-level benches](../../plans/model-level-benches-2026-10.md) are the follow-up.
  - An exhausted window whose scope the manifest maps to no family is a `WARN` naming the scope; nothing is benched.
  - When the pane yields windows, they own the verdict and `Classify` is not asked. A pane with no windows (a CLI with no `windows` block, such as codex today, or a layout that drifted again) falls back to `Classify` and benches the probed family, as before.
- **`ProbeQuota(ctx, families, QuotaReader{Probe, Read, Now})`** probes the families concurrently. It turns each pane's windows into states with `quotastate.StatesOf`: one state per target family, with buckets named `session`, `week` or `5h`. **A per-model window is left out of the family's state** (decided in review round 1, 2026-10-06). The allocator binds on a family's tightest bucket, so one model's drained week would zero the whole family's headroom, the same false cause the per-model rule prevents. The window stays in the recorded observation. A family whose probe fails, or whose pane yields no window, is absent, so the allocator falls back to its floor rather than seeing a fabricated cap. One agy screen yields both an `agy` and an `agy-claude` state.

- **Usage evidence for a failing CLI** (`evidence.go`, since 2026-10-06). `EvidenceSource.Explain(ctx, Query{CLI, Family, Since})` is the query every failure path runs (wired by [internal-usageevidence](internal-usageevidence.md)). `Since` is when the failing attempt started. A cached or recorded observation is reused only while it describes the failure (`reusable`):
  - It is younger than `TTL`. The bound is exclusive: a read exactly `TTL` old is queried again (`TestEvidence_AReadIsReusedForLessThanTheTTLAndQueriedAgainAtExactlyTheTTL`).
  - A `healthy` read was taken at or after `Since`. The bound is inclusive (`TestEvidence_AHealthyReadTakenTheInstantTheFailureStartedDescribesIt`). An earlier one cannot **rule the failure's cause out**, so the CLI is queried again (`TestEvidence_AnObservationOlderThanTheFailureIsNotEvidenceForIt`). A read that rules nothing out (a failed query, no window, an exhausted window) is reused inside the TTL whatever its age. Otherwise a down CLI would be re-booted for every failing attempt: the first cut of the rule did that, and it tripled the e2e suite's time (`TestEvidence_AFailedQueryReadBeforeTheFailureIsStillReusedSoADownCLIIsNotStormed`).
  - An `exhausted` read stops counting once the latest reset of the family's drained windows has come (`resetPassed`). From that instant the CLI is queried again (`TestEvidence_AnExhaustedReadIsReusedUntilItsResetAndQueriedAgainOnceTheResetComes`, delta re-review M1, 2026-10-07). Before, a read whose reset had passed was still reported as "exhausted, verified" for the rest of the TTL while nothing was benched.
  - It reads `cli`'s screen through the same `Prober` seams, judges `family`'s windows, and returns `Evidence{CLI, Family, Verdict, Windows, Detail, ObservedAt, Cached}`. The verdicts:
    - `exhausted`: a family-scoped window of the family is exhausted (`ExhaustsFamily`), or no window was read but the regex matched while this family's failure was explained (`Observation.RegexWallFamily`). A sibling family that reuses that screen gets `unknown`, because only the failing family was benched (`TestEvidence_AReusedRegexWallVerifiesOnlyTheFamilyItWasReadFor`, delta re-review L1);
    - `healthy`: no family-scoped window of the family is exhausted. An exhausted per-model window is named in the detail as a note ("per-model window exhausted, which is no family cause but fails a phase on that model"), never as the verdict;
    - `unavailable`: the probe failed or timed out;
    - `unknown`: there is no usage command, or no window for the family.
  - `Summary()` is the one-line wording every site prints: "quota exhausted, verified by a usage query: …", "quota ruled out by a usage query: …", "the usage query itself failed (…), which points at an auth, install or network cause", or "quota could not be verified: …".
  - The query is bounded by `Timeout`. Its observation, a failed one included (`Observation.Error`; `RegexWallFamily` for the regex fallback), is cached per CLI under the rules above, in memory and through `.evolve/usage-windows.json`, so another process reuses it without probing. What a burst of failures costs is in [internal-usageevidence](internal-usageevidence.md).
  - With `Act`, a fresh query does what the pre-wave probe does with the same reading. It benches the exhausted family until the reset, warns on a per-model window, and records the observation. A regex-only wall benches the **failing** family (an agy-claude failure benches `agy-claude`, never agy's Gemini family). Without `Act` it changes nothing (`evolve doctor live`).
  - **One in-flight query per CLI** (a single-flight: `join` and `land` take the lock briefly, the query runs outside it). Concurrent failures of one CLI wait for the one answer. A query for one CLI never waits on another's; before review round 1 a slow claude query blocked agy's for up to the timeout. Across processes, the recorded windows are the shared cache, so at most the lane width of queries run at once.
  - **A caller's cancellation is never an observation** (delta re-review, 2026-10-07).
    - The leader runs the query in its own goroutine on `context.WithoutCancel(ctx)`, bounded only by `Timeout`.
    - Every caller, the leader included, waits on its own context against the flight. One that stops waiting gets `unknown` ("the caller stopped waiting for the usage query: …"). The query goes on, lands, and is recorded for everyone else.
    - Before the fix the leader queried on its own context. A phase that ended during a stall (the observer's `watchCtx`), or an attempt's deadline, cancelled the probe. "context canceled" was then cached as `unavailable` ("auth, install or network") for the TTL, in memory, for every follower and in `.evolve/usage-windows.json` for every lane.
    - A follower of a query in flight gets a fresh answer (`cached=false`); only a reused read is `cached`.
    - Pinned by `TestEvidence_ALeaderCancelledMidQueryReturnsAtOnceAndNeitherCachesNorRecordsTheCancellation`, `TestEvidence_AFollowerWithALiveContextGetsTheQuerysAnswerWhenTheLeaderIsCancelled`, `TestEvidence_AFollowerOfAQueryInFlightGetsAFreshAnswerNotACachedOne` and `TestEvidence_AFollowerWhoseOwnContextEndsStopsWaitingForASlowQuery`.
- **The pre-wave probe and the evidence share one reading.** `probeOne` is `read` (the pane, the windows, else the regex verdict), then `record`, then `act` (`judgeWindows`, or `benchFamily` for a regex wall). `Explain` calls the same `read`, `judgeWindows` and `benchFamily`.

## Invariants

- **The probe never invents a cap.** An unsupported usage command, a probe error, an unreadable pane, or a healthy pane writes no bench. Pinned by the `TestProber_*` tests in `usageprobe_test.go`.
- **A drained scope benches only its own family; a per-model window benches nothing.** Pinned by `TestProber_ADrainedAgyGroupBenchesItsOwnFamilyUntilItsReset`, `TestProber_AnExhaustedClaudeSessionBenchesClaudeUntilItsReset`, `TestProber_AnExhaustedPerModelWindowIsAWarningAndARecordedFactNotABench` and `TestProber_ParsedWindowsOwnTheVerdictSoTheRegexIsNotConsulted`.
- **The fallback keeps the old verdict and records nothing.** Pinned by `TestProber_APaneWithNoWindowsFallsBackToTheRegexClassifier`.
- **An unmapped drained scope is reported.** Pinned by `TestProber_ADrainedScopeMappedToNoFamilyIsWarnedNotBenched`.
- **The record keeps the latest windows per CLI.** Pinned by `TestRecordObservation_KeepsTheLatestWindowsPerCLI`.
- **One screen can report several families.** Pinned by `TestProbeQuota_OneScreenReportsEveryFamilyItsWindowsTarget`.
- **The reader and the data agree with the live screens.**
  - In `quotastate`: `TestReadWindows_*` (spec literals), and `TestReadWindows_TheJulyClaudeGoldenKeepsItsBucketsThroughStatesOf`, which proves the buckets the budget read before still come out the same.
  - In `bridge`, against the real manifests: `TestUsageWindows_*`.
  - In `cmd/evolve`, through the production prober: `TestNewUsageProber_ReadsEachCLIsWindowsThroughItsManifestAndBenchesTheRightFamily`. Its record check was red before `Record` was wired.
- **Each verdict, the cache, the timeout, read-only mode, the per-model rule, the single-flight and the failing-family regex bench.** Pinned by `TestEvidence_AnExhaustedPerModelWindowIsNotAVerifiedFamilyCause`, `TestEvidence_ASlowQueryForOneCLIDoesNotBlockAnotherAndOneCLIIsQueriedOnce`, `TestEvidence_TheRegexFallbackBenchesTheFailingFamilyNotTheWholeBinary`, `TestEvidence_AnExhaustedWindowIsAVerifiedQuotaCauseAndBenchesItsFamily`, `TestEvidence_HealthyWindowsRuleQuotaOutAndNameTheWindows`, `TestEvidence_AFailedUsageQueryIsUnavailableAndPointsAtAuthInstallOrNetwork`, `TestEvidence_NoUsageCommandOrNoWindowIsUnknown`, `TestEvidence_TheRegexFallbackStillVerifiesAWall`, `TestEvidence_TheCacheLastsTheTTLAndIsSharedThroughTheRecordedWindows`, `TestEvidence_TheQueryIsBoundedByItsTimeout` and `TestEvidence_AReadOnlySourceNeitherBenchesNorRecords`.
- **The refresh bench is bounded.** Pinned by `TestBenchWallUntil_*` in `clihealth`.

## Findings

- **2026-10-06: two usage screens drifted past their regexes.**
  - `evolve bridge control agy usage` showed agy 1.3.0's two-group screen (Claude group 72.68% weekly and 45.36% five-hour remaining; Gemini group 91.70% and 100%).
  - Claude Code 2.1.291's `/usage` showed `N% used`, while its `exhausted_regex` expected `0% left`.
  - Under the old regexes a drained agy group or a fully used claude window went unseen. Where a regex did match agy, it benched the Gemini target whichever group was drained. Since #785 the router routes to `agy-claude`, so a wrong bench moves real dispatches.
- **A `_windows.go` suffix is a build constraint.** The bridge's first `usage_windows_test.go` ran no tests on darwin: Go reads `_windows` before `.go` or `_test.go` as GOOS. The files are named `usage_window_reader*.go`.
- **The bench honors the screen's reset, unlike the reactive agy wall.** `clihealth` deliberately ignores agy's `Resets in 6h12m56s` wall text, because agy once served dispatches inside the window it had claimed. A usage window is different: it is a measurement of a pool at 100% used, read before any dispatch. The bench is still capped at 24 h, after which the canary re-probes it.
- **Still open:**
  - codex's `/status` screen has no `windows` block until it is captured live.
  - [Model-level benches](../../plans/model-level-benches-2026-10.md).

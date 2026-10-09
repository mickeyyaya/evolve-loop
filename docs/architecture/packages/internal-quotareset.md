# internal/quotareset

> The consumer: the quota-pause checkpoint (`internal/checkpoint`, `withQuotaReset`). The operator view: [auto-resume.md](../auto-resume.md).

## Purpose

`internal/quotareset` computes the wake-up time of a quota pause. The checkpoint writes it as `quotaResetAt` with its source in `quotaResetSource`. `evolve estimate-quota-reset [WORKSPACE]` prints the same result.

## Design

`Compute(workspace, Options)` takes the first source that has evidence:

| Order | Source | Input |
|---|---|---|
| 1 | `operator-override` | `Options.ResetAt` (`policy.json` `quota_reset.reset_at`). A value that does not parse as RFC 3339 is kept as text, and the wake time is now. |
| 2 | `parsed` | `<workspace>/quota-reset-hint.txt`, the first 32 bytes, read as `H:MMam` or `H:MMpm`. A past time today moves to tomorrow. |
| 3 | `bench` | `Options.BenchedUntil`, when it is after now. |
| 4 | `usage` | `Options.UsageReset`, a lazy query. It runs only when sources 1 to 3 have no evidence, and only a reset after now counts. |
| 5 | `default` | `Options.DefaultHours`, when it is more than 0 (`quota_reset.default_hours`). |
| 6 | `unknown` | No evidence. The wake time is now. |

The options have no other fields (2026-10-09: the dead `Env` seam and `HoursFn` are removed).

### The quota-pause checkpoint (`internal/checkpoint`, `withQuotaReset`)

- **The walked families.** `pauseForQuota` copies the CLIs of the failed walk (`core.WalkError`, from the runner) to `CycleState.QuotaWalkCLIs`. `walkedFamilies` maps them with `llmroute.Family`.
- **The bench evidence.** `earliestQuotaReset` takes the earliest active bench of a walked family whose reason is a quota wall (`clihealth.QuotaPattern`: `rate_limit`, `exhausted`, `quota_exhausted`, `usage_probe`). A credential bench, a boot-timeout strike, a lapsed bench and a bench of another family do not count.
- **The usage query.** `checkpoint.UsageReset` is a hook. `cmd/evolve` sets it to `productionUsageReset`, which asks the usage screen of each walked family. A family is back when its last exhausted window resets, and the first family back sets the wake time.
- **An unknown reset.** The checkpoint has no wake time, `autoResumeMaxAttempts` is 0, and `operatorAction` says to resume by hand.

## Invariants

- **An unknown reset is never a far-future time** (since 2026-10-09). Before, source 5 was a built-in 5.4167 h (`source=default`). No production code writes the hint file. Thus every pause got that default, also when the bench store had the real reset of the wall. Cycle 1853 paused until `2026-10-10T00:07+0800` with `source=default`, but the agy-claude wall said `Resets in 89h29m48s` ([incident](../../incidents/cycle-1853-secondary-defects.md)).
- **Now the bench or the usage query gives the reset.** With no evidence, the source is `unknown`, and the pause is not auto-resumable. Pinned by `TestCompute_AnUnknownResetIsNowAndSaysUnknownNeverAFarFutureDefault`, `TestCompute_ABenchedFamilysResetIsTheEvidenceBeforeAnyConfiguredDefault`, `TestCompute_TheUsageQueryResetComesAfterTheBenchAndBeforeAConfiguredDefault`, `TestCompute_AUsageResetAlreadyPastIsNoEvidence`, `TestQuotaBoundaryCheckpointer_AnUnknownResetIsNotAutoResumableAndNamesTheOperatorAction`, `TestQuotaBoundaryCheckpointer_ABenchThatIsNoQuotaResetOfTheWalkIsNoEvidence`, `TestQuotaBoundaryCheckpointer_TheUsageQuerySuppliesTheResetOfTheWalkedFamilies` and `TestQuotaBoundaryCheckpointer_TheEarliestActiveBenchSetsTheWakeAt`.
- **A quota pause always has a wake time and a source.** The cycle-656 checkpointer once wrote `quotaResetAt` and `quotaResetSource` empty (inbox defect `quota-pause-wakeat-unpopulated`). Then `QUOTA-PAUSE` printed `wake-at= source=unknown`, and the auto-resume delay in `skills/loop/SKILL.md` had no time to parse. Each source arm has its own checkpoint test in `internal/checkpoint/quota_wakeat_test.go`.
- **The checkpoint stamps the wake time only on a quota pause.** A phase-complete checkpoint has no reset time, and a stamp there invents one. A `Compute` error records `quotaResetSource=unavailable` and no wake time (`computeQuotaReset` is the test seam). Pinned by `TestWithQuotaReset_AnEstimatorErrorRecordsUnavailableAndNoWakeAt`.

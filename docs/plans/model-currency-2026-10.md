# Model currency: every dispatch runs the latest model of its line (2026-10)

> Status: approved by the operator on 2026-10-05. The operator's words: "make sure the model and version will always be checked and point to the latest model".
>
> - **Decision:** a newer CLI version found at a wave boundary is installed **automatically at that boundary**, smoke-tested, and the catalog is then refreshed.
> - **Extends:** [model-discovery-and-catalog.md](../architecture/model-discovery-and-catalog.md).
> - **Decision record:** ADR-0120, which lands with C2.
> - **C3 status (2026-10-27):** the operator approved the C3 stage flip, and it landed. The checked-in policy sets `catalog.auto_refresh: true` and `catalog.refresh_stage: enforce`. Two C3 guards landed with it. The boot refresh and the `catalog_refresh_degraded` ledger event are open. C2 is not built. Item: `model-currency-c3-adopt-only-live-picks`. See [C3 landing notes](#c3-landing-notes-2026-10-27).

## Context: why the loop ran Gemini 3.7 Flash while 3.8 was available

On 2026-10-05, `agy models` listed Gemini 3.8 Flash (High/Medium/Low), but every agy dispatch ran 3.7. The live catalog `.evolve/model-catalog.json` had last been written on 2026-08-14. The latest-model machinery exists: `LineageKey`, `PromoteLatest` and per-CLI freshness. Four independent breaks kept it from ever acting.

| # | Break | Where | Effect |
|---|---|---|---|
| B1 | The tier classifier runs on the first "ready" CLI in a hardcoded list (`[codex, claude, agy]`). It never tries the next CLI. | `cmd/evolve/cmd_models_live.go` `classifierCLIPreference` / `pickClassifierCLI` | codex is installed but has no subscription, so classification fails for **every** family. Each family falls back to the offline `detect` map (3.7). |
| B2 | Degradation is silent. The command prints "Refreshed model catalog" and exits 0. | `liveRefresh` / `models refresh` | Nobody saw that nothing was refreshed. |
| B3 | Reuse is keyed on the exact candidate set. | `modelquery` fingerprint / reuse gate | A new **version** in a known lineage (3.7 → 3.8 Flash) changes the hash. That forces the LLM classifier again, even though the qualitative decision ("Flash High serves balanced") has not changed. |
| B4 | The refresh is never adopted, and the CLIs are frozen. | `.evolve/policy.json` `catalog: {auto_refresh: false, refresh_stage: shadow}`; `looppreflight` `cli-version-freeze` | Even a successful refresh writes only the shadow file. A CLI only lists the models its installed version knows. Sonnet 5.5 was missed in exactly this way until `claude update` (2026-09-30). |

## Principle

Two different kinds of decision are involved, and they belong in different places.

- **Which line serves which tier** ("Flash (High)" for balanced, "Pro (High)" for deep) is qualitative. It changes only when a *new line* appears. The LLM classifier, or the operator, decides it once per line.
- **Which version of that line to run** is numeric and changes with every release. Go decides it, every time, deterministically (rule 5). `PromoteLatest` already implements this within a lineage.

The operator's routing table then decides which **families** may run each tier (`cli_routing.tiers`, [cli-routing-table-2026-10.md](cli-routing-table-2026-10.md)). For example, deep and top run on Claude Opus until Gemini 4 Pro ships. Currency and routing are separate concerns.

## Components (each is one commit with its own tests; red first; unwired before wired)

| # | Component | Change | Fixes |
|---|---|---|---|
| C1 | **Classifier fallback + loud degradation + offline 3.8 baseline** (in flight, `dev/cl-models38`) | The classifier walks every ready CLI until one classifies. Each CLI that falls back to `detect` is reported per CLI. If none classified live, the command exits non-zero. agy `model_tier_map` becomes fast = Gemini 3.8 Flash (Low), balanced = Gemini 3.8 Flash (High). | B1, B2 |
| C2 | **Lineage-keyed reuse** | The reuse gate keys on the **set of lineages** (version-free `LineageKey`s) plus the policy, not on the exact candidates. The classifier runs only for a **new lineage**. See [Component details](#component-details). | B3 |
| C3 | **Always checked, always adopted** | `catalog.refresh_stage: enforce` in the checked-in policy, as the operator decided. The refresh runs at **loop boot** (open, not built) and at cycle start, gated by the TTL. Adoption is **per family** and only from a live listing. See [Component details](#component-details). | B4 (catalog half) |
| C4 | **Boundary CLI update** | New verb `evolve cli update [--dry-run] [--json]`. It runs the updater of each subscribed family before the catalog refresh, at the wave boundary. See [Component details](#component-details). | B4 (CLI half) |
| C5 | **Launch verification** | After a pane boots, the model it reports (agy's banner/footer model; claude's transcript `message.model`) is compared with the catalog's pick for that tier. A mismatch is an abnormal event (`dispatch_model_mismatch`) naming expected and observed, never a block: the policy is "logic, not format". **The agy-claude family slice landed 2026-10-07 and fails over instead of only reporting; see "C5 landing notes" below.** | proves currency on every dispatch |
| C6 | **Classifier routing** | The model-classifier launch resolves its CLI through the CLI routing table (`internal/cliroute`), replacing the `classifierCLIPreference` literal. This joins the L1b "every launch path" list. Filed as an inbox item by C1. | removes the Go-literal preference |

### Component details

**C2, lineage-keyed reuse:**

- When each live candidate's lineage has a classification, the prior lineage → tier decisions stay. `PromoteLatest` then picks the newest version, with zero LLM calls.
- A Signal Center line and a ledger `catalog_new_lineage` entry announce a new lineage, with an inbox suggestion.
- Gemini 4 Pro is in the same lineage as 3.1 Pro (`gemini-pro-(high)`), so the promotion is automatic. The `tiers` decision of the routing table decides if it *runs*.

**C3, always checked, always adopted:**

- A family whose live listing failed keeps its prior pick and writes an abnormal event (`catalog_refresh_degraded`). Its pick never goes down to `detect`.
- The refresh result shows the old → new model of each family.
- **A refresh with zero live classifications does not commit and does not change `FetchedAt`**, for the live and the shadow catalog. Today, a kept prior pick still stamps `FetchedAt`, and that blocks a retry for the 24h TTL.
- C3 moves this predicate into `modelcatalog` (for example `Catalog.LiveCount()`). The verb (`cmd_models.go` `countLiveAndFallback`) and the cycle-start path (`runStagedCatalogRefresh`) both use it.
- Interim split (C1 architecture review, W1): C1 (`dev/cl-models38`) puts the rule in the verb only. The cycle-start path gets it with C3.

**C4, boundary CLI update:**

- The families are the `clis` of the routing table. Until L2, they are each installed family whose `doctor live` boots.
- The updater argv is manifest data (`update_argv`, for example `["claude","update"]`, `["agy","update"]`), not Go.
- The verb records the version before and after. Then it does a smoke test with `evolve doctor live <driver>`.
- A failed smoke test halts the next wave with a loud record that names the family and the version.
- `cli-version-freeze` continues to freeze CLIs *in* a wave. The boundary updater causes an expected version drift. The record holds it, and it gives no warning.

## Landing order

1. **This boundary:** C1, together with the floor-budget P0 and PRs #769 and #770.
   - After the train, rebuild the plane and run `evolve models refresh`.
   - Check `evolve models list`: agy fast/balanced show Gemini 3.8 Flash.
2. **Next boundary:** C2 and C3. C3 is the policy flip. It lands only after C2, so a version bump does not use the expensive path.
3. **In parallel, its own boundary:** C4.
4. **After that:** C5. C6 rides cli-routing L1b.

## Verification

- **C1:**
  - a classifier-fallback test (first CLI fails, second classifies);
  - a loud-degradation test;
  - a live smoke into a scratch evolve dir shows agy fast/balanced = Gemini 3.8 Flash.
- **C2:**
  - a fixture `/model` list where only a version changed (3.7 → 3.8) reuses the prior decisions with **0 classifier calls** and promotes to 3.8;
  - a list with a new lineage calls the classifier exactly once and announces the lineage;
  - `TestPromoteLatest_*` stays green.
- **C3:**
  - a family whose live list fails keeps its prior live pick (never `detect`) and writes `catalog_refresh_degraded`;
  - a boot-time refresh is TTL-gated (open, not built);
  - the checked-in policy resolves to `enforce`;
  - a refresh with zero live classifications writes neither catalog and keeps `FetchedAt`, on the live and the shadow catalog. Thus the next cycle start tries again, and does not wait 24h;
  - the verb (`countLiveAndFallback`) and the cycle-start path (`runStagedCatalogRefresh`) both decide this through the one `modelcatalog` predicate (`Catalog.LiveCount()` or its final name), not two copies.
- **C4:**
  - with a fake updater: version before and after recorded, smoke run, a smoke failure halts the next wave;
  - a dry run changes nothing;
  - a family not in `clis` is never updated;
  - the freeze check still blocks a mid-wave change.
- **C5:** a fake pane that reports a different model writes `dispatch_model_mismatch`.
- **Live**, after C3 and C4: the next wave's dispatch lines show agy at Gemini 3.8 Flash. A future agy release with a newer Flash is picked up at the next boundary with no code change.

## C4 landing notes (2026-10-05)

C4 landed on branch `feat/model-currency` (console lane `dev/cl-modelcurrency`). The package notes are [internal-cliupdate.md](../architecture/packages/internal-cliupdate.md); the operator rows are in [runtime-reference.md](../operations/runtime-reference.md) ("Operator verbs" and "Boundary CLI update").

### What landed, one component per step, red first

| Step | Change | Tests |
|---|---|---|
| Manifest data | `Manifest.UpdateArgv` (`update_argv`): `claude-tmux.json` `["claude","update"]`, `agy-tmux.json` `["agy","update"]`; every other manifest declares none | `TestUpdateArgv_ClaudeAndAgyDeclareTheirOwnUpdater`, `TestUpdateArgv_EveryOtherManifestDeclaresNoUpdater`, `TestUpdateArgv_ParsesFromManifestJSON` |
| `internal/cliupdate` | `Update` (taking the record history), `Plan`, the typed `Result`/`Report`, `Unwalled` (was `Subscribed` until the review round), `Exec`, `GroupRunner`, and the `.evolve/cli-updates.json` record (`Remember`, `LoadRecords`, `CauseOf`) | 44 tests, 100% statement coverage |
| Version probe reuse | `looppreflight.CLIVersion` is the readiness gate's own `<bin> --version` probe, exported; the inventory and the updater both call it | `TestCLIVersion_ReadsTheFirstVersionToken` |
| Expected drift | `cli-version-drift` passes a change only boundary updates explain, warns `self-updated, smoke OK` on a chain through a found self-update, and still warns on any other change | `TestVersionDrift_AChangeTheBoundaryUpdaterRecordedIsExpectedNotDrift` and four siblings; `TestRun_VersionFreeze_ABoundaryUpdateRecordDoesNotUnfreezeASelfUpdater` |
| The verb | `evolve cli update [--dry-run] [--json] [--project-root P]` (`cmd_cli.go`, registered as `cli`) | `TestCLIUpdate_*`, `TestManifestUpdateFamilies_*`, `TestCredentialWall_OnlyACredentialBenchIsAWall`, `TestLiveCheck_ProbesTheFamilysTmuxDriverAndNamesAFailure` |
| Loop wiring | boot: `prepareFreshBatch` → `bootCLIUpdateHalts`, before `loopPreflightHalts`; wave boundary: `probeSyncAndPublish` → `updateCLIsAtBoundary`, after the plane sync | `TestRunLoop_TheBootCLIUpdateRunsBeforeThePreflightGate`, `TestRunLoop_ASmokeFailureAtBootHaltsBeforeAnyCycleNamingTheFamilyAndVersion`, `TestPrepareIteration_ASmokeFailureAtAWaveBoundaryHaltsTheNextWave` and four siblings |

### Decisions taken while building

- **Which families, until L2.** Each installed `*-tmux` family with an updater, with two exceptions. A family on a cli-health credential-wall bench is out (the `Eligible` seam, no probe spent). A family whose `doctor live` probe fails is out (the `Probe` seam), and the smoke of a found version change replaces that probe.
  - **L2 replaces `Eligible` with membership in `cli_routing.clis` of the routing table** (PR #770, L1a, is not merged). Then the `Probe` of each boundary is not necessary. (The 2026-10-06 review round, W2, split the single `Subscribed` seam.)
- **codex declares no updater.** The repo knows its updater, `brew upgrade --cask codex` ([incident](../incidents/2026-10-05-codex-update-menu.md)), but a brew pin refuses it, it is not one argv, and codex has no subscription. codex reports `no-updater`.
- **Only a smoke failure halts the next wave.** The verb exits 1 on `update-failed` or `smoke-failed`. In the loop, an updater error with a passing smoke is a WARN, and the wave runs on the proven CLI. The rule of the operator: the loop recovers a process failure that has a working fallback, and does not block it. A failed smoke means that the CLI of the next dispatch is broken. Each version that the updater can touch gets a smoke test: a change, an updater error, or a new version that the verb cannot read.
- **The freeze check was not relaxed by the records.** `cli-version-freeze` reads pin and updater state, never versions, so a record cannot relax it; it keeps freezing CLIs within a wave. The check that compares versions is `cli-version-drift`. The 2026-10-06 scope note below extended the freeze to agy.
- **One update per boundary.** The first iteration of a process leaves the update to boot. A process that re-execs at a wave boundary skips its boot update, because the boundary that re-exec'd it ran one. `--resume` never updates.
- **No off switch.** The repo adds no feature flags. To hold a family at its version, an operator puts a full manifest copy without `update_argv` in `.evolve/bridge-manifests/<family>-tmux.json`, the bridge's existing override directory.

### Scope note: agy self-updates outside the boundary (2026-10-06)

**What happened.** agy updated itself outside any boundary, 1.2.14 → 1.2.16 → 1.2.17 in about a day.

Then the readiness gate of wave 62 halted on `bridge-boot`: `agy-tmux rc=80 ExitREPLBootTimeout`. The pane was blank after `sandbox-exec … agy --dangerously-skip-permissions`. About a minute later `evolve doctor live agy-tmux` passed, and auto-respond answered a trust prompt. `cli-version-freeze` knew only codex and claude as self-updaters, so nothing froze agy. That broke the C4 invariant: CLIs are frozen in a wave and move only at the boundary.

**Research (read-only, 2026-10-06, agy 1.2.17).** No `agy update` ran and no agy setting was written.

| Looked at | Found |
|---|---|
| `agy --help`, `agy help update`, `agy help install` | no `config` subcommand and no update flag; `update  Update CLI` takes no flags |
| `~/.gemini/antigravity-cli/settings.json` | only `model` and `trustedWorkspaces`: no update key |
| `~/.gemini/antigravity-cli/updater/` and `last_check.timestamp` | the updater's own state (`update_status.json`, `update.lock`) |
| strings of `~/.local/bin/agy` | a launch-time updater (`jetski/cli/updater`: `CheckForUpdate`, `TriggerUpdateAndRelaunch`, "Last check was less than 15 minutes ago, skipping update", "Installing update...") and its off switch: "Auto-update disabled via environment variable %s" with `AGY_CLI_DISABLE_AUTO_UPDATE` |

So agy checks for an update on launch (at most once per 15 minutes), installs it and relaunches itself. The blank pane at wave 62 fits a launch that was busy doing exactly that. The off switch exists, so the scope note's branch 2 applies, and its ask 4 (detection before preflight) was built too.

**What landed (each red first):**

| Step | Change | Tests |
|---|---|---|
| Off switch as manifest data | `Manifest.AutoUpdateOffEnv` (`auto_update_off_env`). `agy-tmux.json` and `agy.json` declare `AGY_CLI_DISABLE_AUTO_UPDATE` and set it to `1` in `default_env`. Thus each realized agy launch carries it: phases, headless, the boot of the readiness gate, `doctor live`, the canary | `TestAgyManifests_EveryLaunchRealizesTheSelfUpdaterOff`, `TestAutoUpdateOffEnv_EveryDeclaredOffSwitchIsSetInDefaultEnv` |
| The recipe boot | the recipe boot (`EnsureSession`) exports the realized env after its `cd`, as the tmux boot does (claude's and codex's `/model` capture). This row first claimed it covered agy's listing; it does not, see the review round below | `TestRecipeBoot_ExportsTheRealizedEnvBetweenTheCdAndTheLaunch`, `TestRecipeBoot_AClaudeModelCaptureLaunchesWithTheRealManifestsDefaultEnv` |
| Freeze evidence | `defaultSelfUpdateEvidence` first reads the `<bin>-tmux` manifest. A declared off switch that `default_env` sets is frozen at the source (the precedent is `autoUpdates: false` of claude). An off switch that is not set is a risk. The halt names the manifest fix, never `brew pin agy`: brew did not install agy, so the pinned lister cannot freeze it | `TestDefaultSelfUpdateEvidence_AManifestThat*`, `TestRun_VersionFreeze_TheShippedAgyManifestFreezesAgyWithoutABrewPin`, `TestRun_VersionFreeze_AnAgyManifestWithoutItsOffSwitchHaltsNamingTheManifestFix` |
| Absorb what still moves | The record holds accepted versions (`baseline`, `update`, `self-update`). A version that is not in the last record of the family gets a smoke boot before its updater runs. The report is `self-updated`. A failed smoke halts, and the record does not keep it. `cli-version-drift` reports it `self-updated, smoke OK` | `TestUpdate_AnUnrecordedVersionChangeIsSmokeBootedBeforeTheUpdaterRuns` and three siblings. `TestVersionDrift_ASelfUpdateTheBoundarySmokeBootedIsReportedAsSelfUpdatedSmokeOK`. **`TestRunLoop_AnUnrecordedAgyVersionChangeIsSmokeBootedBeforeThePreflightGate`** is ask 4 of the scope note. At loop boot, the boundary smoke is the first launch of the self-updated agy, before the readiness gate |

With the switch set, `agy update` at a boundary is the only loop path that moves agy's version. The operator's own `agy` sessions outside the loop still self-update; the found-change step absorbs those.

### Architecture review fix round (2026-10-06, FIX_THEN_MERGE)

| # | Finding | Fix | Tests (each red first: a build red, then the fix reverted shows the failure) |
|---|---|---|---|
| CRITICAL | The off switch reached only realized launches. In the live refresh, `modelquery.AgyLister` (`agy models`) and `HelpEffortLister` (`agy --help`) list agy. These are bare `exec`s at cycle start, in a wave. The version probe (`agy --version`) and the version and deep probes of the bridge doctor were also bare | One source, `bridge.ProcessEnv(bin)`: the process env plus the `default_env` of the `<bin>-tmux` manifest. `modelquery.UseProcessEnv` is the exec seam, and the `init` of `cmd/evolve` installs it. Thus the call sites of `cmd_models_live.go` on the C1 branch do not change. `looppreflight.execVersion` and `bridge.doctorVersion`/`doctorDeep` use it | `TestAgyLister_TheDefaultRunnerExecsAgyModelsWithTheInjectedProcessEnv`, `TestHelpEffortLister_TheDefaultRunnerExecsAgyHelpWithTheInjectedProcessEnv`, `TestLiveRefreshListers_ExecAgyWithItsManifestDefaultEnv` (production `DefaultRouter`/`DefaultEffortListers`), `TestCLIVersion_TheRealProbeRunsAgyWithItsSelfUpdaterOff`, `TestProcessEnv_*`, `TestDoctorVersion_ProbesTheBinaryWithItsFamilyManifestEnv`, `TestDoctorDeep_TheHeadlessAgyProbeRunsWithItsFamilyManifestEnv` |
| W1 | The updater timeout killed only the direct child; a grandchild holding stdout kept `Wait` blocked (`sh -c "sleep 12 & sleep 60"` with 1 s returned after 60 s) | `cliupdate.GroupRunner`: `Setpgid`, `cmd.Cancel` kills the process group, `WaitDelay` 2 s; a deadline reports `-1, context.DeadlineExceeded` | `TestGroupRunner_ADeadlineKillsTheUpdatersWholeProcessGroup` (old runner: failed after 1m0.007s) |
| W2 | The `doctor live` subscription probe ran before the version read. Thus a self-updated CLI that no longer boots was `skipped`, and nothing checked it again before the next wave | The version read is first. The smoke of a found change is the probe, and its failure is `smoke-failed` (halt). `Seams.Subscribed` is split into `Eligible` (low cost: walls; L2: `cli_routing.clis`) and `Probe`. A skip after a probe failure is a `[loop] WARN:` line | `TestUpdate_AFoundChangeIsItsOwnProbe_ABrokenSelfUpdateHaltsInsteadOfBeingSkipped`, `TestPrepareIteration_AProbeFailureSkipIsALoudWarn` |
| W3 | A mid-wave `evolve cli update` was recorded as a boundary `update` and passed the drift check as expected | the verb refuses with exit 2 while `runlease.LiveRuns(<evolve-dir>/runs)` is non-empty, unless `--dry-run` | `TestCLIUpdate_RefusesWhileARunHoldsALiveLeaseUnlessDryRun` |
| N1 | A boundary-refresh re-exec at wave 0 hands off `WavesDone=0`, so the new process ran the boot update again | `loopchain.TakeHandoff` returns `taken`; `takeReexecHandoff` returns `(done, handedOff)`; the boot update skips on `loopConfig.HandedOff` | `TestTakeHandoff_ReportsAHandoffAtWaveZeroApartFromNoHandoff`, `TestBootCLIUpdate_AReexecHandoffLeavesTheUpdateToTheBoundaryThatRanIt` (waves 0 and 2) |
| N2 | An operator who exports the off switch can silently block `agy update` | `Family.AutoUpdateOffEnv` (from the manifest); the env of the updater drops it | `TestExec_TheUpdaterRunsWithoutItsFamilysAutoUpdateOffSwitch` |
| N3 | An unreadable version before the update returned `update-failed` without checking the CLI | it is smoke-tested: `smoke-failed` halts, a passing smoke is `update-failed … smoke OK; the updater did not run` | `TestUpdate_AnUnreadableVersionBeforeTheUpdateIsSmokedAndRunsNoUpdater`, `TestUpdate_AnUnreadableVersionWhoseSmokeFailsHalts` |
| N4 | A cancelled smoke read as `smoke-failed` (a HALT) and the verb exited 1 on SIGINT | every cancellable step maps `ctx.Err()` to `skipped: interrupted`; a skipped result is never recorded; the verb exits 130 | `TestUpdate_AnInterruptIsNeverASmokeFailure` (4 points), `TestRemember_ASkippedFamilyIsNeverRecorded`, `TestCLIUpdate_AnInterruptExitsOneThirtyNotOne` |
| N5 | An old X → Y record can explain a new X → Y that is not recorded, after a rollback | the drift check keeps only records written after the last save of the `cli-versions.json` baseline | `TestVersionDrift_ARecordOlderThanTheBaselineExplainsNothing` |
| N6 | The boundary's worst-case stall was undocumented | below, and in runtime-reference's row | — |

### Open after C4

- The pre-update `doctor live` probe costs one trivial prompt per subscribed family per boundary until L2 replaces the selection with `cli_routing.clis`.
- **Worst-case boundary stall (N6).** Families run in sequence. Each family can take three steps:
  - a 4-minute probe or found-change smoke (the bound of `liveProbeWith`);
  - a 10-minute updater (`updaterTimeout`, now a hard bound through the process group);
  - a 4-minute smoke after the update.

  That is about 18 minutes per family. The 22 minutes of the review counted the probe and the found-change smoke both; since W2, a found change replaces the probe. With claude and agy, the worst case is about 36 minutes at one boundary. When nothing changed and both probes answer, it is seconds. L2 removes the probe. A budget for each boundary is not built.
- The non-interactive behavior of `agy update` is not verified on a live host, because no real updater ran in this change. If it prompts, it reads EOF on its closed stdin. Then the family reports `update-failed` with a passing smoke, which is a WARN, not a halt.
- `claude update` on a claude that brew installed and pinned can refuse. That is an `update-failed` WARN at each boundary, until the operator unpins the family or holds it as above.
- The exact value parsing of `AGY_CLI_DISABLE_AUTO_UPDATE` is not read. The binary shows only the name and "disabled via environment variable". Thus the value is `1`, which is true for a non-empty check and for a boolean parse. It is also not known if the explicit `agy update` obeys it. The boundary runs `agy update` from the environment of the loop, which does not set it.
- (Resolved in the review round, W2.) The subscription probe was the first launch of a self-updated CLI at loop boot. Now the version read is first, and the found-change smoke is the first launch. Thus a slow or broken first launch is `smoke-failed`, not a skip.

## C3 landing notes (2026-10-27)

The operator moved the catalog refresh from `shadow` to `enforce` on 2026-10-27. C2 is not built. The flip does not add LLM calls, because `shadow` already ran the full refresh at each stale cycle start. The flip changes only the file that the refresh writes.

**What changes in a live cycle.** At the cycle start, `planCycle` calls the refresher (`go/internal/core/cyclerun.go`). When `.evolve/model-catalog.json` is older than 24 hours, the refresh lists and classifies the models of each ready CLI. At `enforce` it commits the result to the live catalog through `modelcatalog.Commit`, and dispatch uses the new picks. At `shadow` it wrote only `.evolve/model-catalog.shadow.json`.

**What landed (each red first):**

| Guard | Change | Tests |
|---|---|---|
| The checked-in policy resolves to `enforce` | `.evolve/policy.json` `catalog` block | `TestTheCheckedInPolicy_EnforcesTheStagesTheOperatorMoved` |
| One predicate for zero live classifications | `modelcatalog.Catalog.LiveCount`. The verb (`countLiveAndFallback`) and the cycle-start path (`runStagedCatalogRefresh`) both use it. | `TestCatalog_LiveCountCountsOnlyTheLiveEntries` |
| A refresh with zero live classifications writes neither catalog | `runStagedCatalogRefresh` prints one WARN line and returns. `fetched_at` stays, so the next cycle start retries. | `TestRunStagedCatalogRefresh_ZeroLiveClassificationsWriteNeitherCatalog` |
| A family whose live list failed keeps its prior live pick | `modelcatalog.KeepLivePicks`, called by `commitKeepingLivePicks` at `enforce`. The log line names `catalog_refresh_degraded` and the family. | `TestKeepLivePicks_AFamilyWhoseListFailedKeepsItsPriorLivePick`, `TestRunStagedCatalogRefresh_EnforceKeepsThePriorLivePickOfAFamilyWhoseListFailed` |

**Open after this step.**

- The refresh does not run at loop boot. It runs at each cycle start only.
- `catalog_refresh_degraded` is a WARN line in the phase log, not a ledger or Signal Center event.
- C2 (lineage-keyed reuse) is not built. A version bump thus calls the classifier once, as it did at `shadow`.

## C5 landing notes: the agy-claude family slice (2026-10-07)

This slice landed with the L2 fix round of the CLI routing table (console lane `dev/cl-routingl2`). The reason: L2 moves each deep and top dispatch onto `agy-claude-tmux` first. For a `--model` that it does not know, agy boots its default Gemini model. The design and decisions are in [cli-routing-table-2026-10.md](cli-routing-table-2026-10.md) ("L2 fix round"). The package notes are [internal-bridge.md](../architecture/packages/internal-bridge.md).

**What landed.**

- **Rule:** `launch_model_verification: {label_wait_s: 10}` on `agy-claude-tmux.json`, a typed manifest field the loader validates (it needs `model_family` and `model_label_regex`).
- **Check:** after the REPL boots and before the prompt (a boot smoke too), the bridge does these steps:
  - It reads the pane once a second for up to the wait, and ticks the auto-responder.
  - It reads the bottom six lines with `model_label_regex`. The label on the footer-prefix line decides. No other label counts.
  - No prefix line means that the label is not readable, which gives 87.
  - A different family is a mismatch only at the deadline. The delta round showed that a decision on the first label, or on the last line only, failed over healthy boots.
  - Preflight warns on 87. It does not halt.
- **Fix round 7:** the named `footer` group of the regex is the prefix. A footer label that names two families is no evidence under `modelquery.FamiliesIn`. Before round 6, a toast that named both families, or an output line that ended in `(High)`, verified a Gemini boot. Before round 7, a Claude toast below a Gemini footer did.
- **On a mismatch, or no readable label at the deadline:** the bridge kills the session (a named session explicitly). The launch exits 87 (`ExitModelMismatch`, a default walk trigger), so the walk moves to Claude Code. `BRIDGE_DISPATCH_MODEL_MISMATCH` names the expected and observed model, the label, the cli and the phase.
- **Benches:** the exit does not bench a family by itself. The usage-evidence decorator queries the usage of the family, as for each exit that a quota can explain. An exhausted Claude group benches `agy-claude`; then agy boots its Gemini default, which is the decision of the console. A REPL that stops during the wait exits 81 and keeps the fresh-session retry.

**Where it departs from the row above.** The row says a mismatch is "never a block". The console decided otherwise for this slice. A deep gate that runs the wrong model family is an integrity failure, not a format failure. Thus the recovery rung is failover to the next CLI, and the cycle continues on Claude Code. 

The event is still emitted. The module prefix of the registry makes the event `BRIDGE_DISPATCH_MODEL_MISMATCH`.

**Tests:**
- `launch_model_verification_test.go`, with the real panes of the delta review as fixtures:
  - a wrong family fails over at the deadline and signals;
  - healthy panes with a toast, an extra footer segment or the `(High)` style verify;
  - the family that arrives during the wait verifies;
  - a dialog gets an answer and is no proof;
  - a boot smoke verifies;
  - a label that is not readable fails over;
  - a REPL that stopped exits 81;
  - the loader refuses a rule that it cannot run.
- the mismatch tests of `usageevidence`;
- the seat guards of cmd/evolve, which turn red when the rule is removed.

**What remains of C5.**

1. **Model, not family.** The slice compares the model family. A label with the right family passes, also when its model is not the catalog pick for the tier. Examples: Sonnet for an Opus dispatch, or an older version. Currency needs a comparison of the label with the catalog model for the tier of the dispatch.
2. **Other targets.** `agy-tmux` (Gemini seats) declares no rule. `claude-tmux` has no footer label: its check reads the `message.model` of the transcript, which is not built.
3. **The floor.** A floor seat routes to agy-claude only after L1c judges the floor by model family. The guard for that day is in place.


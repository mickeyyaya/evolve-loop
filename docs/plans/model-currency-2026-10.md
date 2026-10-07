# Model currency: every dispatch runs the latest model of its line (2026-10)

> Status: approved by the operator on 2026-10-05. The operator's words: "make sure the model and version will always be checked and point to the latest model".
>
> - **Decision:** a newer CLI version found at a wave boundary is installed **automatically at that boundary**, smoke-tested, and the catalog is then refreshed.
> - **Extends:** [model-discovery-and-catalog.md](../architecture/model-discovery-and-catalog.md).
> - **Decision record:** ADR-0120, which lands with C2.

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
| C2 | **Lineage-keyed reuse** | The reuse gate keys on the **set of lineages** (version-free `LineageKey`s) plus the policy, not on the exact candidates. When every live candidate's lineage was already classified, the prior lineage → tier decisions are reused and `PromoteLatest` picks the newest version, with zero LLM calls. The classifier runs only for a **new lineage**. A new lineage is announced as a Signal Center line and a ledger `catalog_new_lineage` entry, with an inbox suggestion. Gemini 4 Pro would be same-lineage with 3.1 Pro (`gemini-pro-(high)`), so it is promoted automatically; whether it *runs* is the routing table's `tiers` decision. | B3 |
| C3 | **Always checked, always adopted** | `catalog.refresh_stage: enforce` in the checked-in policy, as the operator decided. The refresh runs at **loop boot** and at cycle start (TTL-gated). Adoption is **per family** and only from a live listing. A family whose live listing failed keeps its prior pick and writes an abnormal event (`catalog_refresh_degraded`); its pick is never downgraded to `detect`. The refresh result shows each family's old → new model. **A refresh with zero live classifications neither commits nor advances `FetchedAt`**, for the live and the shadow catalog alike: today keeping the prior pick still stamps `FetchedAt`, and that blocks a retry for the 24h TTL. C3 moves this predicate into `modelcatalog` (for example `Catalog.LiveCount()`), and both the verb (`cmd_models.go` `countLiveAndFallback`) and the cycle-start path (`runStagedCatalogRefresh`) use it. Interim split (C1 architecture review, W1): C1 (`dev/cl-models38`) puts the rule in the verb only; the cycle-start path gets it with C3. | B4 (catalog half) |
| C4 | **Boundary CLI update** | New verb `evolve cli update [--dry-run] [--json]`. For each family the operator subscribes to (the routing table's `clis`; until L2, every installed family whose `doctor live` boots), it runs the family's updater. The updater argv is manifest data (`update_argv`, e.g. `["claude","update"]`, `["agy","update"]`), not Go. The verb records the version before and after, then smoke-tests with `evolve doctor live <driver>`. The loop's wave-boundary step runs it **before** the catalog refresh. A failed smoke test halts the next wave loudly, and the record says which family and version. `cli-version-freeze` keeps freezing CLIs *within* a wave; version drift caused by the boundary updater is expected and recorded, not warned. | B4 (CLI half) |
| C5 | **Launch verification** | After a pane boots, the model it reports (agy's banner/footer model; claude's transcript `message.model`) is compared with the catalog's pick for that tier. A mismatch is an abnormal event (`dispatch_model_mismatch`) naming expected and observed, never a block: the policy is "logic, not format". **The agy-claude family slice landed 2026-10-07 and fails over instead of only reporting; see "C5 landing notes" below.** | proves currency on every dispatch |
| C6 | **Classifier routing** | The model-classifier launch resolves its CLI through the CLI routing table (`internal/cliroute`), replacing the `classifierCLIPreference` literal. This joins the L1b "every launch path" list. Filed as an inbox item by C1. | removes the Go-literal preference |

## Landing order

1. **This boundary:** C1, together with the floor-budget P0 and PRs #769 and #770.
   - After the train, rebuild the plane and run `evolve models refresh`.
   - Check `evolve models list`: agy fast/balanced show Gemini 3.8 Flash.
2. **Next boundary:** C2 and C3. C3 is the policy flip and lands only after C2, so adoption never runs on the expensive path for a version bump.
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
  - a boot-time refresh is TTL-gated;
  - the checked-in policy resolves to `enforce`;
  - a refresh with zero live classifications writes neither catalog and leaves `FetchedAt` as it was, on the live and the shadow catalog alike, so the next cycle start retries instead of waiting 24h;
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

- **Which families, until L2.** Every installed `*-tmux` family with an updater, minus a family on a cli-health credential-wall bench (the `Eligible` seam, no probe spent), minus a family whose `doctor live` probe fails (the `Probe` seam; a found version change's smoke stands in for it). **L2 replaces `Eligible` with membership in the routing table's `cli_routing.clis`** (PR #770, L1a, is unmerged), which also makes the per-boundary `Probe` unnecessary. (The single `Subscribed` seam was split in the 2026-10-06 review round, W2.)
- **codex declares no updater.** The repo knows its updater, `brew upgrade --cask codex` ([incident](../incidents/2026-10-05-codex-update-menu.md)), but a brew pin refuses it, it is not one argv, and codex has no subscription. codex reports `no-updater`.
- **Only a smoke failure halts the next wave.** The verb exits 1 on `update-failed` or `smoke-failed`. In the loop, an updater error whose smoke passed is a WARN and the wave runs on the proven CLI, because the operator's rule is that a process failure with a working fallback is recovered, not blocked; a failed smoke means the CLI the wave would dispatch is broken. Any version the updater may have touched (a change, an updater error, an unreadable new version) is smoke-tested.
- **The freeze check was not relaxed by the records.** `cli-version-freeze` reads pin and updater state, never versions, so a record cannot relax it; it keeps freezing CLIs within a wave. The check that compares versions is `cli-version-drift`. The 2026-10-06 scope note below extended the freeze to agy.
- **One update per boundary.** A process's first iteration leaves the update to boot; a process re-exec'd at a wave boundary skips its boot update (the boundary that re-exec'd it ran one); `--resume` never updates.
- **No off switch.** The repo adds no feature flags. To hold a family at its version, an operator puts a full manifest copy without `update_argv` in `.evolve/bridge-manifests/<family>-tmux.json`, the bridge's existing override directory.

### Scope note: agy self-updates outside the boundary (2026-10-06)

**What happened.** agy updated itself outside any boundary, 1.2.14 → 1.2.16 → 1.2.17 within about a day. Wave 62's readiness gate then halted on `bridge-boot`: `agy-tmux rc=80 ExitREPLBootTimeout`, with a blank pane after `sandbox-exec … agy --dangerously-skip-permissions`. About a minute later `evolve doctor live agy-tmux` passed, with auto-respond answering a trust prompt. `cli-version-freeze` knew only codex and claude as self-updaters, so nothing froze agy, which broke C4's invariant that CLIs are frozen within a wave and move only at the boundary.

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
| Off switch as manifest data | `Manifest.AutoUpdateOffEnv` (`auto_update_off_env`); `agy-tmux.json` and `agy.json` declare `AGY_CLI_DISABLE_AUTO_UPDATE` and set it to `1` in `default_env`, so every realized agy launch carries it: phases, headless, the readiness gate's boot, `doctor live`, the canary | `TestAgyManifests_EveryLaunchRealizesTheSelfUpdaterOff`, `TestAutoUpdateOffEnv_EveryDeclaredOffSwitchIsSetInDefaultEnv` |
| The recipe boot | the recipe boot (`EnsureSession`) exports the realized env after its `cd`, as the tmux boot does (claude's and codex's `/model` capture). This row first claimed it covered agy's listing; it does not, see the review round below | `TestRecipeBoot_ExportsTheRealizedEnvBetweenTheCdAndTheLaunch`, `TestRecipeBoot_AClaudeModelCaptureLaunchesWithTheRealManifestsDefaultEnv` |
| Freeze evidence | `defaultSelfUpdateEvidence` first reads the `<bin>-tmux` manifest: a declared off switch that `default_env` sets is frozen at the source (claude's `autoUpdates: false` precedent), an unset one is risky; the halt names the manifest fix, never `brew pin agy` (agy is not brew-installed, so the pinned lister cannot freeze it) | `TestDefaultSelfUpdateEvidence_AManifestThat*`, `TestRun_VersionFreeze_TheShippedAgyManifestFreezesAgyWithoutABrewPin`, `TestRun_VersionFreeze_AnAgyManifestWithoutItsOffSwitchHaltsNamingTheManifestFix` |
| Absorb what still moves | the record holds accepted versions (`baseline`, `update`, `self-update`); a version that differs from the family's last record is smoke-booted before its updater runs and reported `self-updated` (a failed smoke halts and is never recorded); `cli-version-drift` reports it `self-updated, smoke OK` | `TestUpdate_AnUnrecordedVersionChangeIsSmokeBootedBeforeTheUpdaterRuns` and three siblings; `TestVersionDrift_ASelfUpdateTheBoundarySmokeBootedIsReportedAsSelfUpdatedSmokeOK`; **`TestRunLoop_AnUnrecordedAgyVersionChangeIsSmokeBootedBeforeThePreflightGate`** (the scope note's ask 4: at loop boot the self-updated agy's first launch is the boundary smoke, ahead of the readiness gate) |

With the switch set, `agy update` at a boundary is the only loop path that moves agy's version. The operator's own `agy` sessions outside the loop still self-update; the found-change step absorbs those.

### Architecture review fix round (2026-10-06, FIX_THEN_MERGE)

| # | Finding | Fix | Tests (each red first: a build red, then the fix reverted shows the failure) |
|---|---|---|---|
| CRITICAL | The off switch reached only realized launches. In the live refresh agy is listed by `modelquery.AgyLister` (`agy models`) and `HelpEffortLister` (`agy --help`), bare `exec`s at cycle start, inside a wave; the version probe (`agy --version`) and the bridge doctor's version and deep probes were bare too | one source, `bridge.ProcessEnv(bin)` (the process env plus the `<bin>-tmux` manifest's `default_env`); `modelquery.UseProcessEnv` is the exec seam, installed by `cmd/evolve`'s `init`, so the C1 branch's `cmd_models_live.go` call sites stay untouched; `looppreflight.execVersion` and `bridge.doctorVersion`/`doctorDeep` use it | `TestAgyLister_TheDefaultRunnerExecsAgyModelsWithTheInjectedProcessEnv`, `TestHelpEffortLister_TheDefaultRunnerExecsAgyHelpWithTheInjectedProcessEnv`, `TestLiveRefreshListers_ExecAgyWithItsManifestDefaultEnv` (production `DefaultRouter`/`DefaultEffortListers`), `TestCLIVersion_TheRealProbeRunsAgyWithItsSelfUpdaterOff`, `TestProcessEnv_*`, `TestDoctorVersion_ProbesTheBinaryWithItsFamilyManifestEnv`, `TestDoctorDeep_TheHeadlessAgyProbeRunsWithItsFamilyManifestEnv` |
| W1 | The updater timeout killed only the direct child; a grandchild holding stdout kept `Wait` blocked (`sh -c "sleep 12 & sleep 60"` with 1 s returned after 60 s) | `cliupdate.GroupRunner`: `Setpgid`, `cmd.Cancel` kills the process group, `WaitDelay` 2 s; a deadline reports `-1, context.DeadlineExceeded` | `TestGroupRunner_ADeadlineKillsTheUpdatersWholeProcessGroup` (old runner: failed after 1m0.007s) |
| W2 | The `doctor live` subscription probe ran before the version read, so a self-updated CLI that no longer boots was `skipped`, and nothing re-checked it before the next wave | the version is read first; a found change's smoke is the probe and its failure is `smoke-failed` (halt); `Seams.Subscribed` split into `Eligible` (cheap: walls; L2: `cli_routing.clis`) and `Probe`; a probe-failure skip is a `[loop] WARN:` line | `TestUpdate_AFoundChangeIsItsOwnProbe_ABrokenSelfUpdateHaltsInsteadOfBeingSkipped`, `TestPrepareIteration_AProbeFailureSkipIsALoudWarn` |
| W3 | A mid-wave `evolve cli update` was recorded as a boundary `update` and passed the drift check as expected | the verb refuses with exit 2 while `runlease.LiveRuns(<evolve-dir>/runs)` is non-empty, unless `--dry-run` | `TestCLIUpdate_RefusesWhileARunHoldsALiveLeaseUnlessDryRun` |
| N1 | A boundary-refresh re-exec at wave 0 hands off `WavesDone=0`, so the new process ran the boot update again | `loopchain.TakeHandoff` returns `taken`; `takeReexecHandoff` returns `(done, handedOff)`; the boot update skips on `loopConfig.HandedOff` | `TestTakeHandoff_ReportsAHandoffAtWaveZeroApartFromNoHandoff`, `TestBootCLIUpdate_AReexecHandoffLeavesTheUpdateToTheBoundaryThatRanIt` (waves 0 and 2) |
| N2 | An operator who exports the off switch would silently block `agy update` | `Family.AutoUpdateOffEnv` (from the manifest); the updater's env drops it | `TestExec_TheUpdaterRunsWithoutItsFamilysAutoUpdateOffSwitch` |
| N3 | An unreadable version before the update returned `update-failed` without checking the CLI | it is smoke-tested: `smoke-failed` halts, a passing smoke is `update-failed … smoke OK; the updater did not run` | `TestUpdate_AnUnreadableVersionBeforeTheUpdateIsSmokedAndRunsNoUpdater`, `TestUpdate_AnUnreadableVersionWhoseSmokeFailsHalts` |
| N4 | A cancelled smoke read as `smoke-failed` (a HALT) and the verb exited 1 on SIGINT | every cancellable step maps `ctx.Err()` to `skipped: interrupted`; a skipped result is never recorded; the verb exits 130 | `TestUpdate_AnInterruptIsNeverASmokeFailure` (4 points), `TestRemember_ASkippedFamilyIsNeverRecorded`, `TestCLIUpdate_AnInterruptExitsOneThirtyNotOne` |
| N5 | An old X → Y record could explain a new unrecorded X → Y after a rollback | the drift check keeps only records written after the `cli-versions.json` baseline was last saved | `TestVersionDrift_ARecordOlderThanTheBaselineExplainsNothing` |
| N6 | The boundary's worst-case stall was undocumented | below, and in runtime-reference's row | — |

### Open after C4

- The pre-update `doctor live` probe costs one trivial prompt per subscribed family per boundary until L2 replaces the selection with `cli_routing.clis`.
- **Worst-case boundary stall (N6).** Families run in sequence, and each can take a 4-minute probe or found-change smoke (`liveProbeWith`'s bound), a 10-minute updater (`updaterTimeout`, now a hard bound through the process group) and a 4-minute post-update smoke: about 18 minutes per family (the review's 22 minutes counted the probe and the found-change smoke both; since W2 a found change replaces the probe). With claude and agy that is about 36 minutes at one boundary in the worst case, against seconds when nothing changed and both probes answer. L2 removes the probe; a per-boundary budget is not built.
- `agy update`'s non-interactive behaviour is unverified on a live host (no real updater ran in this change). If it prompts, it reads EOF on its closed stdin and the family reports `update-failed` with a passing smoke, which is a WARN, not a halt.
- `claude update` on a brew-installed, brew-pinned claude is expected to refuse; that is an `update-failed` WARN at every boundary until the operator unpins or holds the family as above.
- `AGY_CLI_DISABLE_AUTO_UPDATE`'s exact value parsing is unread: the binary shows only the name and "disabled via environment variable", so `1` is set (true under both a non-empty check and a boolean parse). Whether the explicit `agy update` honours it is also unread; the boundary runs `agy update` from the loop's own environment, which does not set it.
- (Resolved in the review round, W2.) The subscription probe used to be a self-updated CLI's first launch at loop boot; the version is now read first and the found-change smoke is the first launch, so a slow or broken first launch is `smoke-failed`, not a skip.

## C5 landing notes: the agy-claude family slice (2026-10-07)

Landed with the CLI routing table's L2 fix round (console lane `dev/cl-routingl2`), because L2 moves every deep and top dispatch onto `agy-claude-tmux` first, and agy boots its default Gemini model for a `--model` it does not recognize. The design and decisions are in [cli-routing-table-2026-10.md](cli-routing-table-2026-10.md) ("L2 fix round"); the package notes are [internal-bridge.md](../architecture/packages/internal-bridge.md).

**What landed.**

- **Rule:** `launch_model_verification: {label_wait_s: 10}` on `agy-claude-tmux.json`, a typed manifest field the loader validates (it needs `model_family` and `model_label_regex`).
- **Check:** after the REPL boots and before the prompt (a boot smoke included), the bridge polls the pane once a second for up to the wait, ticks the auto-responder, and reads the bottom six lines with `model_label_regex`; the label on the footer-prefix line decides; any other label never counts; no prefix line means unreadable, which means 87 (fix round 7: the regex's named `footer` group is the prefix, and a footer label naming two families is no evidence under `modelquery.FamiliesIn`; before round 6, a toast naming both families or an output line ending in `(High)` verified a Gemini boot, and before round 7 a Claude toast below a Gemini footer did); a different family is a mismatch only at the deadline (the delta round: deciding on the first label, or reading only the last line, failed over healthy boots). Preflight warns on 87 rather than halting.
- **On a mismatch or no readable label at the deadline:** the session is killed (a named one explicitly), the launch exits 87 (`ExitModelMismatch`, a default walk trigger) so the walk moves to Claude Code, and `BRIDGE_DISPATCH_MODEL_MISMATCH` names expected, observed, label, cli and phase. The exit benches nothing by itself; the usage-evidence decorator queries the family's usage as for any quota-explainable exit, and an exhausted Claude group benches `agy-claude` (agy boots its Gemini default then; the console's decision). A REPL that dies during the wait exits 81 and keeps the fresh-session retry.

**Where it departs from the row above.** The row says a mismatch is "never a block". The console decided otherwise for this slice: a deep gate running the wrong model family is an integrity failure, not a format one, so the recovery rung is failover to the next CLI, and the cycle continues on Claude Code. The event is still emitted. The registry's module prefix makes the event `BRIDGE_DISPATCH_MODEL_MISMATCH`.

**Tests:** `launch_model_verification_test.go`, with the delta review's real panes as fixtures (wrong family fails over at the deadline and signals; healthy panes with a toast, an extra footer segment or the `(High)` style verify; the family arriving inside the wait verifies; a dialog is answered and is no proof; a boot smoke verifies; an unreadable label fails over; a dead REPL exits 81; the loader refuses a rule it cannot run), `usageevidence`'s mismatch tests, and the cmd/evolve seat guards, which turn red when the rule is dropped.

**What remains of C5.**

1. **Model, not family.** The slice compares the model family. A label that shows the right family but another model than the catalog's pick for the tier (Sonnet where Opus was dispatched, or an older version) passes. Currency needs the label compared with the catalog's model for the dispatched tier.
2. **Other targets.** `agy-tmux` (Gemini seats) declares no rule, and `claude-tmux` has no footer label: its check reads the transcript's `message.model`, which is not built.
3. **The floor.** A floor seat routes to agy-claude only after L1c judges the floor by model family; the guard is in place for that day.


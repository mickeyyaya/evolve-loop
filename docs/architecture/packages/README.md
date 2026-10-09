# Package design notes

One page per Go package directory, holding what the package's code cannot say: its purpose, why it is shaped the way it is, its invariants with their reasons, and what it taught. Each page is named by the directory under `go/` with `/` replaced by `-`. The rule and its reasons are in [code-comments](../../conventions/code-comments.md). The comment-reduction workstream fills these pages as it removes knowledge from comments ([plan](../../plans/comment-reduction-2026-09.md)).

| Package | What it is | Notes |
|---|---|---|
| `internal/bridge/panestream` | reads tmux pane snapshots of an interactive LLM REPL | [internal-bridge-panestream.md](internal-bridge-panestream.md) |
| `internal/bridge/phaseidentity` | the identity statement a tmux driver appends to the pasted prompt | [internal-bridge-phaseidentity.md](internal-bridge-phaseidentity.md) |
| `internal/config` | resolves the routing configuration once, at the composition root | [internal-config.md](internal-config.md) |
| `internal/events/filter` | the one filter grammar of the event channels: the terms, the matcher, the channel selectors and the vocabulary (ADR-0127, E4) | [internal-events-filter.md](internal-events-filter.md) |
| `internal/evalgate` | the verified checks that replace prose contracts between phases | [internal-evalgate.md](internal-evalgate.md) |
| `internal/events/wake` | blocks a channel reader until the kernel posts a change, a process exit or an output hangup (kqueue, inotify); no poll (ADR-0127, E3) | [internal-events-wake.md](internal-events-wake.md) |
| `internal/fleet` | plans and runs concurrent, file-disjoint cycle lanes | [internal-fleet.md](internal-fleet.md) |
| `internal/guards` | the in-process trust kernel: the six guards `evolve guard` runs | [internal-guards.md](internal-guards.md) |
| `internal/inboxbatch` | the inbox item model and the deterministic half of task selection | [internal-inboxbatch.md](internal-inboxbatch.md) |
| `internal/inboxrank` | the one computed inbox priority: a policy-weighted, explainable score and a total order over pending items (ADR-0121) | [internal-inboxrank.md](internal-inboxrank.md) |
| `internal/inboxrank/rankinputs` | the one loader of the rank's inputs for an evolve dir: the policy's `inbox_priority` block and the recurrence ledger's item counts (ADR-0121, P2) | [internal-inboxrank-rankinputs.md](internal-inboxrank-rankinputs.md) |
| `internal/convergence` | the convergence policy's pure decision: the next rung of a repeat-until-accepted loop, the round to land, and what is deferred or filed (ADR-0126) | [internal-convergence.md](internal-convergence.md) |
| `internal/recurrence` | the recurrence ledger over lesson patterns, its escalation policy, and the snapshot read and item counts the inbox rank uses | [internal-recurrence.md](internal-recurrence.md) |
| `internal/ipcenv` | the lane protocol keys a parent evolve process sets for its children, their set (`ProtocolKeys`), and the `EVOLVE_` scrub every judging `go test` runs under | [internal-ipcenv.md](internal-ipcenv.md) |
| `internal/looppreflight` | the readiness gate `evolve loop` runs before the first wave | [internal-looppreflight.md](internal-looppreflight.md) |
| `internal/cliupdate` | updates each subscribed CLI family at a loop boundary, smoke-tests a changed version and records the change for the drift check | [internal-cliupdate.md](internal-cliupdate.md) |
| `internal/usageprobe` | reads each CLI's usage screen into typed windows through its manifest, benches every family already at a cap before a wave's first phase, and records the windows | [internal-usageprobe.md](internal-usageprobe.md) |
| `internal/usageevidence` | queries a failing CLI's usage on every failure path, records the verdict in the workspace and as a signal, and decorates the bridge with it | [internal-usageevidence.md](internal-usageevidence.md) |
| `internal/overlap` | the overlap proof of the landing queue: the zones, the closure, the evidence and the tier, with the `go list` adapter (ADR-0128, Q3) | [internal-overlap.md](internal-overlap.md) |
| `internal/phasecoherence` | drift reports between the hand-edited surfaces that define a phase | [internal-phasecoherence.md](internal-phasecoherence.md) |
| `internal/policy` | loads `.evolve/policy.json` into resolved configuration | [internal-policy.md](internal-policy.md) |
| `internal/profiles` | loads the agent profiles in `.evolve/profiles/` | [internal-profiles.md](internal-profiles.md) |
| `internal/prompts` | loads agent personas and skill docs | [internal-prompts.md](internal-prompts.md) |
| `internal/recovery` | the decisions of the Phase Recovery Pipeline | [internal-recovery.md](internal-recovery.md) |
| `internal/router` | the deterministic phase-routing kernel | [internal-router.md](internal-router.md) |
| `internal/subagent` | dispatches one subagent invocation: profile, token, bridge launch, verify, ledger | [internal-subagent.md](internal-subagent.md) |
| `internal/triagecap` | bounds the coverage floors triage may commit per cycle | [internal-triagecap.md](internal-triagecap.md) |
| `internal/cli/phasecmd` | the `evolve phase` and `evolve phases` commands | [internal-cli-phasecmd.md](internal-cli-phasecmd.md) |
| `internal/cli/guardcmd` | the `evolve guard`, commit-gate and `evolve eval` commands | [internal-cli-guardcmd.md](internal-cli-guardcmd.md) |
| `internal/topngate` | holds a build to the tasks triage selected | [internal-topngate.md](internal-topngate.md) |
| `internal/tokenusage` | measures a phase launch's token usage and context fill | [internal-tokenusage.md](internal-tokenusage.md) |
| `internal/adapters/observer` | the stall observer's core adapter and liveness probes | [internal-adapters-observer.md](internal-adapters-observer.md) |
| `internal/llmroute` | resolves the CLI and fallback chain that runs each phase | [internal-llmroute.md](internal-llmroute.md) |
| `internal/cliroute` | the one CLI routing table (`policy.json` `cli_routing`) and the resolver every launch path calls, with the legacy projection pinned by a golden | [internal-cliroute.md](internal-cliroute.md) |
| `internal/swarm` | provisions and dispatches parallel worker sessions within one phase | [internal-swarm.md](internal-swarm.md) |
| `internal/tmuxtest` | gives a test binary that runs real tmux a tmux server only its own process owns | [internal-tmuxtest.md](internal-tmuxtest.md) |
| `internal/fakeclitest` | stands in for a command-line tool in tests without writing a new executable file | [internal-fakeclitest.md](internal-fakeclitest.md) |
| `internal/adapters/ledger` | the hash-chained append-only ledger, its seals and anchors | [internal-adapters-ledger.md](internal-adapters-ledger.md) |
| `internal/ledgerartifacts` | the ledger's write-once, content-addressed evidence store (`.evolve/ledger-artifacts/sha256/<2>/<62>`) | [internal-ledgerartifacts.md](internal-ledgerartifacts.md) |
| `internal/atomicwrite` | the one crash-safe write-then-rename implementation: plain `Bytes`/`JSON` and the fsyncing `Durable` | [internal-atomicwrite.md](internal-atomicwrite.md) |
| `internal/changedpkgs` | maps a change to its packages, covering tests and importers | [internal-changedpkgs.md](internal-changedpkgs.md) |
| `internal/cyclestate` | the per-cycle state, verdicts and outcome record every phase shares | [internal-cyclestate.md](internal-cyclestate.md) |
| `internal/cycleclassify` | classifies how a cycle ended: quota pause, hang, refusal or failure | [internal-cycleclassify.md](internal-cycleclassify.md) |
| `internal/evalqualitycheck` | grades eval commands for vacuity, diversity and flaky shapes before they run | [internal-evalqualitycheck.md](internal-evalqualitycheck.md) |
| `internal/adapters/bridge` | assembles each phase prompt and turns policy.json into the bridge engine's settings | [internal-adapters-bridge.md](internal-adapters-bridge.md) |
| `internal/coherence` | checks that a cycle's recorded verdicts and artifacts agree with each other | [internal-coherence.md](internal-coherence.md) |
| `internal/reachabilityprobe` | proves a frozen test's pins stay reachable through the import graph | [internal-reachabilityprobe.md](internal-reachabilityprobe.md) |
| `internal/interaction` | records every prompt interaction and correction outcome, and promotes auto-respond rules | [internal-interaction.md](internal-interaction.md) |
| `internal/panewatch` | publishes each tmux phase pane's liveness snapshot for observers and operators | [internal-panewatch.md](internal-panewatch.md) |
| `internal/phaseobserver` | watches one running phase for stalls and dead processes | [internal-phaseobserver.md](internal-phaseobserver.md) |
| `internal/loopwave` | plans, gates and launches each fleet wave | [internal-loopwave.md](internal-loopwave.md) |
| `internal/dossier` | writes, commits and reads the per-cycle dossier record | [internal-dossier.md](internal-dossier.md) |
| `internal/gc` | decides which resources finished cycles left behind are garbage, and releases them | [internal-gc.md](internal-gc.md) |
| `internal/lanerouting` | the one routing predicate: protected surface or a path the build profile's sandbox denies | [internal-lanerouting.md](internal-lanerouting.md) |
| `internal/phasecontract` | the single registry of each phase's deliverable contract: artifact, sections, verdicts and owed files | [internal-phasecontract.md](internal-phasecontract.md) |
| `internal/treedelta` | the byte-exact change between a base and a tree, the proof a carry across a rebase rests on | [internal-treedelta.md](internal-treedelta.md) |
| `internal/triagedecision` | the one reader of the triage report as a decision: strict for the host derivation, lenient for ship | [internal-triagedecision.md](internal-triagedecision.md) |
| `internal/recoveryguard` | the kernel fence around a recovery dispatch: records the run, restores and reports what the agent touched outside its grant | [internal-recoveryguard.md](internal-recoveryguard.md) |
| `internal/phasespec` | loads, validates and merges the built-in and user phase specs into one catalog | [internal-phasespec.md](internal-phasespec.md) |
| `internal/inboxstamps` | lands or discards the loop's own writes to tracked inbox items at a sync with origin (ADR-0112) | [internal-inboxstamps.md](internal-inboxstamps.md) |
| `internal/inboxmover` | moves inbox items through their lifecycle across concurrent lanes and enforces the routing floor | [internal-inboxmover.md](internal-inboxmover.md) |
| `internal/inboxmover/lifecycle` | the pure inbox lifecycle moves: route, promote, release, recover and quarantine | [internal-inboxmover-lifecycle.md](internal-inboxmover-lifecycle.md) |
| `internal/phases/runner` | the shared phase engine: launch, fence, verify, correction ladder and host effects before the judge | [internal-phases-runner.md](internal-phases-runner.md) |
| `internal/phases/audit` | the audit phase's EGPS gate: classifies audit-report.md and acs-verdict.json into a verdict | [internal-phases-audit.md](internal-phases-audit.md) |
| `internal/phases/ship` | the native commit-and-push phase: audit-binding, EGPS gate, atomic commit+ff-merge+push | [internal-phases-ship.md](internal-phases-ship.md) |
| `internal/shipmanifest` | the one selection of which paths Ship commits: declared report manifest, porcelain parsing, the staging pathspec | [internal-shipmanifest.md](internal-shipmanifest.md) |
| `internal/deliverable` | the ADR-0100 declared-deliverables gate: verify, salvage and host effects | [internal-deliverable.md](internal-deliverable.md) |
| `internal/core/advisor` | the routing advisor that plans and re-plans a cycle's phases | [internal-core-advisor.md](internal-core-advisor.md) |
| `internal/phases/runner/verdict` | the judge: classifies a phase attempt from its artifact, pane and snapshots | [internal-phases-runner-verdict.md](internal-phases-runner-verdict.md) |
| `internal/core` | the cycle orchestrator: phase sequencing, gates, recovery and ship (filled by file group) | [internal-core.md](internal-core.md) |
| `cmd/evolve` | the composition root and CLI of the `evolve` binary (filled by file group) | [cmd-evolve.md](cmd-evolve.md) |
| `internal/bridge` | the native agent bridge: drives tmux and headless LLM CLIs for every phase (filled by file group) | [internal-bridge.md](internal-bridge.md) |
| `internal/acssuite` | the deterministic, host-side EGPS predicate-suite runner: Go lane, scope lint, phantom-binding classification, evidence sealing | [internal-acssuite.md](internal-acssuite.md) |
| `internal/acsrunner` | the single-package predicate runner behind `evolve acs run`: `go test -json` into an `acs-verdict.json` | [internal-acsrunner.md](internal-acsrunner.md) |
| `internal/acsverdict` | the one spelling of the `acs-verdict.json` name and path and of the harness-red ids that both verdict writers share | [internal-acsverdict.md](internal-acsverdict.md) |
| `internal/phases/triage` | the cycle-scope task-selection phase: prompt composition, protected-surface routing, premise drift, carry-forward candidates | [internal-phases-triage.md](internal-phases-triage.md) |
| `internal/flagregistry` | the declarative SSOT for every `EVOLVE_*` control flag across every reader surface | [internal-flagregistry.md](internal-flagregistry.md) |
| `cmd/evolve-fake-cli` | the offline stand-in for the claude, codex and agy binaries in E2E tests | [cmd-evolve-fake-cli.md](cmd-evolve-fake-cli.md) |
| `internal/auditchain` | the audit verdict as the conclusion of a seven-link reasoning chain across the phases | [internal-auditchain.md](internal-auditchain.md) |
| `internal/commentaudit` | measures, removes (`strip`) and records (`history`) comments, and proves an edit changed only comments | [internal-commentaudit.md](internal-commentaudit.md) |
| `internal/commitgate` | the pre-commit quality gate `evolve commit-gate run` runs for `/commit` | [internal-commitgate.md](internal-commitgate.md) |
| `internal/codereview` | the code-review phase's kernel half: parses the review report into defect-ledger rows, checks its grammar, and emits `REVIEW_FINDINGS` (ADR-0124) | [internal-codereview.md](internal-codereview.md) |
| `internal/qualityindex` | the shared quality index: the ten dimensions, the thresholds, the Scores and Review Plan grammars, and `Qualifies` (ADR-0124) | [internal-qualityindex.md](internal-qualityindex.md) |
| `internal/stelint` | the ASD-STE100 lint for documents and Go log text, behind `evolve docs ste-lint` and the STE WARN of the build floor | [internal-stelint.md](internal-stelint.md) |
| `internal/dashboard` | the read-only local web UI behind `evolve dashboard` | [internal-dashboard.md](internal-dashboard.md) |
| `internal/faillearn` | the kernel-owned failure floor that writes the retrospective and lesson when the retro cannot run | [internal-faillearn.md](internal-faillearn.md) |
| `internal/gitexec` | the git CLI behind one small injectable type | [internal-gitexec.md](internal-gitexec.md) |
| `internal/events/channel` | one event channel as an append-only log of segments: the locked append with torn-tail repair and rotation, and the read with the cursor rules (ADR-0127) | [internal-events-channel.md](internal-events-channel.md) |
| `internal/events/publisher` | the Signal Center listener of the event channels: routes, the lossless path, the best-effort queue and gap records (ADR-0127, E6) | [internal-events-publisher.md](internal-events-publisher.md) |
| `internal/landed` | whether a worktree's changes since a merge-base are already in `origin/main`, judged per path by a three-way merge | [internal-landed.md](internal-landed.md) |
| `internal/modelquery` | live model-catalog acquisition: each CLI's model list, classified into the canonical tiers | [internal-modelquery.md](internal-modelquery.md) |
| `internal/phases/audit/ciparitygate` | the audit phase's five CI-parity gates | [internal-phases-audit-ciparitygate.md](internal-phases-audit-ciparitygate.md) |
| `internal/phases/retro` | the retrospective phase that runs after a FAIL or WARN verdict | [internal-phases-retro.md](internal-phases-retro.md) |
| `internal/phases/specrunner` | turns a declarative phase spec into a runnable phase with no per-phase Go | [internal-phases-specrunner.md](internal-phases-specrunner.md) |
| `internal/releasepipeline` | the driver behind `evolve release X.Y.Z` | [internal-releasepipeline.md](internal-releasepipeline.md) |
| `internal/releasepreflight` | the read-only gate a release runs before any mutating step | [internal-releasepreflight.md](internal-releasepreflight.md) |
| `internal/rollback` | reverts a failed release in three independently auditable steps | [internal-rollback.md](internal-rollback.md) |
| `internal/scopedelta` | adjudicates a phase agent's out-of-scope changes on what they mean | [internal-scopedelta.md](internal-scopedelta.md) |
| `internal/setup` | the deterministic core behind `evolve setup` and `/evo:setup` | [internal-setup.md](internal-setup.md) |
| `internal/skillcheck` | renders every generated plugin surface from its single source (ADR-0040) | [internal-skillcheck.md](internal-skillcheck.md) |
| `internal/subagent/subagentrun` | the `evolve subagent run` execution path | [internal-subagent-subagentrun.md](internal-subagent-subagentrun.md) |
| `internal/wtcheckpoint` | saves uncommitted worktree work as `refs/checkpoints` snapshots, and lists, restores, prunes and pushes them | [internal-wtcheckpoint.md](internal-wtcheckpoint.md) |

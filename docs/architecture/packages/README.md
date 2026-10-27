# Package design notes

Each Go package directory has one page. The page holds what the code cannot say: the purpose, the reason for the shape, the invariants with their reasons, and the lessons. Each page is named by the directory under `go/` with `/` replaced by `-`. The rule and its reasons are in [code-comments](../../conventions/code-comments.md). The comment-reduction workstream fills these pages as it removes knowledge from comments ([plan](../../plans/comment-reduction-2026-09.md)).

| Package | What it is | Notes |
|---|---|---|
| `internal/bridge/panestream` | reads tmux pane snapshots of an interactive LLM REPL | [internal-bridge-panestream.md](internal-bridge-panestream.md) |
| `internal/bridge/phaseidentity` | the identity statement a tmux driver appends to the pasted prompt | [internal-bridge-phaseidentity.md](internal-bridge-phaseidentity.md) |
| `internal/agentmsg` | the skeleton of cycle agent messaging (ADR-0129, M1). It has the envelope, the address grammar, the identity, the token, the policy engine, the broker, the store and the frame | [internal-agentmsg.md](internal-agentmsg.md) |
| `internal/agentmsg/channelstore` | the `Store` port of the broker on the ADR-0127 channel log: the channel `msg.<cycle>`, the channel `msg.dead.<cycle>` and the `message` record variant (ADR-0129, M2) | [internal-agentmsg-channelstore.md](internal-agentmsg-channelstore.md) |
| `internal/admission` | admission by package cell: the conflict rule, the module graph and the hub cell of capacity 1 (ADR-0139 O3, ADR-0134 S14) | [internal-admission.md](internal-admission.md) |
| `internal/orchestrator` | the pool, the lane lease, the oversight loop and the supply census (ADR-0139 O4, O5, O6) | [internal-orchestrator.md](internal-orchestrator.md) |
| `internal/config` | resolves the routing configuration once, at the composition root | [internal-config.md](internal-config.md) |
| `internal/events/filter` | the one filter grammar of the event channels: the terms, the matcher, the channel selectors and the vocabulary (ADR-0127, E4) | [internal-events-filter.md](internal-events-filter.md) |
| `internal/evalgate` | the verified checks that replace prose contracts between phases | [internal-evalgate.md](internal-evalgate.md) |
| `internal/events/wake` | blocks a channel reader until the kernel posts a change, a process exit or an output hangup (kqueue, inotify); no poll (ADR-0127, E3) | [internal-events-wake.md](internal-events-wake.md) |
| `internal/events/reader` | follows event channels: arm, catch up and wait; the cursor rules, the duplicate window and the last will of a loop (ADR-0127, E5) | [internal-events-reader.md](internal-events-reader.md) |
| `internal/fleet` | plans and runs concurrent, file-disjoint cycle lanes | [internal-fleet.md](internal-fleet.md) |
| `internal/guards` | the in-process trust kernel: the six guards `evolve guard` runs | [internal-guards.md](internal-guards.md) |
| `internal/heldproposal` | the rule, the record and the operator approval of a held proposal (ADR-0157) | [internal-heldproposal.md](internal-heldproposal.md) |
| `internal/inboxbatch` | the inbox item model and the deterministic half of task selection | [internal-inboxbatch.md](internal-inboxbatch.md) |
| `internal/inboxcurator` | the deterministic inbox curator: the add-time intake, the overlap measure and the open items | [internal-inboxcurator.md](internal-inboxcurator.md) |
| `internal/inboxrank` | the one computed inbox priority: a policy-weighted, explainable score and a total order over pending items (ADR-0121) | [internal-inboxrank.md](internal-inboxrank.md) |
| `internal/inboxrank/rankinputs` | the one loader of the rank's inputs for an evolve dir: the policy's `inbox_priority` block and the recurrence ledger's item counts (ADR-0121, P2) | [internal-inboxrank-rankinputs.md](internal-inboxrank-rankinputs.md) |
| `internal/convergence` | the convergence policy's pure decision: the next rung of a repeat-until-accepted loop, the round to land, and what is deferred or filed (ADR-0126) | [internal-convergence.md](internal-convergence.md) |
| `internal/selfheal` | the incident detector of the self-healing loop: typed incident records from signals, cycle state, CI runs and usage readings; it never acts (ADR-0155, X12) | [internal-selfheal.md](internal-selfheal.md) |
| `internal/recurrence` | the recurrence ledger over lesson patterns, its escalation policy, and the snapshot read and item counts the inbox rank uses | [internal-recurrence.md](internal-recurrence.md) |
| `internal/cycleoutcome` | the closeout seam for a cycle that did not ship: the failure lifecycle of the committed ids and the planned no-work hand-over | [internal-cycleoutcome.md](internal-cycleoutcome.md) |
| `internal/bypass` | the audit of a bypass parameter: the reason and cell check, and one ledger entry and one `CONTROL_BYPASS_USED` signal for each use (X18d) | [internal-bypass.md](internal-bypass.md) |
| `internal/ipcenv` | the lane protocol keys a parent evolve process sets for its children, their set (`ProtocolKeys`), and the `EVOLVE_` scrub every judging `go test` runs under | [internal-ipcenv.md](internal-ipcenv.md) |
| `internal/looppreflight` | the readiness gate `evolve loop` runs before the first wave | [internal-looppreflight.md](internal-looppreflight.md) |
| `internal/cliupdate` | updates each subscribed CLI family at a loop boundary, smoke-tests a changed version and records the change for the drift check | [internal-cliupdate.md](internal-cliupdate.md) |
| `internal/practicewatch` | reads the practice sources of each CLI at a loop boundary and files one inbox research item for each source that changed | [internal-practicewatch.md](internal-practicewatch.md) |
| `internal/usageprobe` | reads the usage screen of each CLI into typed windows through its manifest. It benches each family at a cap before the first phase of a wave, and records the windows | [internal-usageprobe.md](internal-usageprobe.md) |
| `internal/usageevidence` | queries a failing CLI's usage on every failure path, records the verdict in the workspace and as a signal, and decorates the bridge with it | [internal-usageevidence.md](internal-usageevidence.md) |
| `internal/overlap` | the overlap proof of the landing queue: the zones, the closure, the evidence and the tier, with the `go list` adapter (ADR-0128, Q3) | [internal-overlap.md](internal-overlap.md) |
| `internal/phasecoherence` | drift reports between the hand-edited surfaces that define a phase | [internal-phasecoherence.md](internal-phasecoherence.md) |
| `internal/policy` | loads `.evolve/policy.json` into resolved configuration | [internal-policy.md](internal-policy.md) |
| `internal/profiles` | loads the agent profiles in `.evolve/profiles/` | [internal-profiles.md](internal-profiles.md) |
| `internal/prompts` | loads agent personas and skill docs | [internal-prompts.md](internal-prompts.md) |
| `internal/quotareset` | computes the wake-up time of a quota pause from the best evidence it has: override, hint, bench, configured hours or unknown | [internal-quotareset.md](internal-quotareset.md) |
| `internal/recovery` | the decisions of the Phase Recovery Pipeline | [internal-recovery.md](internal-recovery.md) |
| `internal/router` | the deterministic phase-routing kernel | [internal-router.md](internal-router.md) |
| `internal/subagent` | dispatches one subagent invocation: profile, token, bridge launch, verify, ledger | [internal-subagent.md](internal-subagent.md) |
| `internal/failurelog` | the failure floor: the failed-cycle taxonomy, the failed-approach record in `state.json`, its expiry and the carryover-todo lifecycle | [internal-failurelog.md](internal-failurelog.md) |
| `internal/triagecap` | bounds the coverage floors triage may commit per cycle | [internal-triagecap.md](internal-triagecap.md) |
| `internal/covchanged` | the changed-function coverage gate: its parsers, its measurement, its verdict and its git adapter (one engine for the verb, the target and the floor) | [internal-covchanged.md](internal-covchanged.md) |
| `internal/cli/agentcmd` | the `evolve agent run` and `evolve agent list` commands: one read-only phase agent with no cycle (ADR-0152) | [internal-cli-agentcmd.md](internal-cli-agentcmd.md) |
| `internal/cli/phasecmd` | the `evolve phase` and `evolve phases` commands | [internal-cli-phasecmd.md](internal-cli-phasecmd.md) |
| `internal/cli/guardcmd` | the `evolve guard`, commit-gate and `evolve eval` commands | [internal-cli-guardcmd.md](internal-cli-guardcmd.md) |
| `internal/topngate` | holds a build to the tasks triage selected | [internal-topngate.md](internal-topngate.md) |
| `internal/tokenusage` | measures a phase launch's token usage and context fill | [internal-tokenusage.md](internal-tokenusage.md) |
| `internal/adapters/observer` | the stall observer's core adapter and liveness probes | [internal-adapters-observer.md](internal-adapters-observer.md) |
| `internal/llmroute` | resolves the CLI and fallback chain that runs each phase | [internal-llmroute.md](internal-llmroute.md) |
| `internal/cliroute` | the one CLI routing table (`policy.json` `cli_routing`) and the resolver every launch path calls, with the legacy projection pinned by a golden | [internal-cliroute.md](internal-cliroute.md) |
| `internal/swarm` | provisions and dispatches parallel worker sessions within one phase | [internal-swarm.md](internal-swarm.md) |
| `internal/tmuxtest` | gives a test binary that runs real tmux a tmux server only its own process owns | [internal-tmuxtest.md](internal-tmuxtest.md) |
| `internal/testhost` | a missing host tool skips a local test and fails a macOS CI test | [internal-testhost.md](internal-testhost.md) |
| `internal/fakeclitest` | stands in for a command-line tool in tests without writing a new executable file | [internal-fakeclitest.md](internal-fakeclitest.md) |
| `internal/faketmux` | stands in for the `tmux` binary in hermetic e2e tests, so a tmux driver runs a full cycle with no tmux server | [internal-faketmux.md](internal-faketmux.md) |
| `pkg/acsassert` | the assertion DSL that ACS Go predicates write against | [pkg-acsassert.md](pkg-acsassert.md) |
| `internal/phaseintegrity` | the production digest source of the per-phase integrity chain and the sanctioned ship-sha re-pin | [internal-phaseintegrity.md](internal-phaseintegrity.md) |
| `internal/preflight` | probes the host once and emits the capability profile the dispatcher configures itself from | [internal-preflight.md](internal-preflight.md) |
| `internal/paths` | resolves the canonical `.evolve/*` layout once per command | [internal-paths.md](internal-paths.md) |
| `internal/panetrust` | the single trust boundary for text captured from a tmux pane | [internal-panetrust.md](internal-panetrust.md) |
| `internal/adapters/ledger` | the hash-chained append-only ledger, its seals and anchors | [internal-adapters-ledger.md](internal-adapters-ledger.md) |
| `internal/ledgerartifacts` | the ledger's write-once, content-addressed evidence store (`.evolve/ledger-artifacts/sha256/<2>/<62>`) | [internal-ledgerartifacts.md](internal-ledgerartifacts.md) |
| `internal/attemptpostmortem` | records how a phase dispatch ended: the cause, the last commands, the suspect command and the pane tail. It renders the record into the next prompt (ADR-0130, R2) | [internal-attemptpostmortem.md](internal-attemptpostmortem.md) |
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
| `internal/wavemeasure` | computes the width, the utilization and the conflict rate of one wave run from its lane intervals and landings | [internal-wavemeasure.md](internal-wavemeasure.md) |
| `internal/dossier` | writes, commits and reads the per-cycle dossier record | [internal-dossier.md](internal-dossier.md) |
| `internal/gc` | decides which resources finished cycles left behind are garbage, and releases them | [internal-gc.md](internal-gc.md) |
| `internal/lanerouting` | the one routing predicate: protected surface or a path the build profile's sandbox denies | [internal-lanerouting.md](internal-lanerouting.md) |
| `internal/phasecontract` | the single registry of each phase's deliverable contract: artifact, sections, verdicts and owed files | [internal-phasecontract.md](internal-phasecontract.md) |
| `internal/treedelta` | the byte-exact change between a base and a tree, the proof a carry across a rebase rests on | [internal-treedelta.md](internal-treedelta.md) |
| `internal/derived` | the one catalog of the generated outputs: their markers and regions, their inputs, the conflict class and the regeneration with the generator of the worktree | [internal-derived.md](internal-derived.md) |
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
| `internal/core/defectledger` | the audit phase's anti-laundering defect ledger and the continuation disposition gate (unit 09, ADR-0103) | [internal-core-defectledger.md](internal-core-defectledger.md) |
| `internal/phases/runner/verdict` | the judge: classifies one try of a phase from its artifact, pane and snapshots | [internal-phases-runner-verdict.md](internal-phases-runner-verdict.md) |
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
| `internal/events/rootwire` | subscribes one Publisher to the Signal Center of a root process and returns the closer that the root defers (ADR-0127, E10) | [internal-events-rootwire.md](internal-events-rootwire.md) |
| `internal/landed` | decides if the changes of a worktree since a merge-base are already in `origin/main`, judged per path by a three-way merge | [internal-landed.md](internal-landed.md) |
| `internal/landed` | if the changes of a worktree since a merge-base are already in `origin/main`, judged per path by a three-way merge | [internal-landed.md](internal-landed.md) |
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
| `internal/aggregator` | merges the artifacts of a fan-out phase into one canonical artifact by per-phase rules, with no model call | [internal-aggregator.md](internal-aggregator.md) |
| `internal/binaryguard` | rejects large compiled executables from a change set before they reach a commit | [internal-binaryguard.md](internal-binaryguard.md) |
| `internal/digest` | projects the role-scoped blocks out of an instruction document, never the full source | [internal-digest.md](internal-digest.md) |
| `internal/gittest` | git-backed test fixtures: creation with background maintenance off, and teardown with retry | [internal-gittest.md](internal-gittest.md) |
| `internal/phaseinventory` | builds the advisory index of every phase the pipeline can run | [internal-phaseinventory.md](internal-phaseinventory.md) |
| `internal/repostate` | which files under a directory git actually tracks, for repo-wide scanners | [internal-repostate.md](internal-repostate.md) |
| `internal/resolvellm` | resolves a phase role to its CLI and model tier from its profile | [internal-resolvellm.md](internal-resolvellm.md) |
| `internal/soakreport` | the read-only soak evidence reporter for the phase-recovery enforce flip | [internal-soakreport.md](internal-soakreport.md) |
| `internal/verifylock` | serializes the expensive go-test verification runs of one host | [internal-verifylock.md](internal-verifylock.md) |
| `test/trustkernel` | black-box tests that pin the trust-kernel safety invariants | [test-trustkernel.md](test-trustkernel.md) |
| `internal/projecthash` | computes the 8-char namespace that isolates one project's state, byte-exact with the bash pipeline | [internal-projecthash.md](internal-projecthash.md) |
| `internal/deliverable/gatesignal` | reports each contract gate decision to the Signal Center under module `gate.contract` | [internal-deliverable-gatesignal.md](internal-deliverable-gatesignal.md) |
| `internal/systemprompt` | resolves the launch-time system prompt for an agent, with the env and profile precedence chain | [internal-systemprompt.md](internal-systemprompt.md) |
| `internal/quotastate` | parses a CLI's usage pane into a structured quota snapshot in its native units | [internal-quotastate.md](internal-quotastate.md) |
| `internal/sizeratchet` | enforces the 50-line function limit across a Go module, with a shrink-only offender list | [internal-sizeratchet.md](internal-sizeratchet.md) |
| `internal/phases/secretleakscan` | the native secret-leak-scan phase: a deterministic regex scan of the added lines of a diff | [internal-phases-secretleakscan.md](internal-phases-secretleakscan.md) |
| `cmd/filter-stdout` | operator utility that writes a workspace's `<phase>-stdout.clean.txt` companion | [cmd-filter-stdout.md](cmd-filter-stdout.md) |
| `internal/kerneltest` | a rename-proof fixture of the shipped phase registry for kernel tests | [internal-kerneltest.md](internal-kerneltest.md) |
| `internal/bridge/keyspec` | classifies tmux key-spec tokens and flags likely typos without blocking a keystroke | [internal-bridge-keyspec.md](internal-bridge-keyspec.md) |
| `internal/addedtests` | derives which test packages a tree adds and the build tags each must run under | [internal-addedtests.md](internal-addedtests.md) |
| `internal/bedrock` | emits the byte-identical trust-boundary prefix of a subagent invocation, for prompt caching | [internal-bedrock.md](internal-bedrock.md) |
| `internal/textutil` | byte-based string-shortening helpers for human-facing diagnostics | [internal-textutil.md](internal-textutil.md) |
| `internal/textcap` | the two rune-cap text rules: trim and ` …[truncated]`, or the bare ellipsis | [internal-textcap.md](internal-textcap.md) |
| `internal/phases/flakererunscan` | the native flake-rerun phase: reruns a test and flags it flaky when its outcomes disagree | [internal-phases-flakererunscan.md](internal-phases-flakererunscan.md) |
| `internal/guardslog` | appends one timestamped guard-log line and returns the close error | [internal-guardslog.md](internal-guardslog.md) |
| `internal/continuation` | the continuation binding: the one schema, the per-cycle manifest, the scope-keyed registry and the release authority gate (ADR-0076) | [internal-continuation.md](internal-continuation.md) |
| `internal/routingtest` | the composable scenario framework for the dynamic phase-routing kernel: specs, bricks, matrices and kernel-floor invariants | [internal-routingtest.md](internal-routingtest.md) |
| `internal/phasetiming` | the per-phase latency record in `phase-timing.json` and its cycle roll-up | [internal-phasetiming.md](internal-phasetiming.md) |
| `internal/bridge/recipe` | scripted multi-step slash-command sequences that run through the tmux REPL of an LLM CLI | [internal-bridge-recipe.md](internal-bridge-recipe.md) |
| `internal/shipwindow` | the lease that serializes the audit-binding-to-push window across concurrent lanes | [internal-shipwindow.md](internal-shipwindow.md) |
| `internal/cyclecost` | per-phase and per-cycle cost and token totals from the phase event streams and the usage sidecars | [internal-cyclecost.md](internal-cyclecost.md) |
| `internal/consensusdispatch` | the cross-CLI consensus audit: filters the voters, writes the worker commands, fans out and aggregates by vote | [internal-consensusdispatch.md](internal-consensusdispatch.md) |
| `internal/fleetbudget` | the pure allocator that sizes a wave of fleet lanes from measured quota and pace, and the shadow join of quota with token burn | [internal-fleetbudget.md](internal-fleetbudget.md) |
| `internal/apicover` | measures the public-API coverage of a package: the exported symbols that no test names, and the named symbols that never run | [internal-apicover.md](internal-apicover.md) |
| `internal/failureadapter` | the pure decision kernel that turns recorded failures into a proceed, retry or block action | [internal-failureadapter.md](internal-failureadapter.md) |
| `internal/commitprefixgate` | refuses a commit whose conventional prefix does not match the diff scope the manifest declares | [internal-commitprefixgate.md](internal-commitprefixgate.md) |
| `internal/bridge/launchoutcome` | what one bridge launch exit means: the exit table, the cause line and the outcome projections | [internal-bridge-launchoutcome.md](internal-bridge-launchoutcome.md) |
| `internal/installer` | copies and removes the evolve-loop agents and loop skill under `~/.claude`, and validates the plugin layout in CI | [internal-installer.md](internal-installer.md) |
| `internal/core/failurelearning` | the deterministic half of learning from a failed phase: the recorder, the floor and the recurrence closure | [internal-core-failurelearning.md](internal-core-failurelearning.md) |
| `internal/core/carryover` | the lifecycle of the `carryoverTodos[]` array in `state.json`: mint admission, closeout merges, retirement and persist | [internal-core-carryover.md](internal-core-carryover.md) |
| `internal/phases/build` | the GREEN code-writer phase: its prompt blocks and its verdict classification | [internal-phases-build.md](internal-phases-build.md) |
| `internal/fanoutdispatch` | runs the commands of a TSV file as bounded parallel workers and merges their exit codes | [internal-fanoutdispatch.md](internal-fanoutdispatch.md) |
| `internal/cyclesimulator` | walks every phase of a cycle with no LLM call and appends simulated ledger entries | [internal-cyclesimulator.md](internal-cyclesimulator.md) |
| `pkg/phaseproto` | the public wire protocol between the orchestrator and out-of-process phase agents | [pkg-phaseproto.md](pkg-phaseproto.md) |
| `internal/research` | the typed, deterministic query port over the lessons corpus | [internal-research.md](internal-research.md) |
| `internal/logfilter` | writes the human-readable clean companion of a raw phase stdout log | [internal-logfilter.md](internal-logfilter.md) |
| `internal/ledgerverify` | asserts from the ledger that a cycle ran its required roles end to end | [internal-ledgerverify.md](internal-ledgerverify.md) |
| `internal/envchain` | the four-tier env-precedence chain, the typed environment getters and the registry of `EVOLVE_*` key names | [internal-envchain.md](internal-envchain.md) |
| `internal/bridge/inbox` | the file-based NDJSON channel for live command injection into a running tmux REPL agent | [internal-bridge-inbox.md](internal-bridge-inbox.md) |
| `internal/reflection` | reads, writes and validates the per-phase Reflection Journal YAML sidecar | [internal-reflection.md](internal-reflection.md) |
| `internal/budgethistory` | rolls recent cycles' timing, cost and token telemetry into the pace estimate that sizes a fleet wave | [internal-budgethistory.md](internal-budgethistory.md) |
| `internal/capability` | parses adapter capability manifests into WARN lines, the dispatch plan and the quality tier | [internal-capability.md](internal-capability.md) |
| `internal/posteditvalidate` | the PostToolUse hook that checks a just-edited json, shell or python file and warns without blocking | [internal-posteditvalidate.md](internal-posteditvalidate.md) |
| `internal/mergegate` | the merge-to-main gate's deterministic kernel: the cadence advisor and the promoter | [internal-mergegate.md](internal-mergegate.md) |
| `internal/builddiff` | derives the diff of a cycle from its recorded base and writes it into the build report | [internal-builddiff.md](internal-builddiff.md) |
| `internal/researchindex` | writes the dated entries of `docs/research-index.md` from one file for each entry | [internal-researchindex.md](internal-researchindex.md) |
| `internal/mergewindow` | decides if a PR merges in a window or at a boundary, from the CI routing table of `required.yml` | [internal-mergewindow.md](internal-mergewindow.md) |
| `internal/dispatchevents` | writes the dispatcher-level abnormal events of a cycle workspace and owns the event vocabulary | [internal-dispatchevents.md](internal-dispatchevents.md) |
| `internal/contextfillcorrelate` | joins the per-phase context fill ratio with the final verdict of each cycle and reports the FAIL rate for each fill band | [internal-contextfillcorrelate.md](internal-contextfillcorrelate.md) |
| `internal/versionbump` | updates the version markers of a release atomically, port of the shell version-bump script | [internal-versionbump.md](internal-versionbump.md) |
| `internal/verifyeval` | re-runs the commands of an eval file in a workspace and compares the outcome with its expectations | [internal-verifyeval.md](internal-verifyeval.md) |
| `internal/runscope` | the one source of the run-, cycle- and lane-scoped names of worktree branches, directories and tmux sessions | [internal-runscope.md](internal-runscope.md) |
| `internal/retrofile` | lifts the preventive actions of a retro report into weighted inbox todos | [internal-retrofile.md](internal-retrofile.md) |
| `internal/log` | JSONL sidecar events, slog channels, the operator console and the safe rendering of untrusted diagnostic fields | [internal-log.md](internal-log.md) |
| `internal/phasewatchdog` | kills the process group of a phase that shows no file activity for longer than a threshold | [internal-phasewatchdog.md](internal-phasewatchdog.md) |
| `internal/acspathscan` | the scan for the repository paths that an ACS predicate reads, used by the tdd protected-predicate route | [internal-acspathscan.md](internal-acspathscan.md) |
| `internal/committedset` | the one answer to which tasks a cycle committed to, read from the lane pin and triage's decision, and which ids triage answered for | [internal-committedset.md](internal-committedset.md) |
| `internal/treefence` | a content fence around the worktree of a read-only phase: it takes a tree snapshot and restores every change the phase made | [internal-treefence.md](internal-treefence.md) |
| `internal/verdictcache` | the content-addressed store that binds a reusable audit verdict to the tree SHA it judged (ADR-0048) | [internal-verdictcache.md](internal-verdictcache.md) |
| `internal/marketplacepoll` | polls the local marketplace checkout until its plugin version matches the release target | [internal-marketplacepoll.md](internal-marketplacepoll.md) |
| `internal/docsfloor` | the docs floor: an architecture-labeled change must touch documentation (ADR-0077) | [internal-docsfloor.md](internal-docsfloor.md) |
| `internal/phases/tdd` | the RED test-writer phase: its verdict mapping, prompt composition and artifact name | [internal-phases-tdd.md](internal-phases-tdd.md) |
| `internal/bridgechain` | the bridge decorator that walks the CLI and tier fallback chain for every launcher | [internal-bridgechain.md](internal-bridgechain.md) |
| `internal/releasetargets` | parses .goreleaser.yml, the release build matrix, into the target list and asset names the release-verify-binaries gate checks | [internal-releasetargets.md](internal-releasetargets.md) |
| `internal/skillinventory` | builds the cached skill inventory by category that Scout reads as routing hints | [internal-skillinventory.md](internal-skillinventory.md) |
| `pkg/naminguard` | the config-driven guard against dead naming tokens that a rename leaves behind; public API | [pkg-naminguard.md](pkg-naminguard.md) |
| `internal/runlease` | the per-run .lease heartbeat file shared by the retention engine and the fleet scheduler: freshness, owner liveness, write ordering | [internal-runlease.md](internal-runlease.md) |
| `internal/phases/intent` | the goal-capture phase: delta mode, the deterministic goal-hash skip and the verdict mapping | [internal-phases-intent.md](internal-phases-intent.md) |
| `internal/phases/debugger` | the ship-recovery diagnosis phase: the decision file, the recovery signals and the safe BLOCK default | [internal-phases-debugger.md](internal-phases-debugger.md) |
| `internal/changeloggen` | renders conventional-commit subjects into a Keep-a-Changelog entry and prepends it idempotently | [internal-changeloggen.md](internal-changeloggen.md) |
| `internal/phaseblock` | the content-addressed, chained digest of each pipeline phase | [internal-phaseblock.md](internal-phaseblock.md) |
| `internal/phaseconfig` | the unified phase descriptor: spec, dispatch config and swarm worker count | [internal-phaseconfig.md](internal-phaseconfig.md) |
| `internal/core/outcome` | the recording chokepoint for every terminal phase disposition | [internal-core-outcome.md](internal-core-outcome.md) |
| `internal/cliadmit` | cross-process admission control for shared LLM CLIs | [internal-cliadmit.md](internal-cliadmit.md) |
| `internal/ciwatch` | the CI watch that files a fix-forward item for a red run | [internal-ciwatch.md](internal-ciwatch.md) |
| `internal/llmcalls` | the model-try ledger and its performance index | [internal-llmcalls.md](internal-llmcalls.md) |
| `internal/core/failurediag` | the phase failure sidecar and the delivery-failure classifier | [internal-core-failurediag.md](internal-core-failurediag.md) |
| `internal/releaseconsistency` | the version-marker consistency check of a release | [internal-releaseconsistency.md](internal-releaseconsistency.md) |
| `internal/adapters/flock` | the blocking and non-blocking file-lock primitives | [internal-adapters-flock.md](internal-adapters-flock.md) |
| `internal/sysexec` | the single seam for running external commands | [internal-sysexec.md](internal-sysexec.md) |
| `internal/sessionrecord` | the per-run tmux session registry schema | [internal-sessionrecord.md](internal-sessionrecord.md) |
| `internal/pruneephemeral` | TTL retention for phase-tracker ephemeral data | [internal-pruneephemeral.md](internal-pruneephemeral.md) |
| `internal/dispositionrouter` | routes a classified failure to the operator console or the lane queue, and stages the intents | [internal-dispositionrouter.md](internal-dispositionrouter.md) |
| `internal/gopkgpattern` | the shared rule that recognizes and normalizes `go test` package patterns in Go source | [internal-gopkgpattern.md](internal-gopkgpattern.md) |
| `internal/phases/registry` | the Factory Method that maps a phase name to its runner factory | [internal-phases-registry.md](internal-phases-registry.md) |
| `internal/treestate` | computes the tree-state SHA, the sha256 of `git diff HEAD`, that binds an audit to a tree | [internal-treestate.md](internal-treestate.md) |
| `cmd/testlatency` | turns a `go test -json` stream into a Markdown latency report | [cmd-testlatency.md](cmd-testlatency.md) |
| `cmd/e2elist` | prints the anchored `go test -run` pattern of the tests that only the e2e build tag compiles | [cmd-e2elist.md](cmd-e2elist.md) |
| `cmd/darwinset` | prints `go/.darwin-set`, the packages whose code or tests differ on macOS | [cmd-darwinset.md](cmd-darwinset.md) |
| `cmd/testrusage` | `go test -exec` wrapper that appends one resource record per test binary | [cmd-testrusage.md](cmd-testrusage.md) |
| `cmd/hostsampler` | samples host CPU, memory, load and disk each second into NDJSON | [cmd-hostsampler.md](cmd-hostsampler.md) |
| `internal/codequality` | runs the deterministic gofmt gates that mirror the CI format step | [internal-codequality.md](internal-codequality.md) |
| `internal/plane` | tells the console checkout from a linked runtime worktree | [internal-plane.md](internal-plane.md) |
| `internal/contextfill` | derives how full the context window of a phase got from its token counts | [internal-contextfill.md](internal-contextfill.md) |
| `internal/solutioncheck` | the deterministic judge of the deliverable shape of a document cycle | [internal-solutioncheck.md](internal-solutioncheck.md) |
| `internal/redteamcheck` | the standing anti-gaming ledger-integrity checks | [internal-redteamcheck.md](internal-redteamcheck.md) |
| `internal/core/evidence` | the commit-as-evidence trailer that proves a phase delivered | [internal-core-evidence.md](internal-core-evidence.md) |
| `internal/routingeval` | the golden corpus that replays advisor responses through the routing floor | [internal-routingeval.md](internal-routingeval.md) |
| `internal/backfill` | rebuilds a missing phase artifact from the clean stdout of the phase | [internal-backfill.md](internal-backfill.md) |
| `internal/skilloverlay` | turns policy-resolved skill overlays into a prompt prefix | [internal-skilloverlay.md](internal-skilloverlay.md) |
| `internal/goalhash` | hashes a loop goal string, byte for byte as the legacy bash pipeline did | [internal-goalhash.md](internal-goalhash.md) |
| `examples/secure-orchestrator` | a reference program with four defenses against agent reward hacking | [examples-secure-orchestrator.md](examples-secure-orchestrator.md) |
| `cmd/evolve/cmdutil` | the small helpers shared by the `evolve` subcommand handlers: help gate, root resolution, argument reorder, env filter and prompts loader | [cmd-evolve-cmdutil.md](cmd-evolve-cmdutil.md) |
| `pkg/version` | the build-stamped binary identity: version string and build commit | [pkg-version.md](pkg-version.md) |
| `internal/phases/buildplanner` | the optional build-planner phase that moves the Builder's design step into its own session | [internal-phases-buildplanner.md](internal-phases-buildplanner.md) |
| `internal/guards/treediff` | the post-phase main-tree diff guard: flags a write that escaped the worktree | [internal-guards-treediff.md](internal-guards-treediff.md) |
| `internal/rawgitratchet` | the ratchet that keeps new raw git fixtures out of the module's tests | [internal-rawgitratchet.md](internal-rawgitratchet.md) |
| `internal/sessionreaper` | removes tmux sessions orphaned by runs whose lease is stale, found only through each run's session registry | [internal-sessionreaper.md](internal-sessionreaper.md) |
| `internal/gcpolicy` | the zero-dependency leaf for the `gc` block of policy.json: the retention engine's configuration vocabulary | [internal-gcpolicy.md](internal-gcpolicy.md) |
| `internal/doctor` | `evolve doctor probe`: finds a CLI tool on PATH and fallback locations and shows the trail | [internal-doctor.md](internal-doctor.md) |
| `internal/failuregrade` | the graduated-enforcement classifier: grades a failure to abort, correct, quarantine or repair | [internal-failuregrade.md](internal-failuregrade.md) |
| `internal/phases/swarmplan` | the optional swarm-plan phase: an LLM planner that splits a task into worker assignments | [internal-phases-swarmplan.md](internal-phases-swarmplan.md) |
| `internal/mintregistry` | the per-project record of advisor-minted phase names that lets the tree-diff guard exempt a sanctioned mint | [internal-mintregistry.md](internal-mintregistry.md) |
| `internal/directives` | the runtime operator-directives cascade: global and per-loop layers re-read each cycle and injected into phase prompts | [internal-directives.md](internal-directives.md) |
| `internal/bridge/capabilities` | the per-CLI capability catalog and its drift check against a live `/help` | [internal-bridge-capabilities.md](internal-bridge-capabilities.md) |
| `internal/detectcli` | detects which AI coding CLI drives the session, and owns the CLI-name alias table | [internal-detectcli.md](internal-detectcli.md) |
| `internal/cyclebudget` | the pure decision of when a loop batch stops: completion-driven, bounded by a safety cap | [internal-cyclebudget.md](internal-cyclebudget.md) |
| `internal/semvercheck` | the one strict `X.Y.Z` version check | [internal-semvercheck.md](internal-semvercheck.md) |
| `internal/detectnested` | detects a nested Claude Code parent from its environment beacons | [internal-detectnested.md](internal-detectnested.md) |
| `internal/dag` | deterministic dependency leveling shared by the wave planner and the merge train | [internal-dag.md](internal-dag.md) |
| `internal/phaseorder` | the pipeline phase order from the registry, with a hardcoded fallback | [internal-phaseorder.md](internal-phaseorder.md) |
| `internal/selfsha` | the sha256 of a file or of the running executable | [internal-selfsha.md](internal-selfsha.md) |
| `internal/auditledger` | the one reader of auditor rows in `.evolve/ledger.jsonl` | [internal-auditledger.md](internal-auditledger.md) |
| `internal/auditcalibration` | a calibration report of auditor narratives against ship-gate outcomes | [internal-auditcalibration.md](internal-auditcalibration.md) |
| `internal/paralleltest` | the parallel-by-default guard of the package test trees | [internal-paralleltest.md](internal-paralleltest.md) |
| `test/structure` | source-structure checks shared between package tests | [test-structure.md](test-structure.md) |
| `test/e2e` | the end-to-end test tier | [test-e2e.md](test-e2e.md) |
| `test/component` | the component test tier | [test-component.md](test-component.md) |
| `test/integration` | the integration test tier | [test-integration.md](test-integration.md) |
| `cmd/apicover` | the standalone binary over `internal/apicover` | [cmd-apicover.md](cmd-apicover.md) |

# Cloud phase cells: run the phase agents of 8 cycles as Claude Code cloud sessions (research, 2026-11)

> The plan that uses this research: [cloud-phase-cells-2026-11](../plans/cloud-phase-cells-2026-11.md). The isolation spec: [llm-isolation.md](../architecture/llm-isolation.md). The pool spec: [wave-orchestrator.md](../architecture/wave-orchestrator.md). The handoff spec: [phase-handoff-contract.md](../architecture/phase-handoff-contract.md). The decision record: none yet. The plan names the ADR that the build adds.

Date: 2026-11-03. Method: code reads with `path:line` on `origin/main` at `0e633c75c`, reads of three specs, ADR-0154 and the fleet research, and live web searches. Each local claim names its file. "Synthesis" marks our own inference.

**Terms used throughout:**

- **Phase cell:** the limits that the pipeline puts around one phase agent ([llm-isolation.md](../architecture/llm-isolation.md) §2).
- **Cloud cell:** a phase cell whose agent runs in a Claude Code cloud session, not on the host.
- **Lane:** one fleet cycle, with its own process and its own worktree.
- **Dispatch commit:** a commit that the pipeline writes and pushes, with the tree and the inputs of one phase.
- **Cell branch:** the remote branch that holds one dispatch commit and the return commits of the agent.
- **Kernel gate:** a deterministic check of the pipeline, for example the handoff reviewer, the EGPS check and the ship gate.

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. Outside practice](#2-outside-practice)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Candidate designs and scores](#4-candidate-designs-and-scores)
- [5. Refinements to adopt](#5-refinements-to-adopt)
- [6. Limits of this research](#6-limits-of-this-research)

## 1. Local evidence

### 1.1 The request

The operator wrote on 2026-11-03: "prioritize claude --cloud as highest priority ... Then full 8 cycles concurrency on claude cloud phase agent cycle". A bridge driver `claude-cloud` is in progress on the branch `cloud/bridge-claude-cloud`. That branch is not on the remote on 2026-11-03 (`git branch -r`). This study does not depend on its code. It uses only the `Driver` interface on `main`.

### 1.2 How a phase agent runs today

**F1.1** A phase runs in this order: prepare, resolve the route, dispatch, host effects, judge (`go/internal/phases/runner/runner.go:165-190`). The artifact path is `<workspace>/<artifact file>` (`go/internal/phases/runner/preparation.go:96`).
*Implication:* the runner reads the artifact from the local workspace. A cloud cell must put the artifact there before the judge runs.

**F1.2** The `Driver` interface has two methods, `Name() string` and `Launch(ctx, cfg *Config, deps Deps) (int, error)` (`go/internal/bridge/driver.go:11-15`). Each driver registers itself and has an embedded manifest (`go/internal/bridge/manifests/`).
*Implication:* a cloud driver fits the interface. `Launch` blocks until the phase ends, so the asynchronous session becomes synchronous inside the driver.

**F1.3** Completion has four modes: `artifact`, `stdout`, `git` and `worktree-evidence` (`go/internal/core/ports.go:177-180`). The `git` mode accepts a new commit only when `evidence.Parse(msg).Verify(phase, challengeToken)` passes (`go/internal/bridge/completion.go:110-159`).
*Implication:* a commit with a challenge token already proves that one seat finished. A remote variant of the `git` mode reuses this proof.

**F1.4** The tmux driver waits for an artifact for 300 s by default (`go/internal/bridge/driver_tmux_repl.go:20`). The stop reviewer extends the window up to 6 times (`go/internal/bridge/stopreview.go:64`). The exit code `81` marks an artifact timeout (`go/internal/bridge/exitcodes.go`).
*Implication:* the wait, the extension and the exit codes exist. A cloud cell needs a longer base wait, because a session first boots a VM and clones.

**F1.5** A phase gets a fresh session for each candidate CLI (`launchWithFreshSession`, `go/internal/phases/runner/dispatch.go:25-91`).
*Implication:* one session for many phases breaks this rule. Each seat needs its own cloud session.

### 1.3 The kernel gates and the trust of an artifact

**F1.6** The handoff reviewer reads the ledger in the run directory, the inbox and the evidence in the worktree. It checks that each `met` predicate is green on the build tree and was red before ([phase-handoff-contract.md](../architecture/phase-handoff-contract.md) lines 160-178).
*Implication:* the reviewer runs the predicates on a local tree. The build tree must be local before the reviewer runs.

**F1.7** The ledger field `author` "comes from the dispatch record, not from the agent" ([phase-handoff-contract.md](../architecture/phase-handoff-contract.md) line 142). Lineage and request records live under `.evolve/lineage/`, because "the repo is read-only for a cell" ([llm-isolation.md](../architecture/llm-isolation.md) lines 584, 713).
*Implication:* a trusted record is a file that the pipeline writes where a cell cannot write. A cell branch is agent-written. The pipeline must never read a trusted record from it.

**F1.8** The ship gate requires an audit entry with `GitHEAD` and `TreeStateSHA`, and an EGPS receipt with `RedCount == 0`. The current HEAD must equal the audited HEAD, or the gate re-binds the audit after a move of `main` (`go/internal/phases/ship/audit.go:27-126`, `:303`).
*Implication:* the ship gate accepts only a tree that the local audit bound. A branch from elsewhere passes only after a local fetch and a local audit.

**F1.9** The ship path pushes `git push origin <commit>:refs/heads/<branch>` (`go/internal/phases/ship/landing/push.go:91-96`). Push-only refuses commits without journal provenance (`go/internal/phases/ship/pushonly.go:64-104`).
*Implication:* the pipeline already pushes branches. A dispatch push is a second pipeline push, not an agent push.

**F1.10** The role guard allows writes in the active worktree only for `build`, `tdd`, `test-amplification` and `bug-reproduction` (`go/internal/guards/role.go:28-84`, `go/internal/phasecontract/source_writers.go:4`).
*Implication:* the same list decides which cloud cells can return source changes. Each other phase returns only its declared files.

### 1.4 The isolation invariants on a remote cell

The invariants are stated in [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) lines 31-43. The guard of each is in [llm-isolation.md](../architecture/llm-isolation.md) lines 1928-1938.

**F1.11** The bridge waits for the declared file of each seat (`Completion: artifact`) ([llm-isolation.md](../architecture/llm-isolation.md) line 1014). A seat file older than its launch is refused with `rule=flow.file.stale` (line 1036).
*Implication:* the importer must write each returned file after the launch time. A copy by the pipeline satisfies this rule.

**F1.12** The network of a local cell stays open: "A cell can reach any host" ([llm-isolation.md](../architecture/llm-isolation.md) line 1950).
*Implication:* the default cloud network ("Trusted", an allowlist, P2.3) is stricter than a local cell. The cloud cell gains on egress.

**F1.13** The read and write limits of a local cell are OS sandbox rules: `sandbox-exec` on macOS and `bwrap` on Linux ([llm-isolation.md](../architecture/llm-isolation.md) lines 772-810).
*Implication:* none of these rules reaches a cloud VM. The VM itself is the read limit, and the importer fence is the write limit (synthesis).

The table maps each invariant to a cloud cell (synthesis, from F1.2 to F1.13).

| Invariant | Local enforcement | Status in a cloud cell | What keeps it |
|---|---|---|---|
| I-1 single launcher | `go/internal/bridge/launch_llm.go` and the AST guard | kept | the `claude-cloud` driver starts `claude --cloud` through the bridge launch API |
| I-2 hermetic environment | `cellEnv` (`go/internal/bridge/hermetic_env.go`) | kept for the local `claude` process. The VM environment is the cloud environment data | an environment with no secret variables, checked by a probe |
| I-3 no nested agency | exec denies, the dispatch credential | kept. The VM holds no dispatch credential and cannot reach the host | the own-harness subagents stay allowed (ADR-0154 amendment) |
| I-4 pipeline-mediated communication | the input index | kept, if the importer copies declared files by name | the importer, and one session for each seat (F1.5) |
| I-5 read isolation | sandbox read denies | stronger for host files: the VM has no host home. Weaker for the repo: the clone has every committed file | the dispatch commit holds only indexed inputs |
| I-6 family-agnostic flow | routing data | kept. `claude-cloud` is a CLI name in routing data, not in flow code | `TestFlowCodeNamesNoFamily` |
| I-7 CLI-neutral guarantees | OS sandbox, fence, gates | the OS sandbox is gone. The fence moves into the importer. The gates stay local | the importer fence and the local gates |
| I-8 typed flow decisions | `go/internal/phasecontract/flow_decision.go` | kept. The importer writes the decision file, and the schema checks it | F1.11 |
| I-9 proven substitution | `TestE2ECycleCLIMatrix` | open until a matrix row runs a fake cloud CLI | a fake `claude` that pushes to a local bare remote |

The brief names I-1 to I-9. Later ADRs add more invariants in force ([llm-isolation.md](../architecture/llm-isolation.md) lines 1939-1946). The four that a cloud cell touches are below (synthesis).

| Invariant | Status in a cloud cell | What keeps it |
|---|---|---|
| I-11 no agent writes its own oracle | at risk: the clone holds the frozen predicates, and the agent can edit them | the importer refuses a return diff that touches the oracle paths of the seat, as the local TDD digest check does |
| I-15 the CLI is the service contract | kept: the driver calls `claude --cloud` and `git`, never MCP | the CLI contract rows |
| I-16 no ambient control | kept: the base wait and the slot cap are manifest and policy data | `TestNoControlFlagReadOutsideTheMarkerList` |
| I-17 the assignment is CLI-neutral | kept: the cloud specifics stay below the driver | `TestNoCLISpecificsAboveTheDriver` |

### 1.5 The fleet and the host

**F1.14** A lane is a local `evolve cycle run` child process with its own worktree under `.evolve/worktrees/` and the branch `cycle-<lane>-<N>` (`go/cmd/evolve/cmd_fleet.go:127-157`, `go/internal/runscope/runscope.go:43-45`). The policy sets `fleet.count: 4` and `min_lanes: 4` (`.evolve/policy.json:121-128`).
*Implication:* width 8 is a policy change. The lane process stays local in each design below.

**F1.15** No admission rule reads the host load. The lane CPU budget is `max(1, NumCPU/width)` (`go/internal/lanebudget/lanebudget.go:20-25`).
*Implication:* at width 8 on 14 CPUs, each lane gets 1 CPU for its test floors. Nothing stops a ninth heavy step.

**F1.16** At width 4 the load average reached 15 to 18 on 14 CPUs. One ship floor ran `go test` at 634% CPU ([wave-orchestrator.md](../architecture/wave-orchestrator.md) line 571). Lane floors pushed the load to 37 to 63 ([fleet-landing-at-scale-2026-10.md](fleet-landing-at-scale-2026-10.md) F2.17).
*Implication:* the floors of the kernel gates are a large part of the host load. A cloud cell moves the agent load off the host, not the gate load.

**F1.17** One Claude Code phase uses about 1.5 GB of memory ([wave-orchestrator.md](../architecture/wave-orchestrator.md) line 562).
*Implication:* 8 local Claude phases need about 12 GB. A cloud cell needs only the small local `claude --cloud` client (synthesis).

**F1.18** The serial landing limit is about 6 lanes. One landing takes about 5 minutes, and the median lane takes 31 minutes ([wave-orchestrator.md](../architecture/wave-orchestrator.md) lines 560, 565).
*Implication:* width 8 queues landings, whatever the cloud does. Width 8 needs the orchestrator-owned integration of ADR-0147 or the group commit of ADR-0134.

**F1.19** The median cycle is 58.9 minutes, with a p90 of 102.4 minutes ([fleet-landing-at-scale-2026-10.md](fleet-landing-at-scale-2026-10.md) F2.14).
*Implication:* a boot cost of a few minutes for each phase is a visible but bounded share of a cycle.

**F1.20** The repository `mickeyyaya/evolve-loop` is public (`gh api repos/mickeyyaya/evolve-loop --jq .visibility` printed `public`, 2026-11-03).
*Implication:* each cell branch is public while it exists. A dispatch commit must hold no private data.

**F1.21** The decision D1 of 2026-11-03 says: "no cell gets a credential", and the pipeline pushes and opens PRs (`docs/plans/architecture-decisions-2026-11.md` on the branch `docs/architecture-decisions-2026-11`, D1).
*Implication:* a cloud session pushes through the GitHub connection of the account. This is a credential that the cell uses, so D1 needs an explicit exception or a compensating control.

## 2. Outside practice

### 2.1 Claude Code cloud sessions

**P2.1** `claude --cloud "<task>"` starts a cloud session. `--remote` is a deprecated alias. `claude -p "<msg>" --cloud <session-id>` sends a follow-up, and `--output-format json` returns `{ok, session_id, url}` ([Claude Code on the web](https://code.claude.com/docs/en/claude-code-on-the-web)).

**P2.2** `--cloud` clones the GitHub remote "at your current branch, not your local checkout, so push first if you have local commits" (same page). Push needs a GitHub connection with push access. Branch protection and rulesets control the push.

**P2.3** The default environment has "Trusted" network access, an allowlist. A request outside the list fails with `403` and `x-deny-reason: host_not_allowed` (same page). A setup script runs at start.

**P2.4** "Cloud sessions stop after a period of inactivity and the session's VM is reclaimed" (same page). The page gives no maximum lifetime and no concurrency cap. It names no CLI verb that stops a session or reads its status.

**P2.5** `claude --teleport <session-id>` pulls a session into a local terminal. It needs a clean tree, the same repository, a pushed branch and the same claude.ai account (same page).

**P2.6** Cloud sessions "count toward your plan's usage limits alongside the rest of your Claude and Claude Code usage, and there's no separate charge for the cloud VM" (same page). Scheduled tasks draw down subscription usage the same way ([routines](https://code.claude.com/docs/en/routines)).

**P2.7** A scheduled task pushes to a branch with the prefix `claude/` "unless your prompt directs it to push to another branch" ([routines](https://code.claude.com/docs/en/routines)).

**P2.8** A user report says that about 10 sessions started at one time after a quota reset failed with "Server is temporarily limiting requests (not your usage limit)" ([anthropics/claude-code#53922](https://github.com/anthropics/claude-code/issues/53922)). This is one report, not a documented limit.

### 2.2 Other remote agents

**P2.9** The GitHub Copilot cloud agent is asynchronous. It works in an ephemeral, firewalled environment, pushes a branch and opens a PR. It cannot approve or merge its own PR ([GitHub docs](https://docs.github.com/en/copilot/responsible-use-of-github-copilot-features/copilot-coding-agent)).

**P2.10** Codex cloud runs each task in a cloud sandbox and returns a diff. `codex cloud exec --attempts N` runs best-of-N attempts, and a local `apply` step applies the chosen diff ([Codex cloud task application](https://codex.danielvaughan.com/2026/04/08/codex-cloud-task-application/), [best-of-N](https://codex.danielvaughan.com/2026/04/01/codex-cloud-exec-best-of-n-attempts/)). These are third-party guides.

**P2.11** The Captain supervisor snapshots the worktree into a dispatch commit and pushes it to a sidecar. It integrates the returned work from the recorded dispatch base ([flanksource/captain#46](https://github.com/flanksource/captain/pull/46)).

**P2.12** File-based dispatch writes the prompt, the event log and a lock file under one directory for each task. That directory is the audit trail and the replay source ([dispatch-opencode skill](https://skillselion.com/skills/cristoslc/dispatch-opencode-skill/dispatch-opencode)).

**P2.13** A sandbox job API returns a job id at once, and workers take the job from a queue ([Turing sandbox architecture](https://sandbox-task.turing.com/docs/guides/architecture/)).

*Common shape (synthesis):* snapshot the input, run in an isolated sandbox, return a diff or a branch, and verify locally before the merge. No source lets the remote agent merge.

### 2.3 A conflict in the billing facts

The operator brief says that cloud sessions bill the cloud-session credit of the account and that routines bill plan usage. P2.6 says that both count toward plan usage limits. This study does not resolve the conflict. The plan measures the usage of each session with the bridge billing snapshot (`go/internal/bridge/billing.go:22`), and it never uses routines in either case.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A dispatch commit as the input snapshot (P2.11) | adopt | it gives the session a tree that the pipeline chose, and the base of the return diff |
| A branch as the return channel (P2.2, P2.9) | adopt | the documented session pushes a branch. No other return API exists (P2.4) |
| Local verify before merge (P2.9, P2.10) | adopt | the kernel gates stay local (F1.6, F1.8) |
| A per-task record directory (P2.12) | adopt | the pipeline writes the session id, the url and the branch under `.evolve/lineage/` (F1.7) |
| Best-of-N attempts (P2.10) | reject for now | it multiplies the usage. The retry ladder already covers a failed seat |
| A PR from the cloud agent (P2.9) | reject | D1 says that the pipeline opens PRs (F1.21) |
| Teleport as the return (P2.5) | reject | it resumes the agent on the host, so the host load comes back |
| Routines as the launcher (P2.6, P2.7) | reject | the operator forbids routines |

## 4. Candidate designs and scores

Each design keeps the lane process local (F1.14). Each design reads the returned work into the local workspace before a kernel gate runs. The designs differ in the unit of work that one cloud session does.

### 4.1 The designs

**A. Phase cells in the cloud.** For each phase, the lane pushes a dispatch commit to a cell branch, and the driver starts one cloud session for the seat. The agent commits its artifact and its source changes to the cell branch, with the challenge token in the message. The driver polls the remote ref, fetches, fences and imports the return. Then the local gates run as today.

**B. Whole-cycle cloud lanes.** One cloud session runs scout, triage, tdd and build on a lane branch. The local pipeline fetches the branch and runs audit and ship.

**C. Hybrid.** Only the source-writer phases go to the cloud: `tdd` and `build` (F1.10). Scout, triage and audit stay local.

**D. One session for each lane.** One cloud session holds the lane. Each phase is a follow-up message (`claude -p ... --cloud <id>`). The local pipeline fetches and gates between the follow-ups.

### 4.2 How each design meets the code

| Point | A | B | C | D |
|---|---|---|---|---|
| Phase order and gates between phases (statemachine) | kept: each phase returns before the next starts | broken: scout to build run without a local gate between them | kept | kept |
| Fresh session for each seat (F1.5) | kept | broken | kept | broken |
| I-4: communication only through declared artifacts | kept | broken: the phases share one context | kept | broken: the context carries over |
| Handoff reviewer for triage to tdd and tdd to build (F1.6) | kept | runs only after the fact, on the final tree | kept | kept |
| Host load off the host | all agent load | all agent load | the load of tdd and build only | all agent load |
| Sessions for each cycle | about 6 | 1 | 2 | 1 |

The phase count of about 6 is scout, triage, tdd, build, audit and the retrospective. Correction rounds add seats.

### 4.3 Scores

Each criterion scores 1 (bad) to 5 (good). "Build size" scores 5 for the smallest build.

| Criterion | A | B | C | D |
|---|---|---|---|---|
| Solves the goal: the phase agents of 8 cycles in the cloud | 5 | 4 | 3 | 4 |
| Isolation invariants I-1 to I-9 kept | 4 | 2 | 4 | 2 |
| Kernel gates kept local and in order | 5 | 2 | 5 | 5 |
| Cost: plan usage for each cycle | 3 | 4 | 4 | 4 |
| Latency: boot and clone overhead for each cycle | 3 | 5 | 4 | 4 |
| Failure handling: blast radius of one bad session | 5 | 2 | 5 | 2 |
| Reversibility: one routing change goes back to local | 5 | 2 | 5 | 3 |
| Build size | 3 | 3 | 3 | 3 |
| **Total (of 40)** | **33** | **24** | **33** | **27** |

A and C tie. The two use the same driver and the same importer. C is A with the routing data set to `tdd` and `build` only. Thus C is the first stage of A, not a rival design (synthesis). A wins on the goal, because the operator asked for the phase agents of the full cycle.

B fails two hard rules: it runs four phases with no kernel gate between them, and it shares one context between phases. D fails I-4 and the fresh-session rule. D still gives one useful part: a follow-up to the same session inside one seat, as the nudge of the stop reviewer (F1.4).

## 5. Refinements to adopt

| Id | Refinement | From |
|---|---|---|
| R1 | Route C first: `tdd` and `build` go to `claude-cloud` for one wave. Then route the other phases | section 4.3 |
| R2 | A remote completion mode reuses the evidence and challenge-token proof of the `git` mode | F1.3 |
| R3 | The importer fences the return: a source-writer phase can change the worktree minus the protected surface and the oracle paths (I-11). Each other phase can add only its declared files | F1.10, F1.13 |
| R4 | The importer copies declared files by name into the workspace and writes no trusted record from the branch | F1.7, F1.11 |
| R5 | The dispatch commit holds the worktree tree and the indexed inputs only. A deny check refuses private paths before the push | F1.20 |
| R6 | A GitHub ruleset limits the push of the cloud actor to `cloud-cell/**`. The probe first finds which GitHub actor a cloud push uses. A local detector alarms on any other moved ref | F1.21, P2.2 |
| R7 | The driver staggers launches and keeps a slot cap through `cliadmit` | P2.8 |
| R8 | A host-load admission rule holds a new lane while the load for each CPU is above a policy value | F1.15, F1.16 |
| R9 | Width 8 waits for the orchestrator-owned integration (ADR-0147) at `enforce` | F1.18 |
| R10 | The driver deletes the cell branch at the end of the seat, and a sweep deletes orphans | F1.20, P2.4 |
| R11 | The bridge billing snapshot records the usage of each session | section 2.3 |
| R12 | A timeout sends one follow-up nudge. A late push to an old cell branch is ignored, because each try has its own branch | P2.4, F1.4 |

## 6. Limits of this research

- No live `claude --cloud` run was made in this study. The boot time, the output of the session id and the push to a branch outside `claude/` are not measured. The plan starts with a probe that measures them.
- The cloud-sessions page was read in part. Its tail came from a summary of the page.
- The burst limit P2.8 is one user report.
- The billing conflict of section 2.3 is open.
- The gate load at width 8 is an estimate. F1.16 measured width 4 only.

# The `evolve` command reference (reference, 2026-11)

**Status:** hand-written from the registry · **Introduced:** 2026-11-24 · **Scope:** all 127 top-level verbs of the `evolve` binary

> The policy that this page measures against: [cli-design.md](../conventions/cli-design.md). The research behind the policy: [cli-interface-design-2026-11.md](../research/cli-interface-design-2026-11.md). The service contract for the envelope and the exit table: [cli-contract.md](../architecture/cli-contract.md).

On 2026-11-24 the operator asked for "a detailed cmd list with clear explanation how each of them works following standard cli naming rule and human understanding".

This page answers in three parts. It maps the verbs by job, so you can find the one you need. It describes each verb in the same fixed form. It then shows where each verb breaks the naming policy, and which name the policy proposes.

**Terms used throughout:**
- **verb**: a top-level word after `evolve`, such as `inbox`. The registry calls it a command.
- **sub-verb**: the word after a group verb, such as `list` in `inbox list`.
- **group**: a verb that takes sub-verbs.
- **flat verb**: a verb with no sub-verbs.
- **seat**: one LLM CLI session that runs one agent role.
- **lease**: a file that marks a run as live. Many verbs refuse to run while a lease is live.
- **boundary**: the pause between two waves, when the operator merges, rebuilds and relaunches.
- **core verb**: a verb with a JSON envelope and a fixed exit table.

Facts come from a read of the source registry at HEAD `e854f84dd`, the installed binary `a099157d`, and read-only probes. Where the help text and the source disagree, the entry says "help text differs" and follows the source.

## Table of contents

- [1. How to read this reference](#1-how-to-read-this-reference)
- [2. Command map](#2-command-map)
- [3. Command entries](#3-command-entries)
- [4. Features without a direct verb](#4-features-without-a-direct-verb)
- [5. Anomalies](#5-anomalies)
- [6. Keeping this page true](#6-keeping-this-page-true)

## 1. How to read this reference

### Effect classes

Each verb has one or more effect classes. The class tells you what the verb can change. Read it before you run an unknown verb.

| Class | Short | Plain meaning |
|---|---|---|
| READ-ONLY | RO | The verb reads files, git state or the network. It changes nothing that you keep. |
| WRITES-LOCAL | WL | The verb writes or deletes files on this machine, in the repository or in `~/.claude`. |
| WRITES-GIT | WG | The verb changes git state: commits, branches, worktrees, refs or a merge. A push is not in this class. |
| WRITES-REMOTE | WR | The verb changes something on a remote: a push, a merged PR, a tag, a release or a deleted remote branch. |
| LAUNCHES-LLM | LLM | The verb starts an LLM seat or an LLM classifier. It can cost money or quota. |
| KILLS/STARTS-PROCESSES | PROC | The verb starts or stops processes: tests, builds, tmux sessions, workers or the loop itself. |

A verb with several sub-verbs shows the union of the classes of its sub-verbs. The entry says which sub-verb has which class. A sub-verb that only reads is marked READ-ONLY in the entry.

### Safety words

- **dry-run**: the verb prints the change and makes no change. Flags of this kind are `--dry-run`, `--check`, `--simulate` and `--ci`. The entry names the real flag.
- **refuses**: a rule stops the verb before it changes anything. Most refusals exit 3 or 1. The entry gives the code.
- **Caution:** a line that starts with this word marks an effect that the name or the summary of the verb does not show.

### Output and exit codes

- `--json` makes a core verb print one JSON envelope on stdout. The entry says if a verb has `--json`, and where it differs.
- The standard exit codes are 0 ok, 1 failed, 2 I/O, 3 refused, 4 queued, 10 usage, 11 input and 130 interrupted. A verb can add 21 to 99.
- Many verbs use their own codes. The entry lists the codes of each verb. Section 5 counts the deviations.

### Conformance marks

The policy [cli-design.md](../conventions/cli-design.md) has rules `CD-01` to `CD-85`. Each entry has a line "Policy conformance". The line lists each rule that the verb breaks, with the reason.

- `conforms` means that the entry found no break.
- A rule id, such as `CD-04`, means a break of that rule. The policy states the rule.
- "proposed, not built" marks a name that the policy approach (noun, then verb) suggests. No such name exists in the binary today. Do not type it.
- The old name stays as an alias for one release after a rename (CD-80).

The column "in --help?" says if `evolve --help` lists the verb. The rule CD-30 asks that it lists all of them.

## 2. Command map

Eleven groups cover all 127 verbs. Each verb is in one group only. The tables list the verbs in the order of the entries.

### 1. Run the loop and waves

Start and stop the autonomous loop, run cycles and waves, and plan campaigns. These are the verbs an operator uses every day.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`loop`](#loop) | Run the dispatcher loop that runs waves of cycles | LLM, PROC, WG, WR | yes | CD-02, CD-03, CD-62 |
| [`loop-stop`](#loop-stop) | Stop the loop cleanly after the current wave | WL | yes | CD-04, CD-05 |
| [`wave`](#wave) | Start, watch, check and stop waves of cycles | LLM, PROC, WL | yes | CD-12 |
| [`cycle`](#cycle) | Run one full cycle and manage paused or failed cycles | LLM, PROC, WG | yes | CD-05, CD-12, CD-52 |
| [`campaign`](#campaign) | Plan and run a multi-cycle campaign | LLM, PROC, WG | yes | CD-12, CD-41, CD-51, CD-61 |
| [`fleet`](#fleet) | Launch N concurrent cycles at once | LLM, PROC | no | CD-12, CD-30, CD-51 |
| [`boundary`](#boundary) | Run the wave boundary chain (stop, merge, rebuild, relaunch) | PROC, WG, WR | yes | CD-33 |
| [`continuation`](#continuation) | Inspect, release or adopt item-to-branch bindings | RO, WL | yes | CD-41, CD-52, CD-61 |
| [`compose`](#compose) | Run an ad-hoc sequence of phases by hand | LLM, WL | yes | CD-03, CD-27, CD-33, CD-41 |

### 2. Integrate and ship

Move work to main: merge PRs, land batches, commit through the gates, and save or recover work in progress.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`pr`](#pr) | Merge reviewed PRs at a boundary | RO, WG, WR | yes | CD-12, CD-61 |
| [`integration`](#integration) | Land reviewed console PRs as one batch | RO, WL, WG, WR | no | CD-12, CD-30 |
| [`ship`](#ship) | Commit and push through the gates | WG, WR | yes | CD-20, CD-33, CD-41, CD-50 |
| [`land`](#land) | Put a patch or salvage on a fresh dev worktree | WG, WL | yes | CD-14, CD-61 |
| [`landing`](#landing) | Approve or reject a proposal held by the ship gate | RO, WL | no | CD-12, CD-14, CD-30, CD-41 |
| [`sync-main`](#sync-main) | Merge origin into a diverged local main | WG | no | CD-04, CD-12, CD-30, CD-33, CD-61 |
| [`commit-gate`](#commit-gate) | Run lint and tests on the staged change and attest the tree | PROC, WL | no | CD-04, CD-05, CD-30, CD-41, CD-76 |
| [`commit-prefix-gate`](#commit-prefix-gate) | Check a commit prefix against the files changed | RO | yes | CD-04, CD-05, CD-41, CD-50, CD-76 |
| [`console-lease`](#console-lease) | Let the operator edit named runtime files for a short time | WL | no | CD-04, CD-20, CD-30, CD-33 |
| [`worktree`](#worktree) | Create, list and remove git worktrees | RO, WG | yes | CD-12, CD-41 |
| [`checkpoint`](#checkpoint) | Save uncommitted work as hidden git refs | RO, WL, WG, WR | yes | CD-61 |
| [`archive`](#archive) | Restore a failed cycle from its archive | WG, WL | no | CD-30, CD-41, CD-61 |
| [`salvage`](#salvage) | Report and list salvaged work of bad verdicts | RO | yes | CD-12, CD-20 |
| [`ci`](#ci) | Classify red CI runs and wait for CI | RO, WL, WR | yes | CD-12 |

### 3. The inbox and planning

Operate the backlog of work items, and read or clear the memory that steers planning.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`inbox`](#inbox) | Operate the backlog of work items | RO, WL | no | CD-12, CD-13, CD-30 |
| [`inbox-mover`](#inbox-mover) | Low-level inbox claim, promote and recover | WL | yes | CD-04, CD-05, CD-15, CD-51 |
| [`carryover`](#carryover) | List and apply decisions on carried-over todos | RO, WL | no | CD-12, CD-30 |
| [`issue`](#issue) | Check and list incident records and evidence | RO, WL | yes | CD-41 |
| [`kb`](#kb) | Query the knowledge-base graph | RO | yes | CD-12 |
| [`lessons`](#lessons) | Report and backfill recurring lessons | RO, WL | no | CD-06, CD-30, CD-62, CD-76 |
| [`failures`](#failures) | Inspect and clear the failed-approach memory | RO, WL | no | CD-06, CD-20, CD-30, CD-61 |
| [`dossier`](#dossier) | Check and publish per-cycle dossiers | RO, WL, WG | yes | CD-12, CD-13 |
| [`postmortem`](#postmortem) | Record why a phase dispatch ended badly | RO, WL | no | CD-12, CD-30, CD-41 |
| [`practice`](#practice) | Watch vendor changelogs and file research items | RO, WL | yes | CD-12 |
| [`routing`](#routing) | Explain, replay or ask for the advisor phase plan | RO, LLM | yes | CD-05 |
| [`convergence`](#convergence) | Decide converge or escalate in a review loop | RO | no | CD-12, CD-30 |

### 4. Inspect and debug

Read the state of the pipeline: status, events, logs, ledger, tokens, health. Most verbs here change nothing.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`status`](#status) | One report of loop, cycles, PRs and CI | RO | yes | CD-33 |
| [`dashboard`](#dashboard) | Serve a live web page of pipeline state | PROC | yes | CD-03, CD-20, CD-33, CD-41 |
| [`events`](#events) | Read the event channels | RO | no | CD-06, CD-12, CD-30 |
| [`signals`](#signals) | Tail Signal Center events and keep the codes doc in step | RO, WL | no | CD-12, CD-30, CD-52 |
| [`logs`](#logs) | Read cycle logs in bounded layers | RO | no | CD-06, CD-12, CD-30, CD-41 |
| [`ledger`](#ledger) | Check and maintain the audit ledger hash chain | RO, WL | yes | CD-12, CD-13, CD-48, CD-52 |
| [`tokens`](#tokens) | Rank token use by phase | RO | no | CD-06, CD-12, CD-30, CD-52 |
| [`ladder`](#ladder) | Report retry-ladder use | RO | no | CD-12, CD-30 |
| [`soak-report`](#soak-report) | Print soak evidence for a cycle range | RO | no | CD-04, CD-30, CD-33, CD-41 |
| [`cloud`](#cloud) | List cloud sessions | RO | no | CD-30 |
| [`audit`](#audit) | Report how well auditor verdicts held | WL | yes | CD-12, CD-41, CD-51 |
| [`context-fill`](#context-fill) | Compare context-window fill with the verdict | RO, WL | yes | CD-04, CD-05, CD-12 |
| [`cycle-health`](#cycle-health) | Run integrity checks on a cycle workspace | RO | yes | CD-04, CD-05, CD-25, CD-33, CD-41 |
| [`selfheal`](#selfheal) | Detect pipeline incidents and propose fixes | WL, LLM | no | CD-11, CD-12, CD-30 |
| [`estimate-quota-reset`](#estimate-quota-reset) | Predict when a CLI quota wall resets | RO | yes | CD-04, CD-12, CD-41, CD-76 |
| [`verdicts`](#verdicts) | Show or empty the verdict store | RO, WL | no | CD-06, CD-13, CD-30, CD-41, CD-62 |
| [`swarm`](#swarm) | Inspect and kill leftover swarm sessions | RO, PROC | no | CD-12, CD-30, CD-51, CD-61 |
| [`preflight-environment`](#preflight-environment) | Report what the host can do | RO, WL | yes | CD-04, CD-14, CD-76 |

### 5. Quality and gates

Run the gates that keep the code and the tests honest: coverage, ratchets, test audits, secret scans and the local CI floor.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`acs`](#acs) | Run acceptance-criteria predicate tests | PROC, WL | yes | CD-12, CD-24, CD-41 |
| [`apicover`](#apicover) | Check that tests name every exported symbol | RO | no | CD-03, CD-20, CD-30, CD-41 |
| [`cover`](#cover) | Coverage gate for changed code and package floors | RO | yes | CD-08, CD-12, CD-24 |
| [`eval`](#eval) | Grade eval files and run their checks | RO, PROC | yes | CD-13, CD-20, CD-41 |
| [`floor`](#floor) | Run the local copy of the CI floor | PROC | no | CD-30, CD-41, CD-51 |
| [`guard`](#guard) | Allow or deny a tool call or ship action | WL | yes | CD-12, CD-52 |
| [`postedit-validate`](#postedit-validate) | Syntax-check a file after an edit | WL | yes | CD-04, CD-12, CD-27 |
| [`quality`](#quality) | Compute and compare quality-index readings | RO, WL | no | CD-08, CD-12, CD-30 |
| [`ratchet`](#ratchet) | Stop function size and raw-git use from growing | RO, WL | yes | CD-12, CD-20, CD-41, CD-51 |
| [`reachability`](#reachability) | Check that a test pin makes no import cycle | RO | no | CD-11, CD-24, CD-30, CD-41 |
| [`scan`](#scan) | Find secrets in a diff | RO | yes | CD-12 |
| [`selfcheck`](#selfcheck) | Let the builder test its own handoff | RO | no | CD-14, CD-30, CD-41, CD-51 |
| [`stage`](#stage) | Check the evidence of a shadow stage | RO | no | CD-12, CD-13, CD-30 |
| [`test`](#test) | Audit test quality against baselines | RO, WL | no | CD-05, CD-12, CD-30, CD-52 |
| [`test-scoped`](#test-scoped) | Run tests of changed packages only | PROC | no | CD-04, CD-05, CD-30, CD-33, CD-52 |
| [`treewalk`](#treewalk) | Detect files that a test leaked into a tree | RO, WL | yes | CD-12, CD-24, CD-41, CD-52 |
| [`comments`](#comments) | Audit and strip narrative code comments | RO, WL | yes | CD-06, CD-14, CD-20, CD-51, CD-61 |
| [`names`](#names) | Find and fix dead old names | RO, WL | yes | CD-06, CD-08, CD-41, CD-61 |
| [`solution`](#solution) | Check a document deliverable against its contract | RO | no | CD-30, CD-41, CD-51 |

### 6. Docs and skills

Keep the generated documents, the skill docs and the guidance registry in step with their sources.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`docs`](#docs) | Check document style and keep the research index | RO, WL | yes | CD-12, CD-51 |
| [`skills`](#skills) | Keep generated skill docs in step and publish skills | RO, WL | yes | CD-41, CD-52 |
| [`skill-inventory`](#skill-inventory) | Build the skill cache | WL | yes | CD-04, CD-05, CD-12 |
| [`flags`](#flags) | Keep the generated flag index in step | RO, WL | no | CD-06, CD-30, CD-52 |
| [`guidance`](#guidance) | Build, check and query the guidance rule registry | RO, WL | no | CD-05, CD-12, CD-30 |

### 7. Code search

Ask the code index who calls a symbol, what the API of a file is, and where a name lives. These verbs replace grep for agents.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`code`](#code) | Indexed read-only code search for agents | RO, WL | yes | CD-12 |
| [`context`](#context) | List and open prepared context files for an agent | RO | no | CD-05, CD-24, CD-30 |

### 8. Routing, models and CLIs

Decide which LLM CLI and model each role uses, and manage the CLI seats, logins and bench state.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`agent`](#agent) | Run one phase agent seat with no cycle | LLM, WL | no | CD-30 |
| [`bridge`](#bridge) | Launch and control agent CLIs in tmux seats | LLM, PROC, WL | yes | CD-11, CD-34 |
| [`auth`](#auth) | Show login times and manage the seat token | RO, WL | no | CD-12, CD-30, CD-61 |
| [`cli`](#cli) | Update the subscribed LLM CLIs | PROC, WL | yes | CD-05 |
| [`cli-routing`](#cli-routing) | Show, check and edit the CLI and model routing table | RO, WL | no | CD-04, CD-05, CD-30, CD-61 |
| [`clihealth`](#clihealth) | Show or clear CLI bench state and read usage meters | RO, WL, PROC | yes | CD-05, CD-12, CD-33 |
| [`models`](#models) | Refresh and show the live model catalog | RO, WL, LLM, PROC | no | CD-06, CD-12, CD-30, CD-61 |
| [`resolve-llm`](#resolve-llm) | Say which CLI and tier a role uses | RO | yes | CD-04, CD-12, CD-41 |
| [`detect-cli`](#detect-cli) | Say which AI CLI drives this session | RO | yes | CD-03, CD-04, CD-12 |
| [`detect-nested-claude`](#detect-nested-claude) | Say if this runs inside a claude -p session | RO | yes | CD-03, CD-04, CD-16, CD-22, CD-50 |

### 9. Release

Prepare, publish, verify and undo a release.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`release`](#release) | Publish a release end to end | WG, WR | yes | CD-02, CD-05, CD-76 |
| [`release-consistency`](#release-consistency) | Check that all version markers agree | RO | yes | CD-04, CD-05, CD-12, CD-51, CD-76 |
| [`release-preflight`](#release-preflight) | Pre-publish gate | RO, PROC | yes | CD-04, CD-05, CD-12 |
| [`release-promote`](#release-promote) | Promote a demoted release to a full release | WR | yes | CD-04, CD-05, CD-51, CD-61 |
| [`release-verify-binaries`](#release-verify-binaries) | Check that all release binaries exist | RO | no | CD-04, CD-05, CD-13, CD-30, CD-51, CD-76 |
| [`release-verify-clis`](#release-verify-clis) | Check that the release binary works for each CLI | WL, PROC | no | CD-04, CD-05, CD-13, CD-16, CD-30 |
| [`rollback`](#rollback) | Undo a failed release | WG, WR | yes | CD-03, CD-62 |
| [`version-bump`](#version-bump) | Move all version markers to a version | WL | yes | CD-04, CD-05, CD-12 |
| [`changelog-gen`](#changelog-gen) | Write a changelog entry from commits | WL | yes | CD-03, CD-04, CD-12, CD-25 |
| [`marketplace-poll`](#marketplace-poll) | Wait until the marketplace shows the new version | WG | yes | CD-04, CD-11, CD-12 |

### 10. Setup and maintenance

Install the tool, check the machine, rebuild the binary and clean up what finished cycles leave behind.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`help`](#help) | Print the top-level command list | RO | n/a | CD-32 |
| [`version`](#version) | Print the build version | RO | yes | CD-41 |
| [`setup`](#setup) | Onboarding: detect CLIs, pick and write a preset | RO, WL, PROC | yes | CD-03, CD-12, CD-41, CD-62 |
| [`install`](#install) | Install agents and the loop skill into ~/.claude | WL | yes | CD-41, CD-61 |
| [`uninstall`](#uninstall) | Remove the manual install | WL | yes | CD-63, CD-76 |
| [`doctor`](#doctor) | Check the machine, the seats and the CLI versions | RO, PROC, LLM, WG | yes | CD-12 |
| [`rebuild`](#rebuild) | Rebuild the evolve binary from HEAD | WL, PROC | yes | CD-33, CD-51, CD-61 |
| [`reset-sha`](#reset-sha) | Re-pin the expected ship binary SHA | WL | no | CD-04, CD-12, CD-30, CD-33, CD-50 |
| [`gc`](#gc) | Release leftovers of finished cycles | PROC, WL, WG, WR | yes | CD-12, CD-62 |
| [`prune-ephemeral`](#prune-ephemeral) | Delete old tracker files and dispatch logs | WL | yes | CD-04, CD-14 |
| [`branches`](#branches) | Audit and prune superseded cycle branches | RO, WG | yes | CD-06, CD-41, CD-62, CD-63 |
| [`backups`](#backups) | Check that backups are safe to delete | RO | yes | CD-06, CD-13, CD-24, CD-33, CD-41, CD-51 |

### 11. Pipeline internals

Verbs that other verbs or phases call. A person rarely types them. They are listed so that nothing is hidden.

| Verb | Purpose | Effect | In --help? | Conformance |
|---|---|---|---|---|
| [`phase`](#phase) | Run one phase in the process or check its deliverable | LLM, WL | yes | CD-05, CD-08, CD-13, CD-27 |
| [`phases`](#phases) | List, validate and create phase definitions | RO, WL | no | CD-05, CD-06, CD-13, CD-14, CD-30 |
| [`phase-inventory`](#phase-inventory) | Build the phase spec cache | WL | no | CD-04, CD-05, CD-12, CD-30 |
| [`phase-observer`](#phase-observer) | Watch a running phase for stalls | WL, PROC | yes | CD-04, CD-05, CD-11, CD-15, CD-25 |
| [`phase-order`](#phase-order) | Print the phase order | RO | yes | CD-04, CD-05, CD-41, CD-76 |
| [`phase-watchdog`](#phase-watchdog) | Kill a stalled phase process group | PROC, WL | yes | CD-04, CD-05, CD-15, CD-25, CD-50 |
| [`serve-phase`](#serve-phase) | Run a phase over the phaseproto protocol | LLM, WL | yes | CD-04, CD-12, CD-27 |
| [`plan-and-execute`](#plan-and-execute) | Run a phase in plan mode, then in edit mode | LLM | yes | CD-03, CD-04, CD-27 |
| [`subagent`](#subagent) | Low-level helpers to run a phase agent and its workers | LLM, WL | yes | CD-11, CD-15, CD-41, CD-76 |
| [`aggregator`](#aggregator) | Merge the artifacts of fan-out workers | WL | yes | CD-03, CD-15, CD-25, CD-51 |
| [`fanout-dispatch`](#fanout-dispatch) | Run worker commands in parallel | PROC, WL | yes | CD-03, CD-04, CD-12, CD-51 |
| [`consensus-dispatch`](#consensus-dispatch) | Run one audit through several CLIs and merge the verdicts | LLM, PROC, WL | yes | CD-03, CD-04, CD-25, CD-76 |
| [`build-invocation-context`](#build-invocation-context) | Print the fixed prompt prefix of a role | RO | yes | CD-03, CD-04, CD-12, CD-51 |
| [`cycle-simulator`](#cycle-simulator) | Walk the cycle plumbing with no LLM | WL | yes | CD-04, CD-05, CD-15, CD-76 |
| [`msg`](#msg) | Messages between phase agents of one cycle | WL | no | CD-12, CD-27, CD-30, CD-33, CD-76 |
| [`handoff`](#handoff) | Check handoff between two phases | RO | yes | CD-20 |

## 3. Command entries

Each entry has the same parts. Read them in this order. The synopsis, what the verb does and how it works come first. Then come its effect and safety, its output and exit codes, who calls it and its policy conformance. The groups follow section 2.

### 1. Run the loop and waves

#### `loop`

```
evolve loop [--project-root P] [--evolve-dir D] [--goal-hash H | --goal-text T | --goal-file F | --goal-from-last] [--strategy balanced|innovate|harden|repair|ultrathink|autoresearch] [--max-cycles N | --cycles N] [--resume] [--auto-resume] [--dry-run] [--reset [--fingerprint F]] [--consensus-audit] [--force-fresh] [--skip-preflight] [--skip-preflight-boot] [--until-inbox-empty] [--preflight-only] [--detach --log FILE] [--relaunch] [--only IDS] [--exclude IDS] [--bypass-policy] [--json] [--sandbox-off ...] [--cli agent=cli]... [--model agent=model]...
evolve loop [strategy] "goal"
evolve loop status [--json] [--project-root P]
```

- **What it does:** Drives the dispatcher loop that runs cycles (waves of lanes) until the goal, the budget or the inbox is done.
- **How it works:** `launchParsedLoop` runs a preflight gate, records the goal and starts a chain or a batch. It starts the lanes (`cycle run`) with LLM seats in worktrees, ships, and writes dossiers, the ledger and `state.json`. `--detach` runs the loop again in a new session.
- **Effect and safety:** LAUNCHES-LLM, STARTS-PROCESSES, WRITES-GIT and WRITES-REMOTE (through ship and `pr`). `--dry-run` prints the configuration JSON and exits 0. `--preflight-only` runs the gate. The verb refuses on an unusable launch (inert goal items, the `.evolve/loop-stop` brake, a live lease). Agent class: denied (`status` allowed).
- **Caution:** `evolve loop` with no argument is a launch, not a help request.
- **Output:** `--json` (with `--detach`; in the foreground it prints a JSON result). `loop status --json`.
- **Exit codes:** 0, 1 failed, preflight blocked or detach not confirmed, 3 routing refused, 10 usage or bad goal, 130 interrupted. `status`: 0, 1, 10.
- **Called by:** the operator, console and `wave next` (the boundary runs `loop --detach`).
- **Policy conformance:** CD-02: a bare `evolve loop` launches in place of a verb list. CD-03: a flat verb with sub-verbs. CD-62: a launch with no `--apply`. Help text differs: top help shows only `--max-cycles N [strategy] "goal"`. About 30 flags and `status` are not documented there.

#### `loop-stop`

```
evolve loop-stop [--release | --wait [--timeout D]] [--json] [--project-root P]
```

- **What it does:** Stops a running loop cleanly after its current wave.
- **How it works:** It writes the brake file `.evolve/loop-stop` with an RFC3339 time. The loop sees the file at the next wave boundary. `--wait` polls the run leases until none is live. `--release` removes the file.
- **Effect and safety:** WRITES-LOCAL (the brake file). It does not signal a process. `--wait` and `--release` exclude each other. `--timeout` needs `--wait`. When `--wait` times out, the brake stays. Agent class: denied.
- **Output:** `--json`. Output on stdout.
- **Exit codes:** 0, 1 wait timed out, 2 I/O, 10 usage.
- **Called by:** the operator and the boundary.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside the `loop` group (it has `loop status`). Proposed `loop stop` (proposed, not built).

#### `wave`

```
evolve wave next [--max-cycles N] [--only IDS] [--exclude IDS] [--merge n,...] [--note TEXT]... [--number N] [--dry-run] [--json] [--project-root P]
evolve wave status [--json] [--project-root P]
evolve wave supply [...]
evolve wave latency [--wave N | --cycles A-B | --check] [--json] [--project-root P]
evolve wave watch [--json] [--project-root P]
evolve wave note add TEXT | list [--json] | clear [--project-root P]
evolve wave stop [--wait D] [--json] [--project-root P]
evolve wave checkin [--json] [--project-root P]
```

- **What it does:** The operator and LLM path to run waves of cycles. Start the next wave, watch it, check its health, queue notes and stop it at a boundary.
- **How it works:** `next` builds a goal from facts (merged PRs, the supply census) and makes the wave record. It then launches the loop through the plane binary and `boundary run`. `status`, `supply`, `watch`, `latency` and `checkin` read `.evolve` state, events and `gh pr list`. `stop` turns on the loop-stop brake file and can wait for idle. `note` edits queued context in the wave store.
- **Effect:** `next` LAUNCHES-LLM and STARTS-PROCESSES (it starts the loop, and writes git indirectly). `stop`, `note add` and `note clear` WRITES-LOCAL. The other sub-verbs are READ-ONLY.
- **Safety:** `next --dry-run` plans without a launch. `next` refuses (exit 3) on a live loop and other guards. `stop` is a brake, not a kill. Agent class: `status`, `supply` and `watch` allowed. `note`, `stop` and `next` denied.
- **Caution:** `wave next` starts the boundary chain: it can merge PRs, rebuild the binary and relaunch the loop. The name "next" does not show this.
- **Output:** `--json` prints an envelope on every exit.
- **Exit codes:** 0, 2 I/O, 3 refused (`next`) or BRAKE (`checkin`), 4 FIX-LOCAL (`checkin`), 10 usage.
- **Called by:** the operator, console and the loop.
- **Policy conformance:** CD-12: `next`, `supply` and `checkin` are not standard verbs. Help text differs: the registry summary omits `latency` and `stop`.

#### `cycle`

```
evolve cycle run --goal-hash H [--goal T] [--project-root P] [--evolve-dir D] [--simulate] [--bypass-policy] [--sandbox-off X] [--pool N]
evolve cycle reset [--project-root P] [--evolve-dir D] [--reason T] [--dry-run] [--force] [--json]
evolve cycle resume [--cycle N [--grant] | --all] [--dry-run] [--json] [--project-root P]
evolve cycle retire --cycle N [--reason T] [--json] [--project-root P]
evolve cycle timing [--project-root] [--evolve-dir] [--json] [--concurrency N=2]
evolve cycle models [--project-root] [--evolve-dir] [--cycle N] [--json]
evolve cycle outputs [--project-root] [--evolve-dir] [--json]
evolve cycle scope-delta [--cycle N | N] [--json]
evolve cycle reaudit --cycle N [--project-root] [--evolve-dir]
```

The no-argument error lists only `run`, `resume`, `reset`, `retire`, `timing` and `models`. `outputs`, `scope-delta` and `reaudit` also exist (help text differs).

- **What it does:** Runs one full pipeline cycle (scout, build, audit, ship). Manages cycles that are paused, failed or sealed.
- **How it works:** `run` drives `internal/orchestrator` over all phases through the bridge and ships through the ship gate. `reset` seals the cycle in progress into an archive. `resume` restarts or grants an auto-resume to paused cycles. `retire` releases a paused cycle. `reaudit` runs the audit again on the current base. The other sub-verbs read run records.
- **Effect:** `run` LAUNCHES-LLM, starts processes, WRITES-GIT (with a push through ship) and WRITES-LOCAL. `reset`, `retire` and `resume` WRITES-LOCAL. `resume` also starts a cycle unless `--all`, `--grant` or `--dry-run`. `reaudit` LAUNCHES-LLM. `timing`, `models`, `outputs` and `scope-delta` are READ-ONLY.
- **Safety:** `reset` refuses a live lease (exit 3) unless `--force`. `run --simulate` makes no LLM call. Agent class: denied.
- **Caution:** `cycle run` ships and pushes code. `cycle reset --force` seals a cycle that a live owner may still use.
- **Output:** `--json` on `reset`, `resume`, `retire`, `timing`, `models`, `outputs` and `scope-delta`. `run` prints text.
- **Exit codes:** `run`: 0, 2 final verdict FAIL, 3 routing refused, 4 system-failure halt, 5 deferred (quota wall), 10 usage. `reset`: 0, 1 nothing to reset, 3 refused, 10. `scope-delta`: 0 clean, 1 outside scope, 2 I/O, 10. Others: 0, 1, 10.
- **Called by:** the loop and the fleet (each lane runs `cycle run`). The operator and console call the other sub-verbs.
- **Policy conformance:** CD-12: `retire`, `reaudit` and `scope-delta` are unusual verbs. CD-05: flat siblings `cycle-health` and `cycle-simulator` sit beside this group. CD-52: `run` uses exit 2 for a FAIL verdict, where the table gives 2 to I/O.

#### `campaign`

```
evolve campaign study --workspace W
evolve campaign replan --workspace W --feedback T
evolve campaign run --plan P [--simulate] [--concurrency N] [--project-root R] [--ignore-progress]
evolve campaign status --plan P [--project-root R]
```

`run` also takes `--cycle-timeout D` (default 2h). The usage text omits it (help text differs).

- **What it does:** Plans a multi-cycle campaign from a goal, revises the plan with feedback, runs it wave by wave and reports progress.
- **How it works:** `study` and `replan` run the preliminary-study phase through the bridge and write `campaign-plan.json`. `run` launches cycles for each wave through `internal/fleet`. It resumes saved progress unless `--ignore-progress`, and it stops cycles that pass the timeout. `status` reads the plan and the progress.
- **Effect and safety:** `study` and `replan` LAUNCH-LLM. `run` LAUNCHES-LLM, starts cycle processes and WRITES-LOCAL and WRITES-GIT (the cycles ship). `--simulate` makes no LLM call. `status` is READ-ONLY. No dry-run.
- **Output:** The plan goes to stdout. No `--json`.
- **Exit codes:** 0, 1 runtime, 2 usage.
- **Called by:** the operator.
- **Policy conformance:** CD-12: `study` and `replan` are not standard verbs. CD-51: usage exits 2. CD-41: no `--json`. CD-61: `run` has no `--dry-run`; it has `--simulate`.

#### `fleet`

```
evolve fleet --count N --goal-hash H [--concurrency N] [--simulate] [--plan FILE]
evolve fleet soak --count N [--evolve-dir D] [--toml-path P] [--goal-hash H] [--pool]
```

- **What it does:** Launches N concurrent cycles at once (ADR-0049 S6), or soak-tests the concurrency plumbing.
- **How it works:** `internal/fleet.Supervisor` starts `evolve cycle run --goal-hash ...` as subprocesses after the cycle-start preflight. `--plan` splits a backlog from the advisor into cycles with separate scopes. `--simulate` makes no LLM call.
- **Effect and safety:** LAUNCHES-LLM and STARTS-PROCESSES (real cycles: worktrees, git, ship). `--simulate` and `soak` need no LLM. `--plan` is required without `--simulate`. No dry-run, no prompt.
- **Output:** No `--json`. One line for each cycle on stderr.
- **Exit codes:** 0 all ok, 1 any failure and also any usage error.
- **Called by:** the operator. No code caller found. `loop` in fleet mode reuses its own launch path, so this verb may be superseded.
- **Policy conformance:** CD-30: not in `--help`. CD-51: usage exits 1, not 10. CD-12: the noun `fleet` is used as the verb (proposed `cycle launch`, proposed, not built). `fleet` passes `-simulate` with one dash to `cycle run`.

#### `boundary`

```
evolve boundary run --goal-text-file F [--merge n,...] [--max-cycles N] [--only IDS] [--exclude IDS] [--dry-run] [--json] [--project-root P]
```

- **What it does:** Runs the low-level wave boundary chain. It stops the loop, merges PRs, tightens ratchets, syncs main, rebuilds, runs self-heal, reports quality, collects garbage and relaunches the loop.
- **How it works:** It builds a step list and runs each step through the in-process verbs. The steps are `loop-stop --wait`, `pr merge`, `ratchet tighten`, `sync-main`, `make build`, `inbox curate --apply`, `selfheal`, `quality report`, `gc`, `loop-stop --release` and `loop --detach --relaunch`. It stops at the first failing step that is not advisory.
- **Effect and safety:** KILLS/STARTS-PROCESSES, WRITES-GIT, WRITES-REMOTE (`pr merge` through `gh`) and WRITES-LOCAL. `--dry-run` prints the ordered plan and runs nothing. A failed step leaves the loop-stop brake on. Agent class: denied.
- **Caution:** The name "boundary run" does not show that it stops the live loop, merges PRs on the remote and starts the loop again. Use `evolve wave next` for the high-level path.
- **Output:** `--json` prints an envelope (plan and step results). Progress goes to stdout, or to stderr with `--json`.
- **Exit codes:** 0, 2 root or I/O error, 10 usage. Otherwise the exit code of the failing step.
- **Called by:** the operator and console.
- **Policy conformance:** Conforms in shape (group and `run`). CD-33: `--help` is not special; it exits 10. Defect: the step `ci latency check` calls a sub-verb that does not exist, so each boundary prints a WARN.

#### `continuation`

```
evolve continuation list
evolve continuation release <scope-id> [--operator] [--force]
evolve continuation adopt <item-id> --worktree <path>
```

All forms take `--project-root P`.

- **What it does:** Inspects, releases or adopts the bindings that tie a work item (scope) to the branch snapshot of an earlier cycle.
- **How it works:** It reads the continuation registry. `release` needs operator authority and checks that the run lease of the bound cycle is not live. It then drops the binding through `internal/inboxmover` and leaves a pointer on the inbox item. `adopt` binds an item to a worktree.
- **Effect and safety:** `list` is READ-ONLY. `release` and `adopt` WRITES-LOCAL. `release` refuses without `--operator` and refuses a live lane unless `--force`. Agent class: `adopt` is denied.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 failure or not bound, 10 usage or refusal.
- **Called by:** the operator and console.
- **Policy conformance:** CD-52: a refusal exits 10, but the rule asks for 3. CD-41: no `--json`. CD-61: `release --force` has no `--dry-run`. The error text spells `-force` and `-operator` with one dash (help text differs).

#### `compose`

```
evolve compose --phases a,b [--ship-anyway] [--dry-run] [--cycle N] [--project-root P]
```

The phase request JSON comes on stdin unless `--cycle` is set. Known phases: `audit`, `build`, `debugger`, `intent`, `retro`, `scout`, `ship`, `tdd`, `triage`.

- **What it does:** Runs an ad-hoc sequence of pipeline phases by hand, such as scout then audit, outside a full cycle.
- **How it works:** It resolves the runner of each phase and runs the phases in order with the given request. It prints each response as JSON. It continues after a phase that is not PASS.
- **Effect and safety:** LAUNCHES-LLM (phases call seats) and WRITES-LOCAL (phase artifacts). `ship` is refused unless `--ship-anyway` is set. Then it also WRITES-GIT. `--dry-run` prints the sequence.
- **Output:** Text and JSON blobs on stdout. There is no `--json` flag.
- **Exit codes:** 0 all PASS, 1 a phase error or not PASS, 3 routing refused, 10 usage or bad JSON.
- **Called by:** the operator, for debug.
- **Policy conformance:** CD-03: flat verb outside the flat list; proposed `phase compose` (proposed, not built). CD-33: `-h` exits 10. CD-27: reads stdin without `-`. CD-41: no `--json`.

### 2. Integrate and ship

#### `pr`

```
evolve pr merge <n>... [--update-branch] [--wait D] [--json] [--project-root P]
evolve pr window <n> | --files-from FILE [--json] [--project-root P]
evolve pr boundary [--json] [--project-root P]
evolve pr refresh [--project-root P]
```

- **What it does:** Merges reviewed PRs at a wave boundary. Classifies PRs as merge-window or boundary changes.
- **How it works:** `internal/cli/landcmd` checks that no run lease is live and that the required CI is green on the verified head. It then runs `gh pr merge --match-head-commit`. `--update-branch` waits at least 60 seconds for the new head. `refresh` rebuilds derived files after a rebase and commits only them.
- **Effect and safety:** `merge` WRITES-REMOTE (`gh`). `refresh` WRITES-GIT (a commit). `window` and `boundary` are READ-ONLY. It refuses while a loop runs or CI is not green. No dry-run. Agent class: denied.
- **Caution:** `pr merge` merges on the remote and cannot be undone by this tool. The boundary runs it.
- **Output:** `--json`. Output on stdout.
- **Exit codes:** 0 merged, 1 refused, 2 I/O or `gh` error, 3 boundary, 10 usage.
- **Called by:** the operator, console and the boundary.
- **Policy conformance:** CD-12: `window`, `boundary` and `refresh` are not standard verbs. CD-61: `merge` has no `--dry-run`. Help text differs: the registry and help omit `boundary`, and the help omits `--json`.

#### `integration`

```
evolve integration watch --tip SHA
evolve integration status [--pool N] [--wave N] [--project-root P] [--json]
evolve integration record-pr --pr N (--reviewed | --class NAME)
evolve integration batch [--dry-run] [--main-red-fix] [--project-root P] [PR...]
```

- **What it does:** Lands reviewed console PRs as one batch behind one CI run. Reports the integration state of a wave.
- **How it works:** `batch` builds a batch branch, pushes it with force-with-lease, runs `gh pr create` and `gh run watch`, then moves `origin/main` forward (`git push origin tip:main`). `record-pr` writes the review record of a PR. `watch` polls `gh run` for a 40-hex tip. `status` reads the wave store.
- **Effect and safety:** `watch` and `status` are READ-ONLY. `record-pr` WRITES-LOCAL. `batch` WRITES-GIT and WRITES-REMOTE (push, PR create and merge, inbox move). `batch --dry-run` previews. It refuses (exit 1) when main CI is not green or a main-red stop holds. `--main-red-fix` lands exactly one PR.
- **Caution:** `integration batch` pushes to `origin/main`. The name "batch" does not show it.
- **Output:** `--json` on `status` only.
- **Exit codes:** `watch`: 0 green, 1 red, 2 no run or `gh` failed, 10. `status`: 0 clean, 1 not clean, 2 root or store absent, 10. `record-pr` and `batch`: 0, 1 refused or not green, 2 git or `gh` failure, 10.
- **Called by:** the operator and console at the wave boundary. `status` is a core verb that is always available.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `batch` and `record-pr` are not standard verbs. Help text differs: the registry summary omits `status` and `--main-red-fix`.

#### `ship`

```
evolve ship [--class cycle|manual|release|trivial] [--dry-run] [--paths a,b] [--auto-confirm REASON] [--push-only] [--bypass-commit-gate] [--bypass-prefix-gate] [--pr] [--pr-body-file F] [--project-root P] [--plugin-root P] "<commit-message>"
evolve ship --push-only
```

- **What it does:** The one sanctioned way to commit and push work. It enforces the gates and writes the ledger.
- **How it works:** `internal/phases/ship` stages the files, checks the attestations (the commit gate for the manual class), commits and pushes. For the release class it also tags. `--pr` then opens the pull request through `gh`. Logs go to stderr.
- **Effect and safety:** WRITES-GIT and WRITES-REMOTE (push, and a PR with `--pr`). `--dry-run` runs the checks only. The manual class asks for a confirmation unless `--auto-confirm` is set (refused inside an LLM cell). The emergency bypass flags are recorded. `--push-only` checks provenance.
- **Output:** No `--json`. Logs on stderr.
- **Exit codes:** 0 OK, 1 failure (also a flag parse error), 2 integrity, 127 missing binary.
- **Called by:** the loop (ship phase), the operator (`--class manual`) and the `evo:commit` skill.
- **Policy conformance:** On the flat list. CD-33: `--help` exits 1. CD-50: 127 is outside the table. CD-20: Go help prints single-dash names. CD-41: no `--json`. Help text differs: the usage error line shows only three flags.

#### `land`

```
evolve land --branch B (--patch F | --salvage <leaf>) [--project-root P]
```

- **What it does:** Puts a patch, or the uncommitted work of a salvaged cycle, on a new dev worktree at the fetched `origin/main`. The work stays uncommitted, so an operator can finish it.
- **How it works:** `runWorktreeCreateDev` makes a dev worktree and a branch under the hub. `landInto` applies the patch, and for a salvage leaf also `untracked.tgz` from `.evolve/operator-salvage/<leaf>`. It never commits or pushes.
- **Effect and safety:** WRITES-GIT (a new worktree and branch) and WRITES-LOCAL. Exactly one of `--patch` and `--salvage` is allowed. No dry-run.
- **Output:** No `--json`. Text on stdout and stderr.
- **Exit codes:** 0, 2 I/O (bad patch, leaf or root), 10 usage, plus the codes of the worktree and apply steps.
- **Called by:** the operator and console after a failed cycle.
- **Policy conformance:** On the flat list. CD-14: the near names `land`, `landing` and `pr merge` confuse a caller. CD-61: it writes git state and has no `--dry-run`.

#### `landing`

```
evolve landing list [--project-root P]
evolve landing approve --cycle N --tree SHA --audit-report PATH --operator [--project-root P]
evolve landing reject --cycle N --tree SHA --operator [--project-root P]
```

- **What it does:** Lets the operator decide a proposal that the ship gate held (ADR-0157).
- **How it works:** `internal/heldproposal` lists the held records in `.evolve`. `approve` and `reject` write a decision after they check the tree digest, the audit report file, `--operator` and the dispatch credential.
- **Effect and safety:** `list` is READ-ONLY. `approve` and `reject` WRITES-LOCAL (a decision record). It refuses without `--operator`, refuses a phase cell with no credential and refuses a tree that differs from the held tree. `approve` needs `--audit-report`.
- **Output:** No `--json`. Text.
- **Exit codes:** 0, 1 refused, 2 I/O, 10 usage.
- **Called by:** the operator only. The ship gate and `wave status` print the hint.
- **Policy conformance:** CD-30: not in `--help`. CD-14: near name `land`. CD-12: `approve` and `reject` are not standard verbs. CD-41: no `--json`.

#### `sync-main`

```
evolve sync-main [--json] [--project-root P]
```

- **What it does:** Reconciles a local main that diverged from origin, by merging only.
- **How it works:** It fetches the branch. It refuses on a live lease or a dirty tree. It then runs `git merge --no-edit origin/<branch>` and aborts the merge on a conflict. It never rebases, force-pushes or pushes.
- **Effect and safety:** WRITES-GIT (a local merge) and a network fetch. It refuses (exit 3) on a live lease or a dirty tree. Agent class: denied. No dry-run.
- **Output:** `--json`.
- **Exit codes:** 0, 1 failure or merge conflict, 3 refused, 10 usage.
- **Called by:** the operator and the boundary.
- **Policy conformance:** CD-30: not in `--help`. CD-04: flat hyphenated name; proposed `main sync` or `git sync` (proposed, not built). CD-12: `sync` is not a standard verb. CD-33: `--help` exits 10. CD-61: it merges and has no `--dry-run`.

#### `commit-gate`

```
evolve commit-gate run --reviewers "<csv>" [--files "<paths>"] [--no-install] [--project-root P]
```

- **What it does:** Runs the pre-commit quality gate on the staged change: lint and targeted tests. It then writes an attestation bound to the tree, which `ship --class manual` checks.
- **How it works:** `internal/commitgate` detects the changed files (or reads `--files`) and runs lint, targeted tests, and comment and history checks. It can install missing tools. It writes `.commit-gate/attestation.json` and prints the tree-state SHA on pass. It reads `CG_ATTEST_DIR` from the environment.
- **Effect and safety:** STARTS-PROCESSES (linters, `go test`) and WRITES-LOCAL (the attestation, and tool installs unless `--no-install`). `--no-install` fails hard in place of an install. It needs a git repository.
- **Output:** The tree SHA on stdout on pass only. Logs on stderr. No `--json`.
- **Exit codes:** 0 pass, 1 fail, 2 git fatal, 3 tool missing, 10 bad arguments.
- **Called by:** the operator and the `evo:commit` skill. `ship` checks the attestation.
- **Policy conformance:** CD-30: not in `--help`. CD-04: hyphenated top-level name (proposed `commit gate run`, proposed, not built). CD-76: reads `CG_ATTEST_DIR` from the environment. CD-05: sibling `commit-prefix-gate` is a flat name beside this group. CD-41: no `--json`.

#### `commit-prefix-gate`

```
evolve commit-prefix-gate --msg "<commit-msg>" [--repo-dir P] [--staged | --diff-ref REF] [--manifest P] [--bypass]
```

- **What it does:** Checks that the prefix of a commit message (`feat:`, `fix:`) matches the scope of the files in the change.
- **How it works:** `internal/commitprefixgate` compares the prefix with a manifest of allowed scopes against the staged diff or a ref range. It logs a bypass in the guards log.
- **Effect and safety:** READ-ONLY, plus one audit line on bypass. `--bypass` works only with `SHIP_CLASS=manual` in the environment.
- **Output:** No `--json`. Diagnostics on stderr.
- **Exit codes:** 0, 1 fail, 2 scope violation, 3 bad arguments, 4 bad manifest. These differ from the shared table.
- **Called by:** the ship gate path only. No script or CI caller found. Possible internal-only verb.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `commit-gate`; proposed `commit-gate prefix` (proposed, not built). CD-76: reads `SHIP_CLASS` from the environment. CD-50: uses exit codes 2, 3 and 4 with other meanings than the table. CD-41: no `--json`.

#### `console-lease`

```
evolve console-lease [--ttl D=30m] [--reason T] [--project-root P=.] [--clear] <path>...
```

- **What it does:** Lets the operator touch exact files in the runtime tree for a short time without the tree-diff guard blocking the next cycle.
- **How it works:** It normalizes the repo-relative paths (they must exist). It writes a JSON lease (paths, expiry, reason) to the git common directory with a temp file and a rename. The cycle start adopts the lease once.
- **Effect and safety:** WRITES-LOCAL (an untracked file in the git common directory). It takes exact paths only, with no globs and no absolute paths. The lease expires. `--clear` removes it. Agent class: denied.
- **Output:** One status line on stdout. No `--json`.
- **Exit codes:** 0, 1 fail, 10 usage.
- **Called by:** the operator and console.
- **Policy conformance:** CD-30: not in `--help`. CD-04: hyphenated flat name; proposed `console lease` (proposed, not built). CD-33: `-h` exits 10. CD-20: Go help prints single-dash names.

#### `worktree`

```
evolve worktree create --cycle N [--project-root P] [--base DIR] [--lane L]
evolve worktree create --dev TASK --branch B [--project-root P]
evolve worktree list [--project-root P]
evolve worktree cleanup [--project-root P] [--base DIR] [--cycle N | --lane L] [--dev TASK | --dev --all] [--dry-run] [--stale [--apply]]
```

- **What it does:** Makes and removes the git worktrees that cycles and dev tasks of the hub use.
- **How it works:** It runs git (`worktree add --detach`, `worktree list`, `worktree remove --force`, `worktree prune -v`). Dev mode adds a worktree at the fetched `origin/main` on a new branch. `cleanup --dev` removes landed trees and deletes their branches. `--stale` lists sealed cycle trees that no run lease holds.
- **Effect:** `create` WRITES-GIT (worktree and branch). `list` is READ-ONLY. `cleanup` WRITES-GIT (it removes trees and deletes branches).
- **Safety:** `cleanup --dev --dry-run` previews. `--stale` is a listing unless `--apply`. Dev cleanup keeps trees whose head is not proven merged. Cycle cleanup with `--force` has no dry-run.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 git failure, 10 usage.
- **Called by:** the loop (cycle setup) and the operator.
- **Policy conformance:** CD-12: `cleanup` is not a standard verb (proposed `worktree prune`, proposed, not built). CD-41: no `--json`. Help text differs: the registry summary omits `--dev` and `--stale`.

#### `checkpoint`

```
evolve checkpoint save [--worktree DIR | --all] [--label T] [--push] [--json] [--project-root P]
evolve checkpoint list [--worktree DIR] [--json]
evolve checkpoint restore <ref> --into DIR [--json]
evolve checkpoint prune [--landed] [--json]
```

- **What it does:** Saves uncommitted worktree work as hidden git refs, so you can restore it later. It does not touch the branch, the index or the tree.
- **How it works:** `internal/wtcheckpoint` writes `refs/checkpoints/*` from a temporary index. The retention comes from `policy.json`. `restore` rebuilds a tree in a new or clean worktree with a detached HEAD.
- **Effect and safety:** `save` and `prune` WRITES-GIT (refs). `restore` WRITES-LOCAL. `list` is READ-ONLY. No dry-run. Agent class: denied.
- **Caution:** `save --push` WRITES-REMOTE: it pushes `refs/checkpoints/*` to origin. The push is opt-in and never the default.
- **Output:** `--json` on all four.
- **Exit codes:** 0, 1 failed, 10 usage.
- **Called by:** the operator and console. The loop calls it before `gc`.
- **Policy conformance:** CD-61: `prune` deletes refs and has no `--dry-run`. Otherwise conforms (group with standard verbs).

#### `archive`

```
evolve archive restore <cycle> [--worktree DIR] [--project-root P]
```

- **What it does:** Rebuilds the branch, patch and untracked files of a failed cycle from its archive leaf, so the work is not lost.
- **How it works:** `gc.RestoreArchive` reads the leaf manifest and the archive ref or bundle. It re-creates the branch and a worktree, by default `.evolve/worktrees/<leaf>`.
- **Effect and safety:** WRITES-GIT (branch and worktree) and WRITES-LOCAL. It refuses a leaf whose hash fails and a branch that already exists. No dry-run.
- **Output:** One line on stdout. No `--json`.
- **Exit codes:** 0 restored, 1 refused or failed, 10 usage.
- **Called by:** the operator, for incident recovery.
- **Policy conformance:** CD-30: not in `--help`. CD-61: it writes git state and has no `--dry-run`. CD-41: no `--json`. The installed binary a099157d does not know this verb. It is in the source registry at HEAD only.

#### `salvage`

```
evolve salvage report [--json] [--project-root P]
evolve salvage list [--json] [--project-root P]
```

- **What it does:** Reports how often a bad verdict was recoverable. Lists the salvage branches that are left over, for the operator.
- **How it works:** `report` summarizes the bad-verdict baseline file in `.evolve` through `internal/deliverable`. `list` reads the operator-salvage leaves of `gc` and runs `git merge-base --is-ancestor <head> origin/main` for each leaf.
- **Effect and safety:** READ-ONLY (local git reads).
- **Output:** `--json`. Text on stdout.
- **Exit codes:** 0, 1 summary read error, 2 list or JSON encode failure, 10 usage.
- **Called by:** the operator.
- **Policy conformance:** CD-12: `report` is not a standard verb (proposed `salvage show`, proposed, not built). CD-20: the usage of `report` prints `-json` and `-project-root` with one dash. Help text differs: `list` prints double-dash flags.

#### `ci`

```
evolve ci classify <run-id|run:N|pr:N|sha:H> [--json] [--rerun] [--project-root P]
evolve ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N] [--project-root P]
evolve ci darwin-set <generate|check> [--project-root P]
evolve ci docs-tests [--project-root P]       (paths on stdin)
evolve ci telemetry --jobs FILE [--job-artifacts "JOB=DIR"]... [--json]
```

- **What it does:** Decides if a red CI run is a real failure, a pre-existing failure, a flake or unknown. Waits for the CI of a SHA, PR or tag. Keeps the darwin test set in step.
- **How it works:** `classify` reads runs, jobs and logs with `gh` and the main-branch history. It files fix-first inbox items for tests that were red on the base. `watch` polls `gh` until the workflows end. `darwin-set generate` writes the darwin test set file.
- **Effect and safety:** `classify` reads the network and WRITES-LOCAL (inbox items). `watch` reads the network. `darwin-set generate` WRITES-LOCAL. No dry-run.
- **Caution:** `classify --rerun` WRITES-REMOTE: it runs `gh run rerun --failed` and starts GitHub jobs again.
- **Output:** `--json` on `classify` and `telemetry`. A tab table otherwise.
- **Exit codes:** `classify`: 0 retry-safe, 1 not retry-safe or no finished run, 2 I/O, 10 usage. `watch`: 0 green, 1 red, 2 not observable, 10 usage.
- **Called by:** CI (`ci docs-tests`), the loop and console review. The boundary calls a `ci latency` sub-verb that does not exist.
- **Policy conformance:** CD-12: `darwin-set` and `docs-tests` are not standard verbs. Help text differs: the registry names only `classify` and `watch`.

### 3. The inbox and planning

#### `inbox`

```
evolve inbox batches | lint | list | show | rank | claims
evolve inbox add --file <item.json|-> [--origin O] [--allow-overlap --reason R]
evolve inbox list [--status ready|console|waiting] [--kind K] [--route R]
evolve inbox rank [--list ...] [--top N] [--explain <id>]
evolve inbox edit | verify | withdraw | release | split | route-console | route-lane | curate | ack-fingerprint
evolve inbox claim <id> --cycle N
evolve inbox consume <item-path> [--resolution T] [--cycle n|console]
evolve inbox quarantine list|release
evolve inbox trailer-check --range <rev-range>
```

Each sub-verb takes `--project-root P`. Most take `--json`. Run `evolve inbox --help` for the exact flag lines.

- **What it does:** Operates the backlog of work items (the inbox) that triage and the lanes consume.
- **How it works:** `internal/cli/inboxcmd` reads and moves JSON item files in `.evolve/inbox/` (processing, consumed, quarantine). It uses `inboxmover`, the curator and the signal center.
- **Effect and safety:** `list`, `show`, `rank`, `lint`, `batches`, `claims`, `quarantine list` and `trailer-check` are READ-ONLY. The others WRITES-LOCAL (inbox files). In the middle of a wave, `edit`, `withdraw`, `verify` and `route-*` are queued and exit 4. An overlap in `add` is refused with exit 3. No dry-run.
- **Output:** `--json` (two hyphens only), as an envelope.
- **Exit codes:** 0, 1 failed, 2 I/O, 3 refused, 4 queued, 10 usage.
- **Called by:** the operator, console and the loop (`claim`, `consume`).
- **Policy conformance:** CD-30: not in `--help` (the registry points to `evolve inbox --help`). CD-12: `rank`, `claim`, `release`, `split`, `consume`, `curate`, `withdraw` and `ack-fingerprint` are not standard verbs. CD-13: `verify` is an avoided word (proposed `inbox check`, proposed, not built). The sub-verbs `route-console`, `route-lane` and `ack-fingerprint` hold two words (proposed `inbox route console`, proposed, not built).

#### `inbox-mover`

```
evolve inbox-mover claim <task_id> <cycle>
evolve inbox-mover promote <task_id> <processed|rejected|retry> [<cycle>] [--commit-sha SHA]
evolve inbox-mover recover-orphans
```

- **What it does:** The low-level inbox claim, promote and recover verb that older phase scripts used. It also recovers orphan claims.
- **How it works:** `internal/inboxmover` moves items between the inbox state directories. `recover-orphans` releases the claims whose holder process is gone.
- **Effect and safety:** WRITES-LOCAL (inbox files). `recover-orphans` releases only claims whose holder is gone. No dry-run.
- **Output:** No `--json`. Logs `[inbox-mover]` on stderr.
- **Exit codes:** `claim`: 0, 1 bad arguments or not found, 2 move failed, 3 routed to console, 4 waiting on a dependency. `promote`: 0, 1, 2. A usage error exits 1.
- **Called by:** the operator, for recovery. Two hint messages tell the operator to run `inbox-mover recover-orphans`. It is a legacy surface of `inbox claim` and `inbox consume`.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name with sub-verbs, beside the `inbox` group. Proposed `inbox claim`, `inbox promote` and `inbox recover` (proposed, not built). CD-15: agent noun. CD-51: usage exits 1.

#### `carryover`

```
evolve carryover apply-decisions [--state P] [--decisions P] [--apply]
evolve carryover list [--json] [--sort age|unpicked] [--limit N] [--state P]
```

- **What it does:** Lists the carried-over todos. Applies a reviewed keep, drop or cluster decisions file to them.
- **How it works:** It reads `carryoverTodos` in `.evolve/state.json`. `apply-decisions` prints a plan. With `--apply` it rewrites `state.json` in a locked update.
- **Effect and safety:** `list` is READ-ONLY. `apply-decisions` is a dry run by default. With `--apply` it WRITES-LOCAL. The default `--decisions` file is a dated name (`.evolve/carryover-decisions-2026-07-21.json`), so the verb is probably a one-time migration tool.
- **Output:** `--json` on `list`.
- **Exit codes:** 0, 1 failed step, 2 state read error, 10 usage.
- **Called by:** the operator.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `apply-decisions` is not a standard verb. Help text differs: the registry and the no-argument error name only `apply-decisions`, but `list` exists. Possible stale one-shot verb.

#### `issue`

```
evolve issue evidence add [--project-root P] <incident-id> <source-file>
evolve issue check [--project-root P] [<incident-id>]
evolve issue list [--project-root P] [--status S] [--code C] [--json]
```

- **What it does:** Keeps the incident records and their redacted evidence bundles checked and listed.
- **How it works:** `internal/incidentbundle` stores evidence in `knowledge-base/incidents/<id>/` with redaction and a byte budget from the policy. `internal/incidentkb` reads the records in `docs/incidents/`.
- **Effect and safety:** `evidence add` WRITES-LOCAL. `check` and `list` are READ-ONLY. Evidence is redacted and size-limited. No dry-run.
- **Output:** `--json` on `list` only.
- **Exit codes:** 0 pass, 1 check failure or error, 10 usage.
- **Called by:** the operator and console.
- **Policy conformance:** Conforms in shape (a nested `evidence add` is allowed). CD-41: `check` has no `--json`.

#### `kb`

```
evolve kb graph neighbors <node-id> [--hops N] [--json] [--project-root P]
evolve kb graph recurrences <fingerprint> [--json] [--project-root P]
```

- **What it does:** Queries the knowledge-base graph that links cycle dossiers and inbox items, to find failures that recur.
- **How it works:** `internal/kbgraph` loads `knowledge-base/` and `.evolve/inbox/`. The hop count has a cap (`kbgraph.MaxHops`).
- **Effect and safety:** READ-ONLY.
- **Output:** `--json`. Usage on stderr.
- **Exit codes:** 0, 1 load error, 10 usage.
- **Called by:** the operator and agents. No code caller found.
- **Policy conformance:** CD-12: `neighbors` and `recurrences` are nouns used as verbs (proposed `kb graph show`, proposed, not built).

#### `lessons`

```
evolve lessons recurrence
evolve lessons backfill [--write] [--project-root P]
```

- **What it does:** Reports which lesson patterns recur and their fix status. Backfills the recurrence ledger from lesson files.
- **How it works:** `internal/recurrence` loads `.evolve/recurrence-ledger.json`. `backfill` scans the lesson directories. With `--write` it replaces the ledger in one atomic write.
- **Effect and safety:** `recurrence` is READ-ONLY. `backfill` is a dry run unless `--write`; then it WRITES-LOCAL.
- **Output:** No `--json`. Text on stdout.
- **Exit codes:** 0, 1 ledger load error, 2 I/O, 10 usage.
- **Called by:** the operator. No caller found.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-62: `backfill` previews by default but uses `--write` where the policy has `--apply`. Help text differs: the usage line shows `recurrence [--project-root P]`, but the function ignores flags and reads `EVOLVE_PROJECT_ROOT` (CD-76). The registry summary omits `backfill`.

#### `failures`

```
evolve failures list [--class C] [--json] [--project-root P]
evolve failures reset [--fingerprint F] --project-root P
evolve failures prune [--dry-run] [--project-root P]
```

- **What it does:** Inspects and clears the failed-approach memory in `state.json`, so that old blockers stop steering the loop.
- **How it works:** `failurelog` works on `.evolve/state.json` (`failedApproaches`, `carryoverTodos`). `reset` with `--fingerprint` also writes `resolved-fingerprints.json`.
- **Effect and safety:** `list` is READ-ONLY. `reset` and `prune` WRITES-LOCAL. `prune` has `--dry-run`. A real `reset` or `prune` refuses (exit 1) without an explicit `--project-root`.
- **Output:** `--json` on `list` only.
- **Exit codes:** `list`: 0, 2 I/O, 10 usage or unknown class. `reset`: 0, 1, 2, 10. `prune`: 0, 1, 2, 10.
- **Called by:** the operator.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-61: `reset` deletes memory and has no `--dry-run`. CD-20: the Go flag package accepts single-dash forms too.

#### `dossier`

```
evolve dossier verify [--json] [--project-root P]
evolve dossier retro-mislabel [--project-root P] [--json]
evolve dossier publish [--dry-run] [--project-root P]
evolve dossier sweep [--dry-run] [--project-root P]
evolve dossier show <cycle> [--json] [--project-root P]
```

- **What it does:** Checks and publishes the per-cycle dossiers (the permanent record of each cycle) in `knowledge-base/cycles`.
- **How it works:** `verify` walks `knowledge-base/cycles` and checks each pair and the policy floor `dossier-closeout`. `publish` moves pending pairs from `.evolve/dossiers-pending` into `knowledge-base/cycles` under a lock. `sweep` recommits pairs that are not committed. `retro-mislabel` reports wrongly labelled retrospectives.
- **Effect and safety:** `verify`, `show` and `retro-mislabel` are READ-ONLY. `publish` WRITES-LOCAL (moves files). `sweep` WRITES-GIT (a commit). `publish` and `sweep` have `--dry-run`. A held lock gives exit 1.
- **Output:** `--json` on `verify`, `show` and `retro-mislabel`.
- **Exit codes:** `verify`: 0, 1, 2, 10. `publish` and `sweep`: 0, 1 lock held, 2 error, 10. `retro-mislabel`: 0, 1, 10.
- **Called by:** the loop (`publish` after ship) and the operator. `verify` is agent class A.
- **Policy conformance:** CD-13: `verify` is an avoided word (proposed `dossier check`, proposed, not built). CD-12: `publish` and `sweep` are not standard verbs.

#### `postmortem`

```
evolve postmortem collect --cycle N --phase P --attempt K (--session UUID | --transcript PATH) --cause CODE [--exit-code E] [--cli C] [--started RFC3339 --ended RFC3339] [--force] [--project-root P]
evolve postmortem show --cycle N --phase P [--project-root P]
```

- **What it does:** Records why a phase dispatch ended badly (ADR-0130), so that the next try sees "Previous attempts".
- **How it works:** It reads a Claude transcript (by session id or path) and writes the try record in the run directory of the cycle. `show` prints the section that the next dispatch receives.
- **Effect and safety:** `collect` WRITES-LOCAL. `show` is READ-ONLY. `collect` refuses to replace a record that exists without `--force` (exit 1).
- **Output:** No `--json`. Output on stdout.
- **Exit codes:** 0, 1 refused, 2 I/O, 10 usage.
- **Called by:** the pipeline runner writes the same record. The verb is for a manual backfill. No caller found.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `collect` is not a standard verb (proposed `postmortem add`, proposed, not built). CD-41: no `--json`.

#### `practice`

```
evolve practice watch [--dry-run] [--json] [--project-root P]
```

- **What it does:** Notices when the changelog or the prompting guide of a CLI vendor changed, and files a research item.
- **How it works:** `internal/practicewatch` fetches each source in the policy (HTTP GET with a redirect guard and ETag). It compares with the stored snapshot, files one inbox research item for each changed source and updates the snapshots.
- **Effect and safety:** It reads the network and WRITES-LOCAL (snapshots and inbox items). `--dry-run` fetches and compares and records nothing. It refuses a bad policy.
- **Output:** `--json`. Output on stdout.
- **Exit codes:** 0, 1 refused or bad policy, 10 usage, 130 interrupted.
- **Called by:** the loop at the boundary (60 second budget) and the operator.
- **Policy conformance:** Conforms in shape. CD-12: the noun `practice` is a loose group name.

#### `routing`

```
evolve routing explain --cycle N [--json] [--project-root P]
evolve routing replay --cycle N [--project-root P]
evolve routing plan --cycle N [--goal-text TEXT] [--project-root P]
```

- **What it does:** Shows why the advisor chose a phase plan for a cycle, replays it, or asks the advisor for a new plan.
- **How it works:** `explain` and `replay` read the recorded decision, `phase-plan.json` and `advisor-response-plan.txt` in the cycle workspace. `replay` parses the saved response again through the integrity floor and compares the run sets. `plan` builds the input in a temp directory and calls the advisor. It writes nothing in `runs/cycle-N`.
- **Effect and safety:** `explain` and `replay` are READ-ONLY. `plan` LAUNCHES-LLM. No dry-run.
- **Output:** `--json` on `explain` only.
- **Exit codes:** 0, 1 root error, 2 advisor not wired or launch failed, 3 replay mismatch, 10 usage.
- **Called by:** the operator.
- **Policy conformance:** CD-05: overlaps with `cli-routing` (two routing nouns; proposed `phase routing`, proposed, not built). Help text differs: the usage line omits `--json` and `--goal-text`.

#### `convergence`

```
evolve convergence decide --input <rounds.json> [--json]
evolve convergence metrics [--json] <rounds.json>...
```

- **What it does:** Says what to do next in a repeat-until-accepted review loop (converge or escalate, never round 4). Measures how loops converged.
- **How it works:** The convergence policy (ADR-0126) reads `rounds.json` and prints the next rung, or the metrics.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json`. Text otherwise.
- **Exit codes:** 0, 1 write error, 2 read error, 10 bad usage or input.
- **Called by:** console review chains. `AGENTS.md` says the pipeline loops do not call it yet. Possible unused verb.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `decide` is not a standard verb; proposed `convergence check` (proposed, not built).

### 4. Inspect and debug

#### `status`

```
evolve status [--json] [--quality report|all] [--project-root P]
```

- **What it does:** Prints one read-only report. It covers the loop, the cycles, the ship streak, the open PRs with their checks and the failing CI on main.
- **How it works:** `buildStatusReport` reads the `.evolve` state and runs `gh pr list --state open --json ...` and the CI state. It renders text or JSON.
- **Effect and safety:** READ-ONLY (network read through `gh`). Agent class: allowed.
- **Output:** `--json` prints one JSON object (also `-json`).
- **Exit codes:** 0, 1 root or encode error, 10 usage or a stray argument.
- **Called by:** the operator, console and the loop.
- **Policy conformance:** On the flat list. CD-33: `--help` exits 10. Help text differs: the registry summary omits `--quality`.

#### `dashboard`

```
evolve dashboard [--addr 127.0.0.1:8090] [--project-root P] [--snapshot]
```

- **What it does:** Serves a read-only live web page of the pipeline state, or prints one JSON snapshot.
- **How it works:** `internal/dashboard` builds a snapshot of the `.evolve` state and serves it over HTTP until Ctrl-C. It listens on the loopback address by default and has no authentication.
- **Effect and safety:** STARTS a long-running HTTP server. It writes no state. `--snapshot` is READ-ONLY. The docs warn that the page shows text that agents wrote, with no authentication.
- **Output:** `--snapshot` prints JSON on stdout. The flag is `--snapshot`, not `--json`. The serving notice goes to stderr.
- **Exit codes:** 0, 1 error, 10 usage or help.
- **Called by:** the operator.
- **Policy conformance:** CD-41: uses `--snapshot` in place of `--json`. CD-33: `--help` exits 10. CD-20: Go help prints single-dash names. CD-03: flat verb outside the flat list; proposed `dashboard serve` (proposed, not built).

#### `events`

```
evolve events channels [--json] [--project-root P]
evolve events kinds [--json] [--project-root P]
evolve events watch --channel C[,C] [--filter EXPR] [--since all|new|last|CURSORS|RFC3339] [--until EXPR] [--json] [--project-root P]
```

- **What it does:** Reads the event channels of the pipeline. Lists the channels, lists the filter words, or streams records.
- **How it works:** It uses `internal/events` to read the segments in `.evolve/events/ch/<channel>` and the kernel wake. `watch` blocks and prints each record until `--until`.
- **Effect and safety:** READ-ONLY. Agent class: allowed.
- **Output:** `--json` (two hyphens only). Records go to stdout and errors to stderr. With `--json`, `watch` prints one record for each line.
- **Exit codes:** 0 ok or stopped, 2 I/O, 3 output gone or no kernel wake, 10 usage, 11 unknown kind or channel.
- **Called by:** the operator, console and LLM seats (event watches).
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-12: `channels` and `kinds` are nouns used as sub-verbs (proposed `events channel list`, proposed, not built).

#### `signals`

```
evolve signals codes generate|check
evolve signals tail [--cycle N | --channel NAME] [--kind K[,K...]] [--code C[,C...]] [--follow] [--json] [--project-root P]
```

- **What it does:** Shows live Signal Center events. Keeps the signal-codes document in step with the code registry.
- **How it works:** `tail` follows the channels or the cycle events in `.evolve` (poll each second) until SIGINT. `codes generate` writes `docs/architecture/signal-codes.md` from the registry. `codes check` compares it.
- **Effect and safety:** `tail` and `codes check` are READ-ONLY. `codes generate` WRITES-LOCAL. `check` is the dry mode. Agent class: `tail` allowed.
- **Output:** `--json` on `tail`.
- **Exit codes:** `codes`: 0, 1 read or write error, 2 drift (`check`), 10. `tail`: 10 usage, 130 interrupted.
- **Called by:** the operator and console watches.
- **Policy conformance:** CD-30: not in `--help`. CD-52: drift exits 2, but the table gives 1 to a failed check. CD-12: `tail` is not a standard verb (proposed `signals watch`, proposed, not built). Help text differs: the registry summary omits `tail`.

#### `logs`

```
evolve logs digest --cycle N
evolve logs events --cycle N [--phase P] [--kind K] [--code C] [--level L] [--from N]
evolve logs failures --cycle N [--phase P] [--kind K] [--code C] [--from N]
evolve logs index --cycle N [--from N]
evolve logs show --cycle N --phase P [--layer d2|d3] [--from LINE]
```

Each form takes `--as PHASE` and `--project-root P`.

- **What it does:** Lets an operator or an agent read the logs of a cycle in bounded layers, in place of whole files.
- **How it works:** `internal/logread` gives each sub-verb a hard line budget (40, 200 or 150). Each output ends with a truncation line that names a narrower query. `--as PHASE` checks the `log_access` rule of the phase in shadow mode and can emit a warning signal.
- **Effect and safety:** READ-ONLY (shadow mode can write a signal record).
- **Output:** No `--json`. Output on stdout.
- **Exit codes:** 0, 1 no log or read error, 10 usage.
- **Called by:** agents and the operator.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-41: no `--json`. CD-12: `digest` is not a standard verb.

#### `ledger`

```
evolve ledger verify [--evolve-dir D] [--project-root P] [--deep] [--json]
evolve ledger tail [--evolve-dir D] [--n N]
evolve ledger seal [--evolve-dir D] [--keep N]
evolve ledger anchor <entry_seq> [--note S] [--line-sha SHA] [--evolve-dir D]
evolve ledger rebaseline --note S [--evolve-dir D]
evolve ledger evidence restore [--evolve-dir D] [--repo DIR] [--dry-run]
```

- **What it does:** Checks the hash chain of the audit ledger. With operator action it seals, anchors or rebaselines the ledger.
- **How it works:** `internal/adapters/ledger` walks `.evolve/ledger.jsonl` (and the sealed segments with `--deep`). `seal` moves old lines to segments. `anchor` and `rebaseline` record a trust point of an epoch in `ledger-anchor.json`.
- **Effect and safety:** `verify` and `tail` are READ-ONLY. `seal`, `anchor`, `rebaseline` and `evidence restore` WRITES-LOCAL. `rebaseline` requires `--note`. `evidence restore` has `--dry-run`. Agent class: `verify` allowed.
- **Output:** `--json` on `verify` only. The human OK line goes to stderr, so stdout is empty without `--json`.
- **Exit codes:** `verify`: 0 ok, 2 chain broken or I/O, 10 usage. `seal` and `anchor`: 0, 1, 2, 10. `tail` and `rebaseline`: 0, 1, 10. `evidence restore`: 0, 1, 2, 10.
- **Called by:** CI, the operator and the loop.
- **Policy conformance:** CD-13: `verify` is an avoided word (proposed `ledger check`, proposed, not built). CD-52: a broken chain exits 2, but the table gives 1 to a failed check. CD-12: `seal`, `anchor` and `rebaseline` are not standard verbs. CD-48: the OK line goes to stderr. Help text differs: the help lists only `verify` and `tail`.

#### `tokens`

```
evolve tokens report [--last N=8] [--project-root P] [--evolve-dir D] [--json]
```

- **What it does:** Ranks which phases use the most tokens over recent cycles. Flags usage that nobody measured.
- **How it works:** It walks the ledgers and phase records of the last N cycles in `.evolve`. It sums `TokenUsage` for each phase and ranks the consumers. It prints a TRIPWIRE when a launch resolved to `source=none`.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json`.
- **Exit codes:** 0, and 10 for usage and also for any other error.
- **Called by:** the operator. No automated caller.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-52: a runtime error exits 10, which is the usage code. CD-12: `report` is not a standard verb (proposed `tokens list`, proposed, not built). Help text differs: the registry shows only `report [--last N]`. Gap: there is no `--by` option.

#### `ladder`

```
evolve ladder report [--json] [--project-root DIR]
```

- **What it does:** Shows how many tries, fixes and tokens each retry-ladder rung and failure class used.
- **How it works:** `internal/retryladder.LoadRuns` reads `.evolve/runs/*/llm-calls.ndjson` and `BuildReport` builds the report.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json` (also `-json`). A tab table on stdout.
- **Exit codes:** 0, 1 encode error, 2 runs directory absent or unreadable, 10 usage.
- **Called by:** the operator. No code caller found.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `report` is not a standard verb (proposed `ladder show`, proposed, not built).

#### `soak-report`

```
evolve soak-report --cycles A-B|N [--project-root P]
```

- **What it does:** Prints the `EVOLVE_PHASE_RECOVERY` soak evidence table for a range of cycles.
- **How it works:** `internal/soakreport` reads the run artifacts of those cycles and renders a table.
- **Effect and safety:** READ-ONLY.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 read error, 10 usage or missing `--cycles`.
- **Called by:** the operator (pipeline dashboard guide). No automated caller. Possibly legacy.
- **Policy conformance:** CD-30: not in `--help`. CD-04: flat hyphenated name; proposed `soak report` (proposed, not built). CD-33: `--help` exits 10. CD-41: no `--json`.

#### `cloud`

```
evolve cloud list [--json] [--project-root P]
```

- **What it does:** Lists the cloud sessions (URL, title, time) that cloud seats launched.
- **How it works:** It reads each launch record under `.evolve` and prints those that have a cloud session. It starts nothing.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json` (an array).
- **Exit codes:** 0, 1 unreadable launch record, 10 usage.
- **Called by:** the operator. Agent class: allowed.
- **Policy conformance:** CD-30: not in `--help`. Otherwise conforms.

#### `audit`

```
evolve audit calibration --output FILE [--project-root P] [--dossiers-dir D] [--runs-dir D]
```

- **What it does:** Reports how well the verdicts of the auditor match what happened later.
- **How it works:** `internal/auditcalibration.Generate` reads `knowledge-base/cycles` and `.evolve/runs`. It writes a Markdown report to `--output` in one atomic write.
- **Effect and safety:** WRITES-LOCAL (the output file only). `--output` is required, its parent directory must exist, and it must not be a directory.
- **Output:** The report goes to the file. No `--json`.
- **Exit codes:** 0, 1 I/O or generate error, 2 usage (also for no sub-verb).
- **Called by:** the operator, for analysis.
- **Policy conformance:** CD-12: `calibration` is not a standard verb (proposed `audit report`, proposed, not built). CD-51: usage error exits 2. CD-41: no `--json`. The name collides with the `audit` pipeline phase.

#### `context-fill`

```
evolve context-fill correlate [--project-root P=.] [--out PATH] [--json]
```

- **What it does:** Compares how full the context window got in each phase with the final verdict of the cycle.
- **How it works:** `internal/contextfillcorrelate` joins the dossiers in `knowledge-base/cycles` with the telemetry in `.evolve/runs`. It prints Markdown or JSON and can write it atomically to `--out`.
- **Effect and safety:** READ-ONLY by default. `--out` WRITES-LOCAL (one file). It refuses to write over a file that is not regular.
- **Output:** `--json`. Markdown on stdout otherwise.
- **Exit codes:** 0, 1, 10.
- **Called by:** the operator, for analysis. No script caller.
- **Policy conformance:** CD-04 and CD-05: hyphenated name beside the `context` group; proposed `context fill correlate` (proposed, not built). CD-12: `correlate` is not a standard verb (proposed `context fill show`).

#### `cycle-health`

```
evolve cycle-health <cycle-N> <workspace>
```

- **What it does:** Runs 11 integrity checks on the workspace of a cycle and says OK or HALT.
- **How it works:** `internal/cyclehealth.Check` reads the run workspace, the ledger and the retry configuration of the policy. It prints the anomalies. It can write a fingerprint file in the workspace.
- **Effect and safety:** READ-ONLY for the project. It can write one fingerprint file in the workspace.
- **Output:** Text on stdout. No `--json`.
- **Exit codes:** 0 OK, 1 HALT or check error, 10 usage.
- **Called by:** the pipeline and skills.
- **Policy conformance:** CD-04 and CD-05: hyphenated name beside the `cycle` group; proposed `cycle health` (proposed, not built). CD-33: `-h` exits 10. CD-41: no `--json`. CD-25: two positional operands.

#### `selfheal`

```
evolve selfheal incidents [--json] [--project-root P]
evolve selfheal propose [--json] [--project-root P]
evolve selfheal record-doc <incident-id> <path> [--json] [--project-root P]
```

- **What it does:** Detects pipeline incidents from signals, cycle state, CI runs and usage. Then proposes and records fixes (ADR-0155).
- **How it works:** `incidents` loads the healing-detector settings of `policy.json`. It runs the detection over the signal channels, the cycle state, `gh run list` results and the usage readings. It writes records in `.evolve/incidents`. `propose` launches an orchestrator agent seat through `agentcmd` and can file inbox items. `record-doc` sets the doc path of an incident.
- **Effect and safety:** `incidents` and `record-doc` WRITES-LOCAL. `propose` LAUNCHES-LLM. The detector stage `off` skips the detection. No dry-run. Agent class: denied.
- **Output:** `--json` prints an envelope on every exit.
- **Exit codes:** 0, 2 I/O, 10 usage, 11 input.
- **Called by:** the wave boundary (`selfheal incidents` and `selfheal propose`) and the operator.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `incidents`, `propose` and `record-doc` are not standard verbs. CD-11: `record-doc` has a hyphen. The bare form was not probed.

#### `estimate-quota-reset`

```
evolve estimate-quota-reset [WORKSPACE]
```

It reads `EVOLVE_PROJECT_ROOT` from the environment.

- **What it does:** Predicts when the quota wall of a CLI resets, so that a paused cycle knows when to wake.
- **How it works:** `internal/quotareset.Compute` reads `quota-reset-hint.txt` in the workspace and the `quota_reset` settings of `policy.json`. It prints the time.
- **Effect and safety:** READ-ONLY.
- **Output:** Text on stdout. No `--json`.
- **Exit codes:** 0, 1 fatal.
- **Called by:** the checkpoint package calls the same function in-process. No script, skill or CI caller of the verb. Possible dead verb.
- **Policy conformance:** CD-04: flat hyphenated name. CD-12: `estimate` is not a standard verb; proposed `quota show` (proposed, not built). CD-76: reads a control value from the environment. CD-41: no `--json`.

#### `verdicts`

```
evolve verdicts stats [--project-root P]
evolve verdicts wipe [--apply] [--project-root P]
```

- **What it does:** Reads or empties the shared verdict store that the build floor and ship use.
- **How it works:** The `verdictcache` store is under the project root. `stats` counts the entries and files. `wipe` only reports what it will remove. With `--apply` it empties the store.
- **Effect and safety:** `stats` is READ-ONLY. `wipe --apply` WRITES-LOCAL. `wipe` previews by default.
- **Output:** No `--json`.
- **Exit codes:** 0, 2 store error, 10 usage.
- **Called by:** the operator. No automated caller.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-13: `wipe` is an avoided word (proposed `verdicts clear`, proposed, not built). CD-41: no `--json`. `--apply` is the policy form (CD-62).

#### `swarm`

```
evolve swarm status --cycle N [--evolve-dir D]
evolve swarm reap --cycle N [--evolve-dir D]
evolve swarm reap-orphans [--evolve-dir D] [--dry-run]
```

- **What it does:** Inspects and removes leftover swarm worker sessions (ADR-0032).
- **How it works:** It reads the swarm registry in `.evolve`. `status` lists the sessions (pgid, tmux, branch). `reap` sends SIGKILL to the process group of each session and kills its tmux session, then marks it as reaped. `reap-orphans` does the same for orphaned sessions.
- **Effect and safety:** `status` is READ-ONLY. `reap` and `reap-orphans` KILLS/STARTS-PROCESSES. `reap-orphans` has `--dry-run`. `reap` has no dry-run and no confirmation.
- **Caution:** `swarm reap` sends SIGKILL to process groups and kills tmux sessions. It has no dry-run.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 registry error, 2 usage.
- **Called by:** the operator. No caller found. Possibly legacy.
- **Policy conformance:** CD-30: not in `--help`. CD-61: `reap` kills and has no `--dry-run`. CD-12: `reap` is not a standard verb (proposed `swarm remove`, proposed, not built). CD-51: usage exits 2.

#### `preflight-environment`

```
evolve preflight-environment [--json | --summary | --write]
```

It reads `EVOLVE_PROJECT_ROOT` and `EVOLVE_PLUGIN_ROOT` from the environment.

- **What it does:** Reports what the host can do (tools, sandbox, worktree base) as a profile.
- **How it works:** `internal/preflight.Probe` measures the sandbox. `--write` saves `.evolve/environment.json`.
- **Effect and safety:** READ-ONLY (the probe can start short subprocesses). `--write` WRITES-LOCAL and is opt-in.
- **Output:** JSON on stdout by default (`--json` is the default). `--summary` prints text.
- **Exit codes:** 0, 10 unexpected argument.
- **Called by:** the operator (getting-started doc). No code caller.
- **Policy conformance:** CD-04: flat hyphenated name; proposed `doctor environment` (proposed, not built). CD-76: reads two control values from the environment. CD-14: overlaps `doctor probe`.

### 5. Quality and gates

#### `acs`

```
evolve acs run --cycle N <pkg> [--evolve-dir D] [--json=true]
evolve acs suite --cycle N [--root R] [--evolve-dir D] [--json=true]
evolve acs tia-report [--evolve-dir D] [--json]
```

- **What it does:** Runs the acceptance-criteria (ACS) predicate tests of a cycle and records green or red.
- **How it works:** `acs run` runs the predicates of one Go package. `acs suite` runs `go test -json -tags acs -count=1` over the whole suite. Both write `acs-verdict.json` in the cycle run directory. `tia-report` rolls up test-impact data from `.evolve/runs`.
- **Effect and safety:** LAUNCHES processes (`go test`) and WRITES-LOCAL. `tia-report` is READ-ONLY. No dry-run. The verb needs an existing `.evolve` directory.
- **Output:** Here `--json` means "write the verdict file" (default true) for `run` and `suite`, and "print JSON" for `tia-report`. `run` always prints the verdict JSON on stdout.
- **Exit codes:** `run`: 0 green, 1 runner or write error, 2 red predicate, 10 usage. `suite`: 0, 1, 2 (red), 10. `tia-report`: 0, 1, 10.
- **Called by:** the build and audit phases, and console review seats.
- **Policy conformance:** CD-41: `--json` has the inverse meaning of the rest of the CLI. CD-12: `tia-report` is not a standard verb (proposed `acs report`, proposed, not built). CD-24: `acs suite` uses `--root` and the group uses `--evolve-dir`. Help text differs: the registry says only "Run ACS predicates", and top help omits `tia-report`.

#### `apicover`

```
evolve apicover [-cover f] [-require-doc] [-enforce] <pkgdir>...
```

- **What it does:** Checks that a test names every exported symbol, so a green test run cannot hide an untested public API.
- **How it works:** `internal/apicover.Run` reads `go tool cover -func` output through `-cover` and walks the package directories by AST. It prints a report.
- **Effect and safety:** READ-ONLY. Without `-enforce` the exit is 0 even with findings.
- **Output:** No `--json`. The report goes to stdout.
- **Exit codes:** 0 (also for `-h`), 2 usage or error, non-zero with `-enforce` when symbols lack coverage.
- **Called by:** `evolve floor` lists it as `make apicover-check`. `make` and the CI workflow use the separate binary `go/cmd/apicover`, not this verb. The verb duplicates that binary.
- **Policy conformance:** CD-30: not in `--help`. CD-20: the registry text and help use single-dash flags. CD-41: no `--json`. CD-03: flat verb outside the flat list; proposed `api cover` or removal as a duplicate (proposed, not built).

#### `cover`

```
evolve cover changed --base <ref> [--profile P] [--packages F] [--json] [--root DIR]
evolve cover strict [--profile P] [--floors P] [--packages F] [--json] [--root DIR]
```

`cover` with flags and no sub-verb runs `changed`.

- **What it does:** The coverage gate. Each function changed since a base ref must be 100% covered, or each package must reach its floor.
- **How it works:** `internal/covchanged` reads the coverage profile (default `go/coverage.txt`, made by `make test-integration`) and the git diff against the base. `strict` compares the floors of each package.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json`. A table on stdout.
- **Exit codes:** 0 clean, 1 refused (below floor), 2 I/O (profile missing), 10 usage.
- **Called by:** CI and `make`. `evolve floor` runs it.
- **Policy conformance:** CD-08: a group that takes a flag with no verb runs `changed` (a catch-all default). CD-24: uses `--root` in place of `--project-root`. CD-12: `changed` and `strict` are adjectives, not verbs (proposed `cover check --changed`, proposed, not built).

#### `eval`

```
evolve eval quality-check [--predicates P] <eval.md>
evolve eval diversity-check <evalsDir>
evolve eval verify <eval.md> <workspace>
```

- **What it does:** Grades an eval file for tautological or weak checks, and runs the verification commands of an eval file.
- **How it works:** `quality-check` parses `eval.md`, rates each command PASS, WARN or HALT, and with `--predicates` adds advisory lints. `diversity-check` checks a directory of evals. `verify` takes the bash blocks of `eval.md` and runs each with `/bin/bash -o pipefail -c` in the workspace.
- **Effect and safety:** `quality-check` and `diversity-check` are READ-ONLY. `verify` KILLS/STARTS-PROCESSES. No dry-run and no prompt.
- **Caution:** `eval verify` runs arbitrary bash that it reads from the eval file. Read the file before you run it.
- **Output:** No `--json`. Human lines on stdout, errors on stderr.
- **Exit codes:** `quality-check` and `diversity-check`: 0 PASS, 1 WARN or error, 2 HALT, 10 usage. `verify`: 0 PASS, 1 FAIL or error, 10 usage.
- **Called by:** the Scout and TDD phases (eval authoring), the audit, and the operator. Agent class: the two checks are allowed.
- **Policy conformance:** CD-13: `verify` is an avoided word (proposed `eval check`, proposed, not built). CD-20: top help prints `-predicates` with one dash. CD-41: no `--json`. Help text differs: the help omits `diversity-check`.

#### `floor`

```
evolve floor run [--tier fmt,vet,...] [--root DIR] [--dry-run]
```

Tiers: `fmt`, `vet`, `integration`, `e2e`, `acs-durable`, `apicover`, `cover`, `ratchet`.

- **What it does:** Runs the local copy of the CI floor, tier by tier, before you ship.
- **How it works:** It runs shell commands for each tier (`go vet`, `make test-integration`, `make test-e2e`, `go test -tags acs`, `make apicover-check`, `make cover-strict`, `evolve ratchet check`). The `fmt` tier uses the Go version pinned in the lint job of the workflow. It prints one PASS, RED or ERROR line for each tier.
- **Effect and safety:** STARTS-PROCESSES (builds and tests; it can touch the build cache and edits no tree files by design). `--dry-run` prints the commands of each tier.
- **Output:** No `--json`. Output on stdout.
- **Exit codes:** 0 all pass, 1 a tier is red, 2 a tier did not run, or a usage error.
- **Called by:** the operator and console, before a ship.
- **Policy conformance:** CD-30: not in `--help`. CD-51: usage exits 2. CD-41: no `--json`.

#### `guard`

```
evolve guard <ship|phase|role|docdelete|chain|triage-floors> [--evolve-dir DIR] [--bypass]      (JSON payload on stdin)
evolve guard triage-floors <workspace>
evolve guard list-audit-fails [--evolve-dir D] [--json]
```

- **What it does:** The trust-kernel gate. It allows or denies a tool call or a ship action, from a hook payload.
- **How it works:** It reads a guard input JSON on stdin. `buildGuard(name)` decides. It appends to `.evolve/guards.log` and `integrity-alarm.jsonl` and prints `{"guard","allow","reason"}`.
- **Effect and safety:** WRITES-LOCAL (log lines only). `--bypass` is an emergency override. A refusal is the deny itself.
- **Output:** A JSON object always on stdout. `[guard:x] DENY:` on stderr.
- **Exit codes:** 0 allow, 2 deny, 10 usage or parse error. `triage-floors` and `list-audit-fails`: 0, 1, 10.
- **Called by:** hooks (`PreToolUse` for ship, phase, role and docdelete in `.claude/settings.json`). Agent class: allowed.
- **Policy conformance:** CD-52: a deny exits 2, but the table gives 3 to a refusal. CD-12: the guard name is the sub-verb. Help text differs: the help omits `--bypass` and `list-audit-fails`.

#### `postedit-validate`

```
evolve postedit-validate [--bypass]      (PostToolUse payload JSON on stdin)
```

- **What it does:** After an edit, checks the syntax of the file (`.json`, `.sh`, `.py`) and warns the agent.
- **How it works:** `internal/posteditvalidate.Run` parses the hook payload and validates by file extension. It writes WARN lines to stderr and to `guards.log`.
- **Effect and safety:** WRITES-LOCAL (a log only). It never blocks. `--bypass` is available.
- **Output:** No `--json`. WARN on stderr.
- **Exit codes:** Always 0.
- **Called by:** the `PostToolUse` hook for Edit and Write in `.claude/settings.json`. Agent class: allowed.
- **Policy conformance:** CD-04: flat hyphenated name. CD-12: `validate` is not a standard verb; proposed `guard postedit` (proposed, not built). CD-27: reads stdin without `-`.

#### `quality`

```
evolve quality report|show --scope wave|tree|judges|replay [--json] --project-root P
evolve quality compare --index PQ1|PQ4 --a WINDOW --b WINDOW [--paired] [--json] --project-root P
```

A WINDOW is `wave:N` or `waves:A-B`. `--project-root` is required. A bare `quality` with no sub-verb runs `report`.

- **What it does:** Computes the quality-index readings (ADR-0153) for a wave, a tree, the judges or a replay, and publishes them as signals. An operator then sees breaches and warnings.
- **How it works:** The readers use `internal/qualitypanel`, `pipelinerates`, `codeshape` and `policy.json`. The results go to the Signal Center, which writes events in `.evolve`. `show` replays readings. `compare` does a statistic on two windows (difference, MDE, McNemar with `--paired`).
- **Effect and safety:** `report` and `compare` WRITES-LOCAL (signal events). `show` is READ-ONLY. No dry-run and no prompt.
- **Output:** `--json` (two hyphens). Human text on stdout, usage and errors on stderr.
- **Exit codes:** 0, 10 usage. A breach does not change the code.
- **Called by:** the wave boundary (`quality report --scope wave`) and the operator. Agent class: `show` allowed, `report` and `compare` by request.
- **Policy conformance:** CD-30: not in `--help`. CD-08: a bare verb runs `report`. CD-12: `report` and `compare` are not standard verbs (proposed `quality list`, `quality show`, proposed, not built).

#### `ratchet`

```
evolve ratchet check [size|rawgit] [--root DIR]
evolve ratchet tighten [--check] [--root DIR] [--project-root P]
```

- **What it does:** Stops function size and raw-git test fixtures from growing. It checks a list of offenders and tightens the list when the code improves.
- **How it works:** `internal/sizeratchet` and `internal/rawgitratchet` scan the module under `--root`. `tighten` rewrites `offenders.json`, the only writer. `--check` reports slack and exits 1 without a write.
- **Effect and safety:** `check` is READ-ONLY. `tighten` WRITES-LOCAL. It refuses to write inside a phase or under a live lease. `--check` is the dry mode.
- **Output:** No `--json`. Summary on stdout, errors on stderr.
- **Exit codes:** 0 clean or tight, 1 offenders, slack or write refused, 2 usage.
- **Called by:** the floor tier and CI (`ratchet check --root .`). The operator calls `tighten`.
- **Policy conformance:** CD-12: `tighten` is not a standard verb. CD-51: usage exits 2. CD-41: no `--json`. CD-20: the Go flag package accepts single-dash forms too. Gap: the duration ratchet runs only as the separate binary `go/cmd/testlatency`.

#### `reachability`

```
evolve reachability check-pin --root DIR --pinning-package P --referenced-package P --symbol S --pkgs a,b
```

All five flags are required.

- **What it does:** Tells a test author if a structural-test pin creates an import cycle.
- **How it works:** `internal/reachabilityprobe` builds the import graph of `--pkgs` under `--root` and tests if the pinning package can reference the symbol.
- **Effect and safety:** READ-ONLY.
- **Output:** No `--json`. A message on stdout, violations on stderr.
- **Exit codes:** 0 no violation, 1 violation, 10 usage or error.
- **Called by:** builder agents, by hand. No script, CI or skill caller found. Possible dead verb.
- **Policy conformance:** CD-30: not in `--help`. CD-11: a hyphenated sub-verb (proposed `reachability check`, proposed, not built). CD-24: uses `--root`. CD-41: no `--json`.

#### `scan`

```
evolve scan secrets [--staged | --diff <ref>] [--json] [--project-root P]
```

- **What it does:** Finds secrets in a staged diff or a ref range before a commit. It masks each match.
- **How it works:** It runs `git diff` (`--cached` or `<ref>`) through `gitexec` and `secretleakscan.ScanDiff`.
- **Effect and safety:** READ-ONLY. `--staged` and `--diff` exclude each other. Agent class: allowed.
- **Output:** `--json`. Text on stdout.
- **Exit codes:** 0 clean, 1 secret found, 2 git error, 10 usage.
- **Called by:** the operator and the commit gate. No script or CI caller found.
- **Policy conformance:** Conforms in shape (noun group). CD-12: the one sub-verb `secrets` is a noun (proposed `scan check`, proposed, not built).

#### `selfcheck`

```
evolve selfcheck build [--worktree DIR]
```

- **What it does:** Lets the builder test its own handoff (deliverable contract and build floor) and repeat until GREEN.
- **How it works:** It runs `phasecmd.BuildSelfCheck` against the `.evolve` of the worktree. This is the same probe as `phase verify build`. It lists the findings with codes.
- **Effect and safety:** READ-ONLY. Agent class: allowed.
- **Output:** Findings on stdout. No `--json`.
- **Exit codes:** 0 GREEN, 1 findings, 2 usage or the handoff cannot be judged.
- **Called by:** the builder agent persona. Docs say to run it.
- **Policy conformance:** CD-30: not in `--help`. CD-14: duplicates `phase verify build` (proposed `phase check build`, proposed, not built). CD-51: usage exits 2. CD-41: no `--json`.

#### `stage`

```
evolve stage verify <key> [--chain PATH] [--project-root P] [--json]
evolve stage report [--chain PATH] [--project-root P] [--json]
```

- **What it does:** Checks the evidence chain of a shadow stage, to decide if the stage can move from shadow to enforce.
- **How it works:** It reads `policy.json` and finds the stage in `gateregistry` by its policy key. It computes the decision version and verifies the chain with a scorer. The chain comes from `--chain`, or is empty. `report` lists the 48 stage keys.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json`.
- **Exit codes:** 0, 1 verification failed, 2 I/O (policy unreadable or version error), 10 usage.
- **Called by:** the operator and the loop. No verb flips a stage today. A flip is a hand edit of `policy.json`.
- **Policy conformance:** CD-30: not in `--help`. CD-13: `verify` is an avoided word (proposed `stage check`, proposed, not built). CD-12: `report` is not a standard verb. Gap: there is no `stage flip`.

#### `test`

```
evolve test audit [--rebaseline <package>] [--check] [--root DIR] [--json]
```

- **What it does:** Audits the quality of tests against baseline shards for each package, so weak tests cannot creep in.
- **How it works:** `internal/testaudit` loads the config and scans the test files under `--root`. `--rebaseline <pkg>` rewrites the shard of that package. `--check` verifies all shards.
- **Effect and safety:** The default list and `--check` are READ-ONLY. `--rebaseline` WRITES-LOCAL (a baseline shard). `--rebaseline` and `--check` exclude each other. No dry-run.
- **Output:** `--json`.
- **Exit codes:** 0, 1 rebaseline failure, 2 I/O, 3 check failed, 10 usage.
- **Called by:** the operator and the CI floor. No `.github` hit found.
- **Policy conformance:** CD-30: not in `--help`. CD-05: sibling `test-scoped` is a flat name beside this group. CD-52: `--check` failure exits 3, where the table gives 1 to a failed check. CD-12: `--rebaseline` is a flag that acts as a sub-verb.

#### `test-scoped`

```
evolve test-scoped [--base REF=main] [--root DIR] [--json]
```

- **What it does:** Runs only the tests of the packages changed since the merge base, and of their importers, as a fast shadow check.
- **How it works:** `changedpkgs.RunShadowTests` diffs against the merge base, finds the importers and runs `go test` on them.
- **Effect and safety:** STARTS-PROCESSES (`go test`). It does not change the tree.
- **Output:** `--json`.
- **Exit codes:** 0 pass, 1 tests failed, 2 I/O, 10 usage.
- **Called by:** nobody automated; plans only. Possible dead or shadow-only verb.
- **Policy conformance:** CD-30: not in `--help`. CD-04 and CD-05: flat hyphenated name beside `test`; proposed `test scoped` (proposed, not built). CD-33: `--help` exits 10. CD-52: failed tests exit 1 with the older constant set.

#### `treewalk`

```
evolve treewalk snapshot --root DIR --out FILE
evolve treewalk check --root DIR --baseline FILE [--allow PREFIX]...
```

- **What it does:** Detects files and empty directories that a test run leaked into a tree.
- **How it works:** `snapshot` records every path under `--root` in a file. `check` walks the tree again and lists each new path that is not allowed.
- **Effect and safety:** `snapshot` WRITES-LOCAL (the `--out` file). `check` is READ-ONLY.
- **Output:** No `--json`.
- **Exit codes:** 0 clean, 1 new paths or I/O error, 10 usage.
- **Called by:** `go/Makefile` (`TREEWALK_SNAPSHOT`, `TREEWALK_CHECK`).
- **Policy conformance:** CD-12: `snapshot` is not a standard verb. CD-24: uses `--root`. CD-41: no `--json`. CD-52: I/O error and a failed check both exit 1.

#### `comments`

```
evolve comments rank [-n N] [dir]
evolve comments verify|check|comments -base <ref> [dir...]
evolve comments history -base <ref> [-label L] [-out DIR] [dir...]
evolve comments strip [dir...]
```

- **What it does:** Audits and strips narrative comments in code. These are the proof tools of the comment campaign.
- **How it works:** A Go AST scan (`internal/commentaudit`). `rank` counts narrative comments by package. `check` and `comments` list the comments added since `-base`. `verify` proves that a change is comment-only. `history` writes a snapshot. `strip` rewrites Go files without comments.
- **Effect and safety:** `rank`, `check`, `comments` and `verify` are READ-ONLY. `history` WRITES-LOCAL (to `-out`).
- **Caution:** `strip` rewrites source files in place. It has no dry-run and relies on git to undo.
- **Output:** Tab-separated text on stdout. No `--json`.
- **Exit codes:** 0, 1 findings or failure, 2 usage.
- **Called by:** the operator and console. A separate binary `go/cmd/commentaudit` duplicates the surface.
- **Policy conformance:** CD-06: plural group name. CD-20: single-dash flags `-n`, `-base`, `-label`, `-out`. CD-14: `check` and `verify` are near synonyms. CD-61: `strip` has no `--dry-run`. CD-51: usage exits 2. Help text differs: the usage line prints `commentaudit`, not `evolve comments`.

#### `names`

```
evolve names [check] [--project-root P]
evolve names fix [--project-root P]
```

- **What it does:** After a rename, finds dead old names in tracked files and optionally fixes them.
- **How it works:** `internal/naminguard` loads the manifest `.evolve/naming.json` and scans the tracked files. `fix` rewrites the tokens.
- **Effect and safety:** `check` is READ-ONLY and is the dry form. A bare `evolve names` runs `check`. `fix` WRITES-LOCAL (tree files) with no confirmation.
- **Output:** No `--json`. Output on stdout.
- **Exit codes:** 0 clean, 1 dead tokens, or a manifest or root error, 10 usage or unknown sub-verb.
- **Called by:** the operator. `release-preflight` runs the scan and tells you to run `names fix`.
- **Policy conformance:** CD-08: with no sub-verb or a leading flag it runs `check` (default sub-verb). CD-06: plural group name. CD-61: `fix` has no `--dry-run`. CD-41: no `--json`.

#### `solution`

```
evolve solution check <slug | solutions/slug> [--project-root DIR] [--registry PATH]
```

- **What it does:** Checks a document deliverable against the contract of the phase registry (ADR-0099).
- **How it works:** It runs the same engine as the build floor and the audit gate on `solutions/<slug>` with `docs/architecture/phase-registry.json`. It prints each finding.
- **Effect and safety:** READ-ONLY. Agent class: allowed.
- **Output:** No `--json`.
- **Exit codes:** 0 pass, 1 findings, 2 usage or registry error.
- **Called by:** the loop in document cycles (`solution-scout`, `solution-build`).
- **Policy conformance:** CD-30: not in `--help`. CD-51: usage exits 2. CD-41: no `--json`.

### 6. Docs and skills

#### `docs`

```
evolve docs ste-lint [--json] [--strict] [--go] [--changed <base-ref> [--added]] [--project-root P] [paths...]
evolve docs index <generate|check> [--project-root P]
```

- **What it does:** Checks documents (or Go comments) against the ASD-STE100 writing rules. Keeps `docs/research-index.md` in step with its entries.
- **How it works:** `internal/stelint` scans `.md` files (or Go files with `--go`) and reports findings as WARN. `docs index generate` rewrites the dated entries of `docs/research-index.md` from `docs/research-index/`. `check` fails when the index is stale.
- **Effect and safety:** `ste-lint` and `index check` are READ-ONLY. `index generate` WRITES-LOCAL. `ste-lint` exits 0 with findings unless `--strict`.
- **Output:** `--json` on `ste-lint`.
- **Exit codes:** `ste-lint`: 0, 1 with `--strict` and findings, 2 I/O, 10 usage. `index`: 0, 1 stale, 2 I/O, 10 usage.
- **Called by:** CI (`docs index check`, `docs ste-lint --strict --changed ... --added`), the operator and the docs-write skill.
- **Policy conformance:** CD-51: `evolve docs` with no sub-verb exits 0, not 10. CD-12: `ste-lint` is a tool name (proposed `docs lint`, proposed, not built).

#### `skills`

```
evolve skills generate|check
evolve skills publish [--target a,b] [--dry-run] [--install] [--check] [--no-prune] [--ollama-base URL] [--codex-home DIR]
```

- **What it does:** Keeps the generated skill docs in step with their source. Publishes skills to other LLM CLIs.
- **How it works:** `generate` and `check` use `skillcheck.Run`: `generate` writes the projected skill docs and `check` reports drift. `publish` renders skills for the codex, agy and ollama targets and writes them. With `--install` it runs `agy plugin` commands.
- **Effect and safety:** `generate` and `publish` WRITES-LOCAL (publish also writes outside the repository, such as the codex home, and can run CLI install commands). `check` is READ-ONLY. `check`, `publish --check` and `publish --dry-run` preview. Stale entries are pruned unless `--no-prune`.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 error, 2 drift (`check`), 10 usage.
- **Called by:** the operator and the `evo:setup` skill. No CI caller found.
- **Policy conformance:** CD-52: drift exits 2, but the table gives 1. CD-41: no `--json`.

#### `skill-inventory`

```
evolve skill-inventory build [--project-root DIR] [--ttl DURATION] [--force]
```

- **What it does:** Builds a cache of the skills in the project, so that phases can load them fast.
- **How it works:** `internal/skillinventory` scans `<project-root>/skills` and writes a cache file. It skips the build when the cache is fresh, unless `--force`.
- **Effect and safety:** WRITES-LOCAL (a cache file in `.evolve`). No dry-run.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 build error, 10 usage.
- **Called by:** skill docs of the loop only. No script caller found. Possibly legacy.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `skills`; proposed `skills inventory generate` (proposed, not built). CD-12: `build` is not a standard verb. Help text differs: the registry summary omits the `build` sub-verb.

#### `flags`

```
evolve flags generate
evolve flags check
```

- **What it does:** Keeps the generated index of `EVOLVE_*` flags in `docs/architecture/control-flags.md` in step with the flag registry.
- **How it works:** `internal/flagregistry.RenderIndex` builds the index. `skillcheck.SpliceMarkedRegion` puts it in the marked region of the document.
- **Effect and safety:** `generate` WRITES-LOCAL (a tracked doc). `check` is READ-ONLY and is the dry form.
- **Output:** No `--json`. Text on stdout, a drift message on stderr.
- **Exit codes:** 0, 1 read, splice or write error, 2 drift (from `check`), 10 usage.
- **Called by:** `internal/derived`, which runs both sub-verbs. `pr refresh` uses that entry.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-52: drift exits 2, but the table gives 1 to a failed check.

#### `guidance`

```
evolve guidance build [--project-root P]
evolve guidance check [--project-root P]
evolve guidance index [--audience all|console|phase|reviewer|brief] [--phase NAME] [--cli NAME] [--json] [--project-root P]
```

- **What it does:** Builds, checks and queries the rule registry in `guidance/rules`, which feeds the instructions of agents.
- **How it works:** `build` writes `guidance/index.md`, `guidance/index.json` and the L0 region of `AGENTS.md`. `check` reports stale output, duplicates, budgets, conflicts and missing enforcers. `index` picks rules by audience, phase and CLI family.
- **Effect and safety:** `build` WRITES-LOCAL (tracked files). `check` and `index` are READ-ONLY. `check` is the dry form. Agent class: allowed.
- **Output:** `--json` on `index` only (schema `evolve.guidance.index/1`).
- **Exit codes:** 0, 1 check problems, 2 I/O, 10 usage, 11 bad registry.
- **Called by:** CI (`guidance check --project-root .`) and agents (`index`).
- **Policy conformance:** CD-30: not in `--help`. CD-12: `build` is not a standard verb (proposed `guidance generate`, proposed, not built). CD-05: close name to `guard`.

### 7. Code search

#### `code`

```
evolve code callers <symbol> [--direction in|out] [--project-root P] [--json]
evolve code api <file> [--project-root P] [--json]
evolve code find <query> [--project-root P] [--json]
evolve code map [dir] [--project-root P] [--json]
evolve code refresh [--project-root P] [--json]
evolve code bench [--backends a,b] [--symbols N] [--rounds N] [--project-root P]
evolve code usage [--since D] [--json]
```

- **What it does:** Gives agents an indexed, read-only way to ask "who calls X", "what is the API of file F" and "find Y", in place of grep.
- **How it works:** It builds a code-index handle and checks that the index is fresh against `origin/main` (exit 21 when stale). It prints one hit per line as `file:line symbol text`. `refresh` runs `git fetch origin main` in the index worktree and rebuilds the index. Each query adds a line to a usage log in the user cache directory.
- **Effect and safety:** Queries are READ-ONLY, plus a small usage log write. `refresh` WRITES-LOCAL and fetches from the network. `bench` STARTS-PROCESSES. `refresh` refuses a dirty index worktree (exit 22).
- **Output:** `--json` prints one `evolve.code.<verb>/1` envelope with the hits and the freshness of the index.
- **Exit codes:** 0, 1 not found, 2 index absent, 3 unsupported or refused, 4 failed, 10 usage, 21 stale, 22 index dirty, 23 fetch failed.
- **Called by:** agents and the pipeline. The boundary runs `code refresh`. Agent class: queries allowed, `refresh` by request.
- **Policy conformance:** Conforms in shape. CD-12: `callers`, `api` and `map` are domain words, not standard verbs.

#### `context`

```
evolve context index [--layer L1|L2] [--required] [--json] [--workspace W]
evolve context open <id> [--json] [--workspace W]
evolve context stats [--phase P] [--json] [--project-root R]
```

- **What it does:** Lets a phase agent list and open the context files that were prepared for it, checked by hash.
- **How it works:** It reads the `inputs/index.json` of the cell. `open` checks the sha256 of the body before it prints it. `stats` reads the access log for open rates.
- **Effect and safety:** READ-ONLY. It refuses a changed body (exit 1).
- **Output:** `--json` (schema constants in `contextcmd.go`).
- **Exit codes:** 0, 1 changed body, 2 no index or I/O error, 10 usage, 11 unknown id or bad index.
- **Called by:** agents and the pipeline. Agent class: `index` and `open` allowed, `stats` denied.
- **Policy conformance:** CD-30: not in `--help` (only `context-fill` is listed). CD-05: split from `context-fill`; proposed `context fill correlate` (proposed, not built). CD-24: uses `--workspace` where `--project-root` is the standard.

### 8. Routing, models and CLIs

#### `agent`

```
evolve agent run <profile> [--input FILE|-] [--file PATH]... [--worktree DIR [--writable PATH]... [--writable-dir DIR]...] [--json] [--keep] [--seat-token-file FILE] [--project-root P]
evolve agent list [--json] [--project-root P]
evolve agent capabilities [--json] [--project-root P]
```

- **What it does:** Runs one phase agent (a "seat") with no cycle, for ad-hoc work or review. Also lists the profiles and the agent class of each verb.
- **How it works:** It resolves a profile, builds the input from stdin, `--input` or `--file`, and launches the seat through the bridge. It then runs a verdict check and a fence check on the worktree and prints the report. It writes `.evolve/agent-runs/<run label>.json`.
- **Effect and safety:** `run` LAUNCHES-LLM and WRITES-LOCAL. A writer profile can edit files in the named `--worktree`. `list` and `capabilities` are READ-ONLY. No dry-run. It refuses an unknown profile, a read-only profile with `--writable`, a writer profile without `--worktree`, terminal stdin and a usage hold.
- **Output:** `--json` prints one envelope (report, verdict, driver, family, model, cost, tokens, launch record).
- **Exit codes:** 0 PASS, WARN or SKIPPED, 1 FAIL, 3 refused, 10 usage, 11 input, 12 fence revert, 21 dispatch error.
- **Called by:** console and operator review seats. Loop code also calls it.
- **Policy conformance:** CD-30: not in `--help`. Help text differs: the registry summary omits `--writable-dir` and `--seat-token-file`.

#### `bridge`

```
evolve bridge launch --cli=NAME --profile=PATH --model=M --prompt-file=PATH --workspace=DIR --stdout-log=PATH --stderr-log=PATH --artifact=PATH [--cycle=N] [--agent=NAME] [--worktree=DIR] [--permission-mode=M] [--allow-bypass] [--stream-output] [--session-name=NAME] [--artifact-timeout-s=N] [-- inner flags]
evolve bridge probe [--cli=NAME]
evolve bridge send --workspace --agent [--kind] [--source] <body>
evolve bridge watch --workspace --agent [--follow]
evolve bridge sessions [--project-root] [--json]
evolve bridge recipe run|list|show
evolve bridge capabilities --cli [--json]
evolve bridge introspect --cli [--pane-file|--workspace] [--allow-bypass]
evolve bridge control <family> <event> --workspace
evolve bridge add-rule
evolve bridge promote-rule --regex --response-keys --corpus [--note --dir --project-root]
evolve bridge version
```

The source also handles `report --workspace [--artifact-name]`, `validate`, `doctor [--cli] [--deep] [--json]` and `billing snapshot|compare`. The usage text omits them (help text differs).

- **What it does:** Launches an agent CLI (claude, codex, agy, gemini) in a tmux seat. Probes which CLIs exist. Sends live commands to a running agent.
- **How it works:** The `internal/bridge` engine runs the CLI driver in tmux with the profile and the prompt. It waits for the artifact and answers known prompts by rule. `probe` and `capabilities` read manifests. It reads the sandbox and bypass opt-ins from `policy.json`. It records bypasses in the signal center.
- **Effect and safety:** `launch` LAUNCHES-LLM and starts tmux processes. `send`, `add-rule` and `promote-rule` WRITES-LOCAL. `probe`, `watch`, `sessions` and `capabilities` are READ-ONLY. `--allow-bypass` is explicit and audited. No dry-run.
- **Output:** `probe` prints JSON. `--json` on `sessions`, `capabilities` and `doctor`. Results go to stdout and logs to stderr.
- **Exit codes:** 0 ok, 2 safety gate, 3 cost leak, 10 bad flags. Also 80 REPL boot timeout, 81 artifact timeout, 85 unknown prompt, 86 respond-loop guard, 99 require-full unmet and 127 missing binary.
- **Called by:** every phase launch. `agent run` and `cycle run` use the same package. The operator calls `probe` and `sessions`.
- **Policy conformance:** CD-11: `--k=v` flag style and hyphenated sub-verbs `add-rule` and `promote-rule` (proposed `bridge rule add` and `bridge rule promote`, proposed, not built). CD-34: usage text misses four handled sub-verbs.

#### `auth`

```
evolve auth store-times [--json]
evolve auth seat-token set --project-root R      (token on stdin only)
evolve auth seat-token clear --project-root R
```

- **What it does:** Shows when the Claude login credentials last changed. Sets or clears the isolated seat token of claude seats.
- **How it works:** `store-times` reads the file time of the credentials file and runs the macOS `security` tool for the attributes of the Keychain item. It reads no secret. `seat-token set` reads a token from stdin and writes a holder file with mode 0600. `clear` removes it.
- **Effect and safety:** `store-times` is READ-ONLY. `seat-token` WRITES-LOCAL (a credential holder file). It refuses while a loop lane is live (exit 3). It needs `--project-root` with `.evolve`. It rejects a value that is not a seat token (exit 11). It never prints the token.
- **Output:** `--json` on `store-times` (schema `evolve.auth.store-times/1`).
- **Exit codes:** 0, 2 home unreadable or write failed, 3 refused (lane live), 10 usage, 11 not a seat token.
- **Called by:** the operator. Agent class: `store-times` allowed, `seat-token` denied.
- **Policy conformance:** CD-30: not in `--help`. CD-12: `store-times` is not a standard verb (proposed `auth show`, proposed, not built). CD-61: `seat-token clear` has no `--dry-run`.

#### `cli`

```
evolve cli update [--dry-run] [--json] [--project-root P]
```

- **What it does:** Updates each subscribed LLM CLI (claude, codex, agy) at a wave boundary and smoke-tests each version that changed.
- **How it works:** For each family it runs the updater command of the manifest, then a smoke test. It attaches usage evidence.
- **Effect and safety:** KILLS/STARTS-PROCESSES and WRITES-LOCAL. `--dry-run` is available. It refuses (exit 3) while any run holds a live lease, unless `--dry-run`. Agent class: denied.
- **Caution:** The name "update" does not show that the verb runs the vendor updater and installs software from the network.
- **Output:** `--json` (core verb envelope).
- **Exit codes:** 0, 1 update or smoke failed, 3 refused (live lease), 10 usage, 130 interrupted.
- **Called by:** the loop at boundaries, and the operator.
- **Policy conformance:** CD-05: `cli`, `cli-routing`, `clihealth` and `cli-health` split one noun into four spellings. Proposed: keep `cli update` and move the others under `cli` (proposed, not built).

#### `cli-routing`

```
evolve cli-routing show [--static] [--json] [--project-root DIR]
evolve cli-routing check [--project-root DIR]
evolve cli-routing explain <agent> [--phase P] [--static] [--project-root DIR]
evolve cli-routing init --clis a,b
evolve cli-routing set <key> [<a,b>] [--model T] [--effort L] [--share-pct N]
evolve cli-routing unset <key>[.effort|.share_pct]
evolve cli-routing migrate [--dry-run]
```

- **What it does:** Shows, checks and edits the table that decides which CLI and model each agent role uses.
- **How it works:** It reads the `cli_routing` table of `policy.json`. `--static` skips the host probe and the bench. `check` reports findings. `set`, `unset`, `init` and `migrate` edit `policy.json` through `cmd/cli_routing_edit.go`.
- **Effect and safety:** `show`, `check` and `explain` are READ-ONLY. `init`, `set`, `unset` and `migrate` WRITES-LOCAL (`.evolve/policy.json`). `migrate` has `--dry-run`. The edit verbs are in the queued class and are denied to agents. Gate-config edits need the operator.
- **Caution:** The registry label says "(read-only)", but `set`, `unset`, `init` and `migrate` write `policy.json`. Help text differs.
- **Output:** `--json` on `show`, `check` and `explain`.
- **Exit codes:** 0, 1 finding or refusal, 10 usage.
- **Called by:** the operator, console and agents (`show`, `check` and `explain` are class A).
- **Policy conformance:** CD-30: not in `--help`. CD-04 and CD-05: hyphenated name beside the `cli` group; proposed `cli routing show` (proposed, not built). CD-61: the edit verbs have no `--dry-run` except `migrate`.

#### `clihealth`

```
evolve clihealth list [--json]
evolve clihealth clear <family>
evolve clihealth usage [--json] [family...]
```

All forms take `--project-root DIR` (default `.`). The alias is `cli-health`.

- **What it does:** Shows or clears the "bench" that marks a CLI as quota-limited or logged out. Reads the usage meter of each CLI.
- **How it works:** `list` and `clear` read and write `.evolve/cli-health.json`. `usage` opens a tmux pane for each family, sends its usage command and parses the result.
- **Effect and safety:** `list` is READ-ONLY. `clear` WRITES-LOCAL. `usage` STARTS-PROCESSES (a real CLI pane per family; no model call). Agents can call `list` but not `usage`.
- **Output:** `--json` on `list` and `usage`.
- **Exit codes:** 0, 1 a family was unreadable or `clear` failed, 10 usage.
- **Called by:** the operator and triage.
- **Policy conformance:** CD-05: four spellings of the `cli` noun; proposed `cli health list` (proposed, not built). CD-12: `usage` as a sub-verb is not standard (proposed `cli health show`). CD-33: `--help` is not handled.

#### `models`

```
evolve models refresh [--source live|detect] [--json] [--evolve-dir D] [--project-root P]
evolve models list [--json] [--evolve-dir D] [--project-root P]
evolve models performance [--json] [--evolve-dir D] [--project-root P]
```

All sub-verbs share one flag set. `--source` has no effect on `list` and `performance`.

- **What it does:** Refreshes and shows the catalog of live models for each CLI. Reports the performance of attempts for each model.
- **How it works:** `refresh --source live` opens each ready CLI through the bridge, captures its `/model` list and runs an LLM classifier chain. It then commits the `.evolve` catalog. `--source detect` uses the manifest map only. `performance` adds up the tries from the `llm-calls` ledgers.
- **Effect and safety:** `refresh` live LAUNCHES-LLM, STARTS-PROCESSES and WRITES-LOCAL. `refresh --source detect` WRITES-LOCAL. `list` and `performance` are READ-ONLY. When no CLI classifies live, `refresh` leaves the catalog as it was and exits 1. No dry-run.
- **Caution:** `models refresh` launches live seats for each ready CLI and runs an LLM classifier. The name "refresh" suggests a cache reload.
- **Output:** `--json` on all. A table on stdout.
- **Exit codes:** 0, 1 refresh failed or no live CLI, 10 usage or bad `--source`.
- **Called by:** the operator. `setup` points to `models refresh`.
- **Policy conformance:** CD-30: not in `--help`. CD-06: plural group name. CD-12: `refresh` and `performance` are not standard verbs (proposed `models generate`, proposed, not built). CD-61: `refresh` has no `--dry-run`.

#### `resolve-llm`

```
evolve resolve-llm <role>
```

- **What it does:** Says which CLI and model tier a phase role will use.
- **How it works:** It loads the root router and calls `router.ResolveRole(role)`. It reads the profiles and the CLI routing from the policy and profile files.
- **Effect and safety:** READ-ONLY. It refuses when no router exists.
- **Output:** JSON always on stdout (`{"cli","model_tier","source"}`). Errors on stderr, tagged `[resolve-llm]`.
- **Exit codes:** 0, 1 not found, unknown flag or error, and the routing-refused code when the role is not routed.
- **Called by:** no script, CI or skill caller found. Docs only. Possible dead verb. `cli-routing explain` covers the same need.
- **Policy conformance:** CD-04: flat hyphenated name. CD-12: `resolve` is not a standard verb (proposed `cli routing explain`, already built). CD-41: JSON is always on, with no `--json` flag.

#### `detect-cli`

```
evolve detect-cli [--json] [--platform CLI]
```

- **What it does:** Says which AI CLI (claude, gemini, codex, antigravity or unknown) drives this session.
- **How it works:** `internal/detectcli` reads environment variables and process hints. It prints the CLI name or the full JSON envelope.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json`. The name on stdout otherwise.
- **Exit codes:** 0, 10 (missing `--platform` value or unknown flag).
- **Called by:** skills (the loop runtime notes) at session start.
- **Policy conformance:** CD-04: flat hyphenated name. CD-12: `detect` is not a standard verb; proposed `cli detect` (proposed, not built). CD-03: flat verb outside the flat list.

#### `detect-nested-claude`

```
evolve detect-nested-claude [--quiet|-q]
```

- **What it does:** Says if this process runs inside another `claude -p` session.
- **How it works:** `internal/detectnested.Detect` checks environment and process markers and prints the result.
- **Effect and safety:** READ-ONLY.
- **Output:** The result on stdout unless `--quiet`. No `--json`.
- **Exit codes:** Always 0, even for an unknown argument and when a nested session is found. The result is in the stdout text only.
- **Called by:** no script, skill or Go caller found. Possible dead verb.
- **Policy conformance:** CD-04: flat hyphenated name. CD-22: short flag `-q` (allowed only for `-h` and `-v`). CD-50: always exits 0, so a script cannot test the result. CD-16: an unknown argument does not fail. CD-03: flat verb outside the flat list.

### 9. Release

#### `release`

```
evolve release <target-version> [--dry-run] [--no-rollback] [--skip-tests] [--strict-pass] [--require-preflight] [--max-poll-wait-s N] [--from-tag TAG] [--skip-content-review REASON]
```

The alias is `release-pipeline`.

- **What it does:** Publishes a release in one command: preflight, changelog, version bump, ship, tag, marketplace poll and automatic rollback on failure.
- **How it works:** `internal/releasepipeline.Run` runs the pre-publish gates and bumps the versions. It commits and pushes through ship. It creates the tag and the GitHub release and polls the marketplace. It rolls back after a failure that follows the push. The repo root is `EVOLVE_PROJECT_ROOT` or the current directory.
- **Effect and safety:** WRITES-GIT and WRITES-REMOTE (push, tag, `gh release`, marketplace poll). `--dry-run` simulates. The pre-publish gates refuse. `--skip-content-review` needs a reason, which the journal records. Agent class: denied.
- **Caution:** `release` pushes a tag and creates a public GitHub release. Run `--dry-run` first.
- **Output:** No `--json`. Logs on stderr.
- **Exit codes:** 0, 1 pre-publish failed, 2 ship failed, 3 post-publish failed, 10 usage.
- **Called by:** the operator through the `evo:publish` skill. `evo:release` gates it.
- **Policy conformance:** CD-05: `release-consistency`, `release-preflight`, `release-promote`, `release-verify-binaries`, `release-verify-clis` and `version-bump` are flat names beside this verb. CD-02: `release` is a flat verb with no sub-verbs; it needs to become a group (proposed `release publish`, proposed, not built). CD-76: reads the root from the environment.

#### `release-consistency`

```
evolve release-consistency [target-version]
```

- **What it does:** Checks that `plugin.json`, `marketplace.json`, `SKILL.md`, `README.md` and `CHANGELOG.md` show the same version.
- **How it works:** `releaseconsistency.Run` reads those files under the project root (`EVOLVE_PROJECT_ROOT` or the current directory). It compares them with the target, or with the version of `plugin.json`.
- **Effect and safety:** READ-ONLY. With no arguments it runs (there is no usage-only mode). Agent class: allowed.
- **Output:** No `--json`. Mismatches on stderr.
- **Exit codes:** 0 consistent, 1 inconsistent, unknown flag, extra argument or error.
- **Called by:** the `evo:release` and `evo:publish` skills, and the release pipeline.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `release`; proposed `release consistency` (proposed, not built). CD-12: `consistency` is not a verb (proposed `release check`). CD-51: usage exits 1. CD-76: reads the root from the environment.

#### `release-preflight`

```
evolve release-preflight <target-version> [--dry-run] [--skip-tests] [--strict-pass] [--allow-red-ci] [--require-ci]
```

- **What it does:** The pre-publish gate. It needs a clean tree, an attached branch and a semver bump. It also needs a recent audit PASS, green gate tests, no WIP commits, green remote CI and a content-review file.
- **How it works:** `releasepreflight.Run` checks the git state, runs the gate tests (unless `--skip-tests`), asks GitHub for the CI run of HEAD and reads `docs/operations/release-content-review.json`.
- **Effect and safety:** READ-ONLY on the tree. It starts test processes and reads the network through `gh`. `--dry-run` is available. `--allow-red-ci` is an explicit, logged override. Agent class: allowed.
- **Output:** No `--json`. Gate lines on stderr.
- **Exit codes:** 0 pass, 1 check failed or error, 10 usage.
- **Called by:** the `evo:publish` and `evo:release` skills (`--require-ci`) and the release pipeline.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `release`; proposed `release preflight` (proposed, not built). CD-12: `preflight` is not a standard verb (proposed `release check`). Help text differs: the error usage line omits `--allow-red-ci` and `--require-ci`.

#### `release-promote`

```
evolve release-promote <tag> [--rerun]
```

- **What it does:** Promotes a release that was demoted to prerelease, once its workflow has succeeded and all its binaries exist.
- **How it works:** It reads the release matrix (`.goreleaser.yml`) and uses `gh run list` and `gh run view` for the release workflow of the tag. With `--rerun` it runs `gh run rerun --failed` and `gh run watch`. It then checks the release assets and sets `prerelease=false` with a `gh api` PATCH.
- **Effect and safety:** WRITES-REMOTE (`gh api` PATCH and workflow rerun). It refuses unless the workflow concluded with success and every asset is present. No dry-run.
- **Caution:** `release-promote` changes a public release on GitHub. It cannot be undone by this tool.
- **Output:** No `--json`. Text.
- **Exit codes:** 0 promoted, 1 refused, 2 usage or `gh` failure.
- **Called by:** the operator (publishing guide). Not in CI or skills.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `release`; proposed `release promote` (proposed, not built). CD-61: it changes shared state and has no `--dry-run`. CD-51: usage exits 2.

#### `release-verify-binaries`

```
evolve release-verify-binaries <tag>
```

- **What it does:** Confirms that each prebuilt binary and checksum exists on a GitHub release tag.
- **How it works:** It reads the matrix of `.goreleaser.yml`, lists the release assets through the GitHub API and prints an ASSET and STATUS table. `GITHUB_TOKEN` is optional.
- **Effect and safety:** READ-ONLY (network read).
- **Output:** A tab table on stdout. No `--json`.
- **Exit codes:** 0 all present, 1 missing, usage or config error.
- **Called by:** CI (`.github/workflows/release.yml`).
- **Policy conformance:** CD-30: not in `--help`. CD-04 and CD-05: flat hyphenated name beside `release`. CD-13: `verify` is an avoided word (proposed `release check-binaries`, proposed, not built). CD-51: usage exits 1. CD-76: reads `GITHUB_TOKEN` from the environment.

#### `release-verify-clis`

```
evolve release-verify-clis
```

The verb ignores all arguments without a message.

- **What it does:** Proves that the released binary installs and works for each LLM CLI target (claude, codex, agy, gemini) and answers each core sub-command.
- **How it works:** It builds a matrix. For claude it installs into a scratch home. For codex and agy it does a dry run on a project target. For gemini it checks the payload. It runs `<binary> <sub> --help` for the core verbs and prints a TARGET and STATUS table.
- **Effect and safety:** WRITES-LOCAL (an install in a scratch home) and STARTS-PROCESSES (the binary with `--help`).
- **Output:** A tab table. No `--json`.
- **Exit codes:** 0 all pass, 1 any failure or path error.
- **Called by:** CI (`release.yml`) and the `evo:publish` skill.
- **Policy conformance:** CD-30: not in `--help`. CD-04 and CD-05: flat hyphenated name beside `release`. CD-13: `verify` is an avoided word (proposed `release check-clis`, proposed, not built). CD-16: an unknown argument does not fail.

#### `rollback`

```
evolve rollback <journal.json> [--reason "..."] [--dry-run]
```

- **What it does:** Undoes a failed release. It deletes the release and the tag and reverts the commit.
- **How it works:** `rollback.Run` reads the release journal and runs `gh release delete`. It deletes the remote tag, runs `git revert` and ships the revert.
- **Effect and safety:** WRITES-GIT and WRITES-REMOTE. `--dry-run` is available. The journal must exist. There is no confirmation.
- **Caution:** `rollback` deletes a public release and a remote tag, and pushes a revert commit.
- **Output:** No `--json`. Logs on stderr.
- **Exit codes:** 0, 1 partial or failure, 2 journal missing or malformed, 10 usage.
- **Called by:** the release pipeline (after a failure that follows the push) and the operator.
- **Policy conformance:** On no list: a flat verb outside the flat list (CD-03; proposed `release rollback`, proposed, not built). CD-62: an irreversible verb that runs by default and previews only with `--dry-run`.

#### `version-bump`

```
evolve version-bump <target-version> [--dry-run]
```

- **What it does:** Moves every version marker to the target version in one atomic step.
- **How it works:** `versionbump.Run` rewrites the plugin and marketplace JSON, `SKILL.md` and `README.md` under the repo root. It prints a result JSON.
- **Effect and safety:** WRITES-LOCAL (tracked files, no commit). `--dry-run` is available. It reports a no-op when the version is already the target.
- **Output:** A result JSON on stdout always. Status on stderr.
- **Exit codes:** 0 (also a no-op), 1 failure, 10 usage.
- **Called by:** the release pipeline and the operator.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `release`; proposed `release bump` (proposed, not built). CD-12: `bump` is not a standard verb.

#### `changelog-gen`

```
evolve changelog-gen <from-ref> <to-ref> <target-version> [--dry-run]
```

- **What it does:** Writes a Keep-a-Changelog entry from the conventional commits between two refs.
- **How it works:** `internal/changeloggen` reads `git log from..to`, groups the commits by type and puts the block at the top of `CHANGELOG.md`. It skips when the entry for the version exists.
- **Effect and safety:** WRITES-LOCAL (`CHANGELOG.md`). `--dry-run` prints the block. The version must be semver.
- **Output:** The block goes to stdout in dry-run. Status lines go to stderr. No `--json`.
- **Exit codes:** 0, 1 bad ref, not semver or write failure, 10 usage.
- **Called by:** `evolve release` (step 2). The operator can call it alone.
- **Policy conformance:** CD-04: flat hyphenated name. CD-03: flat verb outside the flat list. CD-12: `-gen` suffix instead of `generate`. Proposed `release changelog generate` (proposed, not built). CD-25: three positional operands.

#### `marketplace-poll`

```
evolve marketplace-poll <target-version> [--max-wait-s N] [--poll-interval-s N] [--marketplace-dir DIR] [--dry-run]
```

- **What it does:** After a release, waits until the marketplace checkout shows the new version.
- **How it works:** `internal/marketplacepoll` polls the marketplace directory (`~/.claude/plugins/marketplaces/evo`). In each round it runs `git fetch origin main` and `git reset --hard origin/main` there, then compares the version in the manifest.
- **Effect and safety:** WRITES-GIT on the marketplace checkout (outside the repository) and reads the network. `--dry-run` logs only.
- **Caution:** `marketplace-poll` runs `git reset --hard origin/main` in the marketplace checkout in each round. The name "poll" does not show that it discards local changes there.
- **Output:** No `--json`. Logs on stderr.
- **Exit codes:** 0 converged, 1 timeout, 2 runtime error, 10 usage.
- **Called by:** `evolve release` and the `verify-release` skill.
- **Policy conformance:** CD-04: flat hyphenated name; proposed `release poll` (proposed, not built). CD-12: `poll` is not a standard verb (proposed `release watch`). CD-11: flags with a `-s` suffix.

### 10. Setup and maintenance

#### `help`

```
evolve help
```

- **What it does:** Prints the top-level command list.
- **How it works:** It prints the usage text, which is a second list of verbs, and exits 0.
- **Effect and safety:** READ-ONLY.
- **Output:** Text on stdout. No `--json`.
- **Exit codes:** 0.
- **Called by:** the operator. Agent class: allowed. The aliases are `--help` and `-h`.
- **Policy conformance:** On the flat list. CD-32: the list is a second list, not built from the registry. The list omits 45 verbs (see CD-30).

#### `version`

```
evolve version
```

The aliases are `--version` and `-v`.

- **What it does:** Prints the build version.
- **How it works:** `pkg/version.Get()` returns the version. The verb does no I/O except stdout.
- **Effect and safety:** READ-ONLY. Agent class: allowed.
- **Output:** Plain text, for example `evolve dev (a099157d1466, built 2026-11-23T13:02:57Z)`. No `--json`.
- **Exit codes:** 0 always.
- **Called by:** the operator and CI.
- **Policy conformance:** On the flat list. CD-41: no `--json`.

#### `setup`

```
evolve setup detect|recommend|latest [--json] [--evolve-dir D] [--project-root P]
evolve setup apply --preset NAME [--dry-run] [--evolve-dir D] [--project-root P]
evolve setup complete [--evolve-dir D] [--project-root P]
```

- **What it does:** Onboarding. It finds the installed LLM CLIs, suggests presets of models for each phase, writes the chosen preset and marks the first run as done.
- **How it works:** `internal/setup.Detect` probes the CLIs. `Recommend` loads the presets. `latest` runs a live, parallel probe for the latest model of each ready CLI. `apply` merges the preset into `.evolve/policy.json` in one atomic write. `complete` writes a first-run marker.
- **Effect and safety:** `detect` and `recommend` are READ-ONLY. `latest` STARTS-PROCESSES (and can call the LLM). `apply` and `complete` WRITES-LOCAL. `apply --dry-run` prints the merged policy and writes nothing.
- **Output:** `--json` on `detect`, `recommend` and `latest`. `apply` and `complete` have no `--json`.
- **Exit codes:** 0, 1 failure, 10 usage or missing sub-verb.
- **Called by:** the operator through the `evo:setup` skill. The loop nudges on the first run.
- **Policy conformance:** On the flat list but it is a group (CD-03 allows it). CD-12: `detect`, `recommend`, `latest`, `apply` and `complete` are not standard verbs. CD-41: `apply` and `complete` have no `--json`. CD-62: `apply` writes by default.

#### `install`

```
evolve install [--ci] [--help]
```

- **What it does:** Copies the evolve agents and the loop skill into `~/.claude` by hand, without the plugin marketplace.
- **How it works:** `internal/installer` with `--ci` only validates the source. Without `--ci` it copies the agents and the skill into `~/.claude`. It warns when the plugin is already installed.
- **Effect and safety:** WRITES-LOCAL outside the repository (`~/.claude`). `--ci` is READ-ONLY and is the validate mode.
- **Output:** No `--json`. Output on stdout.
- **Exit codes:** 0, 1 failure, 10 bad flag.
- **Called by:** CI (`install --ci`) and the operator.
- **Policy conformance:** On the flat list. CD-61: it copies files and has no `--dry-run`. CD-41: no `--json`.

#### `uninstall`

```
evolve uninstall [--ci]
```

- **What it does:** Removes the agents and the loop skill that were installed by hand in `~/.claude`.
- **How it works:** `installer.Uninstall(homeDir)` deletes the installed files. With `--ci` (or `CI=true` in the environment) `installer.UninstallDryRun` lists them. The project `.evolve` stays.
- **Effect and safety:** WRITES-LOCAL (deletes files in `~/.claude`). `--ci` is the dry run, and its name does not say so. No confirmation.
- **Output:** Text. No `--json`.
- **Exit codes:** 0, 1 home directory or uninstall failure, 10 unknown flag.
- **Called by:** the operator. `install.sh` tells the user to run it.
- **Policy conformance:** On the flat list. CD-63: the dry run is named `--ci`, not `--dry-run`. CD-76: reads `CI` from the environment.

#### `doctor`

```
evolve doctor probe <tool> [--json] [--quiet]
evolve doctor boot <driver> [--sandbox] [--json]
evolve doctor live <driver> [--model M] [--json]
evolve doctor plane [root] [--repair]
evolve doctor seat
evolve doctor versions [--latest] [--json]
```

- **What it does:** Checks the machine and the CLI seats: tool present, CLI boots, CLI works live, plane worktree links, seat token, CLI versions.
- **How it works:** `probe` looks the tool up on PATH and in known places. `boot` and `live` start a real `*-tmux` driver in a seat. `live` sends a real prompt and reads the usage meter. `plane` checks git worktree links. `versions` compares the installed versions with the latest (network).
- **Effect:** `probe`, `seat` and `plane` are READ-ONLY. `boot` STARTS-PROCESSES. `live` LAUNCHES-LLM. `plane --repair` WRITES-GIT (worktree links). `versions --latest` reads the network.
- **Safety:** No dry-run. Agent class: `probe` allowed, `boot` denied.
- **Caution:** `doctor live` sends a real prompt to a model and `doctor plane --repair` rewrites git worktree links. The name "doctor" suggests a read-only check.
- **Output:** `--json` on `probe`, `boot`, `live` and `versions`. Messages go to stderr and the path to stdout.
- **Exit codes:** 0, 1 not found or fail, 2 runtime, 3 post-publish, 10 usage.
- **Called by:** the operator, agents (`probe`) and the release flow.
- **Policy conformance:** CD-12: `probe`, `boot`, `live`, `plane`, `seat` and `versions` are not standard verbs (proposed `doctor check <topic>`, proposed, not built). Help text differs: top help lists only `doctor probe`.

#### `rebuild`

```
evolve rebuild [--project-root P] [--repin]
```

- **What it does:** Rebuilds the plane binary from HEAD after a merge, so the running tool matches the code.
- **How it works:** It runs `go build` into `<root>/go/bin/evolve`, hashes the old and the new binary, and with `--repin` re-pins `expected_ship_sha` in `.evolve/state.json`.
- **Effect and safety:** WRITES-LOCAL (it replaces `go/bin/evolve`, and `--repin` edits `state.json`) and starts a `go build` process. It refuses (exit 1) while any live run lease exists. No dry-run.
- **Output:** No `--json`. One line `rebuild: old -> new` on stdout.
- **Exit codes:** 0, 1 live lease or repin failure, 2 usage, build failed or hash failed.
- **Called by:** the operator at a boundary. `internal/loopchain/refresh.go` boots a rebuild by function.
- **Policy conformance:** On the flat list. CD-33: `--help` exits 2. CD-51: usage exits 2. CD-61: it replaces the binary and has no `--dry-run`.

#### `reset-sha`

```
evolve reset-sha [--project-root P] [--operator]
```

- **What it does:** Re-pins the expected binary SHA of the ship gate to the running binary, after an intended rebuild.
- **How it works:** It hashes the running binary and takes the build commit. It calls the phase-integrity re-pin on `.evolve/state.json`. The commit must be an ancestor of HEAD (`git merge-base --is-ancestor`), unless `--operator` is set.
- **Effect and safety:** WRITES-LOCAL (`.evolve/state.json`). A provenance gate applies and `--operator` overrides it. Agent class: denied. No dry-run.
- **Caution:** `--operator` skips the provenance check, so a binary of unknown origin can become the trusted ship binary.
- **Output:** One line on stdout. No `--json`.
- **Exit codes:** 0 re-pinned, 1 any failure.
- **Called by:** the wave boundary (`reset-sha --operator`) and the operator.
- **Policy conformance:** CD-30: not in `--help`. CD-04: flat hyphenated name; proposed `ship repin` (proposed, not built). CD-12: "reset" names a re-pin. CD-33: `--help` exits 1. CD-50: all failures exit 1.

#### `gc`

```
evolve gc --project-root P [--dry-run] [--only cache|salvage] [--json] [--lease-wait D]
evolve gc check [--project-root P] [--json]
```

- **What it does:** Releases what finished cycles left behind: tmux sessions and sockets, orphan processes, worktrees, branches, run directories, caches and stale remote branches.
- **How it works:** It takes the `.evolve/gc.lock` lease. It plans, then applies the sweeps in this order. The sweeps are continuations, worktrees, dev worktrees, run directories, Go cache, temp files, seat sessions, test cache, claim directories and remote branches. A sweep that stops processes is skipped while any run holds a live lease.
- **Effect and safety:** KILLS (SIGTERM), WRITES-LOCAL (removes directories), WRITES-GIT (deletes worktrees and branches) and WRITES-REMOTE. `--dry-run` previews and forces the remote sweep to shadow mode. A real run refuses without `--project-root` (exit 1). Another gc that holds the lease gives exit 3. Agent class: denied.
- **Caution:** `gc` deletes remote branches through `gh api -X DELETE .../git/refs/heads/...` at the policy stage. It also sends SIGTERM to processes and deletes worktrees. The registry summary does not say so. Run `--dry-run` first.
- **Output:** `--json` (also single-dash). Lines on stdout, notes on stderr.
- **Exit codes:** 0, 1 a sweep failed or root missing, 3 lease held, 10 usage or unknown `--only`. `gc check`: 0, 1, 10.
- **Called by:** the boundary (`gc --lease-wait`), the in-loop hook and the operator.
- **Policy conformance:** On the flat list. CD-12: `gc` is an abbreviation. CD-62: a real run starts without `--apply`; the preview is not the default (proposed `--apply`, proposed, not built).

#### `prune-ephemeral`

```
evolve prune-ephemeral [--dry-run] [--quiet] [--tracker-ttl-days N] [--dispatch-log-ttl-days N]
```

- **What it does:** Deletes old tracker files and dispatch logs in `.ephemeral/`, by age.
- **How it works:** `internal/pruneephemeral.Run` uses the TTLs (default 7 and 30 days; the log TTL comes from `gc.logs.dispatch.ttl_days`).
- **Effect and safety:** WRITES-LOCAL (deletes files). `--dry-run` previews.
- **Output:** No `--json`. Output on stdout unless `--quiet`.
- **Exit codes:** 0, 1 run error, 10 usage.
- **Called by:** the `Stop` hook in `.claude/settings.json` (`prune-ephemeral --quiet || true`).
- **Policy conformance:** CD-04: flat hyphenated name; proposed `gc ephemeral` (proposed, not built). CD-14: overlaps `gc`. Help text differs: the help omits the two TTL flags.

#### `branches`

```
evolve branches audit [--project-root P] [--base main]
evolve branches prune [--project-root P] [--base main] [--dry-run=true]
```

- **What it does:** Finds `cycle-*` branches that a later landing already replaced. Optionally deletes them.
- **How it works:** `core.PruneSupersededOrphans` compares each `cycle-*` ref with the base. With `--dry-run=false`, `prune` asks `gh pr list --head <ref>` for open PRs. It deletes branches that are superseded, not checked out, not bound and not live.
- **Effect and safety:** `audit` is READ-ONLY. `prune` is a dry run by default. With `--dry-run=false` it WRITES-GIT (deletes local branches) and reads from `gh`. It keeps live lanes, branches with an open PR, checked-out branches and bound branches.
- **Output:** Lines on stdout. No `--json`.
- **Exit codes:** 0, 1 error, 10 usage.
- **Called by:** the operator (branch cleanup).
- **Policy conformance:** CD-63: `--dry-run` defaults to true and the inverse `--dry-run=false` is the way to run. CD-62: the verb previews by default but not with `--apply`. CD-06: plural group name. CD-41: no `--json`.

#### `backups`

```
evolve backups verify [--dir D]
```

- **What it does:** Before you delete backup bundles or patches, proves that each head and patch already exists elsewhere (main or another branch).
- **How it works:** It lists `<hub>/backups`. For each `.bundle` it reads the heads and classifies them with git. For each `.patch` it checks that `origin/main` has it applied. The run has a 5-minute timeout.
- **Effect and safety:** READ-ONLY (git read commands). It prints `refusing:` lines for unsafe items.
- **Output:** A table on stdout (`KIND FILE HEAD STATUS`). No `--json`.
- **Exit codes:** 0 all safe, 1 something exists only in the backup, 2 usage or I/O.
- **Called by:** the operator (branch cleanup runbook).
- **Policy conformance:** CD-13: `verify` is an avoided word (proposed `backups check`, proposed, not built). CD-06: plural group name. CD-24: uses `--dir`, not `--project-root`. CD-33: `--help` is not handled; any other argument gives usage with exit 2. CD-51: usage exits 2. CD-41: no `--json`.

### 11. Pipeline internals

#### `phase`

```
evolve phase <name> [--cycle N --project-root P]      (PhaseRequest JSON on stdin)
evolve phase verify <name> [--workspace D] [--worktree D] [--evolve-dir D] [--json]
evolve phase lint <name>
```

Names come from the registry: `audit`, `build`, `debugger`, `intent`, `retro`, `scout`, `ship`, `tdd`, `triage`.

- **What it does:** Runs one pipeline phase in the process (its agent seat), or checks the deliverable of a phase.
- **How it works:** It builds a PhaseRequest from stdin or from `--cycle` (`.evolve/runs/cycle-N/cycle-state.json`), installs the router, resolves the runner and runs it. The runner uses an LLM seat through the bridge. It prints a PhaseResponse JSON. `verify` runs the deliverable and handoff probe. `lint` checks a phase spec.
- **Effect and safety:** Phase run LAUNCHES-LLM and WRITES-LOCAL (workspace artifacts and ledger). `verify` and `lint` are READ-ONLY. It refuses with exit 3 when the routing does not allow the CLI. No dry-run. Agent class: run is by request, `verify` allowed, `lint` denied.
- **Output:** A PhaseResponse JSON on stdout always. `verify --json`.
- **Exit codes:** run: 0, 1 runner error, 3 routing refused, 10 unknown phase or usage, 11 bad stdin JSON. `verify`: 0 pass, 1 fail, 2 error, 10. `lint`: 0, 10.
- **Called by:** the cycle orchestrator and `plan-and-execute`. 43 skill files name it.
- **Policy conformance:** CD-08: the phase name is a catch-all sub-verb. CD-05: `phase`, `phases` and four `phase-*` flat verbs split one noun. CD-27: reads stdin without `-`. CD-13: `verify` is an avoided word. Help text differs: the help lists 8 phases and omits `debugger`, and omits `verify` and `lint`.

#### `phases`

```
evolve phases [--profile-dir D] [--persona-override PATH:NAME] list
evolve phases validate [name] [--strict-provenance]
evolve phases check-coherence [--strict]
evolve phases check-artifact-coherence [--strict]
evolve phases check-provenance --cycle N [--json]
evolve phases add <name>
evolve phases create (--spec FILE|- [--persona FILE|-] [--mint FILE|-] [--root DIR])
```

- **What it does:** Lists, validates and creates the pipeline phase definitions (the phase catalog).
- **How it works:** `phasespec` and `phasecoherence` do the checks. `add` makes a scaffold directory in `.evolve/phases`. `create` writes `phase.json` and a persona `agents/<agent>.md`, rebuilds the inventory and returns a JSON envelope.
- **Effect and safety:** `list`, `validate` and the `check-*` sub-verbs are READ-ONLY. `add` and `create` WRITES-LOCAL. `create` refuses a name that exists. No dry-run. Agent class: `list` allowed.
- **Output:** `--json` on `check-provenance`. `create` prints a JSON envelope.
- **Exit codes:** 0, 1 violation or failure, 2 create collision or unreadable, 10 usage.
- **Called by:** the operator and the `evo:phase-create` skill.
- **Policy conformance:** CD-30: not in `--help`. CD-05: splits the `phase` noun with `phase`. CD-06: plural group name. CD-13: `create` is an avoided word (proposed `phases add`, proposed, not built). CD-14: `add` and `create` overlap.

#### `phase-inventory`

```
evolve phase-inventory build [--project-root P] [--ttl D] [--force]
```

- **What it does:** Builds the cached index of phase specs that the advisor reads.
- **How it works:** `internal/phaseinventory.Build` scans the phase spec roots and writes `.evolve/phase-inventory.json` with a TTL cache.
- **Effect and safety:** WRITES-LOCAL (a cache file). No dry-run.
- **Output:** No `--json`. Output on stdout.
- **Exit codes:** 0, 1 build error, 10 usage.
- **Called by:** `phases create` tells you to run it. ADR-0038.
- **Policy conformance:** CD-30: not in `--help`. CD-04 and CD-05: flat hyphenated name beside `phase`. Proposed `phase inventory build`, or `phases inventory generate` (proposed, not built). CD-12: `build` is not a standard verb (`generate`).

#### `phase-observer`

```
evolve phase-observer [--enforce] [--scope=cycle|phase] <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]
```

- **What it does:** Watches the stream of one running phase for stalls and reports or acts on it.
- **How it works:** `internal/phaseobserver` and `observerengine` tail the stream-json in the workspace and write envelope reports in `.evolve`, as set by `observer.*` in the policy. With `--enforce` the engine can act on the process group.
- **Effect and safety:** WRITES-LOCAL (reports). With `--enforce` it KILLS the process group (`observerengine/respond.go`). `--enforce` is opt-in.
- **Caution:** `--enforce` kills the process group that you pass as `<pgid>`. A wrong pgid kills the wrong processes.
- **Output:** No `--json`. Report files and stderr.
- **Exit codes:** 0, 10 invalid arguments.
- **Called by:** no in-tree caller found. The orchestrator reference doc names it. The legacy shell launcher is gone. Possible dead verb.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `phase`. CD-15: agent noun; proposed `phase observe` (proposed, not built). CD-11: `--scope=VAL` accepts the equals form only. CD-25: five positional operands.

#### `phase-order`

```
evolve phase-order
```

- **What it does:** Prints the phase names in order, from the phase registry.
- **How it works:** `phaseorder.List` reads the registry path of the config. The root comes from `EVOLVE_PROJECT_ROOT`, or from `git rev-parse --show-toplevel`, or from the current directory.
- **Effect and safety:** READ-ONLY.
- **Output:** A list on stdout. No `--json`.
- **Exit codes:** 0, 1 registry read error.
- **Called by:** nobody. There is no caller and no doc mention (0 docs, 0 skills). Strongest candidate for a dead verb. `phases list` shows the same data.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `phase` and `phases`. CD-76: reads the root from the environment. CD-41: no `--json`. Proposed removal, or `phases list` (proposed, not built).

#### `phase-watchdog`

```
evolve phase-watchdog <workspace> <target_pgid> <cycle> <cycle_state_path>
```

Settings come from `observer.{stall_s,watchdog_poll_s,watchdog_warn_pct,watchdog_grace_s,watchdog_disabled}` in the policy.

- **What it does:** Kills a phase process group that shows no file activity for too long.
- **How it works:** `internal/phasewatchdog` polls the file times in the workspace. It warns at a percent of the stall limit, then kills the process group (`KillPgrp`) and records a signal.
- **Effect and safety:** KILLS/STARTS-PROCESSES and WRITES-LOCAL (a signal record). A grace period and a warn threshold apply. The `watchdog_disabled` policy turns it off. No dry-run.
- **Caution:** `phase-watchdog` kills the process group that you pass as `<target_pgid>`. A wrong pgid kills the wrong processes.
- **Output:** No `--json`. Logs on stderr.
- **Exit codes:** 0, 1 invalid arguments.
- **Called by:** no in-tree launcher found. Possible dead verb, replaced by `phase-observer`.
- **Policy conformance:** CD-04 and CD-05: flat hyphenated name beside `phase`. CD-15: agent noun. CD-50: bad arguments exit 1, not 10. CD-25: four positional operands.

#### `serve-phase`

```
evolve serve-phase <name>
```

The name is one of `audit`, `build`, `debugger`, `intent`, `retro`, `scout`, `ship`, `tdd`, `triage`.

- **What it does:** Runs one phase as a subprocess that speaks the `phaseproto` protocol on stdin and stdout.
- **How it works:** It finds the phase factory by name in the registry. `phaseproto.ServeStdio` reads a PhaseRequest from stdin, runs the phase and writes the response on stdout. The phase can launch its LLM seat.
- **Effect and safety:** LAUNCHES-LLM (it depends on the phase) and WRITES-LOCAL in the cycle workspace. It is not for use by hand.
- **Output:** Protocol frames on stdout, not human text.
- **Exit codes:** 0, 1 serve error, 10 missing or unknown name.
- **Called by:** the other end of `phaseproto.SubprocessRunner`. It is internal.
- **Policy conformance:** CD-04: flat hyphenated name. CD-12: `serve` is not a standard verb (proposed `phase serve`, proposed, not built). CD-27: it reads stdin without `-`.

#### `plan-and-execute`

```
evolve plan-and-execute [--plan-output PATH] [--workspace DIR] [--skip-execute] <phase>      (PhaseRequest JSON on stdin)
```

- **What it does:** Runs a phase twice. The first pass is in plan mode. The second pass edits, with the plan.
- **How it works:** It reads a PhaseRequest from stdin and calls `runPhase` in permission mode `plan`. It checks the plan file `<workspace>/<phase>-plan.md`. It then calls `runPhase` in mode `acceptEdits`.
- **Effect and safety:** LAUNCHES-LLM (through the phase). `--skip-execute` stops after the plan pass.
- **Output:** No `--json`. Progress on stdout.
- **Exit codes:** 0, 1 pass A failed or bad stdin, 2 pass B failed, 10 usage or unknown phase, 11 plan file missing.
- **Called by:** no caller in code or skills. Possible dead verb (experimental).
- **Policy conformance:** CD-04: flat hyphenated name that joins two verbs. CD-03: flat verb outside the flat list. CD-27: reads stdin without `-`. Proposed `phase plan` and `phase run --plan` (proposed, not built). Help text differs: the help omits `--workspace`.

#### `subagent`

```
evolve subagent cache-prefix --cycle N --agent NAME --workspace PATH --out PATH [--project-root P]
evolve subagent resolve-tier --profile PATH --cycle N [--project-root P] [--worktree P]
evolve subagent check-token <artifact> <token>
evolve subagent check-ctx-advisory <profile_json> <tokens>
evolve subagent validate-profile [--dispatch-plan-log PATH] <agent>
evolve subagent run <agent> <cycle> <workspace>      (prompt on stdin or PROMPT_FILE_OVERRIDE)
evolve subagent dispatch-parallel <agent> <cycle> <workspace>
```

- **What it does:** Gives low-level helpers to run one phase agent and its parallel workers.
- **How it works:** `run` builds a v2 cache-prefix prompt. It launches the LLM seat through the adapter and the bridge, checks the artifact and writes the ledger. `dispatch-parallel` fans out workers through `fanoutdispatch` and merges them with the aggregator. It refuses when `profile.parallel_eligible` is not true. The other sub-verbs are pure checks. The verb reads `MODEL_TIER_HINT`, `WORKTREE_PATH` and `PROMPT_FILE_OVERRIDE` from the environment.
- **Effect and safety:** `run` and `dispatch-parallel` LAUNCHES-LLM. `cache-prefix` WRITES-LOCAL. The others are READ-ONLY. Agent class: `run` denied, `check-ctx-advisory` allowed.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 run failure, 2 usage or an integrity failure, and the routing-refused code.
- **Called by:** the phases (`subagent run`) and the pipeline.
- **Policy conformance:** CD-11: hyphenated sub-verbs (`cache-prefix`, `resolve-tier`, `check-token`, `check-ctx-advisory`, `validate-profile`, `dispatch-parallel`). CD-76: three control values come from the environment. CD-15: agent noun as a group name. CD-41: no `--json`.

#### `aggregator`

```
evolve aggregator <phase> <output> <worker-artifact>...
```

Phases: `scout`, `research`, `discover`, `audit`, `learn`, `retrospective`, `plan-review`, `audit-consensus`, `cross-cli-vote`.

- **What it does:** Merges the artifacts of fan-out workers into one phase report.
- **How it works:** `internal/aggregator.Aggregate` reads each worker artifact and writes the merged Markdown to the output path.
- **Effect and safety:** WRITES-LOCAL. It overwrites the output path. No git, no LLM, no dry-run.
- **Output:** No `--json`. Diagnostics go to stderr.
- **Exit codes:** 0, 1 bad verdict, 2 usage.
- **Called by:** the pipeline. `consensus-dispatch` starts it as a child. Skill `retro` also names it.
- **Policy conformance:** CD-03: flat verb outside the flat list; proposed `phase aggregate` (proposed, not built). CD-15: agent noun as a verb. CD-51: usage error exits 2, not 10. CD-25: positional operands, three or more.

#### `fanout-dispatch`

```
evolve fanout-dispatch [--cache-prefix-file PATH | --cache-prefix-file=PATH] <commands.tsv> <results.tsv>
```

- **What it does:** Runs a list of worker commands in parallel with a bounded count and collects the results.
- **How it works:** `internal/fanoutdispatch.Run` takes its settings from `fanout` in `.evolve/policy.json`. It reads `commands.tsv`, starts the workers and writes `results.tsv`. It can cancel workers when a consensus forms.
- **Effect and safety:** KILLS/STARTS-PROCESSES (it starts and cancels workers) and WRITES-LOCAL. No dry-run.
- **Output:** No `--json`. Logs on stderr.
- **Exit codes:** 0, 1 a worker failed, 2 setup or usage error.
- **Called by:** `consensus-dispatch`, as a child process.
- **Policy conformance:** CD-04: flat hyphenated name. CD-03: flat verb outside the flat list. CD-51: usage exits 2. CD-12: `dispatch` is not a standard verb; proposed `phase fanout` (proposed, not built).

#### `consensus-dispatch`

```
evolve consensus-dispatch        (inputs from the environment: CYCLE, WORKSPACE_PATH, PROFILE_PATH, PROMPT_FILE)
```

- **What it does:** Runs the same audit through several LLM CLIs and merges their verdicts into one consensus verdict.
- **How it works:** `internal/consensusdispatch` writes a commands file, starts `evolve fanout-dispatch` and then `evolve aggregator` as child processes. The dispatch directory it names, `legacy/scripts/dispatch`, does not exist in this repository. It refuses when `workflow.consensus_audit_enabled` is false in the policy.
- **Effect and safety:** LAUNCHES-LLM (several CLIs), STARTS-PROCESSES and WRITES-LOCAL.
- **Output:** No `--json`.
- **Exit codes:** 0, 1, 2, 10 (from the help text).
- **Called by:** no caller found in scripts, skills, agents or Go. Possible dead or legacy verb.
- **Policy conformance:** CD-04: hyphenated flat name. CD-76: all inputs come from the environment. CD-25: no flags at all. CD-03: flat verb outside the flat list.

#### `build-invocation-context`

```
evolve build-invocation-context <role>
```

Roles: `scout`, `builder`, `auditor`, `retrospective`, `triage`, `plan-reviewer`, `tdd-engineer`, `intent`, `memo`, `inspirer`, `evaluator`, `orchestrator`.

- **What it does:** Prints the fixed "bedrock" prompt prefix of an agent role, so that the LLM prompt cache gets byte-identical prefixes.
- **How it works:** `internal/bedrock.Emit(role)` builds the prefix text and the verb prints it.
- **Effect and safety:** READ-ONLY.
- **Output:** Text on stdout. No `--json`.
- **Exit codes:** 0, 1 fatal, 2 role missing or too many.
- **Called by:** no script or Go caller found. Docs only. Possible dead verb. The error text still says `build-invocation-context.sh`.
- **Policy conformance:** CD-04: flat hyphenated name. CD-03: flat verb outside the flat list. CD-12: not a verb word; proposed `context prefix` or removal (proposed, not built). CD-51: usage exits 2.

#### `cycle-simulator`

```
evolve cycle-simulator <cycle> <workspace>
```

It reads `EVOLVE_PROJECT_ROOT` and `EVOLVE_PLUGIN_ROOT` from the environment.

- **What it does:** Walks the whole cycle plumbing (artifacts, ledger, report) with no LLM call, to test the pipeline.
- **How it works:** `internal/cyclesimulator` writes fake phase artifacts, a retrospective and a report in the workspace, and a ledger entry.
- **Effect and safety:** WRITES-LOCAL (the workspace and the ledger). No LLM and no git. No dry-run.
- **Output:** No `--json`.
- **Exit codes:** 0, 1 runtime error or bad arguments, 2 gate refused.
- **Called by:** no script or Go caller found. Possible dead verb: `cycle run --simulate` and `campaign run --simulate` cover the same need.
- **Policy conformance:** CD-04 and CD-05: hyphenated name beside `cycle`; proposed `cycle simulate` (proposed, not built). CD-15: agent noun. CD-76: reads two control values from the environment.

#### `msg`

```
evolve msg send --to ADDR[,ADDR...] --kind KIND [--subject S] [--ref PATH]... [--reply-to ID] [--correlation ID] [--expires RFC3339] [--id ULID] [--json]      (body on stdin)
evolve msg wait [--deadline DURATION] [--json]
evolve msg reply ID [--kind reply|evidence] [--json]      (body on stdin)
```

- **What it does:** Lets the phase agents of one cycle send messages, wait for messages and answer them (ADR-0129).
- **How it works:** It needs the dispatch id and the message token from the environment. It appends to the `messages.ndjson` outbox of the cycle, writes receipts and waits on the kernel wake.
- **Effect and safety:** WRITES-LOCAL (mailbox and receipts). It refuses without the dispatch id and the token, so only a seat in a dispatch can use it.
- **Output:** `--json`. Output on stdout.
- **Exit codes:** 0, 1 refused, 2 I/O or no receipt, 3 stdout reader gone, 4 not found, 5 wait deadline, 10 usage.
- **Called by:** agent seats in a dispatch. Operators do not use it.
- **Policy conformance:** CD-30: not in `--help`. CD-76: the dispatch id and the token come from the environment. CD-27: it reads stdin without `-`. CD-12: `wait` is not a standard verb (proposed `msg watch`, proposed, not built). `--help` answers "unknown subcommand" (CD-33).

#### `handoff`

```
evolve handoff check --cycle N --pair triage-tdd|tdd-build [--json] [--project-root P]
evolve handoff show --cycle N [--json] [--project-root P]
```

- **What it does:** Tells if the handoff between two phases (triage to tdd, tdd to build) is complete, from the ledger of the cycle.
- **How it works:** It reads the `.evolve/runs/cycle-N` ledger. It prints pass, retry, blocked or invalid.
- **Effect and safety:** READ-ONLY.
- **Output:** `--json` and `-json`.
- **Exit codes:** 0 pass, 1 retry, blocked or invalid, 2 I/O, 10 usage.
- **Called by:** phase agents and the operator. No code caller found; docs only.
- **Policy conformance:** Conforms in shape. CD-20: the verb also accepts `-json`.

## 4. Features without a direct verb

Two reviews matched 52 plans to the verbs of the binary. Most features have a direct verb (about 175 rows). The tables below list the rest. Each row has the verb that can close the gap. None of these verbs is built.

Set A has 27 plans. It has 104 direct rows and 19 loop-only rows that are fine. It has 12 config-only rows (5 with no show verb), 4 loop-only gaps, 0 missing and 4 doc drift.

Set B has 25 plans. It has about 71 direct rows, 17 loop-only rows that are fine, 5 loop-only gaps, 9 config-only rows, 3 missing and 2 doc drift. The set B counts can be off by 2. A few rows mix classes.

### 4.1 Missing verbs and loop-only gaps

| # | Class | Feature | Gap | Suggested verb |
|---|---|---|---|---|
| 1 | MISSING | latency gates | `boundary` calls `ci latency check`, and the sub-verb does not exist. Each boundary prints a WARN. | `evolve ci latency check` |
| 2 | LOOP-ONLY-GAP | latency gates | The duration ratchet runs only as the separate binary `go/cmd/testlatency` in CI. | `evolve ratchet check latency` and `ratchet tighten` |
| 3 | MISSING | incident knowledge base | The acceptance "recall finds a record by a word" has no verb. `kb` has only `graph neighbors` and `graph recurrences`. | `evolve kb recall <words>` |
| 4 | MISSING | token telemetry | `tokens report` has no `--by`. The per-turn sidecar, the usage history and the agent-run ledger have no reader. | `evolve tokens report --by phase,profile,purpose` |
| 5 | LOOP-ONLY-GAP | phase handoff contract | `handoff check --pair` accepts 2 of the 5 pairs. | Accept all 5 pairs in `--pair` |
| 6 | LOOP-ONLY-GAP | LLM isolation | The rows of the isolation matrix run only under `go test`. | `evolve agent matrix [--json]` |
| 7 | LOOP-ONLY-GAP | LLM isolation | The gate-weakening classifier runs only in the build floor and in ship. | `evolve guard gateclass --base B` |
| 8 | LOOP-ONLY-GAP | admission by file cell | No view shows which file cell an item holds, or why admission defers it. | `evolve admission explain <item>` or `wave supply --cells` |
| 9 | LOOP-ONLY-GAP | host-adaptive window | The host-pressure reading and the effective ceiling are only in `host-pressure.ndjson`. | A host-pressure line in `evolve wave status` |
| 10 | LOOP-ONLY-GAP | gate contract | The registered gate `testdup` cannot run alone. `ratchet check size` and `rawgit` cover 2 of 3 seeds. | `evolve gate list` and `evolve gate run <name>` |
| 11 | LOOP-ONLY-GAP | gate contract | Base-red attribution and the main-red stop cannot be replayed for a SHA. | `evolve ci attribute --sha S` |
| 12 | LOOP-ONLY-GAP | shadow-stage evidence and all gate flips | Every stage flip (retry ladder, handoff, integration, admission, review, isolation) is a hand edit of `policy.json`. ADR-0167 says a flip needs `stage verify` evidence. | `evolve stage flip <key> --to enforce`, with `stage status` and `stage migrate` |

### 4.2 Configuration with no show verb

| # | Setting | Gap | Suggested verb |
|---|---|---|---|
| 1 | `code_index.backends` | No verb prints the effective value. | `evolve policy show <key>` |
| 2 | `gc.wave_end.scope` | Same. | `evolve policy show <key>` |
| 3 | The cycle spend limit | Same. | `evolve policy show <key>` |
| 4 | The events configuration | Same. | `evolve policy show <key>` |
| 5 | `window.adaptive` | Same. | `evolve policy show <key>` |
| 6 | `gc.mode` (shadow or enforce) | `stage report` shows `gc.remote_sweep.stage` only. | `evolve policy show <key>` |
| 7 | `telemetry.token_weights` | The setting is not built yet (PR open). | `evolve policy show <key>` |

`stage report` shows the 48 stage keys of the shadow stages. It can show a stage with no flip verb. It cannot show the other settings.

### 4.3 Designed in a plan or doc, not built

These verbs appear in a plan or an architecture doc, and the binary does not know them. The probe answers "unknown command".

- `evolve debug cycle N`, `evolve signals count` and `evolve agent show` (debug-cli plan, marked "proposed").
- `evolve review-class`, `evolve mutate`, `evolve inbox integrity` and `evolve persona strip` (review-routing plan, nothing built).
- `evolve usage statusline`, `evolve ledger query` and `evolve ledger cycle N --summary` (plans that say "not started").
- `evolve skills render` (a residual of the skills-drift plan).
- `evolve stage status` and `evolve stage migrate` (specified in ADR-0167).

### 4.4 Documents that name a verb wrongly

| # | Document | Drift | Fix |
|---|---|---|---|
| 1 | agent-messaging plan (lines 91, 92, 161) | It promises `evolve msg read`. The binary has `send`, `wait` and `reply` only. | Add `msg read`, or change the plan to `msg wait`. |
| 2 | cycle-recycle plan, row X3 | It says there is no `archive restore`. HEAD has it. The installed binary lacks `archive`. | Fix the row. Rebuild the binary at the boundary. |
| 3 | convergence-policy plan, row V8 | It says the metrics verb is not built. It exists. | Fix the row. |
| 4 | comment-reduction plan | `evolve comments` prints a usage line with `commentaudit`. | Print `evolve comments`. |
| 5 | logging-and-process-hygiene plan, H4 | It says "no `logs` verb". `evolve logs` exists. | Fix the plan status. |
| 6 | wave-integration plan, row I8 | It names `evolve wave integration status`. The verb is `evolve integration status`. | Fix the text. |
| 7 | code-review phase | `evolve phase code-review` loads the spec catalog from the project root only, and fails from `go/`. | Resolve the catalog from `--project-root`. |
| 8 | debug-cli plan, C5 to C13 | These items have no inbox item and no PR. | File the follow-up items. |

## 5. Anomalies

All counts below come from the facts of the review. They count the verbs that the facts name. They are not a full scan of the binary.

### 5.1 Help text against source

`evolve --help` lists 81 of the 127 registry verbs. 45 verbs are hidden, and `help` is the 46th that is absent. The entries mark each case as "help text differs". The list below counts the entries that mention a mismatch of the help, the usage line or the registry summary: **31 verbs**.

The main cases:

- `cli-routing`: the registry says "(read-only)", and four sub-verbs write `policy.json`.
- `ledger`: the help lists `verify` and `tail`. Four more sub-verbs exist.
- `bridge`: four handled sub-verbs are not in the usage text.
- `cycle`: three sub-verbs are not in the no-argument error.
- `wave`: the summary omits `latency` and `stop`.
- `phase`: the help lists 8 phases. The registry has 9 (`debugger` is missing).
- `comments`: the usage line prints `commentaudit`.
- `lessons`: the usage line shows a flag that the function ignores.
- `models`, `stage`, `tokens`, `flags`, `audit` and `msg` answer `--help` with "unknown subcommand" and no list of sub-verbs.

### 5.2 Possible dead verbs

The facts found no caller in scripts, CI, skills, agents, hooks or Go code. Only docs mention them. A verb in this list can still be useful to a person.

| Group | Verbs | Count |
|---|---|---|
| Strong: no caller, and the target is gone or replaced | `build-invocation-context`, `consensus-dispatch` (its `legacy/scripts/dispatch` directory is absent), `cycle-simulator`, `detect-nested-claude`, `estimate-quota-reset`, `commit-prefix-gate`, `phase-order` (0 doc mentions), `plan-and-execute`, `phase-watchdog`, `phase-observer`, `fleet`, `resolve-llm`, `reachability`, `test-scoped` | 14 |
| Weak: docs only, or a one-time use | `preflight-environment`, `phase-inventory`, `ladder`, `lessons`, `handoff`, `floor`, `postmortem`, `land`, `kb`, `issue`, `msg`, `soak-report`, `swarm`, `verdicts`, `tokens`, `stage`, `salvage`, `skill-inventory`, `convergence`, `carryover`, `apicover`, `comments` | 22 |

`apicover` and `comments` duplicate the separate binaries `go/cmd/apicover` and `go/cmd/commentaudit`. CI uses the separate binaries. `msg` is for seats only and needs a token from the environment.

### 5.3 Single-dash flags

The Go `flag` package accepts `-x` and `--x`. The policy asks for `--x` (CD-20). The facts show these cases:

- 10 verbs show a single dash in their usage or help. They are `apicover`, `comments`, `eval`, `salvage`, `continuation`, `console-lease`, `dashboard`, `compose`, `detect-nested-claude` and `fleet`. The last one passes `-simulate` to `cycle run`.
- At least 43 verbs accept both forms. The group of verbs on `cmdutil.Flags` and the hand parsers accept `--x` only.
- The dossier counts 11 literals of `"-json"` against 80 of `"--json"`, and 4 single-dash tokens in the top help out of 172.
- Short flags exist only for `-h` and `-v`, except `-n` and `-q` in `comments` and `detect-nested-claude`.

### 5.4 Inconsistent exit codes

The policy table gives 10 to a usage error (CD-51). The facts show these deviations:

| Deviation | Verbs | Count |
|---|---|---|
| Usage error exits 2 | `aggregator`, `apicover`, `audit`, `backups`, `build-invocation-context`, `campaign`, `comments`, `fanout-dispatch`, `floor`, `ratchet`, `rebuild`, `release-promote`, `selfcheck`, `solution`, `swarm`, and the top level (`evolve` alone, `evolve nosuchverb`) | 15 verbs and 2 top-level cases |
| Usage error exits 1 | `fleet`, `inbox-mover`, `phase-watchdog`, `release-consistency`, `release-verify-binaries`, `ship` (flag parse) | 6 |
| Runtime error exits 10 | `tokens` | 1 |
| Missing sub-verb exits 0 | `docs` | 1 |
| Always exits 0 | `detect-nested-claude`, `postedit-validate` | 2 |
| A refusal or a failed check uses another code | `guard` (deny is 2), `flags check` (drift is 2), `signals codes check` (2), `skills check` (2), `ledger verify` (broken chain is 2), `test audit --check` (3), `cycle run` (FAIL is 2) | 7 |
| Private code sets | `commit-prefix-gate` (2, 3, 4), `code` (21 to 23), `agent` (12, 21), `bridge` (80 to 127), `ship` (127), `inbox-mover` (3, 4) | 6 |

The source has two constant sets. `cliExitRefused` is 3, and the older `exitRefused` is 1. `test-scoped` uses the older one.

### 5.5 Verbs that reject or mishandle `--help`

| Case | Verbs | Count |
|---|---|---|
| `--help` or `-h` exits non-zero | `dashboard`, `console-lease`, `compose`, `cycle-health` (10), `status`, `soak-report`, `sync-main`, `test-scoped` (10), `rebuild` (2), `reset-sha` (1), `ship` (1), `boundary` (10) | 12 |
| `--help` is not handled | `backups` (exit 2) | 1 |
| `--help` answers "unknown subcommand" | `models`, `stage`, `tokens`, `flags`, `audit`, `msg` | 6 |
| No help path at all (usage only after a wrong call) | `acs`, `archive`, `audit`, `backups`, `boundary`, `branches`, `campaign`, `carryover`, `ci`, `cli`, `clihealth`, `code`, `comments`, `commit-gate`, `continuation`, `convergence`, `cover`, `cycle`, `dossier`, `doctor`, `context`, `context-fill` | 22 |

A bare call of `loop`, `loop-stop` and `install` acts. It does not print help. Do not probe them with no argument.

### 5.6 Other naming anomalies

- 38 of 127 names hold a hyphen. 17 start with the name of another verb (the `release-*`, `phase-*`, `cycle-*` families and six more).
- The prefix of one noun has four spellings: `cli`, `cli-routing`, `clihealth` and `cli-health` (alias).
- 15 group names are plural nouns: `backups`, `branches`, `comments`, `events`, `failures`, `flags`, `lessons`, `logs`, `models`, `names`, `phases`, `signals`, `skills`, `tokens`, `verdicts`.
- The near-name pairs `land` and `landing`, `phase` and `phases`, `guard` and `guidance`, `gc` and `prune-ephemeral`, `test` and `test-scoped`, `skills` and `skill-inventory`.
- Three spellings of "preview": `--dry-run`, `--dry-run=false` (`branches prune`) and `--apply` (`verdicts wipe`). `uninstall` uses `--ci` as a dry run.
- Five flags name the project root: `--project-root`, `--root`, `--dir`, `--evolve-dir` and `--workspace`. Three verbs take it from the environment only.
- `--json` has other meanings: `acs` (write a file), `dashboard` (`--snapshot`), `preflight-environment` (JSON is the default).
- Control values come from the environment in 14 verbs (CD-76), among them `commit-gate`, `consensus-dispatch`, `msg`, `subagent` and `uninstall`.

## 6. Keeping this page true

This page is written by hand from the registry at HEAD `e854f84dd`. The installed binary is `a099157d`, built 2026-11-23. It is older than HEAD, so it does not know `archive`. The entry of each verb follows the source, and the entry says so where the binary differs.

The page can drift when a verb changes. The proposed verb `evolve cli lint` (policy section 10, [cli-design.md](../conventions/cli-design.md)) will check this page against the registry. It is not built. It must check four things:

1. Each registry verb has one entry on this page, and each entry has a verb in the registry.
2. The column "in --help?" matches the output of `evolve --help`.
3. The conformance marks match the breaks that the lint finds.
4. The names marked "proposed, not built" are not in the registry.

Until the lint exists, a reviewer checks this page when a PR adds, renames or removes a verb. A rename follows the alias rules of the policy (CD-80 to CD-84), and the PR updates the entry of the verb.

The next change of the registry must also list the 45 verbs that `evolve --help` hides (CD-30). The map above and the help then show the same list.

# Architecture decisions D1 to D4: evidence and options (research, 2026-11)

> The plan that uses this research: [architecture-decisions-2026-11.md](../plans/architecture-decisions-2026-11.md). Specs: [llm-isolation.md](../architecture/llm-isolation.md), [phase-handoff-contract.md](../architecture/phase-handoff-contract.md), [inbox-curator.md](../architecture/inbox-curator.md), [wave-orchestrator.md](../architecture/wave-orchestrator.md).

Date: 2026-11-03. Base: `origin/main` at `cd47eb445`. Method: four read-only studies of the runtime repository.

Each local claim names its file, command or log. "Synthesis" marks our own inference. The operator picked a winner for each study by click on 2026-11-03. The plan records the picks.

**Terms used throughout:**

- **Cell:** one sandboxed LLM run that the bridge starts for a phase or an `evolve agent run`.
- **I-5:** the isolation rule that a cell reads only its own cycle, its item lineage and its own CLI files ([ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md)).
- **Stage:** the policy value `off`, `shadow` or `enforce`. At `shadow` a rule logs and does not deny.
- **Curator:** `go/internal/inboxcurator`. It runs at each boundary and proposes consumes and reroutes.
- **Boundary:** the step of the wave that merges PRs and syncs `main`.
- **Handoff ledger:** the JSON file that a phase writes to claim its output ([phase-handoff-contract.md](../architecture/phase-handoff-contract.md)).

## Table of contents

1. D1: keep host secrets out of each cell
2. D2: keep the inbox true to `main`
3. D3: four design-only inbox items
4. D4: bind artifacts to the exact tree

## 1. D1: keep host secrets out of each cell and its children

**Question.** A `console-builder` cell (claude-tmux, macOS `sandbox-exec`) logged `ISOLATION_I5_VIOLATION` (rule `read.home.secret`, stage `shadow`) for reads of `~/.ssh/id_rsa`, `~/.ssh/known_hosts`, `~/.config/gh/config.yml` and `~/.config/gh/hosts.yml`. How does the architecture keep secrets out of every cell and child process, and keep builds, tests and the ship path working?

### 1.1 Local evidence

**F1.1** The secret list is a deny list of 7 paths: `.ssh`, `.aws`, `.gnupg`, `.netrc`, `.config/gh`, `.docker/config.json`, `.kube` (`go/internal/bridge/read_isolation.go:30`).

*Implication:* a new secret path leaks until someone adds it. ADR-0156 already rejects a deny list for the environment for the same reason.

**F1.2** One stage key, `isolation.i5.stage`, controls all I-4 and I-5 read rules: runs, raw output, secrets and inputs (`llm-isolation.md:721`; `go/internal/policy/isolation.go:83`). At `shadow` a guarded rule is `(allow ... (with report))`. At `enforce` it is `(deny ...)` (`go/internal/adapters/sandbox/read_isolation.go:42-65`). `.evolve/policy.json` sets only `isolation.i2.stage=enforce`, so I-5 runs at `shadow`.

*Implication:* a flip to `enforce` also denies the runs and raw-output reads that have no grant yet.

**F1.3** At I-2 `enforce` the cell environment is an allowlist. `GH_TOKEN`, `GITHUB_TOKEN` and `SSH_AUTH_SOCK` do not reach a cell (`go/internal/bridge/hermetic_env.go:28,74-96,149-162`).

*Implication:* I-2 removed the credential variables. The file and Keychain credential paths stay.

**F1.4** The pipeline pushes and opens PRs: `landing/push.go:93-95` builds `push origin <branch>`, and `cmd_ship_pr.go:55` runs `gh` outside the cell. The console builder persona forbids push (`agents/evolve-console-builder.md:16`). The tester bans `gh api/pr/release` (`agents/evolve-tester.md:65`).

*Implication:* no cell needs a credential.

**F1.5** The log counts in `runtime/.evolve/logs/*/loop.log` (`grep -h read.home.secret ... | sort | uniq -c`): 1128 reads of `gh/config.yml`, 1124 of `gh/hosts.yml`, 29 of `.ssh/id_rsa`, 28 of `.ssh/id_rsa.pub` and 28 of `.ssh/known_hosts`. All have `family=claude-tmux`. The gh reads come from every cell kind: router 950, audit 322, build 278, tdd 220, scout 176. The router persona has no `gh` call.

*Implication:* a program other than the agent reads the gh files.

**F1.6** The installed Claude Code 2.1.316 binary has the strings `gh pr view`, `prStatusFooterEnabled` and `github_pr_status_direct` (`strings ~/.local/share/claude/versions/2.1.316 | grep ...`).

*Implication:* the harness runs `gh` for its footer PR badge in every cell. That explains F1.5. The key name comes from binary strings, not from the docs, so a contract row must prove it.

**F1.7** The `.ssh` reads come only from cells that run tests or tools: spec-verify 36, test-amplification 17, context-condense 9 and others. The set `id_rsa`, `id_rsa.pub`, `known_hosts` is what an OpenSSH client reads. The program is not known, because the report reader drops the process name (`go/internal/bridge/read_isolation.go:145,278-285`).

*Implication:* one shadow wave with the process name in the report names the reader.

**F1.8** `origin` is HTTPS. The global git config has `credential.https://github.com.helper = !gh auth git-credential` (`git config --global --get-all`). Apple Git has a built-in layer with `credential.helper=osxkeychain`. `GIT_CONFIG_NOSYSTEM=1` removes that layer (`git config --show-origin --get-all credential.helper`).

*Implication:* a git network call in a cell runs gh and the Keychain helper. The SBPL file deny does not stop the Keychain, which is a system service.

**F1.9** Probes under an SBPL deny of `~/.config/gh` and `~/.ssh`:

| Probe | Result |
|---|---|
| `gh auth status`, no other change | fails hard: "failed to read configuration" |
| the same with `GH_CONFIG_DIR` set to a missing directory | "You are not logged into any GitHub hosts", rc 0, no file written |
| `git ls-remote origin refs/heads/main` | works (public repo) |
| git with `GIT_SSH_COMMAND="ssh -F /dev/null -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=yes"` | an ssh remote fails at once and reads no `~/.ssh` file |
| `HOME` set to an empty directory | `security default-keychain` fails; `GOCACHE` moves under the new `HOME` |

*Implication:* a deny alone makes each gh call crash. A synthetic `HOME` breaks the Claude Keychain login and empties the Go cache.

**F1.10** Production code in `go/cmd/evolve` runs the real `gh` (`cmd_status.go:158`, `cmd_branches.go:131-134`, `cmd_pr.go:205`). Most tests put a stub `gh` on `PATH`. `go/cmd/evolve/main_test.go:22-60` isolates `TMUX_TMPDIR` and `CODEX_HOME` but not `GH_CONFIG_DIR` or the git config. About 15 test files set `GIT_CONFIG_GLOBAL=/dev/null` by hand.

*Implication:* a test that runs the real gh passes on the host and fails in a cell. That is an environment-dependent test.

**F1.11** An open inbox item asks the question for gh: `cells-read-gh-credentials` (P1, security, console route).

### 1.2 Outside practice (live search, 2026-11-03)

**P1.1** OpenAI Codex cloud makes secrets available only to setup scripts and removes them before the agent phase ([Codex cloud environments](https://developers.openai.com/codex/cloud/environments)).

**P1.2** Docker Sandboxes keep credentials on the host and use a credential proxy ([Docker sandbox isolation](https://docs.docker.com/ai/sandboxes/security/isolation/)). Foundry Sandbox adds API keys at an egress proxy ([foundry-sandbox](https://pypi.org/project/foundry-sandbox/)).

**P1.3** "A sandbox confines the process, not the token" ([nav.no](https://ki-utvikling.nav.no/en/news/sandbox-confines-the-process-not-the-token)).

**P1.4** Anthropic sandbox-runtime uses `sandbox-exec` and bubblewrap, a `filesystem.denyRead` example with `~/.ssh`, and a network allowlist proxy ([sandbox-runtime](https://github.com/anthropic-experimental/sandbox-runtime)).

**P1.5** `GH_TOKEN` takes precedence over stored credentials, and `GH_CONFIG_DIR` moves the config directory ([gh environment](https://cli.github.com/manual/gh_help_environment)). gh can keep its token in the system keyring ([cli/cli#7043](https://github.com/cli/cli/pull/7043)). On macOS go-keyring runs `/usr/bin/security` ([go-macos/keychain](https://pkg.go.dev/github.com/go-macos/keychain)).

**P1.6** Git's own test suite runs with `GIT_CONFIG_NOSYSTEM`. `GIT_CONFIG_GLOBAL` replaces `~/.gitconfig` ([git list](https://lore.kernel.org/xmqqczv4vgck.fsf@gitster.g/)).

*Synthesis:* if the gh token is in the login Keychain, a cell can run `/usr/bin/security find-generic-password -s gh:github.com -w`. The SBPL deny does not stop it. The X5 contract row keeps `security` allowed for the Claude login (`llm-isolation.md` section 9). The study did not read the Keychain. The operator must check it (plan, D1-OP1).

### 1.3 Windows and Linux (live search, 2026-11-03)

The operator wrote: "The solution needs to consider in different platform such as Linux and windows to be able to adapt the same settings".

**P1.7** On Linux, `bwrap` already masks files with `--tmpfs` and binds (`go/internal/adapters/sandbox/read_isolation.go:103-120`). Landlock and seccomp are the other native layers ([Codex sandbox internals](https://codex.danielvaughan.com/2026/05/03/codex-cli-sandbox-internals-seatbelt-bubblewrap-landlock-windows-dacl/)).

**P1.8** On Windows no single primitive isolates a CLI agent. The working pattern has three parts. They are a synthetic SID, a write-restricted token and a dedicated user ([Windows sandboxing for coding agents](https://agentpatterns.ai/security/windows-sandbox-primitives-coding-agents/); [Codex Windows engineering](https://codex.danielvaughan.com/2026/05/14/codex-cli-windows-sandbox-engineering-restricted-tokens-acls-elevated-architecture/); [Codex Windows architecture](https://codex.danielvaughan.com/2026/07/18/codex-cli-windows-sandbox-architecture-powershell-ast-safety-elevated-unelevated-appcontainer-restricted-tokens/)).

**P1.9** AppContainer fits packaged apps with declared capabilities, and fails for open-ended agent work. OpenAI reportedly judged AppContainer, Windows Sandbox and Mandatory Integrity Control. None was enough alone ([OpenAI: building the Codex Windows sandbox](https://openai.com/sk-SK/index/building-codex-windows-sandbox/)).

**P1.10** Windows Sandbox is a disposable Hyper-V VM. It has the strongest isolation and suits computer-use agents more than a host checkout. Job objects with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` clean up a process tree and do not isolate files. Microsoft's MXC layers a UI-restriction job object on AppContainer ([MXC internals](https://www.originhq.com/research/mxc-execution-containers-internals); [vetto Windows sandbox](https://docs.rs/crate/vetto/0.2.9/source/src/sandbox/windows/mod.rs)). WSL2 gives Linux isolation, at the cost of file-system drift.

*Caveat:* most of these sources are secondary. Verify against Microsoft Learn and the Codex repository before a build.

*Implication:* the credential-free tool view (section 1.5) works on any OS, because it is environment and config data. The file-deny layer has a macOS and a Linux realization today. The Windows realization is an open item.

### 1.4 Root cause

Synthesis from F1.1, F1.3, F1.6 and F1.8:

1. The cell has the host user's full tool identity. `HOME`, the git config layers and the gh config are the operator's.
2. Three programs use that identity with no agent request: the Claude harness, git and tests.
3. The secret rule is a deny list, which drifts.
4. No cell needs a credential (F1.4).

### 1.5 Options

| Option | Idea | Fact against it |
|---|---|---|
| A | Flip `isolation.i5.stage` to `enforce` now | F1.2 (also denies runs and raw output); F1.9 (gh crashes); F1.8 (Keychain helper stays open) |
| A' | A separate stage for the secret rules | Fixes the coupling only. gh still crashes and the helper stays |
| B | A synthetic `HOME` for each cell | F1.9: breaks the Keychain login and the Go cache. The fix puts the OAuth token in the environment, which is worse |
| C | The pipeline is the only credential holder. The cell gets a credential-free tool view | Leaves a direct `security` read of a gh Keychain item (plan D1-OP1) |
| D | One container or microVM for each cell | Largest change. The bridge drives host tmux panes. Weakens the login on macOS |
| E | A `$HOME` allowlist from `home_reads` | Needs one or two measuring waves. Alone, it leaves the Keychain path open |

The tool view of option C is a set of values in `cfg.Realization.Env` for each sandboxed cell:

- `GH_CONFIG_DIR=<ws>/.cell/gh` (a missing path);
- `GIT_CONFIG_NOSYSTEM=1`;
- `GIT_CONFIG_GLOBAL=<ws>/.cell/gitconfig`, with `user.name`, `user.email` and `safe.directory` only;
- `GIT_TERMINAL_PROMPT=0`;
- `GIT_SSH_COMMAND` with the options of F1.9.

 These are tool settings for the child. The pipeline reads none of them, so ADR-0156 rule 10 does not apply.

| Criterion (1 to 5) | A | A' | B | C | D | E |
|---|---|---|---|---|---|---|
| Fixes the root cause | 1 | 2 | 3 | 4 | 5 | 4 |
| Closes file, helper and test paths | 2 | 2 | 3 | 4 | 5 | 3 |
| Keeps builds, tests and ship working | 1 | 2 | 2 | 5 | 2 | 4 |
| LLM and token cost | 5 | 5 | 3 | 4 | 1 | 3 |
| Build size | 5 | 4 | 2 | 3 | 1 | 2 |
| Reversible | 5 | 5 | 3 | 5 | 2 | 4 |
| Fit with ADR-0154 to 0156 | 2 | 4 | 3 | 5 | 3 | 5 |
| **Total (35)** | **21** | **24** | **19** | **30** | **19** | **25** |

### 1.6 Winner and grafts

**Winner: C with all grafts** (operator pick, 2026-11-03).

- Fact: no cell needs a credential (F1.4), and three programs read it without a request (F1.5 to F1.8).
- Implication: the fix must remove the identity from the tools. A denied read then has no consumer, and the `enforce` flip breaks nothing.
- Decision: the pipeline holds each credential. Each cell gets the tool view. The secret rules get their own stage.
- Consequence: a prompt-injected cell cannot push, open a PR or read the SSH key. A gh call in a cell says "not logged in" on every host.

Grafts:

- the separate secret stage (from A');
- a `$HOME` allowlist E, queued next;
- the per-cell directory only for tool config (from B);
- `bwrap` on Linux with the same tool view (from D);
- a hermetic test helper, the process name in read reports, the Claude PR footer off through manifest data and a wider secret list.

### 1.7 Cross-platform design (operator request)

| Layer | macOS | Linux | Windows |
|---|---|---|---|
| Credential source (held by the pipeline only) | Keychain | libsecret, or a file with mode 0600 that only the pipeline reads | Credential Manager |
| Cell tool view (data) | same five values | same five values | same five values, with Windows path forms |
| File deny | `sandbox-exec` (SBPL) | `bwrap` | open item (section 1.3) |

Rule: one `CredentialSource` interface with three realizations. The pipeline calls it to run `git push` and `gh`. A cell never calls it. The tool-view values live in manifest and routing data, so the settings are the same on each OS.

### 1.8 Limits

- `security` stays allowed for the Claude login. A cell can read a Keychain item that trusts `/usr/bin/security`.
- The network stays open. C removes credentials, not egress.
- The program behind the `.ssh` reads is unknown until the process name is in the report.
- The I-4 reads (`read.runs.unrelated` 1450 for build, 538 for scout) are not secrets. They stay at `shadow` on `isolation.i5.stage`.

**What changes our mind:**

- A phase must do an authenticated action itself. Then add a typed request verb, not a token.
- The process name shows that the harness reads `.ssh`. Then the harness needs its own setting.
- Egress becomes the main risk. Then use a proxy or option D.

## 2. D2: keep the inbox true to `main` at scale

**Question.** The console inbox goes stale and the rank loses trust. Which mechanism keeps 300+ console items true to `main` with no manual audit?

### 2.1 Local evidence

**F2.1** A lane landing consumes its item. A console landing does not. A lane ship reads the `Closes-Inbox:` marker of its build report (`go/internal/phases/ship/postship.go:148,263,296`). Since 2026-10-01, 135 commits that add a file under `.evolve/inbox/consumed/` are lane cycles and 172 are console commits (`git log origin/main --since=2026-10-01 --diff-filter=A -- '.evolve/inbox/consumed/*'`). Of 1,684 commits on `origin/main`, 0 bodies hold a `Closes-Inbox:` line.

*Example:* `lane-test-floors-oversubscribe-the-host` landed in `b7c4c3d76` (#942, 2026-10-18). It stayed at rank 6 for 16 days. `reviewer-profiles-may-write-their-report` landed in PR #1220 and the merge message names no id.

*Implication:* the link between a landing and its item is optional on the console path.

**F2.2** The curator completion evidence is "named `Test...` functions exist" (`go/internal/inboxcurator/landing.go:49-64,124,167-180`). Of 292 tracked console items, 16 have no acceptance and 17 have no `files`. 36 name a test. 1 has a test name in every criterion. Of the 20 DONE or OBSOLETE items, 18 name no test.

*Implication:* the deterministic consume path reaches about 0.3% of the inbox. The curator is blind by structure.

**F2.3** The curator is at `shadow` (`.evolve/policy.json:148-149`). The `inbox` channel holds 493 `INBOX_CURATOR_PROPOSAL` events from 8 runs: 343 non-deterministic reroutes, 8 deterministic reroutes, 99 sweep consumes, 43 landing consumes, 0 applied (`evolve signals tail --channel inbox --code INBOX_CURATOR_PROPOSAL --json`).

*Implication:* 60 proposals per run are noise. Nobody reads them.

**F2.4** `acs-predicate-hardening`, `bridge-prompt-hardening` and `ledger-binding-runid` (created 2026-08-27, kind `sweep`) have prescriptions and no `acceptance`. `suite-family-flake-deflake` (2026-07-29) has neither `acceptance` nor `files`. Most cited paths exist. The cited lines drifted.

*Implication:* a "path does not resolve" lint catches few of them.

**F2.5** `evolve inbox verify` writes `premise_verified_at` (`go/internal/inboxmover/lifecycle/verify.go:13-36`). No production code reads it. The rank `age` factor raises old items (`inbox_priority.age_halflife_days: 30`).

*Implication:* a stale item climbs the rank.

**F2.6** Seams to reuse: `inboxmover.ClosesInboxIDs` (`go/internal/inboxmover/closesmarker.go`); `inboxcurator.Landed` (`gitlog.go:27-67`); the PR view in `evolve pr merge` (`go/cmd/evolve/cmd_pr.go:309-350,383-411`); the `inbox lint` job in `.github/workflows/required.yml:110-130`.

**F2.7** The manual audit of 2026-11-03 covered 309 items with 4 agents, in 7 to 10 minutes, for about 650k tokens. Result: 15 DONE, 5 OBSOLETE, 38 PARTIAL, 251 OPEN.

### 2.2 Outside practice (live search, 2026-11-03)

**P2.1** GitHub closes a linked issue when a PR with a closing keyword reaches the default branch ([GitHub docs](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/linking-a-pull-request-to-an-issue)). Linear has closing words and non-closing words (`ref`, `part of`) ([Linear GitHub integration](https://linear.app/docs/github)).

**P2.2** Age-based stale bots close valid issues. Projects settled on "the bot labels, a human closes" ([Gutenberg](https://wptavern.com/gutenberg-contributors-consider-implementing-a-bot-to-close-stale-issues); [NixOS](https://discourse.nixos.org/t/stale-bot-for-nix-just-closed-a-bunch-of-stuff/18661)).

**P2.3** LLM triage agents label and comment. Their writes go through an allowlist ([gh-aw AI issue triage](https://github.github.io/gh-aw/guides/ai-issue-triage/)).

**P2.4** Acceptance criteria that map to tests give a "ready" signal ([Microsoft playbook](https://microsoft.github.io/code-with-engineering-playbook/agile-development/team-agreements/definition-of-done/)).

*Synthesis:* practice closes work by an explicit link at landing time. It treats age as a label. It uses executable acceptance as a backstop. An LLM only proposes.

### 2.3 Root cause

1. The landing link is optional on the console path (F2.1).
2. Completion evidence is prose (F2.2).
3. Nothing pushes an item to a re-check (F2.3, F2.5).

### 2.4 Options and scores

| Option | Idea |
|---|---|
| A | `Closes-Inbox`, `Refs-Inbox` or `Closes-Inbox: none` trailer on every PR, checked in CI and `pr merge`, deterministic consume |
| B | Typed `probes` on items, run at each boundary |
| C | Age decay and a lint for dead paths |
| D | An LLM curator seat at each boundary (about 650k tokens a day) |
| E | The commit tool pre-fills the trailer from a branch claim |

| Criterion (1 to 5) | A | B | C | D | E |
|---|---|---|---|---|---|
| Fixes root cause 1 (landing link) | 5 | 2 | 1 | 2 | 4 |
| Fixes root cause 2 (obsolete, missed links) | 1 | 4 | 2 | 4 | 1 |
| Safety (no wrong close) | 4 | 3 | 4 | 2 | 4 |
| LLM cost for each boundary | 5 | 5 | 5 | 1 | 5 |
| Build size | 4 | 2 | 4 | 3 | 3 |
| Reversible | 5 | 4 | 5 | 4 | 5 |
| Fit with rules | 5 | 5 | 4 | 2 | 4 |
| Matches practice | 5 | 4 | 2 | 3 | 4 |
| **Total (40)** | **34** | **29** | **27** | **21** | **30** |

### 2.5 Winner and grafts

**Winner: A, with grafts B, C, D and E** (operator pick). Curator `enforce` follows one shadow wave.

- Fact: lanes consume because the landing carries an id. Console PRs carry none (F2.1).
- Implication: a backstop that infers completion afterwards works against a growing pile. A link at landing stops the growth.
- Decision: require the trailer on every PR to `main`. A landed `Closes-Inbox` id is a deterministic consume.
- Consequence: a landed console item is consumed at the next boundary with 0 LLM tokens.

Why not B first: 291 of 292 items have no machine form (F2.2). B needs a backfill. A needs none.

Grafts:

- B for new items only. Intake refuses an item whose probes already pass.
- C as a `freshness` factor keyed on `premise_verified_at`, with a dead-path lint at warn level only.
- D as an on-demand LLM curator that only proposes.
- E as a trailer that the commit tool pre-fills.

### 2.6 Disposition of the four stale items (operator pick)

| Item | Facts | Decision |
|---|---|---|
| `acs-predicate-hardening` | 1 member landed. `go/acs/cycle1517/predicates_test.go` is gone. 8 path:line refs drifted | Retire. File 2 small items: `pkg/acsassert` passes `--end-of-options` and validates the SHA; the consumed-record lookup refuses more than 1 record per id |
| `bridge-prompt-hardening` | 24 members over unrelated surfaces. 3 cited paths gone. No 8-rune floor in `driver_tmux_submitverify.go` | Retire. Keep 1 item for the 8-rune floor in `pendingAtInputLine` |
| `ledger-binding-runid` | `.evolve/ledger.json` is gone. Ship-binding work landed in `fb2025b24` | Retire |
| `suite-family-flake-deflake` | Evidence is cycles 1173 to 1178 (July). No recurrence since | Retire |

### 2.7 Limits

The trailer closes only what landings name. It does not retire an item that other work made obsolete. The grafts B and C cover that case. A wrong id closes open work. The control: the id must be open, `Refs-Inbox` exists for partial work, and a consume reverses with `git mv`.

**What changes our mind:**

- ADR-0157 enforce moves most items to lanes. Then A shrinks to the CI check and B grows.
- The shadow wave shows PRs that close work no item names. Then C and B matter more.
- A wrong close happens more than once a week in shadow. Then keep trailer consumes as proposals.

## 3. D3: four design-only inbox items

**Question.** Four console items have a design and no build-ready acceptance. For each: build, defer behind a measurement, or retire?

| Item | Weight | Subject |
|---|---|---|
| `self-consistency-on-decision-phases` | 0.4 | vote N times on decision seats |
| `verdict-sentinel-as-tool-call` | 0.4 | the audit verdict as a tool call |
| `breaker-fingerprint-similarity-tier` | 0.45 | a similarity tier in the failure breaker |
| `lane-alternates-on-refusal` | 0.4 | alternates when triage refuses a lane item |

### 3.1 Measurements

Window 2026-10-18 to 2026-11-02, 48,266 signal events. 141 retained cycles have `phase-timing.json` (cycles 1972 to 2126).

| Measure | Value | Source |
|---|---|---|
| Contract checks verified, rejected | 1012, 3 (2 missing `triage-decision.json`, 1 missing retro report) | signals channel |
| Bad-verdict classifications, all time | 16 in 10 cycles. The last was cycle 1811 on 2026-10-06. 0 in 4 weeks | `.evolve/bad-verdict-baseline.jsonl` |
| `ISOLATION_I8_VIOLATION` | 158, all `flow.fallback.prose` for routing digest headers. 0 `flow.fallback.stdout` | isolation.violation channel |
| Audit narrative against gate conflicts | 5 of 153 cycles. The gate resolved all 5 | `audit-fail-reason.json` |
| Triage with no work | 9 of 141 cycles (6.4%). 7 protected surface (3 host-refused, 4 LLM-dropped), 2 stale and correctly dropped | `phase-timing.json`, `triage-decision.json` |
| LLM minutes lost for each empty cycle | 1.0 to 2.5 in 8 cycles. Cycle 2117 (agy) lost 17.2. About 29 in total | `phase-timing.json` |
| Lane backfills | 126 `LOOP_ORCH_LANE_BACKFILLED`, 14 `LOOP_ORCH_SUPPLY_SHORT` | loop channel |
| Breaker misses at token-LCS 0.9 or more | 2 pairs: 2071/2072 (0.993) and 2049/2051 (0.981). Both are normalizer gaps. No halt was missed: each group has 2 members and the ceiling is 3 | `go/internal/core/failure_digest.go:153,165-171`; `policy_failure.go:75` |

*Implication:* none of the four failure classes is live now.

### 3.2 Outside practice (live search, 2026-11-03)

**P3.1** Self-consistency with k=3 gives 0 to 2 points in agent settings. Acting only when all k runs agree gives 6 to 14 points ([arXiv 2602.11619](https://arxiv.org/pdf/2602.11619)). Majority vote can lower accuracy ([arXiv 2608.11403](https://arxiv.org/pdf/2608.11403)). Resampling one model keeps its systematic errors ([LLMs as a Jury](https://www.alphaxiv.org/abs/2607.10139)).

**P3.2** Claude Code headless has `--json-schema`, and it can return valid but fabricated values ([docs](https://docs.claude.com/en/docs/claude-code/headless); [mer.vin](https://mer.vin/news/why-claude-json-schema-returns-valid-json-with-fabricated-values/)). `codex exec --output-schema` constrains the final message ([Codex non-interactive](https://developers.openai.com/codex/noninteractive)). Both flags apply to headless entry points. The bridge drives interactive REPLs, and the codex catalog lists `--output-schema` only for `codex exec resume` (`go/internal/bridge/capabilities/catalogs/codex-tmux.json:44`). *Synthesis:* a tool-call verdict in a REPL needs a host tool, which means MCP. The rule `no_mcp_cli_only` forbids MCP.

**P3.3** Learned crash-similarity methods address corpora of thousands ([GPTrace](https://arxiv.org/abs/2512.01609); [Igor](https://www.nebelwelt.net/files/21CCS.pdf)). *Synthesis:* with 32 digests and 2 normalizer gaps, a better normalizer is cheaper.

**P3.4** Do not retry a refusal with the same prompt. Reframe it or fall back ([refusal fallback](https://www.developersdigest.tech/library/skills/refusal-fallback-handling)). Deterministic re-routing with the LLM as last resort cuts cost ([arXiv 2603.01548](https://arxiv.org/pdf/2603.01548)).

### 3.3 Options and winners

| Item | Options (score of 35) | Winner |
|---|---|---|
| `self-consistency-on-decision-phases` | A1 build as specified (16); A2 agreement filter on triage (20); A3 add a cross-family agreement metric to Q8 (27); A4 retire (26) | Retire, with A3. All 9 bad menus were bias, not variance. The audit conflict is deterministic (`go/internal/phases/audit/audit.go:399-404`) |
| `verdict-sentinel-as-tool-call` | B1 as specified (15, needs MCP); B2 vendor schema flags (17); B3 audit verdict as an I-8 seat, file `audit-verdict.json` (33); B4 retire (26) | Retire. Record B3 as the successor, built when I-8 goes to `enforce` or a bad verdict recurs. The item cites `go/internal/bridge/streamjson_verdict.go`, which does not exist on `origin/main` |
| `breaker-fingerprint-similarity-tier` | C1 token-LCS tier (23); C2 fix the normalizer (35); C3 retire (26); C4 embedding similarity (12) | C2. Fold `cycle=N`, `cycle-N-<ulid>` suffixes and finding counts. Pin 2071/2072 and 2049/2051 as must-merge, 2054/2093 and 2073/2089 as must-not-merge |
| `lane-alternates-on-refusal` | D1 alternates in the lane (19); D2 retire (27); D3 directory-prefix seed rule (34); D4 re-dispatch triage (16) | Retire, plus the routing rule: a declared directory with a protected file beneath it routes to the console. Cycle 1994 declared `go/internal/inboxmover` and the protected file was `inboxmover.go` |

### 3.4 Operator picks

Retire all four. Move the self-consistency metric into Q8. File the audit-verdict typed seat for its trigger. Replace the similarity tier with the normalizer fold. Add the directory-with-protected-file routing rule.

**What changes our mind:**

- Q8 shows a seat with less than 90% cross-run agreement. Build the agreement filter, not the vote.
- A non-recoverable bad verdict recurs. Build B3 at once.
- A breaker group of 3 near-identical digests has distinct fingerprints that the folds miss.
- Empty commitments exceed 15% of a wave. Build D1.

## 4. D4: bind generated or claimed artifacts to the exact tree

### 4.1 Q4a: the handoff ledger `output_digest`

**F4.1** The spec gives the field one job: "`output_digest` is the tree digest of the output, so the audit binding (`go/internal/phases/ship/audit.go:27`) and the ledger name the same tree" (`phase-handoff-contract.md:141`). The field exists (`go/internal/handoff/types.go:69`).

**F4.2** The audit binding tree is `LedgerEntry.WorktreeTreeSHA`. `auditBindingEntry` writes it (`go/internal/core/phase_bindings.go:127,145`) through `worktreeContentSHA` (`phase_bindings.go:152-166`). That function is in package `core`. Ship reads the value (`go/internal/phases/ship/audit.go:68,129-131`).

*Implication:* the premise of the H9 branch ("no tree-digest function is reachable from core") is false. `core` also imports `treefence` already (`go/internal/core/head_moved_rebind.go:54`).

**F4.3** `HandoffReviewer.Review` receives `Worktree`, `ProjectRoot` and `Workspace` (`go/internal/core/reviewer.go:11-28`). Every input of `worktreeContentSHA` is in hand.

**F4.4** The check validates the ledger against the request, then resolves each pointer. Resolution checks that a path, test or predicate is declared. It never compares the ledger to a tree (`go/cmd/evolve/cmd_handoff.go:240-273`; `go/internal/handoff/evidence/evidence.go:29-149`).

**F4.5** For `tdd-build`, `request_digest` comes from the predicates in `go/acs/cycleN`. A build retry changes code and not predicates, so `request_digest` is constant across build rounds. H9 stamps `author` on every review (H9 `handoff_persona.go:53-86`, branch `feat/handoff-h9-persona-ledgers`).

*Implication:* a round-1 ledger still validates in round 2 and counts for an output it did not describe. The re-stamp makes it look fresh.

**P4.1** SLSA Build L3: the control plane generates the provenance, not the tenant, and the provenance names the output by digest ([SignPath](https://docs.signpath.io/slsa-attestations/definitions); [Chainguard](https://www.chainguard.dev/docs/compliance/slsa/slsa-chainguard)). *Synthesis:* the orchestrator is the control plane and the agent is the tenant.

| Criterion | A stamp and refuse stale | B drop the field | C check-time value in `check` | D agent verb |
|---|---|---|---|---|
| A ledger for an old output does not count | yes | no (F4.4, F4.5) | yes, with an extra ledger hash | no, the agent can launder |
| Same tree as the audit binding | yes | n/a | yes | depends on the agent |
| Agent cannot forge | yes | n/a | yes | no |
| Build size | about 20 lines | spec edit | about 35 lines | verb and prompt |

**Winner: A** (operator pick). The pipeline stamps `output_digest` with `worktreeContentSHA` when the ledger has none. A stamped digest that differs from the current tree gives the check outcome `invalid` (stale) and no re-stamp.

The stage is the per-pair `policy.HandoffConfig[pair].Stage`. No new policy key. Graft from C: copy the tree into `check.tree`. The stamp lives in H9 (branch `feat/handoff-h9-persona-ledgers`).

**What changes our mind:** the check re-derives each `met` status from the tree (spec section 5 step 4, not built). Then B is viable. If agents edit ledgers in place often, add the ledger-bytes hash from C.

### 4.2 Q4b: generated PRs that rot (PR #1070)

**F4.6** The step `ratchet-tighten` (`go/cmd/evolve/cmd_boundary_ratchet.go`) runs after `pr merge` and before `sync-main` (`cmd_boundary.go:187-199`). It replaces the tip of an open `chore/ratchet-tighten-` PR with `--force-with-lease`, or opens a new one ([runtime-reference.md:53](../operations/runtime-reference.md)).

**F4.7** `pr merge` merges only the numbers in `--merge n,...` (`cmd_boundary.go:190-191`). No step names the ratchet PR. It waits.

**F4.8** A direct commit to `main` was reverted by an operator decision on 2026-10-20: "the boundary step must not commit straight to main" (commit `534a1d88d`).

**F4.9** `gh pr view 1070`: OPEN, `MERGEABLE`, `CLEAN`, one commit `fffa3cf27` regenerated 2026-11-02, created 2026-10-20, one file `go/internal/sizeratchet/offenders.json`. All required checks pass. The head parent is on `origin/main`, and the 16 newer commits do not touch the ratchet packages. On an export of today's `main` with the PR's list, `evolve ratchet check size` prints `ratchet check: clean` and `evolve ratchet tighten --check` prints `ratchet tighten: tight`.

**F4.10** Each boundary force-pushes a new tip, which restarts required CI (about 20 minutes).

*Implication:* the PR does not rot in content. It rots in ownership. No actor merges what the generator opens.

**P4.2** Dependabot stops rebasing after 30 days ([GitHub changelog](https://github.blog/changelog/2023-04-13-automatic-rebases-on-dependabot-pull-requests-stop-after-30-days-of-inactivity/)). Renovate lets the generator own the merge, after passing checks ([Renovate automerge](https://docs.renovatebot.com/key-concepts/automerge/)).

| Option | Result |
|---|---|
| A merge #1070 now | Valid today. Does not fix the cause (F4.7) |
| B regenerate one PR (current) | Already shipped (F4.6). It is the state that rotted |
| C tighten on `main` | Rejected: the 2026-10-20 decision (F4.8) |
| D tighten in each lane | Rejected: "One writer. No lane edits `offenders.json`" (`docs/architecture/packages/internal-sizeratchet.md:14`) |
| E the generator owns the merge | Chosen |

**Winner: E, with A done now** (operator pick). The step merges the open ratchet PR when three gates pass:

- CI is green on the head (through the `pr merge` function, not raw `gh`).
- The head parent is an ancestor of `origin/main`.
- `sizeratchet.Scan` and `rawgitratchet.Scan` are clean on post-merge `origin/main` plus the PR lists.

The key is `boundary.ratchet_merge.stage`, `shadow` first. The step skips the force-push when the regenerated list equals the tip.

**What changes our mind:**

- The operator lifts the 2026-10-20 rule. Then C is smaller.
- Branch protection requires up-to-date branches. Then gate 2 becomes `--update-branch` and a CI wait.

# Evolve Loop

Current guarantees and supported modes: [runtime contract](docs/architecture/current-runtime-contract.md).

**An autonomous pipeline that improves your codebase across many cycles. Its structure detects when the AI tries to fake the result.**

> **The pitch, in one sentence:** Any agent can write a feature overnight. Evolve Loop is the layer that decides if that code is *safe to merge*. It decides adversarially and structurally, with a memory that compounds across runs.

The mental model is **CI/CD for AI-written code**. You give it a goal: "add dark mode," "harden the auth flow," "pay down concurrency debt". Optionally, you also give it a number of cycles. If you do not give the count, the advisor decides how many cycles the work needs. It runs unattended:

- it finds the work, plans it and writes it;
- an audit agent reviews the work adversarially;
- it ships only what passes deterministic checks;
- it records failures as durable lessons, and it gets the relevant lessons for later phases.

After more than 1,300 autonomous cycles, the trust layer is structural, not aspirational:

- **typed routing authority** keeps all operator-owned control-plane work out of autonomous lanes (ADR-0074);
- a **build handoff floor** rejects a red build before the build gets to review;
- **graduated remediation** fixes a minor defect of a correct implementation in the phase, and does not discard the work;
- a **failure-disposition contract** classifies every failed cycle (honest rejection, pipeline fault, or operator-owned). It routes the cycle to the place where a fix is possible.

The full evidence trail is in [docs/research/lessons-and-resolutions-2026-07.md](docs/research/lessons-and-resolutions-2026-07.md).

### You steer, it drives

The full external surface is small, so this section lists all of it. The loop finds out everything else by itself.

**What you control:**

- **One command**: `/evo:loop "add dark mode"`. Optionally, add a strategy (`harden · repair · innovate · balanced`), a hard cycle bound (`--cycles N`), or `--resume`. `--resume` continues from a validated durable phase boundary.
- **The backlog**: file a JSON todo into `.evolve/inbox/` with `evolve inbox add`, and change or show it with `evolve inbox edit`, `withdraw`, `verify`, `show` and `list`. A computed rank sets the order: `evolve inbox rank --explain <id>` shows how `priority_class`, `weight` and four other factors give the score. `route` sets the ownership. `"console-manual"` marks an item as operator-owned, so autonomous lanes structurally cannot draw it. `"lane"` overrides a false positive. Routing is enforced plumbing, not a suggestion in a prompt.
- **Policy, not code**: `.evolve/policy.json` holds the fleet width (`fleet.count`), the gate stages, the budgets and the CLI routing table (`cli_routing`). Zero feature flags.
- **Per-phase model routing**: `evolve cli-routing init --clis <a,b,c>` starts the routing table. Then `evolve cli-routing set agents.<agent> <clis>` pins who does what, and `evolve cli-routing explain <agent>` shows the chain of an agent. `--cli` / `--model` change one run, inside the allowed set of the table.
- **Work in progress**: `evolve checkpoint save`, `list`, `restore` and `prune` keep the uncommitted work of a worktree as git refs.
- **Releases**: `evolve release X.Y.Z` (or `/evo:publish`) does the preflight gates, the changelog, an atomic version bump, a CI-verified publish and an auto-rollback.
- **Observability on demand**: `evolve doctor` (environment probes), `evolve inbox batches` (how the backlog will group), `evolve dossier` (per-cycle verdict records), `evolve cycle-health` (11-signal integrity fingerprint).

**What the loop completes and finds out for you:**

- It finds and scopes the work (scout → triage). It puts related backlog items into one cycle. It automatically re-weights pain that recurs.
- It routes each phase to the correct model and CLI. It detects quota exhaustion from the real provider surface. It fails over across model families during a run. At each wave boundary, it installs a newer Claude Code or agy and smoke-tests it (`evolve cli update`).
- It writes tests that fail before it writes code. It self-verifies the build at handoff (a red build never gets to review). Then an audit agent reviews the result adversarially.
- It ships only through deterministic gates. When a correct implementation has one minor gate defect, the loop repairs the defect in the phase and does not discard the work. When only the deterministic audit gates force a FAIL, the cycle gets a bounded Build repair round.
- It classifies every failure (honest rejection / pipeline fault / operator-owned). It keeps the worktrees for supported continuation and operator salvage. It quarantines poison tasks that cannot pass. It files the follow-up work into its own backlog.
- It checkpoints phase boundaries and validates the identity on resume. It keeps recoverable work after interruptions. It gets the relevant durable lessons for later phases.

It works with four CLI families: Claude Code, Codex CLI, Antigravity (agy), and local models through Ollama. It can route a different LLM to each stage of the work.

> **Prefer to see it?** The **[Evolve Loop landing page](https://mickeyyaya.github.io/evolve-loop/)** shows the same story visually: the moved bottleneck, the pillars and the self-caught incident (flagship version: **[noir](https://mickeyyaya.github.io/evolve-loop/noir/)**).

---

## Quick Start

**Prerequisites:** one supported CLI ([Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex CLI](https://github.com/openai/codex), or [Antigravity](https://antigravity.google)), and a repo that you want to improve. Local models through [Ollama](https://ollama.com) are also routable. The installer automatically installs the rest (`git`, `jq`, `tmux`).

**Install — one line.** The installer detects your OS/arch and downloads the prebuilt `evolve` binary (or, as a fallback, builds from source). Then it installs evolve for the CLI(s) that you have:

```bash
curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh
```

If you do not trust `curl | sh`, examine the script first: `curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh -o install.sh && less install.sh && sh install.sh`. If you already use Claude Code, you can add the plugin directly instead: `/plugin marketplace add mickeyyaya/evolve-loop`, then `/plugin install evo@evo`.

**Locked-down / corporate environment?** You can do an air-gapped install of a pre-approved binary (no compiler, no network). The installer verifies the binary against its published SHA256 fingerprint: `sh install.sh --binary <artifact> --checksums checksums.txt`. See [docs/operations/corporate-deployment.md](docs/operations/corporate-deployment.md).

**Windows:** the runtime of the autonomous loop is Unix-based (tmux, bash), so run it under **[WSL2](https://learn.microsoft.com/windows/wsl/install)**. Install WSL, open your WSL shell (for example, Ubuntu), then run the same one-liner there. Inside WSL, it installs exactly as on Linux. The `/evo:*` skills *also* install natively in Claude Code on Windows, with the `/plugin` commands above. Only the loop runtime needs WSL.

### Run — from one command to full control

Start with only a goal. Use a flag only when you want more control. Each step below adds one flag and tells what occurs behind the scenes.

**1 · Just a goal** — the advisor decides everything:

```bash
/evo:loop "add dark mode"
```
> *Behind the scenes:* Scout reads your repo and the goal. Then the advisor **composes the pipeline itself**: which phases to run, which LLM + model runs each phase, and how many cycles. It continues until the work backlog is empty, up to a safety cap. You configure nothing.

**2 · Steer the approach** with a strategy:

```bash
/evo:loop harden                              # stability + tests
/evo:loop repair "fix the auth bug"           # smallest-diff bug fix
/evo:loop innovate "explore concurrency"      # new capabilities
```
> *Behind the scenes:* a strategy (`balanced` default · `harden` · `repair` · `innovate` · `ultrathink` · `autoresearch`) changes **what Scout looks for and how strict Audit is**. The phase spine stays the same. The posture is different.

**3 · Bound the cycles** when you want a hard limit:

```bash
/evo:loop --cycles 3 "add dark mode"
```
> *Behind the scenes:* `--cycles N` is a **contract**: N requested cycles, subject to integrity halts and interruption. If you omit it (step 1), the advisor decides the count instead.

**4 · Control the models** with a one-time setup:

```bash
/evo:setup                          # pick a preset once
/evo:loop "harden the auth flow"
```
> *Behind the scenes:* setup writes **per-phase model pins** to `.evolve/policy.json`. It prefers different model families for Build and Audit when they are available. If the policy declares a `cli_routing` table, setup refuses to write. Change the table with `evolve cli-routing set` instead. Family diversity is a routing preference. It does not guarantee independent judgment.

**5 · Resume** a run that was interrupted:

```bash
/evo:loop --resume
```
> *Behind the scenes:* a checkpoint (for example, a quota wall mid-cycle) resumes from the latest durable phase boundary, after identity validation. An interrupted phase can run again.

A hands-on walkthrough of your first cycle: [docs/getting-started/your-first-cycle.md](docs/getting-started/your-first-cycle.md).

### Setup & configuration (optional)

You can run the loop with **zero configuration**. By default, each phase runs the CLI chain of its profile. The loop skips a CLI that you do not have, and it falls back to an installed CLI. When you want control, it is one command:

```bash
/evo:setup
```

It detects which LLM CLIs you have and explains the pipeline. Then it offers **three presets**: **Recommended**, **Economy** (cheaper/faster models), and **Max-quality** (strongest models). You make **one choice**, and it writes the per-phase model routing for you. You can run it again at any time, because it is idempotent. A project that declares a `cli_routing` table uses `evolve cli-routing` instead, because setup does not write beside a table.

**All you need is an LLM CLI subscription.** Evolve drives the CLIs that you are already signed in to. Your Claude, Codex, or Gemini subscription is enough to run the loop.

Everything else has sane defaults. The only knobs are in `.evolve/policy.json`, and all of them are optional:

| Setting | Default | What it does |
|---|---|---|
| per-phase model pins | none (the profile chains) | which LLM + model runs each phase (written by `/evo:setup`; refused beside a `cli_routing` table) |
| `cli_routing` | not set | one routing table for every dispatch: the allowed CLIs, the default chain and the chains for each tier, role and agent. `evolve cli-routing` is its only writer |
| `workflow.cycle_budget` | `enforce` | `enforce` = the advisor decides the cycle count; `off` = without `--cycles`, the loop runs a single cycle |
| `workflow.max_cycles_cap` | `25` | safety ceiling for the number of cycles that the advisor can run |

Most people run `/evo:setup` one time and never open the file.

### Run several features at once

Give each feature its own loop and run the loops **in parallel**. Each loop has its own git worktree and branch, and its own LLM. The loops do not interfere with each other. Only green changes merge to `main`, one at a time.

```bash
# Fan a backlog into N concurrent, file-disjoint cycles (CLI-native):
evolve fleet --count 3 --goal-hash <hash> --plan backlog.json
```

Or run a separate `/evo:loop` in each of several git worktrees, one for each feature. Each loop can use the CLI that you prefer (Claude · Codex · Antigravity · Ollama). In both methods, the isolation is by **worktree + branch**, so concurrent loops cannot corrupt each other. For safety, the merge to `main` is **serialized**.

This is safe parallelism across **independent** work. Your machine and your LLM rate limits set the bound. It is not magic infinite scale.

---

## The problem it solves

Autonomous agents are now good enough to write a feature, fix a bug, or refactor a module unattended. Thus, the bottleneck moved. The question is no longer *can the agent write the code?* The question is now *can you merge what it wrote without a second read of every diff?*

The default answer of the industry is "ask another LLM if it looks done." That judge is the weakest link in the whole system. A single model that grades code is usually a sibling of the model that wrote the code.

That judge has the same blind spots, prefers its own style, and rewards confident prose over correctness. It will happily tell you that a hallucinated test suite passes. Worse, an agent under pressure to "finish" learns to *game* any judge that you give it. It stubs the test that fails, narrows the assertion, writes an `echo PASS` script, or declares victory in the summary while the code is broken.

The design of Evolve Loop is about that exact failure. Two things make it different from a normal CI run or a single-agent loop:

- **The reviewer can use a different model family from the author**, and its prompt tells it to challenge the work. Family separation is a routing preference, subject to the available providers and to failover.
- **The merge gate is deterministic code, not a model's opinion.** The verdict is a set of executable checks, and their exit codes *are* the decision. The model can write eloquent prose all day. Nothing ships unless the checks come back green.

---

## What it does best

If you read only one section, read this one. Evolve Loop is designed to do these things better than a plain agent loop or a single long-running skill:

- **Accomplish long, multi-step work: split it into isolated phases.** One prompt cannot hold a big task, so the pipeline decomposes the task into a fixed spine of small, independent stages. Each stage has one job, its own context, and a single artifact that it must produce. The bound on complexity is per phase, not per task.
- **Catch the AI when it games its own success criteria.** These controls work together: adversarial cross-family review, deterministic verdicts, mutation testing that rejects fake tests, and a tamper-evident ledger. They make "lie about being done" structurally hard, not only discouraged by a prompt.
- **Get smarter every run.** The loop distills failures into lesson files and feeds them back into the planning of the next cycle. Recall helps later attempts to avoid known failures. A claim of reduced recurrence needs a measurement.
- **Survive long unattended runs.** The loop expects quota walls, rate limits, and context-window failures. It checkpoints the work in flight, so the work is resumable and not discarded.
- **Stay vendor-flexible.** Route Scout to the Gemini of Antigravity, the builder to Claude Sonnet, and the auditor to Claude Opus. Use any mix that you trust, also local models through Ollama. The pipeline does not change.
- **Run lean on context.** Each phase boots with only the context that it needs. Per phase, the loop removes the redundant tool schemas, MCP servers, skills, and repo instructions that every turn silently reads again (config-injected, no code changes). The measured result is **~39% fewer context tokens per cycle**. Thus, long unattended runs cost less and hit context walls later. Full record: [token-optimization campaign](docs/research/token-optimization-2026/part5-campaign-implementation-2026-07-17.md).

It is **not** a code-writing agent that chases benchmarks. It is the governance and trust layer that you put *around* such an agent.

---

## How it works: isolated phases and validated artifacts

Every cycle runs the same spine of phases:

```
INTENT → SCOUT → TRIAGE → [PLAN-REVIEW] → [TDD] → BUILD → [CODE-REVIEW] → AUDIT → SHIP → LEARN
```

- **Intent** turns a vague goal into a structured spec (goals, non-goals, constraints, acceptance criteria). It must challenge at least one premise.
- **Scout** explores the codebase, does the necessary research, and proposes work.
- **Triage** sets the bounds of the scope of *this* cycle: what to do now, what to defer, and what to drop.
- **Plan-Review** *(optional)* sends the plan out to four lenses (product, engineering, design, security) before any code exists.
- **TDD** *(optional, on by default)* writes tests that fail and that encode the acceptance criteria. A *separate* agent writes them, not the agent that will implement the change.
- **Build** implements the change in an isolated git worktree.
- **Code-Review** *(code cycles, in shadow)* reviews the build independently on the ten dimensions of the shared quality index (`/evo:quality-index`, `/evo:architecture-review`). It is not a gate.
- **Audit** adversarially reviews the work and runs the deterministic check suite.
- **Ship** commits only if the verdict is green.
- **Learn** captures a carryover note on success, or a structured failure lesson on failure.

The important part is not the list of phases. It is how the phases connect.

**Phases receive explicit context and artifacts.** The host assembles goals, task contracts, relevant lessons, and upstream outputs. Phases can produce several reports and sidecars. The durable state and the ledger record the execution and the recovery.

**Phase processes use scoped access.** Cycles that write code have dedicated git worktrees. Supported OS profiles enforce the declared filesystem restrictions. They also permit the necessary access to the workspace, the scratch area and the CLI state. These capabilities are different for each platform and transport. See the [runtime contract](docs/architecture/current-runtime-contract.md).

**The host owns the phase order and the ship checks.** The Go state machine enforces the required spine. Ship checks the host-bound audit evidence for the cycle, the run, the round, and the tested tree. An audit that the host rejected cannot authorize a ship through a narrative PASS. OS confinement is a separate boundary, with explicit unsupported and opt-out states.

That last point is the whole design philosophy in one line: **LLMs do the qualitative work; deterministic code owns every gate.** Only a model does these judgment calls well: *what* to build, *how* to build it, and *what looks wrong*. The phase order, the scope enforcement, the ship verdict, and the audit trail are mechanical. Thus, they are in code, where the model gets no vote.

### A cycle, end to end

For example, you run `/evo:loop --cycles 1 "make the export endpoint resilient to large payloads."`. The phases use the goal, the selected task contract, the code, and the upstream evidence:

- **Intent** states the goal again as a spec, and it pushes back: *is "large" 10 MB or 10 GB? Is streaming acceptable, or must the response stay synchronous?* It records the assumptions that it uses.
- **Scout** reads the spec and traces the endpoint through the codebase. It sees that the endpoint now buffers in memory, and it proposes a fix.
- **Triage** decides that the streaming rewrite is in scope for this cycle. It defers the unrelated retry-logic cleanup that it also found.
- **TDD** first writes tests that fail: a 2 GB payload must not exhaust memory, and the response must stay correct.
- **Build** implements streaming in its own throwaway worktree until those tests pass. It stays within the supported filesystem policy.
- **Audit** (preferably on a different model family) tries to *break* it: edge cases, regressions, and if those tests actually test anything. Then it runs the check suite.
- **Ship** sees a green verdict and commits. (A red verdict routes the cycle to a retrospective, not to a merge.)
- **Learn** records what carried over for next time.

No step accepts the *word* of the previous step for anything. It reads the file. That is why the same machinery handles a one-line fix and a multi-file refactor, and its shape does not change. The work is always a chain of small, isolated stages, each with a defined input and a defined output.

---

## Catching the AI when it games the system

The threat is not a malicious human. It is the LLM that does what LLMs do under pressure to look successful. It confabulates "looks done" verdicts, it hallucinates evidence, and it takes the path of least resistance to a green checkmark. Evolve Loop answers this structurally, in three layers.

### Layer 1 — Host integrity checks

The Go state machine owns the required phase order. The sanctioned Ship path validates the host-bound audit before it accepts a cycle commit. Where the transport supports them, CLI hooks give more phase, role, and ship checks. Read-only phase fences detect source changes after execution and restore the source. The prevention depends on the applied OS policy.

The hash-chained ledger makes changes to the recorded history detectable by verification. Operator bypasses, recovery anchors, and confinement opt-outs are explicit administrative paths. The [trust contract](docs/concepts/trust-architecture.md) describes them.

### Layer 2 — OS isolation

Profiles can require OS confinement. The current macOS implementation supports the linked-worktree policy. Linux bubblewrap cannot currently express its required Git metadata writes narrowly enough. It refuses mandatory linked-worktree launches. Read denials and write denials are distinct controls. For the tested behavior and the limits, see [supported isolation capabilities](docs/architecture/recovery-isolation-policy.md).

### Layer 3 — Anti-gaming workflow defaults

The actual *detection* of gaming is in this layer. Each default targets one specific way that a single-model loop gets fooled:

| How an AI fakes success | How Evolve Loop catches it |
|---|---|
| **Self-preference / sibling bias** — a judge from the same model family as the author has the same blind spots, and it rubber-stamps. | Setup and routing prefer a different model family when one is available. The instructions tell the auditor to look for counterexamples. This is a preference, not a hard family invariant. |
| **Style over substance** — judges reward confident, verbose, well-formatted prose. | The ship verdict is **not prose**. It is the exit codes of an executable check suite. Eloquence earns nothing. |
| **Tautological tests** — the agent writes a check that cannot fail (`echo PASS`, assert-true, grep-for-its-own-output). | **Mutation testing** breaks the code deliberately and rejects each check that still passes. |
| **Compounding hallucination** — fake code *and* fake tests reinforce each other. | A **separate agent** writes the tests **before** the code, so the implementer cannot write its own bar. |
| **Evidence fabrication** — the agent cites files that it did not touch, or claims work that it did not do. | The pipeline checks Scout claims against the real diff. It binds audit citations to the files that are actually in the change set of this cycle. |
| **Narrative drift** — the summary says one thing; the ledger says another. | The pipeline auto-renders the key report sections from the ledger truth, not from model prose. Thus, the story cannot diverge from what occurred. |

The net effect: the model is free to *try* any of these. But it cannot make them ship.

### …and rectifies it

Detection is only half of it. When a cycle fails, a retrospective agent reads the wreckage and writes a structured root-cause lesson with a concrete prevention rule. This also applies when the framework catches *itself* at gaming. The pipeline verifies that the lesson exists on disk, and merges it into the long-term memory of the project. The planning of the next cycle reads it automatically. Over time, whole *classes* of gaming get structural fixes, not only one-off patches (see the incidents below).

Deep dive: [docs/concepts/trust-architecture.md](docs/concepts/trust-architecture.md) — the full threat model and the catalog of gaming patterns.

---

## Failures become lessons (memory that compounds)

At best, most agent loops have single-loop learning: detect an error, retry the action. Evolve Loop adds the second loop: detect the error, question the *assumption* that caused it, and change the policy so that the error cannot recur.

When a cycle fails, the retrospective writes a lesson such as this one:

> "When adding a new state field a phase depends on, grep every regression check that reads that phase's inputs and update the fixtures." — distilled from the cycle where exactly that omission broke the run.

That lesson is durable. The cycle that learned it shipped long ago, but the rule still fires in every future planning pass. Dozens of such lessons accumulate, and some are from 60+ cycles back and still relevant. Thus, the longer you run the system, the harder it is to fool.

This is a multi-agent version of the Reflexion pattern (Shinn et al., 2023), with one deliberate twist. The *verdict* that triggers reflection is never a model claim: it is the deterministic check suite. The auditor can write prose. It cannot fake the gate that decides if a lesson was necessary.

Deep dive: [docs/concepts/self-evolution.md](docs/concepts/self-evolution.md).

---

## Built to survive long runs

Long unattended cycles fail routinely: subscription quotas exhaust, APIs return errors, and models hit context limits. The contract is simple: **work in flight survives common failures.**

- The reasoning of a failed cycle becomes a lesson.
- A quota wall in the middle of a cycle checkpoints the worktree and the state, so a single `--resume` continues from where it stopped.
- Recoverable failures keep the edits of the builder and do not discard them.

The canonical incident that motivated this contract was 30 minutes of build work lost to a quota wall in the middle of an audit. Checkpoint-resume now prevents exactly this.

---

## How it compares

Most "autonomous" coding tools collapse to one model that does everything and grades its own homework:

```
   ┌─────────── same model (or same family) ───────────┐
USER →  write code  →  "does this look done?"  →  ship
   └────────────── one judgment, one blind spot ───────┘
```

Evolve Loop splits the roles and gives the *final* verdict to code:

```
USER → Scout → (model A writes failing tests) → (model B writes code)
                                                      │
                  (model C, different family, told to refute) audits
                                                      │
                       deterministic check suite — green or it doesn't ship
                                                      │
                            ship → a durable lesson is written
```

**Versus other long-running agent skills** (`/goal`, self-evolving/self-improving agents, skill frameworks):

- These skills usually gate on a small validator LLM or on exit criteria that a skill describes. They keep memory in the conversation or in a notes file. They run on one CLI, and they treat anti-gaming as a convention.
- The verdict of Evolve Loop is the exit codes of deterministic checks. Its memory is durable lesson files that feed back into planning. It routes across Claude/Codex/Antigravity/Ollama per phase. Its anti-gaming is structural, at three layers.

**Versus the famous code-writing agents** (Devin, OpenHands, SWE-agent, Aider): these agents optimize *how well the agent solves the task*. Usually, SWE-bench is their benchmark. Evolve Loop is on a different axis. It is a **trust-and-governance pipeline** that can *drive* those agents, and it adds the verification, learning, and recovery layer on top. It does not compete on raw coding ability. It decides if the code is safe to merge **unattended**.

Select by your single biggest constraint: human-in-the-loop control (Aider), hands-off autonomy (Devin/OpenHands), or **unattended trust** (Evolve Loop).

### The honest tradeoffs

Evolve Loop is **not** always the correct choice:

- **Higher friction.** A full phase spine per cycle takes ~15–30 minutes of wall-clock. Adversarial mode (two model families) costs more per cycle than a single-model loop. A plain `/goal` loop is faster and simpler.
- **Steeper learning curve.** The trust kernel, the check suite, the router, and the recovery model are concepts that you must learn.
- **Optimized for trust, not speed.** The fastest autonomous coding is a single-agent loop. The *safest* merge is this one.

Best fit: teams or solo developers that run long unattended cycles on production code, where a bad merge is expensive. Full comparison: [docs/comparisons/long-running-claude-skills.md](docs/comparisons/long-running-claude-skills.md).

---

## Proof: the framework caught its own bugs

The strongest evidence that the anti-gaming is real: it caught *itself* again and again.

- **The model-swap experiment.** A different model on Scout and the builder shipped a damaged cycle. It also exposed seven distinct bugs in the framework, and the audit and grounding checks of the framework caught several of them. In the next two cycles, all seven got structural fixes (grounding verification, audit-citation binding, ledger-truth report rendering).
- **A trust-kernel ordering breach.** A worktree commit briefly got to `main` before the post-audit gate was able to verify it. This exposed a short ordering window. Commit self-attestation and a pre-merge tree verification shipped within hours.
- **An autonomous goal divergence.** Triage chose to ship something that was not in the stated plan of the operator, and it was right. The stated item was already done, so the deviation saved a wasted cycle. (By design, the goal text of the operator is *input*, not an unquestionable directive.)

The value proposition was never "cycles never fail." It is that failures **produce durable lessons and structural fixes**. That is the most valuable kind of bug report. Case studies: [docs/incidents/](docs/incidents/).

---

## Learn more

The README is the surface. The depth is in `docs/`:

- **Concepts (teaching-first):** [overview](docs/concepts/overview.md) · [self-evolution](docs/concepts/self-evolution.md) · [trust architecture](docs/concepts/trust-architecture.md) · [error recovery](docs/concepts/error-recovery.md) · [pluggability](docs/concepts/pluggability.md)
- **Architecture (reference-first):** [docs/architecture/](docs/architecture/) — per-phase mechanics, the check-suite format, the routing kernel, the checkpoint-resume protocol.
- **Engineering chronicle (narratives):** [docs/chronicle/](docs/chronicle/README.md) — stories at the workstream level: what the project built, why, and what it taught.
- **Incidents (case studies):** [docs/incidents/](docs/incidents/) — the self-caught bugs, in forensic detail.
- **Decisions:** [docs/architecture/adr/](docs/architecture/adr/) — every architectural choice, with its context and its consequence.
- **See it visually:** the **[landing page](https://mickeyyaya.github.io/evolve-loop/)** shows the whole pitch (bottleneck → pillars → self-caught incident → quick start) as one scrollable page.

---

## Contributing

Contributions are welcome. Evolve Loop itself runs this project. Every commit on `main` is a reviewed manual ship of an operator, or an automated cycle ship with a full audit trail. Start with [CLAUDE.md](CLAUDE.md) (the runtime contract) and [AGENTS.md](AGENTS.md) (cross-CLI invariants).

If you find a gaming pattern that the framework did not catch, please file an issue. Give the cycle number, the relevant reports, and what you expected versus what shipped. Those are the most valuable reports that we get.

---

## Version

**Current (v22.27)** — the full release history is in [CHANGELOG.md](CHANGELOG.md). To cut a release, use `evolve release X.Y.Z`.

| Version | Date | Notes |
|---|---|---|
| v22.27 | Oct 8 | The `cli_routing` table runs agy first. This release adds the code-review phase, the convergence policy (both shadow), the computed inbox rank, `evolve checkpoint` and `evolve cli update`. |
| v22.26 | Sep 30 | `evolve gc` removes what finished cycles leave. The build handoff floor runs the repo-contract pack. This release also adds `evolve loop-stop`, `evolve inbox route-console` and the `NO_WORK` result. |
| v22.25 | Sep 28 | A byte-identical rebase carries its audited verdict to ship (ADR-0105). Protected control-plane items go to the console. This release adds the code-comment convention and `commentaudit`. |
| v22.24 | Sep 15 | Every fallback chain ends with every available CLI (ADR-0104). The Signal Center collects all signals (ADR-0101). Phase boundaries verify declared outputs (ADR-0100). |
| v22.23 | Sep 11 | Document cycles get a deliverable contract (ADR-0099). This release adds the Task Contract block and `evolve dashboard`. Repair rounds increase the tier and the effort (ADR-0096). |
| v22.22 | Sep 1 | This release adds the build-explanation deliverables. A zero `ContractVersion` cannot turn off the audit gate. The core, cmd and ship tests rejoin the lane gate. |
| v22.21 | Aug 31 | An audit FAIL gets up to two repair rounds in its cycle (ADR-0092). One policy table decides every retry (ADR-0093). The audit binding fails closed. |
| v22.20 | Aug 25 | Submit-verify results go to a durable ledger. The SELECT menu drops from 65 cards to 22. A judgment phase's FAIL no longer halts the loop. |
| v22.19 | Aug 18 | The tree-drift checks accept the inbox consumption of a PASS ship. The closure-claim gate has fewer false positives. This release adds a fresh-base collision guard. |
| v22.18 | Aug 15 | The PASS ship commit consumes its own inbox items. The retrospective moves to Claude at the deep tier. `/evo:setup` offers a live latest-model probe. |
| v22.17 | Aug 13 | Audit verdicts show each step from evidence to verdict (ADR-0087, ADR-0088). A salvage layer repairs recoverable verdicts. The loop checks the outputs of each phase. |
| v22.16 | Aug 12 | Three FAILs in a row halt the batch. `--push-only` recovers an attested commit that did not land. Deliverable contracts can name more than one artifact. |
| v22.15 | Aug 6 | Latest-model selection keeps to one lineage and writes in shadow first. New evals are tracked again. A documentation audit refreshes the README and the site. |
| v22.14 | Aug 5 | Ship-time **repo-contract scanner pack** (default enforce: lane ships can no longer red `main`). Boot-time **binary staleness self-heal** (`boot.binary_refresh=auto`). Engineering chronicle (17 narratives) + doc-root consolidation into `docs/` |
| v22.13 | Aug 4 | Composed-gate apicover check now enforces (six-recurrence warnship class); bounded retry on worktree provisioning; a failed publish demotes its own assetless tag listing; channel-e2e deflake |
| v22.12 | Jul 30 | Deep-tier artifact budgets (six missing_artifact deaths in one day) + contract-gate CLI escalation; verified-bytes single read. The e2e budget moved into the make recipe (the assetless-tag class), and a failed publish now demotes its own listing |
| v22.11 | Jul 30 | The FAIL-side count of tries reaches fleet lanes (ADR-0079 `cycleoutcome` seam). Red predicates diagnose themselves: full-stream evidence files + bounded-retry outcome |
| v22.10 | Jul 29 | **Runtime/console plane separation (ADR-0080)** — the loop owns its checkout; hub-resident console lease; verification single-flight |
| v22.9 | Jul 28 | Identical-fingerprint breaker requires a content-bearing identity — ends the false-halt class; duration tokens folded out of fingerprints |
| v22.8 | Jul 28 | Campaign `PARTITION` + fleet lane menus; persona-budget in-lane gate at the build floor |
| v22.7 | Jul 22 | Build handoff floor + graduated remediation (the discard-sound-work class); statemap CAS floor closes the cycle-999/1000/1001 shared-state disease |
| v22.6 | Jul 21 | Ship-manifest naming test + token-usage research — no runtime behaviour change |
| v22.5 | Jul 20 | Universal CLI fallback — route to any installed LLM when the configured chain is absent instead of halting; cross-lane mint registry (ADR-0073) |
| v22.4 | Jul 18 | Per-model quota-wall detection in `exhausted_regex`; goal-stall escalation; tier-fallback failover on exit-85 |
| v22.3 | Jul 17 | **System-failure policy (ADR-0072):** loop halts on forged verdicts instead of retrying; clean-exit verdict-authority fix; token-telemetry + clean-boot campaign (−39% context/cycle) |
| v22.2 | Jul 16 | Spine artifact floor armed; static/dynamic boundary-leak closures; inbox batch classifier; reconcile-on-quota-teardown |
| v22.1 | Jul 8 | Landing-page interactive labs + demos; llms.txt |
| v22.0 | Jul 5 | Self-healing SELF_SHA auto-repin + fleet 2-wide concurrency; "top" canonical model tier |
| v21.9 | Jul 3 | Quota-aware fleet budgeting wired into the live fleet wave |
| v21.8 | Jul 3 | Dynamic fleet budgeting (`fleet.budget` + native usage probe) |
| v21.7 | Jul 3 | Fleet wave guards |
| v21.6 | Jul 2 | Dynamic model routing — advisor `{cli,tier}` axis + clamp |
| v21.5 | Jun 30 | ACS predicate-quality safeguard preserved under context compaction |
| v21.4 | Jun 29 | Claude Code 2.1.195 plugin-schema fix; codex native install; agy skill trees; `/evo:` command namespacing |
| v21.3 | Jun 26 | claude-tmux boot-dialog dismissal + prompt-delivery fix |
| v21.2 | Jun 26 | Loop lease fencing + tmux session GC |
| v21.1 | Jun 24 | Prebuilt binaries for 13 Unix targets + install.sh OS/arch detection |
| v21.0 | Jun 24 | `/evo:` plugin namespace rename; removed the strict-audit gate dial |
| v20.4 | Jun 24 | Public OSS-mirror release automation |

---

## License & Links

- **License:** Apache-2.0 — see [LICENSE](LICENSE) (third-party notices in [NOTICE](NOTICE))
- **GitHub:** [github.com/mickeyyaya/evolve-loop](https://github.com/mickeyyaya/evolve-loop)
- **Install:** `curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh` — or `/plugin marketplace add mickeyyaya/evolve-loop` in Claude Code
- **Changelog:** [CHANGELOG.md](CHANGELOG.md)

The design uses ideas from these sources:

- Reflexion (Shinn et al., 2023);
- double-loop learning (Argyris & Schön, 1978);
- the reward-hacking literature (Skalse et al., 2022; Weng, 2024);
- the LLM-as-judge bias research that motivates multi-annotator and adversarial evaluation.

Full bibliography: [docs/architecture/phase-architecture-citations.md](docs/architecture/phase-architecture-citations.md).

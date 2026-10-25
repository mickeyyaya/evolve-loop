# Plan: the pipeline is the only controller, and each LLM agent runs in an isolated cell

- **Status:** proposed, 2026-10-25. The operator reviews this design set before any code.
- **Design:** [llm-isolation](../architecture/llm-isolation.md). **Decisions:** [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) and, for §11, [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md). **Research:** [llm-isolation-2026-10](../research/llm-isolation-2026-10.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)
11. [Part II: self-healing within boundaries](#11-part-ii-self-healing-within-boundaries)

## 1. Request

The operator, 2026-10-25:

> "the pipeline evo loop itself should the center of invoking all LLM cli by design... evo loop itself control all the interactions... There is no dependency between a LLM to control over another LLM directly."

> "the agent should be isolated and let the pipeline to handle the flow and communication. Isolation is the key... start with the full docs writing first."

> "Make sure everything is well documented and explained."

Later on the same day, the operator set the limit of read isolation:

> "phase agent can read the output / deliverables from previous phases and also when retry and failed recover cycle, those suggestions and improvement recommendations should be accessible when needed (not necessary to be loaded at the beginning)."

The request has three parts. They are an architecture review, a refactor that removes each LLM dependency with no break of the pipeline structure, and the full documents first. This plan covers the refactor. The research holds the review. The operator also wants the rule because it keeps the system clear of one vendor's agent that drives another vendor's agent.

## 2. Facts

The research ids are in [llm-isolation-2026-10](../research/llm-isolation-2026-10.md). F1.1 to F1.11 are the gaps G1 to G11.

| Fact | Research id |
|---|---|
| The phase order, the state machine and the typed phase contracts already obey the rule. | F1.0 |
| Each LLM child gets the full environment of the launcher, and a test pins this. | F1.1 |
| Seven launch paths start an LLM binary outside the bridge launch path. | F1.2 |
| A phase pane has no depth marker, so the refusals of nested dispatch do not fire. | F1.3 |
| An agent can read the raw logs of other agents and the runs of other cycles. | F1.4 |
| The Claude floor and about 15 flow defaults are Go constants. | F1.5 |
| The kernel guards exist only as Claude Code hooks. | F1.6 |
| Five flow-steering outputs are parsed from stdout spans or prose headers. | F1.7 |
| Output parsers and collectors are Go switches on the family name. | F1.8 |
| The instruction files are written for Claude Code, and `GEMINI.md` is stale. | F1.9 |
| The console runs builds and reviews as in-process Claude Code subagents. | F1.10 |
| No test proves that another family can fill a seat. | F1.11 |
| A strict action environment and a pure shell clear the inherited environment by default. | P2.1 |
| Vendors and OWASP advise least privilege and a sandbox for each coding agent. | P2.2 |
| An orchestrator that alone routes work and messages is the common multi-agent shape. | P2.3 |
| `sandbox-exec` and `bwrap` can deny the exec of named binaries and reads of paths. | P2.4 |
| The vendors each offer schema-checked structured output. | P2.5 |
| `AGENTS.md` is a cross-vendor instruction file. | P2.6 |

## 3. Goals and non-goals

Goals:

1. **G-A One controller.** The pipeline is the only launcher, router and carrier of messages.
2. **G-B Isolated cells.** Each agent runs in a cell with a hermetic environment, limited reads and no LLM exec.
3. **G-B2 Knowledge on demand.** A phase reads the deliverables of earlier phases. A retry reads the lessons of its lineage when it needs them.
4. **G-C Family-neutral flow.** No family name in flow logic. Floors and defaults are routing data.
5. **G-D Proven substitution.** A test runs a full cycle with each family as the only route.
6. **G-E Visible state.** Each invariant shows its stage and its violations in the Signal Center.

Non-goals:

- A change to the phase order, the transitions, the phase contracts or the gates.
- A change of today's routing values. The Claude floor stays, as data.
- A container runtime for each cell. Approach C is out of scope (§4).
- A new transport for ADR-0129 messages. The cycle broker already carries them through the pipeline.

## 4. Candidate designs

Five designs. C1 is a gate: a design that fails it is out, whatever its score. The other criteria have a weight. Each score is 1 (poor) to 5 (strong). The maximum total is 70.

| Criterion | Weight | Why it matters |
|---|---|---|
| C1 Obeys the rule: no LLM controls another LLM | gate | The operator's rule. |
| C2 Closes the gaps G1 to G11 | 3 | The audit found 11 gaps. A design that leaves some open is not done. |
| C3 Strength of the isolation | 2 | A prompt injection must not cross a cell. |
| C4 Keeps the phase order, the contracts and the gates | 3 | The operator asked for no break of the pipeline structure. |
| C5 Delivery risk and size | 2 | Each component lands in one PR, at a boundary, with the floor green. |
| C6 Runs on the macOS host of the loop | 2 | The loop runs on macOS today, with `sandbox-exec` (F1.4, P2.4). |
| C7 One rule for every family | 2 | A guarantee that holds only for one vendor is not neutral. |

The designs:

- **A. Harden in place.** An allowlisted environment, one launch API and OS sandbox denies. An artifact index with read grants, a floor in data, typed flow files and a substitution matrix.
- **B. Out-of-process phase workers.** Each phase runs as a subprocess over `pkg/phaseproto` (`ServeStdio` at `go/pkg/phaseproto/serve.go:16`, `SubprocessRunner` at `go/pkg/phaseproto/subprocess_runner.go:24`). The cell gets an input envelope and returns an output envelope.
- **C. A container for each cell.** `bwrap` on Linux or a container runtime. Each cell has its own file system view and network.
- **D. A privileged controller LLM.** One LLM directs the other agents and carries their messages.
- **E. Vendor-native sandboxes only.** Each CLI turns on its own sandbox and hooks. The pipeline adds nothing.

| Design | C1 | C2 ×3 | C3 ×2 | C4 ×3 | C5 ×2 | C6 ×2 | C7 ×2 | Total |
|---|---|---|---|---|---|---|---|---|
| A Harden in place | pass | 5 | 3 | 5 | 4 | 5 | 5 | **64** |
| B Phase workers over `phaseproto` | pass | 5 | 4 | 4 | 2 | 5 | 5 | 59 |
| C Container for each cell | pass | 3 | 5 | 4 | 1 | 2 | 5 | 47 |
| E Vendor-native sandboxes only | pass | 2 | 3 | 5 | 4 | 4 | 1 | 45 |
| D Privileged controller LLM | **fail** | 1 | 1 | 3 | 3 | 5 | 1 | 32 (out) |

The reasons for each score:

- **A** closes all 11 gaps with code that exists: `ipcenv.Scrub` (`go/internal/ipcenv/scrub.go:10`), the SBPL and `bwrap` generators, `cliroute`, `phasecontract` and `fakeclitest`. Its isolation is as strong as the OS sandbox, so C3 is 3. Each component is one PR.
- **B** gives a stronger seam, because a cell sees only its envelope. It moves every phase to a new runner at once, so C4 is 4 and C5 is 2. A is a step on the path to B: the artifact index of A becomes the envelope of B.
- **C** gives the strongest cell. Apple's `container` tool runs Linux containers in lightweight VMs, but only on macOS 26 with Apple Silicon (P2.4). The CLIs then run as Linux builds, with no Keychain login and no host tmux pane, so C6 is 2. It does not close the floor, the parsers, the typed outputs or the instruction files, so C2 is 3.
- **E** costs least. agy and codex panes get no hooks today (F1.6), and ollama has no sandbox of its own. Each vendor has a different rule, so C7 is 1.
- **D** fails the gate. It puts one LLM over the others, which is the dependency that the operator forbids.

**Why A wins.** A has the highest total. It is the only design that closes all gaps with no change to the phase structure. B stays the next step. When A is at `enforce`, the artifact index and the declared artifacts are the envelope that B needs.

**What changes our mind:** a supported host where the OS sandbox cannot deny reads or exec. On that host, B or C becomes the cell.

## 5. Decisions

| Id | Decision | Because (research id) |
|---|---|---|
| D1 | Choose design A. Keep B as the next step after A reaches `enforce`. | §4, F1.0 |
| D2 | Build the substitution matrix first. Each open gap is an expected failure row. | F1.11 |
| D3 | A child environment is an allowlist plus the manifest `env_passthrough` and `default_env`. | F1.1, P2.1 |
| D4 | One bridge launch API starts each LLM binary. An AST guard pins it. | F1.2 |
| D5 | Each phase pane carries a depth marker. The sandbox denies the exec of LLM binaries. | F1.3, P2.4 |
| D6 | The pipeline writes an index of readable artifacts (`inputs/index.json`): earlier deliverables and the item lineage. The sandbox grants exactly those paths. | F1.4, P2.3, operator refinement §1 |
| D6b | The agent opens an indexed artifact only when it needs it. Nothing is preloaded into the prompt. | operator refinement §1 |
| D7 | The family floor moves into `cli_routing.floors`, with today's Claude floor as its value. | F1.5 |
| D8 | The OS sandbox, the fence and the gates hold each guarantee. Hooks are defence in depth. | F1.6, P2.2 |
| D9 | Each flow-steering output is a declared JSON file that a `phasecontract` schema checks. | F1.7, P2.5 |
| D10 | Parsers, collectors and transcripts are manifest capabilities. Go keeps one parser for each format. | F1.8 |
| D11 | One neutral source renders the instruction file of each CLI. | F1.9, P2.6 |
| D12 | The console runs builds and reviews through `evolve agent run`. | F1.10 |
| D13 | Each invariant starts at `shadow` and emits `ISOLATION_I<n>_VIOLATION` through the Signal Center. | ADR-0153 pattern |

## 6. Components

Each component is one PR, with a red test first. The spec section is the acceptance source. Each component adds its guard rows to the X1 matrix.

| Id | Component | Closes | Invariants | Red test first | Depends on | Stage when it lands |
|---|---|---|---|---|---|---|
| X1 | Substitution harness | G11 | I-9 | The matrix row "agy is the only route" fails on the `cli:"claude"` pin. | none | `shadow`: open rows are expected failures |
| X2 | Hermetic driver environment | G1 | I-2 | `CLAUDECODE=1` on the parent reaches the child. | X1 | `shadow` one wave, then `enforce` |
| X3 | Single launcher | G2 | I-1 | The AST guard lists the launch paths outside the bridge launch API: 3 outside `internal/bridge`, and the others inside it. | X2 | `enforce` (a static guard) |
| X4 | No nested agency | G3 | I-3 | A phase pane runs `claude --version` with exit 0. | X2 | `shadow`, then `enforce` |
| X5 | Artifact index and read isolation | G4 | I-4, I-5 | A retry opens its predecessor's retrospective, and cannot open an unrelated cycle's report. Today both reads pass. | X1, X4 | `shadow` (log denials), then `enforce` |
| X6 | Floors and defaults as routing data | G5 | I-6 | A grep guard finds `claude-tmux` in flow code. | X1 | `enforce` with the same values |
| X7 | CLI-neutral guarantees | G6 | I-7 | An agy pane writes to a protected path with exit 0. | X4, X5 | `enforce` |
| X8 | Typed flow outputs | G7 | I-8 | The advisor parse accepts a JSON span in prose. | X1, X5 | `legacy` fallback, then `enforce` |
| X9 | Manifest parsers and collectors | G8 | I-6 | A new family manifest with no Go switch has no usage parser. | X3, X6 | `enforce` |
| X10 | Neutral instruction source | G9 | I-6, I-7 | A generated `GEMINI.md` differs from the checked-in file. | X1 | `enforce` (a generator check) |
| X11 | Console through the bridge | G10 | I-1, I-9 | A console review runs with no bridge launch record. | X3, X6, X10 | `enforce` for the console briefs |

## 7. Order

1. **X1** first. The matrix shows each gap as a red row. Each later PR turns its rows green, so progress is measured, not claimed.
2. **X2**, then **X3**. The single launch API uses the hermetic environment, so X2 comes first.
3. **X4** next. It needs the X2 environment to carry the depth marker.
4. **X5** and **X6** in parallel. They touch different packages (`sandbox`, `cliroute`).
5. **X7** after X4 and X5, because it adds write rules to the same sandbox profile.
6. **X8** after X5, because the typed outputs are files in the declared artifact set.
7. **X9** after X3 and X6, because the manifest is the single source of each capability.
8. **X10**, then **X11**. The console seat needs the neutral reviewer profiles.

The order puts the proof first and the visible changes to the agents last. One live wave runs after X2 to X4 land, before X5 starts.

## 8. Verification

| Goal | Measure | Before (2026-10-25) | After |
|---|---|---|---|
| G-A One controller | spawns of an LLM binary outside the bridge launch API (AST guard) | 7 paths (F1.2) | 0 |
| G-B Isolated cells | sandbox-contract rows for exec, network and cross-cycle reads | 0 rows (F1.11) | each row green |
| G-B Isolated cells | env-leak test: a parent variable reaches a seat | yes (F1.1) | no |
| G-B2 Knowledge on demand | a retry opens its predecessor's retrospective; an unrelated cycle's report is denied | both open (F1.4) | the first opens, the second is denied |
| G-C Family-neutral flow | family names in flow logic (grep guard) | about 15 sites (F1.5) | 0 outside routing data and manifests |
| G-D Proven substitution | matrix rows: a full scout-to-ship cycle with one family as the only route | 0 of 5 (F1.11) | 5 of 5; the ollama row covers only the seats that its capabilities allow |
| G-E Visible state | invariants with a guard test and a signal code | 0 of 9 | 9 of 9 |

Each component also runs the TDD floor, measured 100% coverage on changed functions, an independent review, the whole-tree guard packages and the composed full floor. The live check: one real wave after X2 to X4, read through `evolve signals tail`. The ship rate PQ1 must stay inside its interval, with no new system-fail codes.

## 9. Risks

| Risk | Control |
|---|---|
| An allowlist drops a variable that a CLI needs (for example, a keychain or proxy variable). | X2 starts in `shadow`: it logs each dropped name for one wave. The manifest `env_passthrough` adds the needed names. |
| A sandbox deny stops a legal phase action. | Each deny rule starts in `shadow` with a logged denial. A rule goes to `enforce` after a clean wave. |
| The index misses a file that a persona or a retry needs. | The phase contract declares the inputs, and the lineage rule is data. In `shadow`, each denied read is a signal, so a missed lesson shows up before `enforce`. |
| A wrong lineage link lets a retry read an unrelated cycle. | The lineage comes only from pipeline records (`lane-scope.json` todo ids and the continuation chain), never from agent text. A test pins an unrelated cycle as denied. |
| The floor move changes a routing value. | X6 asserts that `evolve cli-routing explain` prints the same chain for each agent before and after. |
| The typed output change breaks the advisor during a wave. | X8 keeps the stdout parser as `legacy` for one wave, and emits a signal on each fallback. |
| The work spans many waves and the plan goes stale. | Each component re-reads the spec and re-verifies its file:line citations before it starts. |

## 10. Open questions

1. Q1. Which `$HOME` paths are secret? The spec gives a starting list. The operator confirms it at X5.
2. Q2. Does agy honour an allowlisted environment with its keychain login? X2 `shadow` answers it.
3. Q3. Does the console seat move to another family by default, or stay Claude with the option? X11 keeps Claude and adds the option.
4. Q4. When does design B start? This plan proposes the first wave after X8 reaches `enforce`.

## 11. Part II: self-healing within boundaries

### 11.1 Request

The operator set the north star on 2026-10-25:

> "build the pipeline that auto agile loop which is driven by LLM agents, the pipeline defines the clear and clean boundary to make each agent and do it works within the given scope without gaming the system. Even error or accident happened, the loop itself can detect the issue and fix the pipeline issues with orchestrator agent analysis and actions to trigger phase agents to fix."

The research is in [§5 of the dossier](../research/llm-isolation-2026-10.md#5-self-healing-within-boundaries). The design is [Part II of the spec](../architecture/llm-isolation.md#17-self-healing-within-boundaries).

### 11.2 Candidate designs

The criteria are C1 to C7 of §4, with C2 read as "closes the self-healing goals": detect, diagnose, repair, verify, with no gaming. The maximum total is 70.

| Design | C1 | C2 ×3 | C3 ×2 | C4 ×3 | C5 ×2 | C6 ×2 | C7 ×2 | Total |
|---|---|---|---|---|---|---|---|---|
| H1 Agent proposes from a closed catalog, Go disposes | pass | 5 | 4 | 5 | 3 | 5 | 5 | **64** |
| H2 Deterministic playbooks only, no LLM diagnosis | pass | 2 | 5 | 5 | 4 | 5 | 5 | 59 |
| H3 The console repairs each pipeline defect (today) | pass | 2 | 5 | 5 | 5 | 5 | 1 | 53 |
| H4 An orchestrator agent with dispatch tools | **fail** | 4 | 1 | 3 | 3 | 5 | 3 | 45 (out) |

- **H1** reuses the advisor clamp and the `failure-decision.json` enums (F3.1, F3.2). Its isolation is 4 because the agent proposes and cannot act. The delivery is five components, so C5 is 3.
- **H2** cannot diagnose a new failure class, so C2 is 2. It is the fallback: the detector and the catalog of H1 work with no agent, and a `low` diagnosis escalates.
- **H3** is safe and slow. Each repair waits for a person, and only a Claude Code session can hold the seat today (F1.10), so C7 is 1.
- **H4** fails the gate. It is an LLM that controls LLMs.

**Why H1 wins.** It is the only design that meets the north star and keeps ADR-0154. **What changes our mind:** a false-diagnosis rate above 30% in `shadow` for two windows. Then T1 stays with H3 and H2.

### 11.3 Decisions

| Id | Decision | Because |
|---|---|---|
| D14 | The orchestrator agent writes `diagnosis.json`. It has no dispatch tool. | F3.2, P3.4, ADR-0154 I-1, I-3 |
| D15 | The action catalog is closed Go with preconditions, tier limits and budgets. It extends the `failure-decision.json` enums. | F3.1, P3.2 |
| D16 | The constitution is T0. Only the operator changes it. A diff classifier flags a weaker gate. | F3.3, F3.5, P3.3 |
| D17 | Checks freeze at handoff. A T1 judge is from another family. No index is an agent target. | F3.6, P3.3, ADR-0153 rule 8 |
| D18 | Budgets for each window, breakers for each incident class, and escalation after two failed repairs. | P3.2, ADR-0126 |
| D19 | An incident closes on four conditions: a green reproduction, a clear signal, no index regression, a retrospective. | P3.5, ADR-0151, ADR-0153 |
| D20 | Self-healing starts at `shadow`. Each tier flips on measured evidence and an operator change. | F3.5, P3.5 |

### 11.4 Invariants

| Id | Invariant | Guard | Signal |
|---|---|---|---|
| I-10 | Constitution: no agent changes a kernel surface or weakens a gate. | the gate classifier and the ADR-0064 P2 check | `HEALING_I10_VIOLATION` |
| I-11 | No agent writes its own oracle. | the TDD digest; a T1 judge from another family | `HEALING_I11_VIOLATION` |
| I-12 | The orchestrator agent proposes; the pipeline disposes. | the catalog validator; no dispatch without an accepted proposal | `HEALING_I12_VIOLATION` |
| I-13 | Bounded autonomy. | budgets, breakers, converge-or-escalate | `HEALING_I13_VIOLATION` |
| I-14 | An incident closes only with evidence. | the four close conditions | `HEALING_I14_VIOLATION` |

### 11.5 Components

| Id | Component | Invariants | Red test first | Depends on | Stage when it lands |
|---|---|---|---|---|---|
| X12 | Incident detector and incident records | I-14 | A red `main` and a `QUALITY_PQ1_BREACH` open no incident record. | X1, X5 | `shadow` |
| X13 | Orchestrator-agent profile, evidence index, diagnosis schema | I-12 | A diagnosis with an action outside the catalog passes the parse. | X5, X8, X12 | `shadow` |
| X14 | Action catalog, validator, budgets and breakers in Go | I-12, I-13 | A fourth T1 repair in one window runs. | X12, X13 | `shadow` |
| X15 | Surface tiers and the gate classifier | I-10, I-11 | A cycle diff that adds `t.Skip` to a guard test ships. | X5 | `enforce` for the classifier |
| X16 | Repair-cycle path: the loop builds T1 repairs, and T0 goes to the operator | I-11, I-14 | A T0 `pipeline-repair` item reaches a loop lane. | X14, X15 | `shadow`, then `enforce` for each tier |

### 11.6 Order

X12 to X16 start after X1 to X5, because isolation comes first. X13 also needs X8, because the diagnosis is a typed flow output. The orchestrator agent must run in a cell with an index before it reads any evidence. The order is X12, then X13 and X15 in parallel, then X14, then X16.

### 11.7 Verification

| Goal | Measure | Before (2026-10-25) | After |
|---|---|---|---|
| Detection | incidents that the detector opens for a red `main`, a breach or a stall | no incident record | one record for each |
| No gaming | classifier cases for each weakening pattern of the spec §20.3 | 0 | 7 of 7 |
| Bounded | a repair past a budget or an open breaker | not measured | 0 |
| Repair quality | repair success rate and false-diagnosis rate, as Wilson intervals | no data | recorded in `shadow` |
| Time to green | OQ4 (ADR-0153) | no data (ADR-0153 component Q4) | does not regress at the T1 flip |

### 11.8 Risks

| Risk | Control |
|---|---|
| A wrong diagnosis starts a useless repair. | Budgets and breakers. The verify step reopens the incident. Two failures escalate. |
| The classifier misses a new form of weakening. | Each escaped form becomes a new classifier case. A weakened gate on `main` sends T1 back to `shadow`. |
| A repair cycle repairs the code that runs it. | T1 repairs run in a lane worktree and land at a boundary, like any change. T0 never runs in the loop. |
| The orchestrator agent games its false-diagnosis rate. | The rate comes from verify outcomes that the pipeline writes. The agent never sees it. |

### 11.9 Open questions

1. Q5. Which family holds the orchestrator seat? The proposal: the deep chain of today's advisor, as routing data.
2. Q6. The flip thresholds of the spec §20.2 have no baseline. The operator confirms them after two `shadow` windows.

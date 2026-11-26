# Plan: the persona diet by usage order

- **Status:** proposed, 2026-11-25.
- **Extends:** [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) and [model-capability-practice-2026-10.md](model-capability-practice-2026-10.md). It does not replace them.
- **Research:** [persona-prompt-standards-2026-11.md](../research/persona-prompt-standards-2026-11.md).
- **Convention:** [persona-writing.md](../conventions/persona-writing.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate approaches](#4-candidate-approaches)
5. [Decisions](#5-decisions)
6. [Order by usage](#6-order-by-usage)
7. [Separate components](#7-separate-components)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The operator asked the loop to improve every LLM prompt in the pipeline to current standards, and to learn from the agy CLI. The operator wants the documents first and the persona changes after them.

## 2. Facts

| Fact | Source |
|---|---|
| 113 files are in `agents/`. | research F1.1 |
| Six phases hold nearly all launches in 200 cycles: scout 200, router 200, triage 199, tdd 172, build 169, audit 152. | research F1.2 |
| The loud-capital counts are tdd 16, builder 18 and auditor 14. | research F1.3 |
| Both vendors warn that old scaffolding hurts new models. | research P2.9 |
| Two sites ask for a reasoning field. | convention section 3 |
| The agy driver message is stale. | research F1.6 |
| `AGENTS.md` is 53,657 bytes and agy cuts a rule file at 24,000 bytes. | research F1.5, P2.28 |
| ADR-0133 decision 4 asks for one persona per change, emphatic capitals first. Decision 5 asks for a before and after table for each change. | ADR-0133 |

**Where this plan goes further than ADR-0133.** Decision 4 trims one persona per change, capitals first. This plan widens it in two ways. First, each core change applies the full shape of the convention, not only the capitals. The shape has seven parts, a closing checklist and a reference-file split. Second, personas with no launch in 200 cycles change in groups (D5).

The reasons are these. The research shows that the structure affects both families, not only the capitals (P2.9, P2.12). A persona with no launches gives no reading to wait for.

## 3. Goals and non-goals

**Goals:**

- G1: each persona follows the standard, with a checked output contract.
- G2: no quality measure gets worse: ship rate, retries, contract blocks, tokens.
- G3: a lint holds the standard after the diet.
- G4: the core phases change first, because they carry the cost.

**Non-goals:**

- a change of the model or the effort of a route;
- a removal of a guard;
- a new persona.

## 4. Candidate approaches

Each criterion has a score from 1 (bad) to 5 (good). Risk is scored so that a safe approach scores high.

The lint (C4) is a separate component that any approach can add. So each approach below gets the lint, and the G3 column scores the same for all three. The approaches differ only in how they group the persona changes.

| Approach | What it does | G1 coverage | G2 quality held | G3 stays held | Cost to run | Safety | Total |
|---|---|---|---|---|---|---|---|
| A. One measured persona per change, in usage order (ADR-0133) | Change one persona. Wait for 10 cycles. Compare. | 2 | 5 | 5 | 1 | 5 | 18 |
| B. One big rewrite of all 113 files | Rewrite each file to the standard in one change. | 5 | 1 | 5 | 4 | 1 | 16 |
| C. Hybrid | Measured single changes for the core phases. Grouped changes for personas with no launch in 200 cycles. | 4 | 4 | 5 | 3 | 4 | 20 |

**Why A scores low on coverage and cost.** The core phases need about 10 cycles each for a reading. A chain of 14 persona changes takes a long time. About 100 personas have 8 launches or fewer in 200 cycles, and a reading takes much longer.

**Why B scores low on quality.** No measure backs a rewrite of 113 files. ADR-0133 rejected a big cut for this reason. Three personas are console-only and carry contract text (see section 6).

**Why C wins.** With the lint scored equal, C still wins on coverage and cost. The core phases carry the cost and have enough launches for a reading in 10 cycles. A persona with no launches has no reading to wait for, so a group change costs little there.

**Recommendation: C.**

**What changes our mind.** If a core change lowers the ship rate, stop the group changes and go back to A for all personas. If the lint blocks too many correct personas, move its rules from the gate to a warning.

## 5. Decisions

| Id | Decision | Because |
|---|---|---|
| D1 | Take approach C. | Section 4 |
| D2 | P1 ships the convention only. The lint (C4) starts as a warning after P3, the first two persona changes. It becomes a gate after P7. | A lint with no worked example is guesswork, and a gate needs the core personas to pass it first. |
| D3 | A core phase changes one persona per change, with the reference file of the persona in the same change. | The persona and its reference file hold one contract. |
| D4 | Each core change records the before and after table of ADR-0133: ship rate, retries, contract blocks and tokens. | ADR-0133 decision 5 |
| D5 | A persona with no launch in 200 cycles changes in a group of up to 10, by the same mechanical rules. | The group has no reading to protect. |
| D6 | A pin test changes in the same change as the text that it pins, with the reason in the commit. | Section 6 |
| D7 | The four components in section 7 are separate inbox items. They do not wait for the diet. | They have separate causes and tests. |
| D8 | The first reading is taken from the cycles before the change, taken from the ledger. The second reading needs 10 cycles after it. | ADR-0133 consequences |

## 6. Order by usage

Launch counts come from `.evolve/evidence/usage.txt`. The files come from `ls agents`. The class of each persona comes from `go/internal/guards/integrity_surface.go:178-183`.

**Integrity surface.** The builder, builder reference, auditor, auditor reference, retrospective and memo personas are console-only (`InstructionGuarantee` or `AuditVerdictGuarantee`). A cycle cannot edit them. The console makes each change to them. The retrospective reference is not on that list in the lines read here. Read the whole list before the change (**UNVERIFIED** for the other files).

**Pins to keep.** Each step runs these before it commits:

- the persona tests in `go/internal/prompts` (`router_persona_test.go` caps the prose region of the router at 3,000 bytes);
- the other persona text tests of `go/internal/prompts`: `persona_house_rules_test.go`, `persona_stopcriterion_dedupe_test.go`, `persona_verdict_unfenced_test.go`, `persona_handoff_ledger_test.go`, `orchestrator_persona_test.go` and `reflector_compaction_test.go`;
- the persona contract test in `go/internal/explanationdocs/persona_contract_test.go`;
- `go/internal/router/header_persona_pin_test.go`, which needs the header lines in `evolve-scout.md`, `evolve-scout-reference.md` and `evolve-triage.md`;
- `go/internal/phasecoherence` (`persona_frontmatter_ledger_open_test.go`, `persona_strip_operational_test.go`);
- the acs predicates that read persona text. Find them with `grep -rl 'agents/evolve' go/acs`. The list includes `go/acs/cycle1452`, `cycle1633`, `cycle2044`, `cycle2536` and `cycle2538`.

The test command is `go test -count=1 ./internal/prompts/... ./internal/explanationdocs/... ./internal/router/... ./internal/phasecoherence/...` from `go/`, plus the acs packages of the step.

**Readings to take.** For each step: ship rate, retries, contract blocks and tokens for the 10 cycles before and the 10 cycles after. Add the loud-capital count and the line count of the persona.

| Rank | Phase | Launches | Persona files | Console-only | Step notes |
|---|---|---|---|---|---|
| 1 | scout | 200 | `evolve-scout.md`, `evolve-scout-reference.md`, `evolve-scout-scan.md` | No | The router header pin needs the header lines. |
| 2 | router | 200 | `evolve-router.md` | No | The prose region has a 3,000-byte test cap and the file needs a `<!-- TSC applied` marker. The generated recipe table is not prose. The persona has no loud capital in the metrics. |
| 3 | triage | 199 | `evolve-triage.md`, `evolve-triage-reference.md` | No | 26,317 bytes. The header pin needs two header lines. |
| 4 | tdd | 172 | `evolve-tdd-engineer.md`, `evolve-tdd-engineer-reference.md` | No | 34,054 bytes. The largest persona, with 16 loud capitals. |
| 5 | build | 169 | `evolve-builder.md`, `evolve-builder-reference.md` | Yes | 26,906 bytes, 18 loud capitals. The explanation contract stays. |
| 6 | audit | 152 | `evolve-auditor.md`, `evolve-auditor-reference.md` | Yes | The verdict contract stays. Apply the reviewer rule of the convention. |
| 7 | retro | 36 | `evolve-retrospective.md`, `evolve-retrospective-reference.md`, `evolve-reflector.md` | Yes (main file) | The reflector has a compaction test. |
| 8 | fault-localization | 30 | `evolve-fault-localization.md` | No | |
| 9 | memo | 20 | `evolve-memo.md`, `evolve-memo-reference.md` | Yes | |
| 10 | bug-reproduction | 10 | `evolve-bug-reproduction.md` | No | |
| 11 | rest | 8 or fewer each | test-amplification, spec-verify, behavior-baseline, behavior-compare, code-review, architecture-design, docs-review and the other files | No | Group change (D5). The reviewer files `evolve-diff-*.md` render into `.claude/agents/`, so run `evolve skills generate` after a change. |

**Steps in order.** Each step is one change, apart from P12, which is a series of group changes.

1. P1: ship the convention (this document set) only.
2. P2 to P7: change the personas of ranks 1 to 6, one by one. Read after 10 cycles. Start the next step only when the reading of the last step shows no loss. The lint (C4) starts as a warning after P3 and becomes a gate after P7 (D2).
3. P8 to P11: ranks 7 to 10, by the same rule.
4. P12: group the rest by family of phase, up to 10 files each.
5. P13: set the line budgets and the lint to the new sizes.

**Two steps can run at once** only when their pins do not overlap and their readings do not share cycles. The scout and the router steps are a likely pair.

## 7. Separate components

Each component becomes one inbox item at the next boundary. This plan does not file them.

| Id | Component | Red test first | Files | Notes |
|---|---|---|---|---|
| C1 | The debugger reasoning field | A parser test that reads a decision file with the new field and the old field | `go/internal/phases/debugger/debugger.go:40`, `agents/evolve-debugger.md:62`, `skills/loop/phase3-build.md:222` | The parser reads `json:"reasoning"` at line 40, and line 39 already holds `json:"root_cause"`. Rename `reasoning` to `rationale`, keep one release of read support for the old name, and change the persona. In the build phase text, rename the failed-approach field `reasoning` to `root_cause`. A persona-only edit breaks the contract. |
| C2 | The stale agy driver message | A driver test that checks the message and the launch flags | `go/internal/bridge/driver_agy.go:31-34`, `:41` | The message says agy has no `-m` flag. agy 1.5.17 has `--model`, `--effort` and `--json-schema`. The headless path passes only `-p` and the permission bypass. Decide if the driver passes the model and the effort from the route. Use data in the route, not a flag in code. |
| C3 | The size of `AGENTS.md` for agy | A test that checks the byte size of each rule file that agy loads | `AGENTS.md` (53,657 bytes), `GEMINI.md` (8,484 bytes) | agy cuts a rule file at 24,000 bytes. First find out if `agy -p` loads the file in a worktree (**UNVERIFIED**). Then load the digest and keep the full rules in skills. `AGENTS.md` is the source of the rendered copies, so change it and run `evolve skills generate`. |
| C4 | A persona-standard lint | A failing fixture for each of L1 to L7 | A new check in the docs or commit gate; `agents/` | Build the rules of the convention section 9, and fix the one allowed list of tag names for L7. Start as a warning after P3. Make it a gate after P7 (D2). |

## 8. Verification

| Goal | Measure | Target |
|---|---|---|
| G1 | Count of personas that pass the lint L1 to L7 | All 113 at the end of P13 |
| G2 | Ship rate, retries, contract blocks and tokens, 10 cycles before and after each core change | No measure worse. The token count falls. |
| G3 | The lint runs in the commit gate | A fixture of each rule fails |
| G4 | The order of the changes | Rank 1 to 6 first |

The before and after table has this shape:

| Persona | Cycles | Ship rate | Retries | Contract blocks | Tokens | Loud capitals | Lines |
|---|---|---|---|---|---|---|---|
| `evolve-scout.md` before | 10 | | | | | | |
| `evolve-scout.md` after | 10 | | | | | | |

The cells are empty on purpose. Each step fills them from a command that prints the numbers.

## 9. Risks

| Risk | Control |
|---|---|
| A pin test fails after a text change | Run the pins of section 6 before the commit. Change the pin with the text (D6). |
| A diet change hides a contract the model needs | Keep the output contract and the guard names. Compare the contract blocks in the reading. |
| A console-only persona changes in a cycle | The integrity surface blocks the edit. The console makes the change. |
| An agy model drops a limit in a shorter persona | The closing checklist repeats the limits (convention section 1). |
| The 10-cycle reading is too noisy | Read 20 cycles for a phase with a low ship rate. Report the noise in the table. |
| The lint blocks a correct persona | Start as a warning (C4). |
| Rendered copies drift from their source | Run `evolve skills generate` and `evolve skills check` after a change to a reviewer persona. |

## 10. Open questions

1. Does `agy -p` load `AGENTS.md` and `GEMINI.md` from a worktree? The answer sets the priority of C3.
2. Does the check paragraph of research E1a cause over-verification on Opus 5.5? Take a reading in P5.
3. Which retrospective files are console-only? Read `integrity_surface.go` in full before P8.
4. Does the router header pin cover the router persona? The pin reads the scout and the triage files only.

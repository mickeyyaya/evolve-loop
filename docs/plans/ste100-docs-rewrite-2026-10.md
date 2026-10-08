# Plan: rewrite all documentation in Simplified Technical English (ASD-STE100)

- **Status:** in progress, opened 2026-10-07.
- **Standard:** [ste100-writing.md](../conventions/ste100-writing.md).
- **Operator request (2026-10-07):** "improve the readability by strictly follow ASD-STE100 policy and rewrite all docs wording by strictly follow ASD-STE100 format".
- **Operator decisions (2026-10-07):**

| # | Question | Decision |
|---|---|---|
| T-D1 | Which documents? | All of `docs/` and the root files: `README.md`, `CLAUDE.md`, `AGENTS.md` and `CHANGELOG.md`. Agent personas (`agents/`) and skills (`skills/`) are not in scope. |
| T-D2 | How does the repository keep new documents in STE? | A deterministic lint reports a WARN. It does not block. |
| T-D3 | Does STE apply to more than documents? | Yes. The operator said on 2026-10-07: "all the logs, generated text, any written docs should also strictly follow ASD-STE100 format". STE also applies to log and error text in Go code. It also applies to generated text: generator templates, phase-agent reports, inbox items, dossiers, release notes, CHANGELOG entries, commit messages and pull request bodies. It applies to every document that anyone writes. Machine-read parts keep their exact form. |
| T-D4 | Which agents do the bulk rewrite (S2–S8)? | agy writes and Claude checks. Gemini 3.8 Flash High rewrites each batch through the bridge. A Claude agent checks the meaning of each batch. A small agy pilot comes first and compares its quality with the S1 pilot. |

## 1. Why

The documents are hard to read. Many sentences have more than 40 words, many paragraphs have more than six sentences, and one idea often has two or three names. Agents read these documents as instructions. A long sentence with two meanings gives two interpretations.

## 2. Scope

The measured inventory, on main at 3de30ec3a:

| Area | Files | Words | Class |
|---|---|---|---|
| `docs/architecture` (design, ADRs, packages, decomposition) | 334 | 818,000 | rewrite |
| `docs/research` and `docs/private/research` | 186 | 301,000 | rewrite |
| `docs/incidents` | 76 | 100,000 | rewrite |
| `docs/reports` | 35 | 87,000 | rewrite |
| `docs/operations` | 26 | 69,000 | rewrite |
| `docs/plans` | 11 | 43,000 | rewrite |
| `docs/chronicle`, `docs/superpowers`, `docs/adr`, and the small directories | 54 | 66,000 | rewrite |
| `docs/*.md` | 12 | 12,000 | rewrite |
| `README.md`, `CLAUDE.md`, `AGENTS.md` | 3 | 9,400 | rewrite |
| `CHANGELOG.md` | 1 | 146,000 | rewrite, last |
| `docs/history/code-comments` | 559 | 639,000 | record (quotations) |
| `docs/explain/builds` | 102 | 74,000 | record (quotations) |

A **record** keeps its text. A code-comment archive quotes removed comments, and a build explanation quotes a cycle's output. The standard keeps quotations exactly as they are. Only the framing prose of a record (a header, an index line) follows STE.

About 1.7 million words need a rewrite.

## 3. Strategy

1. **Standard first (S0).** The house rules, this plan and the lint land as one docs-and-tool change. A rewrite without a measurable target cannot be verified.
2. **Lint, not a prompt, for the measurable rules.** `evolve docs ste-lint` checks sentence length, paragraph length and the house list of words. A rewrite batch must end with zero lint findings outside quotations. The lint is WARN everywhere else.
3. **Meaning is the hard part.** Each batch gets an independent check that every fact, number, path, link and decision is still present. The check follows the convergence policy (ADR-0126): one full check, at most one fix round, then a verify-only check.
4. **Tests pin documentation text.** Many tests and ACS predicates read documents. Every batch runs the test floor before it lands.
5. **No conflicts with open work.** A batch skips a file that an open pull request or a lane owns. The batch takes that file after the owner merges.
6. **Land at wave boundaries.** Each batch is a docs pull request in the next boundary train.

## 4. Batches

| # | Batch | Words (about) | Notes |
|---|---|---|---|
| S0 | The standard, this plan, `evolve docs ste-lint` and its build-floor WARN | — | the lint parses the standard's word table: one home |
| S1 | Pilot: the root files (not `CHANGELOG.md`), `docs/conventions`, `docs/*.md`, `docs/operations` (not `runtime-reference.md`), `docs/getting-started`, `docs/guides`, `docs/concepts`, `docs/reference`, `docs/testing`, `docs/comparisons` | 95,000 | the operator checks a sample before S2 |
| S2 | `docs/operations/runtime-reference.md`, `docs/plans` | 60,000 | after the open lanes that own `runtime-reference.md` merge |
| S3 | `docs/architecture/*.md` (design documents) | 210,000 | 4 batches |
| S4 | `docs/architecture/adr` | 152,000 | 3 batches. An ADR's decision text keeps its meaning exactly. |
| S5 | `docs/architecture/packages` and `docs/architecture/decomposition` | 457,000 | 8 batches |
| S6 | `docs/research`, `docs/private/research` | 301,000 | 6 batches |
| S7 | `docs/incidents`, `docs/reports`, `docs/chronicle`, `docs/superpowers`, `docs/adr`, the framing prose of the records | 230,000 | 5 batches |
| S8 | `CHANGELOG.md` | 146,000 | 3 batches, last, with a freeze at a boundary: every open landing adds an entry at the top |
| S9 | Phase-agent output: one shared STE prompt overlay for every phase that writes text | — | one home (the skill-overlay mechanism). The persona texts do not change (T-D1). The audit does not fail a cycle for style. |
| S10 | Log and error text in Go: `ste-lint --go` checks the string literals of log, error and signal calls; then a rewrite by package | — | tests, golden files and acked fingerprints pin some messages. Each batch updates them in the same change. `key=value` fields and codes stay exactly. |
| S11 | Generator templates: the generated documents and generated text | — | the generator changes, then the output is generated again |
| S12 | Written text from the console and the lanes: briefs, commit messages, pull request bodies, inbox items, memory files | — | in effect at once, 2026-10-07 |

At most four agents work at the same time. A generated file changes at its generator: for example, `evolve signals codes generate` writes `docs/architecture/signal-codes.md`.

## 5. Done for one batch

1. Every file in the batch has zero `ste-lint --strict` findings. The batch report lists each accepted exception: a heading that something reads, and a generated row.
2. Every relative link resolves.
3. The meaning check finds no lost fact, number, path, link or decision.
4. `go test -count=1 ./...` and `make test-acs-durable` pass.
5. The batch lands as a docs pull request.

## 6. Risks

| Risk | Control |
|---|---|
| A rewrite changes a meaning | The independent meaning check, and the rule "keep the meaning and split the sentence" |
| A test pins old wording | The test floor runs on every batch. The batch changes the test only when the test pins prose, not behaviour, and says so. |
| A rewrite conflicts with an open lane | The batch skips files that open work owns |
| The cost is high | The pilot measures words per batch and cost. The operator decides the bulk dispatch (Claude lanes, or agy through the bridge) with the pilot data. |

## 7. Status

| # | Status |
|---|---|
| S0 | ◐ the lint and the build-floor WARN passed review (J_0, then a verify-only check: MERGE); the landing is next |
| S1 | ◐ 21 files are rewritten and checked; the docs pull request is next |
| S2–S11 | ☐ not started |
| S12 | ☑ in effect from 2026-10-07 |

## 8. The S1 pilot data

- **Scope:** 21 files. The words went from 33,440 to 36,557 (+9.3%).
- **Lint:** 306 findings before, 12 after. The 12 are in README rows that `versionbump` writes, and S11 changes that generator.
- **Meaning:** an independent check found 23 findings in 8 files and 5 findings in 13 files. All were fixed. No fact, number, path or link was lost. The findings were of three kinds:
  - an actor that the source does not name;
  - a changed force;
  - an open list that became closed.
- **Tests:** `go test -count=1 ./...` and `make test-acs-durable` passed. No test changed.
- **Cost:** 31.5 minutes and about 1 million tokens. That is about 33,000 source words for each million tokens, including the meaning check.
- **Lessons:**
  - Keep a heading that a test, a persona or a predicate cites.
  - Keep the `| vX.Y |` prefix of the README version rows.
  - Keep the force of a rule: "may not" becomes "must not", and "may" becomes "can".
  - A lane must not start its own sub-agents.

## 9. Open item: how agy runs the bulk batches (T-D4)

The operator chose "agy writes, Claude checks" (T-D4). Two facts block a direct path today:

1. The console cannot start a bridge dispatch outside a cycle. `evolve subagent run` needs a cycle number and writes the ledger, and the house rule forbids a made-up cycle number.
2. A loop cycle does not fit as it is:
   - `deliverable_kind=document` has a fixed shape: `solutions/<slug>/` with candidate options (ADR-0099). A rewrite of existing documents does not have that shape.
   - The advisor chooses the build tier. An inbox item cannot ask for the balanced tier, so a deep build routes to Claude.

The design must choose one home for "a documentation rewrite runs on agy". The candidates:

| Option | Change |
|---|---|
| A | A new deliverable kind, `doc-rewrite`, with its own contract. A registry condition sets the build tier to balanced for it. |
| B | A user phase `doc-rewrite` with a profile on `agy-tmux` that can write only `docs/**`, chosen by the advisor for these items |
| C | A `cli_routing` rule for the work role of a documentation rewrite |

Until the design lands, the batches wait. The S1 pilot used Claude lanes.

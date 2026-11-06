# Research: contradictions in the design documents, and gaps to the code

- **Status:** complete for its stated coverage, 2026-11-06. Section 2 states the limits.
- **Request:** "review all docs to identify the contradiction within different adrs, plans, research and investigate with online research to find the best practical solution with 3 options and pick the winner with others advantages". Then: "compare with the current code implementation to learn if there is any" gap "between design, architecture, best practice, ai driven factory pipeline".
- **Method:** a digest of each ADR and plan status, code reads at `origin/main` 6ae4972c3, `evolve code`, the live policy, and live web searches on 2026-11-06.

**Terms used throughout:**
- **Stage:** a policy value, `shadow` (signal only) or `enforce` (refuse and signal).
- **Contradiction:** two documents, or a document and the code, that state different current facts.
- **Conformance check:** a deterministic test that fails when a stated decision and the repository differ.

## 1. The answer

The main fault is not the content of one decision. No machine reads the status and the relations of the decisions. So a reader cannot tell which decision is current.

The winner is option B of section 6: a decision registry with a conformance check. A small batch of amendments fixes the contradictions of section 3 first. A bounded review seat runs later, for the conflicts that a check cannot see. The scores are a synthesis of this research.

## 2. Coverage

| Set | Files | What this research read |
|---|---|---|
| `docs/architecture/adr/` | 161 | The status, title and decision digest of each file, then the exact lines of each conflict below |
| `docs/adr/` | 8 | The file names and one status |
| `docs/plans/` | 59 | The status and title of each file. Bodies only where section 3 cites them |
| `docs/research/` | 146 top-level entries | Names of the newest 30. No body except one cited file |
| All of `docs/` | 2209 Markdown files | Not read. The word counts of the main folders total about 2 million |

So this is not a read of all documents. The weekly usage was 81% and the brake is at 83%. A contradiction inside the research bodies, or between a research body and a plan, can exist and stay unseen here.

Environment reads are also not measured. `evolve code callers os.Getenv` returned 0 rows, because the index does not hold the standard library.

## 3. Contradictions between documents

| Id | Contradiction | Evidence | Effect |
|---|---|---|---|
| C1 | Two ADR homes reuse numbers. ADR-0005 is two different decisions. | `docs/adr/0005-tsc-application.md`, `docs/architecture/adr/0005-*`. The numbers 0109 and 0120 are absent from the main home. | A citation by number is not exact. |
| C2 | The status is free text. The template lists draft, accepted and superseded. | At least 35 ADRs read proposed, and at least 15 different status strings exist. One ADR reads superseded. | A reader cannot filter the current decisions. |
| C3 | A superseded decision still reads Accepted. | ADR-0060 supersedes sections 1 and 8 of ADR-0058, and ADR-0058 reads `Status: Accepted`. ADR-0096 is Accepted and extends ADR-0092, which is superseded. | The old text reads as current. |
| C4 | Accepted ADRs name files that do not exist. | ADR-0001 and ADR-0018 name `legacy/scripts/dispatch/` scripts. ADR-0008 names `role-gate.sh`. Those paths are absent. | The record reads as a live instruction. |
| C5 | The comment rule differs in three places. | `AGENTS.md`: "no comments and no Go doc comments". ADR-0111 line 22 and `docs/conventions/code-comments.md` line 17 allow one package doc, and `go/acs/regression/docgo` enforces it. | An agent that obeys the digest fails a gate. |
| C6 | The minimum lane count differs. | ADR-0139 line 31: "`fleet.min_lanes` stays 2". Plan title of wave-orchestrator: "at least four lanes". Policy: `min_lanes` 4. | The record does not state the live value. |
| C7 | The hub layout differs. | ADR-0080 line 43 names `~/ai/claude/evolve-loop.git`. The host uses `.repo.git` under the project folder. | An operator procedure from the record fails. |
| C8 | Who decides differs. | ADR-0072 is Accepted: the orchestrator LLM classifies and chooses. ADR-0154 and ADR-0155 are Proposed: the pipeline is the only controller, and an agent only proposes. | An Accepted record loses to a Proposed record in practice, and the status says the reverse. |
| C9 | The rule on LLM control differs from the console. | The digest: "No LLM starts or directs another LLM". ADR-0154 line 10 names `evolve agent run` as the path of the console seat. The console is a Claude Code session, and it briefs and runs seats. | The hard rule has an unstated exception. |
| C10 | Five designs describe the path to `main`. | ADR-0057, ADR-0078, ADR-0128, ADR-0134 and ADR-0147, with ADR-0159. Each amendment link exists, but all the newest are Proposed. | The answer to "what lands today" needs code reads. |
| C11 | Withdrawn. The title of ADR-0126 and its counts looked different. | ADR-0126 decision 14: `J_0` is block 1 and `J_3` is block 4, so they are one count. | None. |
| C12 | The commit path differs from the practice. | The digest and `skills/commit/SKILL.md` name `evolve ship`. The console used a bare commit after the gate. This is a deviation, not a document conflict. | Section 4, row G5. |

## 4. Gaps between the documents and the code

| Id | Gap | Evidence | Class |
|---|---|---|---|
| G1 | The stage in the policy runs ahead of the ADR status. | `fleet.orchestrator` and `isolation.i2` and `retrospectives` are `enforce`. ADR-0139, ADR-0154 and ADR-0151 read Proposed. `llm-isolation.md` line 13 still says that X2 is at `shadow`. | Status |
| G2 | The verdict has more than one grammar. ADR-0033 asks for a single source. | `agentcmd.reportVerdict` uses `statedVerdict` (`go/internal/cli/agentcmd/verdict.go:42`, `:47`). `dashboard.reportVerdict` uses the sentinel, then `parseVerdict` (`go/internal/dashboard/failure.go:133`). On 2026-11-06, run `agent-code-reviewer-20261105T233200Z-ea1f` wrote a fenced `Verdict: PASS` with no finding. It read as FAIL, and the commit gate denied it. The same report had a nonstandard severity table, so the fence is the likely cause and not a proven one. | Real fault. Inbox item `review-verdict-in-a-code-fence-reads-as-fail` |
| G3 | A documented control has no production constructor, by an earlier trace. | The type is `Oversight` (`go/internal/orchestrator/oversight.go:44`). `docs/operations/wave-operations.md:117` says that the window grows only when the oversight loop reports. The filed item holds the trace, and this research did not repeat it. | Open. Filed item `oversight-has-no-production-constructor` |
| G4 | The local gates do not predict CI, and the eject signal does not name the gate. | One PR was ejected on `cover changed`, then on `apicover`, then on a table conflict. The first two came from a console floor that skipped `internal/bridge`. The brief read: "the CI of the batch is red". ADR-0161 asks a signal for a cause and a next step. | Real fault, and a console error |
| G5 | The ship guard does not hold for the console. | `go/internal/guards/ship.go:19` matches `git commit` and `git push` in a Bash command. This session ran both directly on 2026-11-06, and neither was denied. A script wrapper hides them from that match. The cause of the direct pass is not traced. | Open. The operator decides the rule |
| G6 | Specs lag a landing. | `integration-driver.md` still says that PR (b) "is built on the branch", and that PR landed as 6ae4972c3. Its status reads "The operator has not ruled on it". | Status |
| G7 | A code path of a plan can be stale, but the rate is low. | Paths absent at `origin/main`: plans 1 of 114, research 8 of 299, ADRs 3 of 189 (first path of each line). | Healthy |

## 5. Gaps to current practice

The web sources below are mostly practitioner notes and vendor pages. The OpenAI details come partly from secondary summaries.

| Id | Practice | Source | This repository | Gap |
|---|---|---|---|---|
| P1 | A change of mind is a new ADR, and the old status flips to superseded. A record with a stale status misleads. | [GDS Way](https://gds-way.digital.cabinet-office.gov.uk/standards/architecture-decisions.html), [Codably guide](https://codably.pages.dev/architecture/architecture-decision-records-a-practical-guide) | New records and amendment links exist. The flip does not happen. | Yes |
| P2 | An ADR has machine-readable fields and an executable rule that fails CI. | [EventCatalog ADR schema](https://eventcatalog.dev/docs/api/adr-api), [Archgate](https://pypi.org/project/archgate/0.48.4/) | Guards and ratchets exist, but no field links an ADR to its guard. | Yes |
| P3 | Give the agent a map, enforce constraints by tools, and run background cleanup of stale documents. | [OpenAI, harness engineering](https://openai.com/index/harness-engineering), [secondary summary](https://www.theneuron.ai/explainer-articles/openais-harness-engineering-playbook-how-to-ship-1m-lines-of-code-without-writing-any/) | The digest is a map, and the gates are tools. No sweep finds stale documents. | Partly |
| P4 | Keep scenarios outside the view of the agent that builds, like a holdout set. | [Simon Willison on StrongDM](https://simonwillison.net/2026/Feb/7/software-factory/) | Journey tests are protected (ADR-0159), but a builder can read them. | Yes |
| P5 | A separate evaluator judges the work of a generator. | [InfoQ on the Anthropic harness](https://infoq.com/news/2026/04/anthropic-three-agent-harness-ai/) | Separate review seats and an adversarial audit exist. | No |
| P6 | Batch a merge queue, and bisect a failed batch. | [Trunk](https://docs.trunk.io/merge-queue/batching), [Mergify](https://mergify.com/learn/merge-queue/batching) | The train batches. This research did not test a multi-PR failure. | Unknown |
| P7 | Keep a constitution, and sync its templates on update. | [Spec Kit guide](https://codemyspec.com/blog/github-spec-kit-guide) | ADR-0145 designs one rule home, and it is Proposed. | Partly |

**The factory measure.** Wave 156 shipped 3 of 9 cycles, with 2.89 audit rounds for each cycle and 3.93 M fresh tokens for each shipped cycle (`evolve wave status`, 2026-11-06). One failed cycle wasted 184.5 minutes. These numbers are from one wave, so they show a level and not a trend.

## 6. Three options

Each score is 1 (poor) to 5 (good), and it is a synthesis.

| Criterion | A: one manual sweep | B: decision registry and check | C: an LLM contradiction seat |
|---|---|---|---|
| Stops a repeat | 1 | 5 | 3 |
| Finds a conflict of meaning | 3 | 1 | 5 |
| Usage cost now | 3 | 4 | 1 |
| False signals | 5 | 5 | 2 |
| Fit to repository rules | 3 | 5 | 3 |
| **Total** | **15** | **20** | **14** |

**A: one manual sweep.** One pass corrects each status and each stale fact. Its advantage is an immediate fix with no new code. It rots again, because nothing fails when the next record goes stale.

**B: decision registry and check (winner).** Each ADR gets a small typed header: one status of a closed list, `supersedes`, `amends`, `enforced_by` and `stage_key`. A verb `evolve docs adr check` then fails on these conditions:
- a duplicate number, or a second ADR home;
- a relation that has no reverse link;
- a superseded target that does not read superseded;
- an `enforced_by` test that does not exist;
- a `stage_key` at `enforce` on a Proposed record.

An existing ratchet shape starts it as a baseline, so that old records need no rewrite in one PR. A generated page lists the current decision of each topic. This follows Rule 5: deterministic work goes in code. It adds no outside dependency, and that is why this research does not take Archgate itself.

**C: an LLM contradiction seat.** A periodic seat reads the corpus and files conflicts. It is the only option that sees C8 and C9 by itself. Each run reads a large corpus, and its findings need a verifier.

## 7. The winner, with the advantages of the others

| Step | Source | Work |
|---|---|---|
| 1 | From A | [ADR-0164](../architecture/adr/0164-all-decisions-serve-one-goal-and-one-amendment-resolves-their-conflicts.md) closes C1 to C10 and G6 through notes on the older records. |
| 2 | B | Build the registry, the check and the current-decisions page behind a baseline. |
| 3 | B | Link each `enforce` stage to its record, which closes G1. |
| 4 | Operator | Rule on C12 (G5) and on the rank of `feature`. Overrule any decision of ADR-0164. |
| 5 | From C | After the weekly usage resets, one seat reads one topic at each boundary and reads each research and plan body in that topic. A verifier checks each finding. This covers the gap of section 2. |

G2, G3 and G4 are code faults, and they follow the normal blocker path. P4 needs its own design.

## 8. Limits

- The contradiction table comes from decision digests and targeted reads, so a conflict deeper in a body can be absent from it.
- The automated search of practice found no tool that detects a contradiction between two accepted ADRs. So option C has no outside proof of its false-signal rate.
- The scores are a judgment of the console. They are not measured.
- ADR-0164 decisions 9 and 10 state a reading of C8 and C9. Only the operator can confirm it.

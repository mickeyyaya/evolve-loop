# Failure-Rate Review: Cycles 1481–1503 (post-v22.18.0 era)

**Window:** 2026-08-16 → 2026-08-17, batches 20260816a/b/c/d + 20260817a (and 20260817b's first fail).
**Raw numbers:** 23 lane cycles → **3 PASS·ships** (1482, 1494, 1498), **~17 FAILs**, 4 batch halts (2 breaker, 2 SYSTEM). Console response in the same window: **4 pipeline PRs merged** (#468 sentinel tolerance, #469 closure-claim compound/path false-REDs, #470 verdict-cache salvage + Put-site operand, #471 lineage-scoped closure demotion), 1 salvage landing recovering 4 lane attempts.
**Companions:** [2026-08-14-wave4-staging-halt.md](2026-08-14-wave4-staging-halt.md) · [2026-08-15-false-walls-and-repick-class.md](2026-08-15-false-walls-and-repick-class.md).

---

## 1. Complete failed-cycle ledger

| Cycle | Lane | Failed at | Class (§2) | One-line root cause | Status |
|---|---|---|---|---|---|
| 1481 | context-fill-telemetry-and-cap | audit WARN→FAIL | F (+E) | Attested a cycle-1478 salvage the reflog disproves (narrative-integrity M1); + 3 quality MEDIUMs | item parked |
| 1483 | minted-phase + dead-api-sweep | build | C | Hit its own quarry: KindJSON contract demands pure JSON **and** a sentinel tail; evolving the render red'd the test pinning the contradiction | item parked |
| 1484 | context-fill (re-pick) | build floor | B→G | Re-dispatched via registry; then collided with the wave-1-shipped "unmeasured families stay unmeasured" contract | parked |
| 1485 | verdict-cache | apicover floor | E | New export `WithVerdictCacheLookupHook` named by a test but 0% executed (false-green) | landed via #470 |
| 1486 | carryover-retirement | EGPS + closure flags | H (+A) | `ship_eligible=false`; 2 closure flags later shown to be the #469 false-positive class | parked (5 burns) |
| 1487 | context-fill (burn 3) | build floor | B | **Consumed/parked scope re-dispatched by the continuation registry** — quarantine wasn't transactional | remediated + 0.86 filed |
| 1488 | verdict-cache | EGPS red=1 | C | Own TDD-authored predicate unsatisfiable: `FileContains` used for an ABSENCE assertion (documented cycle-352 anti-idiom; `FileNotContains` existed) | fixed in #470 |
| 1492 | verdict-cache (retry) | EGPS red=1 | D | All own predicates green; inherited 1488 predicate carried UNREPAIRED — the retry built new work instead of fixing the anchor | fixed in #470 |
| 1493 | carryover-retirement | SYSTEM halt | **A** | **Manufactured FAIL**: test-amplification phase authored a `$`-anchored `-run` meta-test (guaranteed red) into `go/acs/`; + closure-claim path-ref false-positive. Task work was audit-green | #469 + persona rules + 0.87 design item |
| 1495 | verdict-cache (retry 3) | EGPS red=2 | D | 1488 anchor STILL unrepaired + 1492's cascading suite-green meta-predicate | salvaged → #470 |
| 1496 | carryover-retirement | audit H1 | **H** | **Real security catch**: agent-writable `VerifiableBy` executed with no authority clamp (command-injection surface — flaw traced to the ITEM's own prescription) | folded into salvage recipe |
| 1497 | sleep-time (consumed scope!) | build floor | B | Wave planner mints lanes **directly from registry bindings** — third dispatch source, missed by consumption | bindings released; 0.86 planner-seam |
| 1499 | tokenopt-handoff-digests | build floor | F | One-line compile error (unused import) **survived 3 corrections** — correction prompts don't reliably carry the failing tool output | correction-fidelity → 0.87 item |
| 1500 | carryover-retirement (+triage) | audit H1 | E | Real progress (wave-level retirement wiring built) but `cmd_loop_wave.go:323-405` zero executed coverage | parked w/ recipe |
| 1502 | triage-synthesis (split) | SYSTEM halt | **A** | 165-green/0-red cycle FAILed on the closure gate's line-locality rule while the same report's per-id dispositions covered all 4 inherited ids | **#471** |
| 1503 | lane-retirement + dead-api | drain FAIL | B/C | Re-bundled under a RENAMED alias — the registry held the scope under two names | aliases released |
| 17b-1 | tokenopt (retry) | scout gate | F | Scout decomposed into 2 new sub-slugs but never materialized their evals | in-flight |

## 2. Failure taxonomy with counts

| Class | Name | Count | Nature | State |
|---|---|---|---|---|
| **A** | Pipeline-manufactured false FAILs (gate false-positives, phase-authored red artifacts, parser strictness) | 3 full + 2 partial | NOT task quality — the pipeline red-ing green work | **Dead**: #468/#469/#471 all regression-pinned; EGPS-attribution design at 0.87 |
| **B** | Retired-work re-dispatch (three non-transactional dispatch stores: inbox, carryovers, **continuation registry incl. aliases**) | 4 this era (+3 prior) | Pure waste (~2M tokens/burn); gates rejected honestly every time | Operationally stopped (flock releases, parks); structural: `park-consume-releases-continuation-binding` 0.86 + the parked retirement mechanism itself |
| **C** | Self-authored unsatisfiable contracts/tests (inverted absence oracle, contradictory minted contract, `$`-anchored discovery regex) | 3 | Authoring-time defects only discovered by burning the cycle | `acs-absence-primitive-and-unsatisfiability-lint` 0.8; persona rules landed; **the primitive already existed — a knowledge-injection gap, not a tooling gap** |
| **D** | Inherited-anchor recurrence (retries build new work, never repair the prior attempt's red artifact) | 2 | Findings injection lacks "repair the red FIRST" priority | Anchor-first policy → §4.4 |
| **E** | Honest verification-floor catches (false-green exports, zero-coverage new code) | 3 | Real quality floor working; agents under-test new surfaces | Floors correct; guidance §4.6 |
| **F** | Mechanical/process slips (compile error through 3 corrections, missing evals, false attestation) | 3 | Correction-loop information fidelity + agent honesty | §4.3; the false attestation was caught by narrative-integrity checking |
| **G** | Same-batch semantic coupling (sibling ship changes a contract under a running lane) | 1 | Scheduling, not quality | §4.7 |
| **H** | Real substantive audit findings (VerifiableBy injection; ship-eligibility) | 2 | **The system working exactly as designed** | Security finding folded into salvage recipe |

**Honest fail-rate read:** raw ≈74% (17/23). Removing class A (manufactured) and class B (waste on retired work) — both now structurally addressed — the true task-level fail rate on genuine attempts is ≈50%, and the failures show *convergence*: the retirement item advanced mechanism → security clamp → production wiring across attempts; verdict-cache's product code was green from attempt 2 on. Both items were, or will be, landed by console salvage in ~1–2 focused hours each.

## 3. What went RIGHT (the denominator matters)

1. **Zero forged PASSes; every gate rejection in this ledger was honest.** The three gate defects that existed (sentinel one-byte strictness, closure-claim compound/path/line-locality) produced false *FAILs*, never false *PASSes* — the safe failure direction — and all three are now dead with the live artifacts checked in as regression fixtures.
2. **The audit chain caught a false attestation** (1481: claimed salvage disproved by reflog) and **a real command-injection design flaw** (1496 H1) — including a flaw in the operator-side item prescription itself.
3. **The breaker/halt machinery fired per policy every time** (3-consecutive ceiling, ADR-0072), forcing the per-cycle deep-dives that produced this taxonomy.
4. **Convergent retries**: each retry failed at a *later* pipeline stage when findings were accurate — evidence the findings-injection loop works when the finding is correct (and is poison when it isn't: see 1502's release note).

## 4. Improvement plan (ranked by expected fail-rate reduction)

1. **Land the retirement mechanism (parked, 5 convergent burns, salvage recipe in the item).** Kills class B at the source: pre-dispatch `verifiableBy` probe (WITH the 1496 authority clamp: no shell chaining, confinement seam, provenance check) + retirement transactional across ALL THREE dispatch stores. Expected: −4 to −7 burns per week at current rates. *Console salvage, next session.*
2. **Registry lifecycle transactionality** (`park-consume-releases-continuation-binding` 0.86): planner-seam guard (a scope with no live pending item is skipped + released at MINT time), release-on-consume/park, halt-path preservation, `evolve continuation list/release` CLI. Companion of (1).
3. **Correction-loop information fidelity** (extends `contract-correction-stale-artifact-freshness` 0.87): a correction re-prompt MUST embed (a) the exact failing tool output (compiler error, gate diagnostic — the 1499 unused-import line survived 3 corrections that evidently never showed it), and (b) a content-hash freshness check so re-validation of unrewritten bytes is classified, not counted. 2026 literature directly supports this: typed recovery signals + diagnosis-before-recovery outperform blind retry ([DARC, arXiv:2608.11772](https://arxiv.org/abs/2608.11772)), and same-agent self-correction without new evidence is systematically weak ([Self-Correction Illusion, arXiv:2606.05976](https://arxiv.org/pdf/2606.05976)) — our cross-family adversarial audit already implements the "correct others" half.
4. **Anchor-first retry policy** (new, cheap, prompt-side): the continuation findings block must LEAD with "these exact artifacts are RED on your base — repair or retract them BEFORE any new work", listing paths. Cycles 1492/1495 each re-derived green product work while a ten-line broken test sat untouched (its correct fix already written in a sibling's comment!).
5. **Authoring-time unsatisfiability lint** (`acs-absence-primitive-and-unsatisfiability-lint` 0.8): eval-quality pre-flight flags positive-primitive-with-absence-wording and contradictory minted contracts (KindJSON + sentinel-tail). The 2026 test-generation literature warns exactly this: agents "hallucinate assertions that do not reference actual application behavior," and test *volume* does not correlate with resolution rate — quality gates at authoring time do ([Rethinking Agent-Generated Tests, Feb 2026](https://zylos.ai/research/2026-03-05-ai-agent-automated-test-generation/); [AI-augmented testing 2026 guide](https://qaskills.sh/blog/ai-augmented-software-testing-2026-guide)).
6. **New-surface coverage discipline** (persona-level): any new exported symbol or fail-closed branch must be *executed* by a same-cycle test — the apicover false-green and zero-coverage floors keep catching this post-hoc; a builder-persona rule + the uncovered-lines list injected into the FIRST correction shortens the loop.
7. **Same-subsystem wave scheduling**: two items touching one subsystem (context-fill ×2) should share a lane or sequence across batches — a sibling ship mid-batch changed a contract under the running lane (1484). Orchestration-level failure classes (stale context, contradictory intermediates) are the second pillar of the 2026 self-healing literature ([Self-Healing Orchestrators, arXiv:2606.01416](https://arxiv.org/html/2606.01416v1); [Self-Healing Framework, arXiv:2605.06737](https://arxiv.org/abs/2605.06737)).
8. **EGPS authorship/retraction model** (`phase-authored-red-tests-poison-egps` 0.87, design task): a phase that self-FAILs its own authored test must not leave it as ship-blocking evidence — with the anti-gaming constraint (no laundering path for builder reds) explicitly preserved.

## 5. Verification of this review

Every row in §1 traces to on-disk evidence: `.evolve/runs/cycle-N/{audit-fail-reason.json,audit-report.md,acs-verdict.json}`, batch logs `.evolve/logs/batch-2026081*.log`, and the archived escalation dossiers (`pipeline-escalation.resolved-pr4*.json` in the halt cycles' run dirs). The class-A fixes are pinned by checked-in live artifacts: `go/internal/phasecontract/testdata/cycle1478-trailing-brace.md`, the cycle-1493 line-36 fixture in `closure_claim_compound_test.go`, and the six demotion tests in `closure_claim_demotion_test.go`.

## Sources

- [DARC: Diagnosis Before Recovery — Turning Agent Failures into Selective Self-Correction (arXiv:2608.11772, Aug 2026)](https://arxiv.org/abs/2608.11772)
- [The Self-Correction Illusion: LLMs Correct Others but Not Themselves (arXiv:2606.05976, 2026)](https://arxiv.org/pdf/2606.05976)
- [A Self-Healing Framework for Reliable LLM-Based Autonomous Agents (arXiv:2605.06737, May 2026)](https://arxiv.org/abs/2605.06737)
- [Self-Healing Agentic Orchestrators for Reliable Tool-Augmented LLM Systems (arXiv:2606.01416, 2026)](https://arxiv.org/html/2606.01416v1)
- [Why AI Agents Fail: A Taxonomy of Failure Modes (SSRN 6572478, Apr 2026)](https://papers.ssrn.com/sol3/papers.cfm?abstract_id=6572478)
- [AI-Powered Automated Test Generation for Software Engineering Agents (Zylos Research, Mar 2026)](https://zylos.ai/research/2026-03-05-ai-agent-automated-test-generation/)
- [AI-Augmented Software Testing in 2026 (QASkills guide)](https://qaskills.sh/blog/ai-augmented-software-testing-2026-guide)

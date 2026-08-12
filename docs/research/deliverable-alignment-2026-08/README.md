# Deliverable Alignment Across LLM CLIs — Strategic Evaluation (2026-08-04)

> Operator question: *"determine if there are more effective AI architecture
> solutions for aligning deliverables across our LLM CLI … I'm not sure if the
> current 'find one, solve one' approach works."*
>
> **Method.** Two parallel research tracks, then fusion: (1) a complete survey
> of this repo's own alignment mechanisms and their measured results (docs/,
> kb/, ADRs, live runtime state, incident reports); (2) online state-of-the-art
> research (2024–2026) on schema-conformant agent deliverables under our exact
> constraint — heterogeneous *interactive* CLIs driven through tmux, no decoder
> access, deliverables as terminal text + files on disk. Sources cited inline.

---

## 1. The verdict on "find one, solve one"

The honest answer is: **it is the right *verification* posture and the wrong
*prevention* strategy — and the data says we already knew which layers each
belongs to, without having named it.**

**Where it demonstrably works.** Every one of the last week's 7 closed failure
classes stayed closed. The adversarial audit's find-one record is the best
catch-rate per unit deployed in the whole system: a CRITICAL reproduced against
the compiled binary (cycle-1255), a false "diff is empty" build claim
(cycle-1259), a fail-open probe blocked in review, a gameable trust path in the
defect-ledger mechanism itself (cycle-1282's `TestPOC_A`). The
`continuation-defect-ledger` grind — 5 rounds, 4 distinct real defects, then a
PASS — is find-one-solve-one operating *as a hardening crucible*: each found
defect was real, narrower than the last, and in a mechanism that must be
bulletproof precisely because it will gate other work.

**Where it demonstrably fails.** Three measured patterns:

1. **Class recurrence in costumes.** The scope disease needed four incidents
   (apicover exports, stub coherence, routingtest keystone, phasespec catalog)
   before a class-level mechanism was attempted — and that mechanism (TIA) is
   dormant and structurally blind to 2 of the 4 costumes (import-graph-only).
2. **Fix latency is a failure mode of its own.** The contract-block CLI
   escalation item (0.95) sat stranded for two batches while its class recurred
   twice (batch-18 agy 7/7 → batch-21 triage cycle-1215). ADR-0034 exists
   because ~60% of fix commits before it were the *same* misplaced-deliverable
   class, found and solved one at a time.
3. **The frontier moved above the artifact layer.** The 2026-08-04 batch
   integrity review found the code layer clean under live mutation testing —
   and the real gaming in **status accounting** (defect laundering, fabricated
   activation provenance, label-derived ledgers), a layer that had *no*
   contracts at all until §3.8/§3.9.

**The synthesis:** find-one-solve-one is how you *harden a mechanism*; it is
not how you *choose an architecture*. The strategic question is which layer
each failure class belongs to — and the empirical record (this repo's, and the
literature's) converges on the same four-layer answer (§4).

## 2. What we have built, and what it measurably did

Full inventory with file references in §A of the local survey; condensed:

| Mechanism | Class | Measured result | Known ceiling |
|---|---|---|---|
| Unified deliverable contract + `RenderContractTail` (ADR-0034) | generation-point | Motivating class was ~60% of fix commits; tail-placement restatement got compliance the prefix block did not; batch-19+ `bad_verdict` = **zero** after the tail landed | Saturates at model capability: agy ignored the identical contract 7/7 — "format adherence is a model property past some point, not a prompt property" |
| Contract gate + self-check + breaker | post-hoc gate | Same `Verify` for agent self-check and host gate (cannot drift); verify-read race closed with a 500ms grace window | Breaker is a **fail-open ratchet**: 3 live CIRCUIT OPEN firings, all on a weak-CLI phase — a systematically non-compliant CLI demotes enforce→advisory |
| Correction/retry ladder (≤2, salvage→live-fix→re-dispatch) | retry-correction | Salvage rung deterministic ("would have fixed 265 in milliseconds"); correction prompts empirically work on capable models | Same-CLI re-dispatch was the batch-18 hole; **landed** since — identity-gated CLI escalation (cycle-1289) plus, for phases already on the top family, a breaker-neutral salvage re-prompt (cycle-1300, `061345a4`; §6.2). Residual ceiling: `cli_fallback` itself still fires only on infra exits, and the top-family rung can only re-prompt the same model |
| Verdict sentinel + file-authoritative transport (ADR-0072) | transport | Killed the verdict-format-drift class; "pane never a verdict source" | **The worst incidents were the transport's own**: 862–899 false-FAIL storm (10 cycles, 3 green features discarded in a livelock) was the harness forging FAILs, not agents misbehaving |
| EGPS / ACS predicates (v11 Go-native, TDD-authored, red-team) | verification-by-predicate | 2026-08-04 review live-mutated shipped predicates: **zero tautological, zero tampered, all 7 FAILs honest** | Contention flakes ≈ 20% of waste; dead-red corpus pollution; green-by-skip latent traps |
| Adversarial audit (Opus vs Sonnet, evidence-demanding) | adversarial review | Best catch-rate in the system (examples in §1); exactly what the judge-bias literature prescribes (cross-family, self-preference bias arXiv:2410.21819) | WARN prescriptions had **no post-ship enforcement** (F3); its own deliverable was the top contract-violator on a weak CLI |
| ADR-0072 floor + fingerprint breaker | process supervision | Would have stopped the 862 storm at cycle 862 instead of 899; halted 2 real pipeline defects this week | Needed 4 generations of fingerprint-identity repair to stop false halts; `ship\|unknown` class tag still content-free |
| §3.8/§3.9 + continuation-defect-ledger (landed cycle-1286) | process supervision (accounting) | First mechanization of the accounting layer; its own 5-round hardening found 4 real bugs incl. a pre-plantable trust path | Newest front; per-defect dispositions not yet enforced pipeline-wide |

**Top empirical observation** (local survey): the project's own stability
research already concluded **~65% of alignment failures were harness defects,
not agent defiance** — and the single largest loss event was the harness
discarding verified-green work. Investment in verdict *transport* robustness
has paid better than any agent-facing constraint.

## 3. State of the art, 2025–2026 (what the field settled on)

Condensed from the online track; full citations at the bottom.

1. **Native structured outputs are decoder-level and real** (OpenAI strict
   json_schema; **Anthropic structured outputs, public beta Nov 2025**; Gemini
   responseSchema) — but on *CLI surfaces*: Claude Code exposes them headless
   (`claude -p --json-schema`) and via Stop/PostToolUse **hooks** in-session;
   Codex `--output-schema` exists but is unreliable with tools active
   (openai/codex#15451); Antigravity has only artifact conventions; **Ollama
   gives full grammar enforcement via its API — we own that decoder today.**
2. **The constraint tax is real but localized** ("Let Me Speak Freely?"
   arXiv:2408.02442 vs the dottxt rebuttal; 2025–26 follow-ups): the settled
   rule is *constrain the envelope, never the reasoning*. Harness-side
   validation of free-form work pays zero tax — which is our architecture.
3. **Schema-Aligned Parsing beats both constraining and naive rejection**
   (BAML SAP: lenient, logged, bounded coercion of what the model clearly
   intended — their benchmark had SAP + a small model beating a big model with
   constrained decoding). Production data: unenforced JSON malforms 8–15% of
   calls; **one feedback retry lifts parse success ~60%→97%; two rounds ≈ 95%
   of recoverable failures; beyond two, resampling dominates.**
4. **Verifier architecture:** process supervision beats outcome-only and the
   gap widens on long-horizon agents (arXiv:2305.20050, PRM survey
   2510.08049); stacks of *weak deterministic verifiers* approach
   strong-verifier power (Weaver, arXiv:2506.18203); LLM judges carry position
   and self-preference bias — cross-family adversarial judging is the
   published mitigation; **SWE-agent's shipped-guardrail rule: nothing gates
   unless its false-positive rate is provably ~0** (our 1054/1060 breaker
   lesson, independently derived).
5. **Repair economics:** self-repair often loses to resampling at equal budget
   and **critic quality is the bottleneck — escalate the auditor, not the
   generator** (ICLR 2024 arXiv:2306.09896); cascade escalation carries a
   double-billing penalty (arXiv:2606.27457).
6. **Industry consensus for exactly our constraint profile** (Anthropic
   multi-agent system, Cognition, SWE-agent ACI, OpenHands typed-event SDK):
   *contracts live in the harness type system; the model is never trusted to
   self-certify; deliverables are first-class typed objects; reasoning stays
   unconstrained; envelopes are extracted/validated post-hoc.* No serious
   product uses decoder constraints for inter-step contracts.

## 4. The layer model — where each failure class belongs

Fusing §2's measured record with §3's literature:

| Layer | Failure classes it owns | Our mechanism (state) | Verdict |
|---|---|---|---|
| **L1 Generation-point** (prompt contracts, tail restatement, in-session hooks) | format/placement classes | ADR-0034 + RenderContractTail (shipped, working) | Correct and near-ceiling; the remaining lever is per-CLI: Claude hooks, Ollama grammar |
| **L2 Transport & salvage** (file-authoritative verdicts, grace windows, lenient extraction) | harness-side losses (~65% of failures), partial writes, malformed-but-recoverable output | file-authoritative + backfill (shipped); **schema-aligned salvage layer MISSING** | Highest-leverage gap — every salvage is a whole cycle saved |
| **L3 Verification** (predicates, gates, adversarial audit, metamorphic invariants) | semantic wrongness, gaming, vacuous work | EGPS + adversarial audit (shipped, best-in-system); cross-artifact invariant stack partial (`coherence`) | Keep; extend with the weak-verifier invariant stack; every gate needs FP≈0 evidence |
| **L4 Process/accounting supervision** (fingerprint breaker, defect ledgers, disposition artifacts, §3.9) | cross-cycle laundering, status fiction, fix latency, lost prescriptions | ADR-0072 (shipped) + continuation-defect-ledger (landed cycle-1286) + §3.8/§3.9 (policy) | The open frontier; mechanize dispositions pipeline-wide |

"Find one, solve one" is L3/L4's *operating mode* — correct there. The
operator's instinct is right about L1/L2: those layers should be **class-
preventive by construction**, and each new instance found there is a signal the
layer needs a mechanism, not another patch.

## 5. Ranked portfolio (what to build, in order)

| # | Move | Layer | Why this rank | Risk (named) | Queue state |
|---|---|---|---|---|---|
| 1 | **Contract-block CLI escalation** — on the 2nd contract block, re-dispatch the phase on the strongest available family (design already in the item) | L1/retry | Already-proven class (agy 7/7 twice); the literature's "escalate on capability ceiling"; the item has been stranded while its class recurred — worst fix-latency instance | Cost of strong-family retries; needs the fingerprint breaker integration | **landed** — cycle-1289 identity gate (§6.1) + cycle-1300 top-family salvage rung (§6.2, `061345a4`); item consumed cycle-1304 |
| 2 | **Schema-aligned salvage layer** (BAML-SAP pattern, in-harness Go): lenient, *logged*, bounded extraction of fenced/mislabeled JSON, displaced sentinels, trailing commas — before any retry/rejection | L2 | Converts whole-cycle losses to zero-cost saves across all four CLIs; no constraint tax; production data says 8–15% of calls are recoverable-malformed | Coercion masking semantic drift — every coercion logged + surfaced in audit; never invent values | **NEW — filed 0.9** |
| 3 | **Two-stage verdict minting**: phase writes free-form report; a cheap constrained extractor (headless `claude -p --json-schema`, or Ollama grammar-constrained = hard guarantee at zero API cost) mints the machine JSON from report+workspace | L2 | The decoder-free realization of "reason free, constrain the envelope"; removes sentinel burden from reasoning agents entirely; subsumes `verdict-sentinel-as-tool-call` (0.86) | A faithful extractor can mint a well-formed *wrong* verdict — must fail-on-ambiguity and cross-check against artifacts (move 5) | extend existing 0.86 item |
| 4 | **Retry ladder economics alignment**: keep cap=2; on 2nd failure escalate the *critic/diagnoser*, not just the generator; integrate with the fingerprint breaker so identical failures don't burn the cap | L1/L3 | Directly from the repair-economics literature; our ladder is right-shaped, mis-aimed | Double-billing curves; budget per cycle | fold into item #1's landing |
| 5 | **Cross-artifact metamorphic invariant stack** as a distinct deterministic verdict authority: sentinel == standalone JSON; claimed counts == parsed runner output; referenced paths exist; provenance chains intact | L3 | Weak-verifier aggregation ≈ strong verifier at near-zero cost; immune to judge bias; several invariants exist scattered (`internal/coherence`) — unify them | SWE-agent rule: an invariant without proven FP≈0 becomes a flake generator — advisory until evidenced | **NEW — filed 0.85** |
| 6 | **Claude Code Stop-hook finish-gate** (in-session): refuse turn-end until deliverables exist and validate; tmux re-prompt equivalent for other CLIs | L1 | Only in-session enforcement any of our CLIs offers; one turn cheaper than post-hoc correction | Enforcement asymmetry across CLIs skews per-CLI stats; needs the `stop_hook_active` escape | **NEW — filed 0.8** |
| 7 | **Accounting-layer mechanization** — per-defect dispositions enforced pipeline-wide, ledger writes from diffs + runtime artifacts | L4 | The integrity review's frontier; first mechanism landed cycle-1286 after a 5-round hardening that proved the need | Its own gameability (cycle-1282's pre-planted-ledger POC — fixed) | in flight (0.95 landed; residuals queued) |

Explicitly **not** recommended: decoder-level constrained decoding on hosted
CLIs (unreachable + constraint-tax risk on reasoning), unbounded repair loops
(dominated by resampling), single-model self-certification anywhere, and a
generic "more predicates" push (the predicate layer is already clean under
mutation testing — its waste is contention flakes, a different problem).

## 6. Experience record for the new moves (to be extended per §3.8)

Each portfolio item that lands must append its issue/gap/solution and measured
before/after here or in `docs/operations/batch-integrity-review-2026-08-04.md`.
Baselines to measure against: `bad_verdict` = 0 since the contract tail
(batch-19+); contract-gate CIRCUIT OPEN firings = 3 (all weak-CLI
adversarial-review); recoverable-malformed rate on our CLIs = **measured** —
15 of 167 `bad_verdict` blocks are classifier-recoverable (9.0%), the salvage
layer's first deliverable, landed in cycle-1389 and recorded in full in §7
(source of record; re-derivable from the preserved sidecar
`.evolve/runs/cycle-1389/bad-verdict-baseline.jsonl`); whole-cycle losses to
deliverable failures this week = documented per-cycle in the batch integrity
review.

### 6.1 Landed — fingerprint-gated CLI escalation (cycle-1289, item rank 1)

**Issue.** The contract-block CLI escalation landed by PR #390
(`go/internal/core/contract_escalation.go`) triggered on a raw counter: the
correction ladder in `go/internal/core/cyclerun_review.go` escalated whenever
`ReviewResult.Blocks >= contractEscalateAtBlock` (2). `Blocks` counts *blocks*,
not *defects*, so it never asked whether block 2 was the same violation as
block 1.

**Gap.** Two genuinely DIFFERING contract violations on one phase — block 1
misses a section heading, block 2 misses the verdict sentinel — read as one
incapable-CLI signature and spent round 2's budget on a different CLI family.
That is over-eager escalation: two honest defects the same CLI can fix, not a
CLI that cannot render the contract. It also meant the escalation ladder and the
blocker breaker carried two unrelated notions of "this failed twice" — the
ambiguity the live inbox item (weight 0.96) named as
*"integrate with the fingerprint breaker so identical blocks share identity"*.

**Solution.** The trigger is now gated on failure IDENTITY as well as count.
`contractBlocksShareIdentity` (`contract_escalation.go`) compares the current
block's reason against the block that triggered the previous correction, using
the blocker breaker's OWN primitive — `normalizeReasonForFingerprint`
(`go/internal/core/failure_digest.go`) — rather than a second hashing scheme.
Because that primitive projects a reason onto its defect identity (dropping
identity-noise such as go-test duration tokens and narrative verdicts), two
blocks reporting the *same* defect in verbatim-different text still escalate,
where raw string equality would have wrongly suppressed them.

The rule is deliberately **"prior reason known AND differing ⇒ suppress"**, not
"equal ⇒ escalate". The contract-gate breaker is process-global, so a cycle that
aborted mid-ladder leaves it HOT and the next phase can arrive at `Blocks >= 2`
on its ladder's FIRST block — with no prior reason to compare. Under an
equality-only rule that case would silently stop escalating, deleting the
hot-breaker escape hatch PR #390's review established on purpose. Family
selection (`contractEscalationCLI`) and the `policy.ValidatePin` guardrail
(`escalationAllowed`) are untouched.

**Measured before/after.** Behavior is exercised by the three-axis suite in
`go/internal/core/contract_escalation_test.go`, driving the real
`Orchestrator.RunCycle`: NEGATIVE (differing reasons ⇒ no escalation, RED before
this change), SEMANTIC (same defect, duration-token-different text ⇒ still
escalates — the discriminator against a raw-equality fix), EDGE (hot breaker, no
prior reason ⇒ escalation still fires). 10/10 contract-escalation tests PASS.
Live escalation-firing counts remain to be measured against the baseline in §6
(contract-gate CIRCUIT OPEN firings = 3 at the time of writing).

### 6.2 Landed — top-family salvage rung (cycle-1300, closes item rank 1)

**Issue.** The live inbox item's `[LIVE EVIDENCE 2026-08-05]` addendum recorded the
first strong-family block series: a `claude-tmux` adversarial-review phase blocked
twice on the same defect, escalation correctly reported *"no other CLI family
available in its chain"* — and the circuit opened anyway.

**Gap.** The ladder had exactly one remedy for an identical second block, and that
remedy is *buy a different family*. A phase already dispatched on the strongest
family has nothing to buy, so §6.1's identity gate fired, found no target, and the
gate fell through to demotion. That is the breaker's fail-open ratchet reached
without any repair having been attempted — and the demotion record could not tell
that case apart from one where a remedy ran and failed, because `escalated=false`
alone spans both.

**Solution.** `composeContractSalvageRetry`
(`go/internal/core/contract_escalation.go`) adds a second rung: when the escalation
target is empty, round 2 spends its budget on the *diagnosis* instead — a structured
re-prompt carrying the validator's verbatim output and requiring each bracketed
`[violation_code]` to be answered by naming the exact heading or path it refers to.
It is breaker-neutral by construction: no extra dispatch, no extra correction
consumed, `ReviewResult.Blocks` untouched, so the circuit still opens on the third
strike as the last resort. `contractDispatch.salvageRetried` then rides alongside
`escalated=` in the demotion WARN and the `contract_gate_demoted` ledger entry, so
"salvage attempted and failed" is legible as distinct from "no remedy was possible".
This is the repair-economics prescription (arXiv:2306.09896) the item cited, applied
where a family swap is unavailable.

**Measured before/after.** `go/internal/core/contract_salvage_retry_test.go` drives
the real `Orchestrator.RunCycle` across six axes: fires when no other family exists;
does NOT fire when an escalation target does exist (the discriminator against a
blanket second rung); not on the first block; not on a non-contract rejection;
ledger records `salvage_attempted`; WARN distinguishes attempt from no-remedy.
`go test ./internal/core/... -run Contract` PASS. Escalation-firing and
CIRCUIT-OPEN counts still measure against the §6 baseline (3 firings, all
weak-CLI adversarial-review). With this rung landed, the rank-1 portfolio item was
consumed in cycle-1304 (`.evolve/inbox/consumed/2026-08-04T07-15-00Z-contract-block-cli-escalation.json`).

### 6.3 Measured — recoverable-malformed baseline (cycle-1389, item rank 2 input)

**Issue.** The baseline line above could not be closed by argument: the rank-2
portfolio item `schema-aligned-salvage-layer` is only worth building if a
meaningful share of `bad_verdict` blocks are *shape* failures rather than
genuinely missing verdicts, and that share had never been counted.

**Gap.** `deliverable.Verify` emitted `bad_verdict` as one undifferentiated
code, so the salvage layer's addressable population was not merely unknown but
unknowable from the existing logs — no artifact distinguished a readable
verdict rejected on shape from no verdict at all.

**Solution.** A log-only classifier,
`go/internal/deliverable/salvage_instrument.go`, records one JSONL baseline
record per `bad_verdict` strictly after the gate's decision, changing no
block/approve outcome. Driving it over the full historical corpus produced the
audited table in **§7**, which is this entry's measured before/after and the
source of record for the figures quoted in §6's baseline line above. The
finding narrowed the portfolio rather than confirming it: one shape dominates
the recoverable set and the extraction pass stays deferred on evidence — see
**§7** for the counts, the caveat about the counterfactual sweep, and the
consequence for the rank-2 item.

## 7. Baseline — the recoverable-malformed `bad_verdict` rate, measured (cycle-1389)

**Issue.** §6 recorded the recoverable-malformed rate on our own CLIs as *"not
yet instrumented (the salvage layer's first deliverable is the measurement)"*.
The rank-2 portfolio item `schema-aligned-salvage-layer` proposes a lenient
extraction pass that recovers a clearly-intended verdict a strict parse
rejected. Building that pass without a measured rate would be speculative: the
whole case for it rests on how often a `bad_verdict` block is a *shape* problem
rather than a genuinely absent verdict.

**Gap.** `deliverable.Verify` reported `bad_verdict` as a single undifferentiated
code. Nothing in the pipeline distinguished "the agent emitted a verdict we
could plainly read but rejected on shape" from "the agent emitted no verdict at
all" — so the salvage layer's addressable population was unknown, and unknowable
from the existing logs.

**Solution (this cycle — instrumentation only, no extraction).**
`go/internal/deliverable/salvage_instrument.go` adds a pure, log-only
classifier, `ClassifyBadVerdict(content string) BadVerdictClassification`, over
the exact bytes the verdict was computed from (`Result.Content`, the single-read
seam). It recognises the three shapes the portfolio item names — a
fenced/mislabeled JSON payload (`fenced-json`), a sentinel payload with a
trailing comma (`trailing-comma`), and a bare unwrapped verdict object in prose
(`displaced-line`) — and reports `Recoverable=false, Pattern=""` for anything
with no verdict-bearing JSON at all. **No coercion ships**: `Result.OK` and
`Result.Violations` are untouched, and the gate's block/approve decision is
byte-identical with and without the classifier.

**Wiring proof.** The classifier is not a dead helper. It is reached from the
real host contract gate, `deliverable.Reviewer.Review` — the seam
`cmd/evolve/cmd_cycle.go:620` wires behind `core.DeliverableReviewer` — strictly
*after* the `res.OK` branch, so it can observe only failures and can influence
no decision:

```go
// go/internal/deliverable/reviewer.go — Reviewer.Review
if res.OK {
	resetBreaker(bp)
	return core.ReviewResult{Approve: true}
}
// Observability-only, strictly after the OK branch and strictly before any
// decision is computed: record what a future salvage stage WOULD have
// recovered from this bad_verdict. Never reads a decision, never writes one.
recordBadVerdictBaseline(roots, in.Phase, res, r.logf)

reason := summarize(in.Phase, res)
```

`recordBadVerdictBaseline` (`salvage_instrument.go`) fires only when the result
carries `CodeBadVerdict`, and appends one JSONL record — `phase`,
`artifact_path`, `recoverable`, `pattern`, `reason` — to
`.evolve/bad-verdict-baseline.jsonl` via the existing `log.SidecarWriter`. A
write failure is logged and swallowed: telemetry never outranks the gate's
fail-safe posture.

**Measured baseline.** Produced by driving the production reviewer
(`NewReviewerWithCatalogStageReportSize`, the exact `cmd_cycle.go:620`
constructor) at `ContractGate=enforce, PhaseIO=enforce` over this repo's entire
historical deliverable corpus — every `audit`/`build`/`scout`/`triage`/`tdd`
report under `.evolve/runs/cycle-*/` — and counting the resulting
`.evolve/bad-verdict-baseline.jsonl` in this cycle's worktree. Counts below are
`wc -l` / `uniq -c` on that file, not estimates. The sweep is deterministic —
two independent runs produced byte-identical tallies — and the sidecar it wrote
is preserved as `.evolve/runs/cycle-1389/bad-verdict-baseline.jsonl` so the
counts below can be re-derived rather than taken on trust:

| Measure | Count |
|---|---|
| Deliverables reviewed | 5528 |
| `bad_verdict` blocks (baseline records written) | 167 |
| — classifier-**recoverable** | **15 (9.0%)** |
| — `fenced-json` | 13 |
| — `displaced-line` | 2 |
| — `trailing-comma` | 0 |
| — not recoverable | 152 (91.0%) |
| Phases affected | `audit` only (167/167) |

**What the numbers say — and the honest caveat.** Two findings, and one
qualification that must travel with them.

1. *The addressable rate is ~9%, and one shape dominates it.* Of 167 blocks,
   only 15 carry a recoverable JSON verdict, and 13 of those 15 are
   `fenced-json` — an agent rendering the payload as displayable JSON instead of
   the sentinel comment. `trailing-comma` never occurred once in 5528 reports,
   despite being the shape the portfolio item led with. A salvage stage built to
   the item's original three-shape spec would spend two-thirds of its surface on
   patterns with zero observed incidence.
2. *The 91% is not a salvage problem at all.* Sampling the not-recoverable
   records (`cycle-1`, `cycle-10`, `cycle-100` audit reports) shows zero
   occurrences of the string `evolve-verdict`: these are legacy **prose-verdict**
   reports written before the sentinel existed. No JSON-shape salvage layer can
   recover them, because there is no JSON to salvage — they are recovered today
   by the legacy prose fallback (`deliverable.go:342-348`), which is exactly what
   `PhaseIO=enforce` gates off.
3. *Caveat — this is a counterfactual sweep, not observed production traffic.*
   Production runs below `PhaseIO=enforce`, where the prose fallback rescues most
   of these; §6's "`bad_verdict` = 0 since the contract tail" remains true of
   live cycles. The sweep answers a different, and the actually decision-relevant,
   question: *if PhaseIO were promoted to enforce, what would break and how much
   of it is mechanically recoverable?* The historical corpus skews toward early
   cycles, so the 91% legacy share is an upper bound on what a promotion would
   hit today.

**Consequence for the portfolio.** The extraction/coercion pass stays deferred,
now on evidence rather than on caution — and its scope is narrower than proposed:
`fenced-json` is the only shape with a non-trivial observed rate, so a
single-shape recovery would capture 13 of the 15 addressable cases for a
fraction of the surface. The real prerequisite for a `PhaseIO=enforce` promotion
is not the salvage layer at all but a decision about legacy prose-only reports.
The `salvage-enforce-stage-dial` item stays deferred: a rollout dial for a stage
that does not exist is premature. This section is the measurement §6 asked for;
re-run it after any promotion and append the delta.

### 7.1 The rate is now computed, not hand-read (cycle-1407)

**Issue.** §7's own closing instruction — *"re-run it after any promotion and
append the delta"* — was not executable. The baseline above was produced by
driving the reviewer and reading the resulting JSONL by hand. Any operator
asking "is the extraction stage worth building yet?" had to repeat that manual
exercise, so the portfolio gate's deciding number was, in practice, stale
between whoever last did the arithmetic.

**Gap.** `salvage_instrument.go` had been appending
`.evolve/bad-verdict-baseline.jsonl` since cycle-1389 and **nothing had ever
read it back**: grepping the tree for the filename outside the writer and its
own tests returned only the writer. Eighteen cycles of measurement with zero
readers — the instrumentation-first mandate half-executed.

**Solution.** `go/internal/deliverable/salvage_report.go` adds
`SummarizeBadVerdictBaseline(io.Reader) (BaselineSummary, error)`, a pure fold
over the sidecar, and `go/cmd/evolve/cmd_salvage.go` surfaces it:

```
$ evolve salvage report            # prose
$ evolve salvage report -json      # {"total":…,"recoverable":…,"rate":…,"by_pattern":{…}}
```

Read it as follows. **`rate`** is the *recoverable-malformed* rate —
`recoverable / total` over `bad_verdict_classified` records only — i.e. the
share of rejected deliverables whose verdict a lenient reader could plainly have
recovered. That share, not the raw failure count, is the extraction stage's
addressable population. **`by_pattern`** splits the recoverable half by shape
(`fenced-json`, `trailing-comma`, `displaced-line`), and it is the actionable
half: the shapes have very different recovery costs, so a single headline number
cannot size the stage. A run where one pattern dominates argues for a
single-shape recovery (§7 found exactly that: `fenced-json`, 13 of 15 cases);
a flat spread argues for a general parser or for not building one at all.
Records emitted by other producers sharing the sidecar are skipped, and a torn
JSONL line is a loud error rather than a silently-dropped denominator — an
under-counted denominator biases the rate in the direction that flatters
building the stage.

**Also in this cycle — the classifier stopped trusting quoted sentinels.**
`ClassifyBadVerdict` selected the *first* `evolve-verdict` span in a document.
In adversarial-review reports that span is routinely a decoy the author quoted
while describing a bypass, so the classifier never reached the report's own
verdict and recorded `Recoverable=false` for reports that were plainly
recoverable — i.e. the baseline above was measured by a reader with the same
first-sentinel-wins defect the cycle-1298 corpus exists to document, and the
`cycle-641` lesson names. Selection is now quote-aware and tail-anchored
(matching `phasecontract.ParseVerdictSentinelFull`), pinned by
`TestClassifyBadVerdict_QuotedDecoyCorpus` against that corpus in both
directions. Any rate recorded before cycle-1407 is a floor, not a measurement.

## 8. Landed — the salvage stage itself (cycle-1392, corrected cycle-1397)

The extraction/coercion pass §7 deferred is now built. `SalvageVerdict`
(`go/internal/deliverable/salvage_extract.go`) reformats and reparses a
bad_verdict, and it acts only when **all four** of these hold:

1. the bad_verdict is the **SOLE** violation — the guard is
   `Result.onlyViolation(CodeBadVerdict)`, not a membership test;
2. `ClassifyBadVerdict` classifies the content as recoverable;
3. exactly one verdict-bearing candidate exists anywhere in the content; and
4. the **repaired bytes re-verify clean** against the phase contract
   (`repairVerdict` → `verifyMarkdown`), rather than `OK` being hand-set.

Any other outcome is a refusal (`applied=false`, `Result` unchanged — neither
`Result.Content` nor the artifact on disk is touched), and a field value is
never invented on any path.

On the APPROVED path the repaired bytes are the ones that survive: salvage
returns them as `Result.Content`, and the gate (`Reviewer.Review`) writes them
back over `ArtifactPath` before it approves. If that write fails the salvage is
**refused** and the phase blocks — the one place the gate deliberately fails
closed rather than open.

> **Correction (cycle-1441, audit H1 HIGH).** The first cut of the paragraph
> above read "the stored `Result.Content` bytes are never mutated" and the code
> matched it: `salvageVerdictWith` re-verified `repaired`, then returned a
> struct-copy of the ORIGINAL with only `OK`/`Violations` flipped, and
> `Reviewer.Review` discarded the salvaged `Result` outright. The gate therefore
> reported `OK: true` over a byte stream that was never the byte stream it
> verified, and the artifact a downstream phase re-read was still malformed —
> so `ParseVerdictSentinelFull` failed on it and `releasepreflight.extractVerdict`
> fell through to a prose scan that could resolve a FAIL-sentinel deliverable to
> PASS. "Never mutates" was the right invariant for the REFUSAL path and the
> wrong one for the approval path; the two are now stated separately, and pinned
> by `go/acs/cycle1442/predicates_test.go` (001 in-memory, 002 on-disk through
> the real gate, 003 the refusal invariant).

> **Correction (cycle-1397).** The first cut of this section, and of cycle
> 1392's build report, claimed a multi-violation failure could never be
> salvaged. That was **false of the code as shipped**: the guard was a
> membership test, so a bad_verdict co-occurring with `missing_section` or
> `missing_challenge_token` WAS salvaged wholesale — every violation erased,
> including the cycle-269 anti-forgery proof-of-read check. The cycle-1392
> audit disproved the claim with an executed probe (CRITICAL-1), and the same
> audit's MEDIUM-3 flagged that flipping `OK=true` from classification alone
> skipped `RequireFailureContext`, laundering a malformed FAIL sentinel into
> an approval. Both are fixed above. The multi-violation claim now rests on an
> executed assertion, not on narration: `go/acs/cycle1397/predicates_test.go`,
> `TestC1397_001_SalvageVerdict_MultiViolationNeverSalvaged` (unit seam) and
> `TestC1397_002_ReviewerWiring_MultiViolation_BlocksAndLogsNoSalvage`
> (production `Reviewer.Review` seam); the re-verify obligation is pinned by
> `TestC1397_005_ReviewerWiring_RepairedFailWithoutContext_NotApproved`.

Wired at the same seam
`recordBadVerdictBaseline` uses in `Reviewer.Review` (reviewer.go), strictly
after the unconditional baseline record and before the block decision: a
successful salvage approves the phase and resets the contract-gate breaker,
same as a clean pass.

**Every coercion logged + surfaced (inbox `fix` text).** A successful salvage
appends one record to a NEW `salvage-applied.jsonl` sidecar under `.evolve`
(parallel to, and in addition to, `bad-verdict-baseline.jsonl` — the
instrumentation stays unconditional, salvage is additive, never a
replacement). `SalvageSummaryLine(evolveDir)` renders "Salvaged verdicts: N
(pattern breakdown)" from that same sidecar — single source of truth, no
second counter — and is silent (`""`) at zero records, so a cycle that salvages
nothing adds no noise.

> **Correction (cycle-1399).** Through cycle 1397 that renderer had **no
> production caller** at all: it was declared, documented here as "surfaced",
> and called only from tests (cycle-1392 audit LOW
> `dd17d798e155571ecd91be63e14050ab6`). "Surfaced" was therefore false of the
> code as shipped — the same overclaim class as the HIGH above, one severity
> down. It is now wired at the salvage seam itself: `Reviewer.Review` emits the
> line through the gate's own `logf` immediately after the sidecar append
> (`go/internal/deliverable/reviewer.go:149`), so the operator-visible surface
> is the `[contract-gate]` log stream, **not** a section of `audit-report.md` —
> composing it into the report body is still unbuilt (queued as
> `surface-salvage-in-audit-report`). Reachability is pinned by an executed
> assertion, not narration:
> `go/acs/cycle1399/predicates_test.go`,
> `TestC1399_001_SalvageSummaryLine_HasProductionCaller` (AST call-site scan
> over every non-test file under `go/`) and
> `go/internal/deliverable/reviewer_salvage_surface_test.go`,
> `TestReviewerReview_SalvageSurfacesSummaryLine` — which drives a salvageable
> deliverable through the real `Reviewer.Review` seam and asserts the gate's own
> log stream carried `Salvaged verdicts: 1 (fenced-json=1)`.

**Measured before/after.** §7's counterfactual sweep found 15/167 (≈9.0%)
recoverable-malformed `bad_verdict` blocks, 13 of them `fenced-json`. Cycle
1392's own eval suite (`go/acs/cycle1392/predicates_test.go`,
`TestC1392_001`–`TestC1392_007`) drives all three shapes plus the ambiguity
and absence negative controls through the real `Reviewer.Review` production
seam — `TestC1392_007_ReviewerWiring_SalvageApplied_ApprovesAndLogsSidecar`
is the reachability proof: a trailing-comma bad_verdict flips `Approve` from
false to true and writes both sidecars. The applied-count these tests exercise
is the mechanical proof the wiring works; the live-traffic *before/after rate*
delta (post-salvage unrecoverable share vs. §7's 91%) is future work once
`PhaseIO=enforce` is promoted and salvage runs against real production
`bad_verdict` blocks — §7's own caveat about counterfactual-sweep vs. observed
traffic still applies, and the 91% legacy-prose share is unaffected by this
stage (it targets JSON-shape recovery only, per §7 finding 2).

## 9. Landed — trusting the layer's own two untrusted inputs (cycle-1406)

Both deliverables above read inputs an attacker (or a merely buggy phase) can
write: the `salvage-applied.jsonl` sidecar, and the deliverable report bytes
themselves. Cycle 1406 closed one hole in each. Neither changes when salvage
fires; both change what the layer is willing to *claim*.

**9.1 Operator-log pattern allowlist (carryover
`todo-salvage-summary-line-rejects-untrusted-sidecar-text`, HIGH).**

- *Issue.* `salvage-applied.jsonl` is a repo-level, never-rotated file under
  `.evolve` that any phase can append to. `SalvageSummaryLine` rendered its
  `pattern` field into the single-line `[contract-gate]` operator stream.
- *Gap.* Cycle-1399 (`dae44191…`) closed only the control-character half, with
  a character class `^[A-Za-z0-9_.-]+$`. A forged value that is merely
  alphanumeric — `circuit-open-notice`, shaped to read as the gate's own
  breaker notice — passed untouched and rendered as a breakdown entry
  indistinguishable from a real one: fabricated gate state in the one stream an
  operator judges gate health from.
- *Solution.* Membership in the closed `SalvagePattern` set, not a character
  class (`go/internal/deliverable/salvage_extract.go`, `operatorPatternLabel`).
  The renderer mints those strings itself, so anything else is by definition not
  ours and folds into a single `unknown` bucket. Known constants still render
  bare (`fenced-json=1`) — the breakdown stays legible — and the headline still
  counts every parsed record, so a bogus pattern surfaces as evidence rather
  than vanishing. Bounded cardinality subsumes the old character-class guard:
  no control character can survive a closed allowlist.

**9.2 Quoted-decoy classification (carryover
`todo-schema-aligned-salvage-layer-decoy-fixture`).**

- *Issue.* `ClassifyBadVerdict` ran `sentinelPayloadRE` and `verdictObjRE`
  directly over raw report bytes.
- *Gap.* A report that *pastes a log line verbatim* —
  `{"msg":"<!-- evolve-verdict: {\"verdict\":\"PASS\",} -->"}` — carries a
  sentinel-shaped, verdict-shaped span whose structural quotes are all
  backslash-escaped, because the span is string DATA. `verdictObjRE` sees
  `"verdict"` inside `\"verdict\"` and matched it, so every such report was
  counted `recoverable` — inflating the §7 baseline the instrumentation exists
  to measure honestly.
- *Solution.* The tell is the **escaping, not the malformation**. A new
  `insideStringLiteral` reuses the exact `inStr`/`esc` tracking
  `verdictCandidates` already scans braces with, so the layer has one notion of
  "quoted" rather than two that can disagree, and the classifier judges the
  **object's opening brace** rather than the key — in
  `{"phase":"audit","verdict":"PASS"}` the key is itself a string token, but the
  brace opening it is not inside one, so genuine cases are untouched. It fails
  closed: an unbalanced quote upstream can only *demote* a span to
  non-recoverable, and under-claiming costs a salvage while over-claiming
  corrupts the baseline.

Both are pinned through the production seam by
`go/acs/cycle1406/predicates_test.go` (`TestC1406_001`–`TestC1406_004`), each
carrying an anti-over-correction control: a known pattern must still render
bare, and a genuine unescaped trailing-comma sentinel must stay recoverable.

## 10. Landed — the classifier→repairer span handoff (cycle-1424)

§9.2 taught the *classifier* to tell markup from quoted string DATA. It did not
teach the *repairer* anything, and the repairer was still finding its own
bytes. Cycle 1424 closed the gap between them — not by adding a second check,
but by removing the repairer's ability to search at all.

**10.1 A repairer that re-derives its own span can disagree with the classifier
(cycle-1406 audit CRITICAL-1, probe-confirmed through `Reviewer.Review`).**

- *Issue.* `ClassifyBadVerdict` qualified a span and returned only a *verdict*
  about it (`Recoverable`, `Pattern`). `repairVerdict` then re-found the bytes
  to rewrite by itself — the trailing-comma branch took
  `sentinelPayloadRE.FindStringSubmatch(content)`, the FIRST match, with none
  of the string-literal awareness §9.2 had just added to the classifier.
- *Gap.* §9.2 is a *classifier-side* fix, so it could not reach this: the two
  sides simply asked different questions of the same bytes. A report quoting a
  decoy `PASS` sentinel in prose was CLASSIFIED on its own genuine, malformed
  `FAIL` (the classifier skipped the quoted decoy) and REPAIRED on the decoy
  (the repairer did not). Because `ParseVerdictSentinelFull` takes the LAST
  parseable sentinel, the phase's own `FAIL` — left malformed and therefore
  unparseable — lost to the freshly repaired `PASS`, and the gate approved. One
  stray `"` turned a phase's failure into an approval.
- *Solution.* Structural, not another guard. `BadVerdictClassification` now
  carries the QUALIFIED BYTE OFFSETS it admitted — `span` (the range to replace
  with a canonical sentinel line) and `payload` (the JSON object's own bytes
  inside it) — in `go/internal/deliverable/salvage_instrument.go`, and
  `repairVerdict` (`go/internal/deliverable/salvage_extract.go`) performs one
  branch-free replacement of exactly that span. Its own extraction primitive
  was deleted rather than fixed. Adding a second quoting check inside the
  repairer would have closed the one reproduced instance and left two
  independently-drifting notions of an admissible span — the drift the
  cycle-1406 audit explicitly warned about; taking the offsets instead means
  "repair a span the classifier rejected" has no expressible form.

Pinned through the production seam by `go/acs/cycle1424/predicates_test.go`:
`TestC1424_001_Review_DeniesApproval_WhenRepairedSpanDivergesFromQualifiedSpan`
is the reproduction, `TestC1424_002_Review_DeniesApproval_AcrossSpanDivergenceShapes`
re-runs it with the stray quote hosted by a fence so the criterion is the CLASS
rather than one fixture, and
`TestC1424_003_Review_StillSalvages_EveryGenuineShape` plus
`TestC1424_004_Review_KeepsAmbiguityRefusal_ForUnquotedTwinSentinels` are the
anti-over-correction controls — all three recoverable shapes must still
salvage, and the audit's own unquoted control must still block.

## 11. Landed — an ambiguity guard that could be silenced by quoting (cycle-1428)

Cycle 1424's own audit found the sequel to §10 in the code §10 left standing:
the *counter* the ambiguity guard reads was still the string-aware scan, and
§10 had just proved that prose quoting is attacker-controlled.

**11.1 One unpaired quote made the `> 1` ambiguity guard vacuous (cycle-1424
audit `d4982b388c4982275303ee68529b9313d`, CRITICAL).**

- *Issue.* `candidateCount` counted verdict-bearing objects with a single
  string-aware scan, so a `{` or `}` inside a JSON string never moved the depth.
- *Gap.* Balanced quoting is a JSON property, not a markdown one. A single
  unpaired `"` anywhere in prose leaves the scan believing every later byte is
  inside a string literal, so **both** candidates of a two-candidate report
  vanish and `candidateCount(res.Content) > 1` reads a genuinely ambiguous
  report as unambiguous. Classification does not vanish with it: step 2 computes
  parity FENCE-LOCALLY (`verdictObjSpan` runs over the fence body alone), so it
  still qualified a stray fenced `PASS` beside the phase's own malformed `FAIL`
  — the §10 laundering, reached by a different door. Neither §9.2 (classifier
  quoting) nor §10 (span handoff) covers it, because the defect is in the guard
  that decides whether to salvage *at all*.
- *Solution.* `candidateCount` (`go/internal/deliverable/salvage_extract.go`)
  now runs BOTH readings of the bytes — string-aware, and brace-only, where
  quotes are not structure — and takes the LARGER. A counter that can only
  undercount implements "refuse when the attacker permits"; ambiguity is what
  EITHER reading can see. Neither reading dominates the other, which is why both
  are kept: the brace-only reading cannot be silenced by quoting, and the
  string-aware reading still catches the mirror case where a string CONTAINS a
  stray brace and a brace-only scan would merge two objects into one. The
  asymmetry is the one `insideStringLiteral` is already built on — over-counting
  costs a salvage that would have been logged, under-counting costs the gate.

Pinned at the production seam in
`go/internal/deliverable/salvage_extract_test.go`
(`TestReview_RefusesSalvage_WhenAnUnpairedQuoteHidesASecondCandidate`, RED
before this change and GREEN after, with
`TestReview_StillSalvages_TheGenuineSoleCandidate` as the anti-over-correction
control), and every prior pin — `go/acs/cycle1392`, `go/acs/cycle1397`,
`go/acs/cycle1399`, `go/acs/cycle1404`, `go/acs/cycle1406`, `go/acs/cycle1424`
— re-run green on the change, which is what shows the new pessimism did not buy
its refusal by breaking salvage.

## 12. Landed — the same guard, silenced by a brace instead of a quote (cycle-1432)

§11's fix was defeated by one different character. Cycle 1428's audit recorded
the inherited CRITICAL as *not closed* (`d5bc991f500c121b0a173752a05ffccef`),
and cycle 1432 re-probed it at the production seam.

**12.1 A stateful counter has one silencer per reading (cycle-1428 audit
`d5bc991f500c121b0a173752a05ffccef` / inherited
`d4982b388c4982275303ee68529b9313d`, CRITICAL).**

- *Issue.* §11 answered the unpaired-quote silencer with `max(string-aware,
  brace-only)`. Both readings are stateful scans, and each carries its own
  one-character silencer in ordinary prose.
- *Gap.* The unpaired `"` still zeroes the string-aware reading (`aware=0`), and
  a single unmatched `{` opens a depth the brace-only reading never closes, so
  every later object is absorbed into it and only the truncation fallback span
  is emitted (`raw=1`). `max(0, 1) == 1` is not `> 1`, the refusal never fires,
  and the fence-local classifier still qualifies a decoy `PASS` beside the
  phase's own displaced `FAIL`. The probe found the class is **wider than the
  ledger recorded**: the unmatched brace defeats the guard *on its own*, with no
  quote noise present — the class is one character wide, not two.
- *Solution.* `candidateCount`
  (`go/internal/deliverable/salvage_extract.go:181-201`) now takes the max of
  **three** readings, the third being stateless: `verdictKeyCount` counts the
  `"verdict":` keys themselves, carrying no scan state at all, so no earlier
  byte can change what a later one means and no unbalanced delimiter can
  desynchronise it. It is consulted only through the max, so it can add
  refusals and never remove one — a stray `"verdict":` in prose costs a salvage,
  which is the side of the asymmetry this gate is built on. Both structural
  readings are kept: they are what produce spans; the stateless one only counts.

Pinned as a **class**, not a fixture — the shipped §11 pin was a fixture pin
(`df8ed646d2c48edcd558afb056a08d28d`), which is why the sibling shape shipped
through it. `go/acs/cycle1432/predicates_test.go` drives `Reviewer.Review` over
{balanced, unpaired-quote, unmatched-brace, quote-and-brace} at 2 candidates
asserting refusal
(`TestC1432_002_Review_AmbiguityRefusal_HoldsAcrossProseNoiseShapes`) and over
the same four shapes at 1 candidate asserting salvage still happens
(`TestC1432_003_Review_StillSalvages_SoleFencedJSONCandidate`), so the stricter
counter cannot be satisfied by refusing more broadly.
`TestC1432_001_Review_RefusesSalvage_WhenQuoteAndBraceNoiseHideASecondCandidate`
is the direct reproduction, RED before this change and GREEN after; every prior
pin — `go/acs/cycle1392`, `go/acs/cycle1397`, `go/acs/cycle1399`,
`go/acs/cycle1404`, `go/acs/cycle1406`, `go/acs/cycle1424`, `go/acs/cycle1428`
— re-runs green.

The same cycle removed a cycle SCHEDULE from a permanent regression floor
(`.evolve/evals/schema-aligned-salvage-layer-guard.md`, cycle-1428 audit
`dd245084ac28b86b9f21d3a882240e327`): a floor that says "cycle 1428 is
documentation-only" expires silently the moment that cycle passes, so it now
states the invariant (sole-violation exclusivity, never a membership test over
the violation-code set) instead.

## 13. Landed — the stage itself, salvaged out of ten stranded worktrees (cycle-1441, 2026-08-12)

### 13.1 The salvage-before-requeue landing (cycle-1441)

Sections 8–12 above describe a stage that, until this cycle, existed on **no
branch that ever merged**. The extraction half of `schema-aligned-salvage-layer`
was built, corrected five times against its own audits, and left in place: ten+
near-identical `cycle-42824668-{1397,1399,1404,1406,1422,1424,1427,1428,1432,1434}`
worktrees each carry the same auto-committed
`"salvage snapshot (ADR-0076 continuation-on-fail)"` commit, and `gh pr list`
shows no PR ever touched `salvage_extract.go`.

**What landed.** The most advanced snapshot — worktree `cycle-42824668-1434`,
commit `a2d65920` — ported onto `main` at `d20efc51`:
`go/internal/deliverable/salvage_extract.go` (the stage), its
`salvage_extract_test.go` / `reviewer_salvage_surface_test.go` /
`salvage_keycase_test.go` suites, the `reviewer.go` wiring, and the
`BadVerdictClassification` span/payload offsets §10's handoff needs.

**Wiring proof.** `salvageVerdictWith(res, r.resolver, roots, r.phaseIO)` is
called from `go/internal/deliverable/reviewer.go:138` — strictly after the
`res.OK` early return and strictly before the block decision is computed, so the
seam's only caller is the production gate, not a test.
`go/acs/cycle1441/predicates_test.go` reaches it through `Reviewer.Review`
rather than calling it directly.

**One deliberate divergence from the snapshot.** The snapshot's own
`ClassifyBadVerdict` rewrite was NOT taken. `main` advanced past it at
cycle-1407: the classifier there is quote-aware (a sentinel contained in a
closed inline-code span is prose *discussing* a verdict) and tail-anchored, and
`salvage_instrument_test.go` carries a symbol tripwire that fails the build if
either historical helper name reappears in a production source. Taking the
snapshot wholesale would have re-red'd `main` on that tripwire and dropped the
1407 defence. What was ported instead is the part §10 actually requires — the
qualified byte offsets — computed inside `main`'s classifier by BLANKING the
echoed-sentinel and fence ranges rather than deleting them, so every offset into
the working copy is still an offset into the original content. The snapshot's
string-literal-escaping reading of "quoted" is deferred, not adopted: it is a
second notion of quoting alongside 1407's, and reconciling the two is its own
cycle.

**Why it stranded, recorded so it does not recur (scout B1).** Nothing technical
blocked it — every snapshot builds and tests green in place. The stage was
always a *continuation artifact* of a cycle whose selected task was something
else, so no cycle ever carried it to ship, and each failure produced one more
snapshot of the same work. The lesson generalises past this item: in a
continuation-worktree repo, "not on `main`" and "not built" are different facts,
and a scout should `find .evolve/worktrees -name '<expected-new-file>'` before
treating an inbox `feature` item as greenfield.

### 13.2 Operator surface — measured potential vs. actual saves

`evolve salvage report` now reports `saved` — coercions the gate actually
performed, folded from `.evolve/salvage-applied.jsonl` by
`deliverable.CountSalvageApplied` — beside `recoverable`, the measured potential
from `bad-verdict-baseline.jsonl`. They are different numbers by construction: a
stage that refuses on ambiguity or fails the re-verify pass makes potential
exceed practice, and collapsing them would tell an operator the gate salvaged
reports it in fact blocked. The key is always emitted, so a consumer never
special-cases the fresh-project-root state.

## Sources (online track)

Anthropic structured outputs (platform.claude.com, Nov 2025) · OpenAI
structured outputs · Gemini responseSchema · Claude Code hooks + Agent SDK
structured outputs (code.claude.com) · Codex non-interactive + issues #15451/
#19816/#4181 · Ollama structured outputs (GBNF) · Antigravity artifacts ·
JSONSchemaBench arXiv:2501.10868 · "Let Me Speak Freely?" arXiv:2408.02442 ·
dottxt "Say What You Mean" · Castillo replication · DSPy assertions
arXiv:2312.13382 · BAML Schema-Aligned Parsing + "false confidence" ·
Instructor reask · Pydantic-AI output · guardrails-ai · Weaver weak verifiers
arXiv:2506.18203 · "Let's Verify Step by Step" arXiv:2305.20050 · PRM survey
arXiv:2510.08049 · "Judging the Judges" arXiv:2406.07791 · self-preference
bias arXiv:2410.21819 · debate protocols arXiv:2402.06782, NeurIPS 2024 ·
scalable oversight arXiv:2504.18530 · metamorphic testing arXiv:2511.02108 ·
SWE-agent arXiv:2405.15793 · Anthropic multi-agent research system (Jun 2025) ·
Cognition "Don't Build Multi-Agents" · OpenHands SDK arXiv:2511.03690 ·
self-repair economics arXiv:2306.09896, arXiv:2604.10508 · cascade
double-billing arXiv:2606.27457 · production JSON failure-rate reports
(tensoria.fr, 2025).

Local track: `docs/architecture/deliverable-contract.md` · ADR-0034/0039/0042/
0072/0076/0077 · `docs/architecture/egps-v10.md` ·
`docs/operations/false-fail-recovery-862-899.md` ·
`docs/operations/change-log-2026-07-30.md` ·
`docs/operations/batch-integrity-review-2026-08-04.md` ·
`docs/research/llm-output-stability-2026-07/` ·
`docs/research/failed-loop-analysis-2026-07/` · live runtime state
(`contract-gate-breaker.json`, `pipeline-escalation.json`, run dirs).

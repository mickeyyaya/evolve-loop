# LLM output stability: the contract tail and the capability ceiling

**Period:** 2026-07-29 → 2026-08-05 · **Status:** shipped (escalation landed cycle-1289; salvage layer queued)
**Primary artifacts:** commits `891e50f4` (#375), `9574e66c`/`d255583e` (#383, v22.12.0) · [docs/architecture/deliverable-contract.md](../../docs/architecture/deliverable-contract.md) · [ADR-0034](../../docs/architecture/adr/0034-unified-deliverable-contract.md) · [docs/research/llm-output-stability-2026-07](../research/llm-output-stability-2026-07/README.md) · [docs/research/deliverable-alignment-2026-08](../research/deliverable-alignment-2026-08/README.md)

## Problem

A phase agent's deliverable is prose in a file with a machine-readable contract on top: required
sections, a verdict sentinel, an exact path. In batch-18, agy — then the pinned CLI for
adversarial-review — **failed that contract 7/7 times across correction re-dispatches**, and the
contract-gate circuit breaker demoted the gate enforce→advisory in *both* wave-1 lanes: "a
gate-weakening outcome" (commit `891e50f4`;
[change-log-2026-07-30.md §8](../../docs/operations/change-log-2026-07-30.md)). Two compounding
holes: corrections re-dispatched to the **same** CLI forever, because `cli_fallback` fires only on
infra exit codes, never on contract blocks; and the breaker's only remaining move when corrections
kept failing was to weaken the gate — a fail-open ratchet. The same class recurred on triage in
batch-21 (cycle-1215) while its fix sat stranded in a locked claim
([change-log §2, §8](../../docs/operations/change-log-2026-07-30.md)).

## Context & evidence

- **The asymmetry that pointed at the fix.** The contract's requirements already existed in the
  prompt — as a `## Deliverable Contract` block in the cacheable prompt *prefix* (ADR-0034
  lineage). But the *correction* prompt, which restates identical requirements at the **turn
  tail**, "already got compliance the prefix block did not"
  ([deliverable-contract.md, "Why the same facts appear twice"](../../docs/architecture/deliverable-contract.md)).
  Same facts, different placement, different behavior — a controlled experiment the harness had
  been running by accident.
- **The research pass predicted it.** The stability review names the mechanism: Claude follows
  turn-tail instructions more reliably than system-ish preamble, XML-tagged sections parse
  unambiguously, and "our required-sections + sentinel template currently live in the persona body
  (system-ish, far from generation); the correction prompt that *does* work puts them in the turn
  tail. That asymmetry is a free experiment"
  ([llm-output-stability §4.3](../research/llm-output-stability-2026-07/README.md)).
- **The larger frame:** the same review's headline is that ~65% of enterprise-AI failures are
  harness defects, confirmed locally — six of the session's six fixes were harness-class, and even
  the one apparent model failure (agy 7/7) was "*also* half-harness" because the correction loop
  could never leave the failing CLI ([llm-output-stability §0](../research/llm-output-stability-2026-07/README.md)).

## Approaches considered

- **Decoder-level constrained decoding** (FSM token masking, 100% schema-valid). Not available:
  the loop drives interactive CLI REPLs under tmux — "an architectural constraint worth naming
  rather than wishing away" ([llm-output-stability §4](../research/llm-output-stability-2026-07/README.md)).
  The later strategic survey also ranks it explicitly **not recommended** on hosted CLIs, citing
  the constraint-tax literature: "constrain the envelope, never the reasoning. Harness-side
  validation of free-form work pays zero tax — which is our architecture"
  ([deliverable-alignment §3.2, §5](../research/deliverable-alignment-2026-08/README.md)).
- **More/better correction prompting on the same CLI.** Refuted by the live data: "corrections
  re-issued verbatim to agy converged 0 times in seven; claude complied"
  ([change-log §8](../../docs/operations/change-log-2026-07-30.md)).
- **Reroute the phase's profile pin.** Done immediately (2026-07-29): adversarial-review moved
  agy → claude/deep, following the house pattern for auditor-class phases. This retired the
  "timeout-prone-phase → agy" doctrine *for this phase*; the lagging contract test that still
  pinned agy turned main red on the release commit and was fixed in `891e50f4` (#375, v22.11.1).
  A pin change treats the instance, not the class — acknowledged in the commit itself: "revisit
  when contract-block-cli-escalation lands."
- **Contract restatement at the generation point** (the tail). Chosen — see Decision.
- **Escalate the CLI on repeated contract blocks.** The class fix, queued at 0.9→0.95→0.96 (it was
  stranded twice — itself a documented fix-latency lesson,
  [deliverable-alignment §1.2](../research/deliverable-alignment-2026-08/README.md)), landed later
  fingerprint-gated at cycle-1289 (see [Fingerprint identity](2026-07-fingerprint-identity.md)).

## Decision & reasoning

Restate the **machine half of the contract at the generation point**: immediately after the
`DELIVERABLE PATH:` footer, `phasecontract.RenderContractTail` appends one XML-tagged
`<deliverable-contract>` block — artifact path, required sections, verdict sentinel with its
allowed verdicts, and the `evolve phase verify` self-check command
([deliverable-contract.md](../../docs/architecture/deliverable-contract.md)). The reasoning chain:
the correction prompt's tail placement demonstrably worked where the prefix did not; recency
dominance is a model property you design *with*, not against; and duplicating the facts is safe
only because "every string in the block is projected from the ONE phasecontract template source
(no second template)" — the writer and the detector cannot drift apart (commit `9574e66c`). The
cache trade-off is explicit: the invariant block stays in the cacheable prefix, the per-cycle tail
lives past the cache boundary anyway.

The named limit of the whole approach, recorded *before* the fix shipped: "**Format adherence is a
model property past some point, not a prompt property**" — agy ignored the identical contract 7/7;
no placement fixes that ([llm-output-stability §4.1](../research/llm-output-stability-2026-07/README.md)).
Prompt work raises the floor for capable models; only CLI escalation addresses the ceiling.

## Implementation

Shipped in #383 (`9574e66c`, squashed from `d255583e`, v22.12.0, 2026-07-30) as one of three
"house rules reach the agents" items. Verification details that mattered:

- **Opus review HIGH (BLOCK→resolved):** the tail's exemplar showed a bare PASS sentinel for
  audit — but audit's FAIL/WARN verdict must carry a structured failure block, and "the tail is
  the recency-dominant copy, so that was the version the auditor would follow on the one phase
  whose verdict gates ship." The tail now renders the failure-bearing exemplar with an explicit
  note.
- **Test polarity as anti-gaming:** the failure-bearing exemplar must be *REJECTED* by the
  production detector (the cycle-603 placeholder-echo guard stops a printed example reading as a
  real verdict); and an assertion wanting a new exported accessor was reshaped to assert
  observable shape — "adding an export whose only caller is a test is precisely the dead-seam
  pattern this change forbids" (commit `9574e66c`).
- Tests: `go/internal/phasecontract/render_tail_test.go`,
  `go/internal/adapters/bridge/bridge_contract_tail_test.go`; suite run recorded in the commit
  across phasecontract/bridge/prompts/core/cmd/deliverable.

## Results (measured)

- **`bad_verdict` = 0 in post-fix batches.** The strategic survey's measured-result table:
  "tail-placement restatement got compliance the prefix block did not; batch-19+ `bad_verdict` =
  **zero** after the tail landed," recorded again as the §6 baseline ("`bad_verdict` = 0 since the
  contract tail (batch-19+)") ([deliverable-alignment §2, §6](../research/deliverable-alignment-2026-08/README.md)).
- **The ceiling held too:** the same table row records the saturation point — the identical
  contract was ignored 7/7 by agy — so the tail is scored "correct and near-ceiling" at L1, with
  the remaining lever being per-CLI enforcement (Claude hooks, Ollama grammar)
  ([deliverable-alignment §4](../research/deliverable-alignment-2026-08/README.md)).
- **Breaker firings baseline:** contract-gate CIRCUIT OPEN firings = 3 at survey time, all on a
  weak-CLI phase ([deliverable-alignment §6](../research/deliverable-alignment-2026-08/README.md)).
- **Class fix landed:** contract-block CLI escalation, fingerprint-gated, cycle-1289 — 10/10
  escalation tests PASS; live firing counts still to be measured against the §6 baseline
  ([deliverable-alignment §6.1](../research/deliverable-alignment-2026-08/README.md)).
- **Coda (2026-08-05)** [session-evidence]: the breaker demoted the contract gate on a
  CLAUDE-TMUX adversarial-review (`bad_verdict` ×3; escalation had nowhere stronger to go, gate
  advisory for the run) — the fail-open ratchet now observed on the strong family, which moves the
  open question from "which CLI" to "what happens when the strongest family is the one failing."

## Retrospective — what we learned

- **Placement is a first-class variable.** The cheapest reliability win of the month came from
  moving existing requirements to where the model actually attends. The harness had the A/B data
  (prefix block vs correction tail) for weeks before anyone read it as an experiment.
- **Prompt work has a ceiling, and the ceiling is the model.** Past that point the only honest
  moves are escalation (change the model) or salvage (accept and repair the output). Both are now
  mechanism: fingerprint-gated escalation (landed) and the schema-aligned salvage layer (queued
  0.9, [deliverable-alignment §5.2](../research/deliverable-alignment-2026-08/README.md)).
- **A breaker that can only weaken the gate is a ratchet toward fail-open.** Demotion
  enforce→advisory was designed as a safety valve; under a systematically non-compliant CLI it
  became the failure mode. Escalation gives the ladder somewhere to go besides down — and the
  2026-08-05 coda shows the question recurs even at the top of the ladder.
- **Never depend on output identity; depend on contract satisfaction.** Temperature-0 determinism
  is a myth under batched inference; the stability review reframes the whole fingerprint campaign
  as "the correct response to a fundamental property, not a workaround for sloppy agents"
  ([llm-output-stability §1](../research/llm-output-stability-2026-07/README.md)) — the same
  conclusion the [fingerprint entry](2026-07-fingerprint-identity.md) reaches from the incident side.
- **Single-source the contract or the writer and detector drift.** The tail's design rule — every
  string projected from one template — is the same no-second-template discipline that #368's
  writer/detector no-drift tests enforce on failure reasons.

## Links

- [docs/architecture/deliverable-contract.md](../../docs/architecture/deliverable-contract.md) · [ADR-0034](../../docs/architecture/adr/0034-unified-deliverable-contract.md)
- Commits: `891e50f4` #375 (v22.11.1) · `9574e66c`/`d255583e` #383 (v22.12.0) · [CHANGELOG.md](../../CHANGELOG.md)
- [change-log-2026-07-30.md](../../docs/operations/change-log-2026-07-30.md) §5 (verify-read grace window), §8 (contract-gate CLI escalation, batch-18/21 evidence)
- Research: [llm-output-stability-2026-07](../research/llm-output-stability-2026-07/README.md) ·
  [deliverable-alignment-2026-08](../research/deliverable-alignment-2026-08/README.md) (constraint tax §3, layer model §4, cycle-1289 landing §6.1)
- Sibling entries: [Fingerprint identity](2026-07-fingerprint-identity.md) (the identity primitive the escalation gate reuses) ·
  [False-FAIL storm](2026-07-false-fail-storm.md) (why transport outranks agent-facing constraints) ·
  [Quota-detection regex drift](2026-07-quota-regex-drift.md)

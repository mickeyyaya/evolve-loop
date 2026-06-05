# PR ↔ ADR/Design-Doc Documentation Audit (2026-06-05)

> Request: verify every merged PR maps to an ADR or design document; create documentation for anything genuinely uncovered. Scope: all **60 merged** PRs in #1–#62 (#37 and #48 were closed without merge — out of scope). Method: two parallel audit agents (PRs 1–31, 32–62) extracted doc references from each PR body, verified files on disk, and keyword-searched `docs/architecture{,/adr}/`, `docs/superpowers/`, `knowledge-base/research/` for unreferenced coverage; the two candidate gaps were then adversarially re-verified by hand.

## Verdict

**60/60 merged PRs accounted for.** 48 PRs MAPPED to an existing ADR / architecture doc / spec / research note; 12 N/A (test-only, CI-mechanical, reverts, non-architectural fixes); **1 genuine gap** (PR #31) — closed by writing [docs/architecture/model-discovery-and-catalog.md](../../docs/architecture/model-discovery-and-catalog.md); **1 weak gap** (PR #11) — resolved as MAPPED-by-pattern, recorded below.

## Mapping table (abridged — full agent tables reproduced at bottom)

| Cluster | PRs | Canonical doc(s) |
|---|---|---|
| Phase-event stream | #1 | ADR-0020 |
| Dynamic routing / advisor | #4, #14–#17, #32, #50, #51, #56 | ADR-0024, `dynamic-phase-routing.md`, `ai-driven-routing-2026-06-03.md` |
| Live injection / channel | #5, #59 | ADR-0023, ADR-0036/0037, `bidirectional-channel.md` |
| Artifact tolerance | #6, #7 | ADR-0024 Step 0 + cycle-108 research |
| Commit/ship integrity | #10, #19, #18, #42 | ADR-0012, ADR-0027 (commit-as-evidence), `audit-constitution.md` |
| Adversarial testing | #12 | ADR-0025 |
| User-defined phases | #13, #58 | ADR-0028, ADR-0035, `user-defined-phases.md` |
| Cycle-119 incident set | #20–#23 | `docs/incidents/cycle-119-artifact-timeout-and-cross-cli-trust.md` (+ ADR-0029) |
| Multi-CLI bridge | #24, #26, #27 | ADR-0029, ADR-0031, ollama/codex research dossiers |
| Observer | #25 | ADR-0030 |
| Swarm | #29 | ADR-0032 |
| Unified config / step 9 | #30, #41, #43, #44, #46, #47 | `policy-config.md`, `step9-llm-config-removal.md`, `setup-onboarding.md` (ADR-0027-setup) |
| Model catalog | #31 | **was the gap** → `model-discovery-and-catalog.md` (written 2026-06-05) |
| Test architecture | #8, #33, #35, #36, #40 | `testing-strategy.md` |
| Verdict/deliverable contracts | #53, #54, #55, #60 | ADR-0033, ADR-0034, `deliverable-contract.md`, `knowledge-base/research/verdict-and-gate-proxy-failure-class-2026-06-03.md`, `docs/superpowers/specs/2026-06-04-orchestrator-contract-correction-retry-design.md` |
| Cycle reset | #9 | `auto-resume.md` |
| N/A (test/CI/revert/non-arch) | #2, #3, #34, #38, #39, #45, #49, #52, #57, #61, #62 | — |

## The two gap dispositions

### PR #31 (model catalog) — GENUINE GAP, now closed

The catalog is *dispatch-authoritative* (`tier → model` for every phase launch) yet was only referenced as a consumer dependency in `policy-config.md` / `step9-llm-config-removal.md`; nothing documented its schema, per-CLI discovery, LLM tier classification, live-vs-detect provenance rule, TTL/refresh, or fail-open fallback. **Resolution:** [docs/architecture/model-discovery-and-catalog.md](../../docs/architecture/model-discovery-and-catalog.md) (request → approaches → chosen solution, with file:line anchors and the Step-10c status corrected: auto-refresh + dispatch overlay are implemented, not deferred).

### PR #11 (builder Output-template cold-move) — WEAK GAP → MAPPED by pattern

PR #11 applies the **established cold-move pattern** documented across ADR-0013…ADR-0016 (token-optimization campaign Stages 6–9): move a cold-path persona section to the Layer-3 reference doc, leave a one-line pointer. The PR body itself cites the convention (mirrors ADR-0014's `posthoc-enforcement` pointer). The *specific* move is recorded here as the campaign-ledger entry an auditor would look for:

| Move | File | Lines | Mechanism |
|---|---|---|---|
| `## Output` section (build-report template + Ledger-Entry JSON) | `agents/evolve-builder.md` → `agents/evolve-builder-reference.md#output-template` | 328 → 264 (−64, ~20%) | byte-faithful relocation; pointer convention per ADR-0014 |

No new ADR warranted — a routine application of an accepted decision. If a future cold-move *changes* the pattern (e.g. dynamic loading), that's the moment for ADR-0038.

## Corrections to the audit agents' raw output

- Agent 2 claimed ADR-0033/0034/0035 "numbered files don't exist yet" — **false**; `docs/architecture/adr/0033…0035-*.md` all exist on disk. Coverage is stronger than that agent reported.
- Agent 1 proposed a `token-optimization-roadmap.md` for PR #11 — declined; ADR-0013–0016 already carry the pattern, and a roadmap doc would duplicate them (DRY for docs).

## Recommendation adopted

PR descriptions in this repo generally already cite their ADR (e.g. "#58 … (ADR-0035)"). Keep doing this — the two gaps were precisely the PRs whose bodies cited no doc.

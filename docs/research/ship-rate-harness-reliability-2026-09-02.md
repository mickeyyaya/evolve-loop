# Ship-rate hardening for weaker-model phases — synthesis and ranked proposals

**Date:** 2026-09-02 · **Plane measured:** `runtime/` (loop paused after cycle 1606) · **Companion files:** [literature survey with 60 sources](ship-rate-harness-reliability-2026-09-02-sources.md) · [dashboard UI patterns](pipeline-dashboard-patterns-2026-09-02.md) · **Design:** [spec](../superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md)

> **Question asked.** Why do so few cycles ship, and what can the *harness* do so that a build phase running on a lower-capability model at lower reasoning effort still does exactly what was asked?
>
> **Answer in one paragraph.** The pipeline verifies only at the end (audit) and repairs by repeating — same model, same effort, same prompt shape — while dropping the auditor's actual finding and keeping the acceptance contract two file-hops away from the builder's prompt. The literature says that combination cannot converge for weaker models: they do not self-correct without external, specific, execution-grounded feedback; repair gains concentrate in the first two rounds and then require changing the *inputs* (feedback, context, or tier); and the agent-computer interface moves success as much as the model does. The measured data agrees: ship probability is 100 % after one audit round and 0 % after four. The fixes are mostly plumbing that already has a declared home in config.

---

## 1. Measured state

All numbers from on-disk artifacts, joined by the outcome-taxonomy pass (cycles 1560–1606; run dirs exist for 1589–1606, earlier cycles from committed dossiers + ledger).

| Metric | Value | Source |
|---|---|---|
| Ship rate, 46 closed cycles (1560–1605) | **9 / 46 = 19.6 %** — SLO is ≥ 60 % (`internal/cyclehealth/outcome.go:20`, a comment; nothing computes the rate) | ledger `role=ship` `git_head` ⟷ dossier `final_verdict` (PASS ⟺ shipped, 9/9) |
| Zero-ship streak | 1595–1605 = **0 / 11**; last autonomous ship `20e839ee` 2026-08-31 | `recent-outcomes.md` |
| Failing phase | audit **30 / 37 (81 %)**, triage 4, tdd 1, build 1, ship 1 | dossier `failure.fingerprint` |
| Deterministic pre-class | verdict-fail 15 · gate-block 10 · infra-error 9 · unknown 2 | `failure-digest.json` |
| Ownership of recent failures | `legit-rejection` claimed 10/14, but `root_cause.layer` is `pipeline-code`/`harness` for 6 of them; `level=system` for 7/14 | `disposition.json`, `failure-decision.json` |
| Ship probability by audit rounds | **1 → 100 % (1/1) · 2 → 50 % (1/2) · 3 → 17 % (1/6) · 4+ → 0 % (0/2)** | `audit_dispatches` × outcome |
| Build routing on every repair round | `codex-tmux` · tier `balanced` · `effort_level: "medium"` — unchanged across rounds | `llm-calls.ndjson`, `.evolve/profiles/builder.json` |

### Root-failure buckets, nine most recent FAILs

| Bucket | Cycles (primary) | Also contributing |
|---|---|---|
| (f) gate false-RED / pipeline defect | 1603, 1601, 1596, 1595 | |
| (g) infra / CLI transport | 1598, 1599, 1600 (one fingerprint, three cycles) | |
| (a) builder over-claim (`Closes-Inbox` on a one-fifth-met item; evidence paths that resolve to no file) | 1604 | 1605, 1597, 1595 → **4 / 9** |
| (c) new export with zero production callers | 1605 | 1601 |
| (d) tests/evals not bound (false-green, untagged, feature deletable with suite still green) | 1602, 1597 | |
| (e) attestation SHA ≠ base-bound diff | — | 1603, 1604, 1605 → **3 / 9** |
| (i) repair-round state contamination (round N inherits round 1's artifact) | 1595, 1596, 1603 | fixed for one vector by #529 |

Where a defect was re-cited across rounds, the builder's response was **narrative compliance without the substantive remedy** in every observed case: 1604 deleted the `Closes-Inbox:` line but never queued the remainder; 1605 edited one paragraph of the explanation doc and added no production caller; 1606 M3 "round 1 issued this exact line as a required correction and it was not applied."

## 2. Verified architectural gaps

Each gap was confirmed by reading the cited source, not inferred from behaviour.

| # | Gap | Evidence |
|---|---|---|
| **G1** | **Escalation declared but unreachable.** `builder.json` declares `model_tier_overrides.audit_retry_2plus: "deep"`; `subagent/modeltier.go:166` `activeSituation` returns `""` for every cycle > 1; `subagent/run.go:266` never passes the repair count. Three tests guard the key's value, none that it fires. | source read 2026-09-02 |
| **G2** | **Auditor findings never reach the builder.** `core/repair_eligibility.go:81` seeds the brief from `audit-fail-reason.json` (gate strings) only; `audit-report.md` HIGH findings with `path:line` are dropped. 1605 H1 survived 3 rounds; 1596's round-4 builder received one truncated defect. | source read + `audit-report.round2.md:109` |
| G3 | Acceptance criteria two hops from the prompt (slug only; `triage-report.md` → `.evolve/evals/<slug>.md`). | `build-prompt.txt:106` |
| G4 | tdd→build contract is free-text `agent-mailbox.md`; ACS predicate names absent from the prompt. | `build-prompt.txt` |
| G5 | All deterministic checks run after the full build budget, at audit. | `phases/audit/audit.go:296-400` |
| G6 | Learning loop write-only: `instinctSummary` has no Go producer; `.evolve/genes/` empty; ~650 lessons re-enter through an optional grep. | grep |
| G7 | `evalgate` fails open on ambiguity. | `evalgate/reviewer.go:11-18`, 1605 H2 |
| G8 | No capability-aware prompt adaptation; `internal/capability` models transport only. | `capability.go`, `qualitytier.go` |
| G9 | Phase order in three places. | `phaseorder.go:17`, `router.go:18`, `phase-registry.json` |
| G10 | Repair-round prompts overwritten in place (only verdict/report archived). | run dir listing |
| G11 | `policy.json` advertises pins it does not contain. | file |

## 3. What the evidence says the harness should do

Mapped from the literature survey (labels: [P] paper with numbers, [D] vendor doc/engineering post; URLs in the sources file).

| Principle | Strongest evidence | Consequence for evolve-loop |
|---|---|---|
| Interface design moves weak-model success as much as model choice | SWE-agent ablations: 18.0 % vs 11.0 % shell-only; lint-on-edit +3 pp; summarized feedback +6 pp [P] https://arxiv.org/html/2405.15793 | Put the contract *in* the prompt; give exact, consolidated gate commands; return specific, actionable gate text |
| Models do not self-correct without external signal; feedback quality is the bottleneck | Huang et al. [P] https://arxiv.org/abs/2310.01798 · Olausson et al. (stronger feedback model → "substantially larger gains") [P] https://arxiv.org/abs/2306.09896 · Self-Debug: unit-test feedback up to +12 pp vs +2–3 pp explanation-only [P] https://arxiv.org/abs/2304.05128 | The auditor (deep tier) must author the finding the builder receives, verbatim with `path:line` and failing-test output — not a paraphrase and not only gate strings (G2) |
| Split reasoning from editing | Aider architect/editor: o1-mini 61.1 → 71.4 %; every model improved with a strong planner + cheap editor [D] https://aider.chat/2024/09/26/architect.html | Strong triage/tdd/audit + cheap build is the right shape *if* the plan/finding is precise; today the finding is lost in transit |
| Cap repair at ~2 rounds, then change inputs | "How many tries" — gains concentrate in the first two rounds [P] https://arxiv.org/abs/2604.10508 · cascades escalate on weak-model failure at ~40 % cost [P] https://arxiv.org/abs/2310.03094 · FrugalGPT [P] https://arxiv.org/abs/2305.05176 | Round ≥ 2 must escalate tier/effort (G1) or restart with a rewritten task in fresh context |
| Completion must be minted by the harness from evidence | Anthropic long-running harness: `passes` flipped only after testing; "unacceptable to remove or edit tests" [D] https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents · Kalai et al.: binary grading rewards guessing [P] https://arxiv.org/abs/2509.04664 · "Confident and Wrong": 100 % submit, 44 % resolve; failures are repeated misinterpretation, not noise [P] https://arxiv.org/abs/2603.25764 | `Closes-Inbox` written by a checker, never by the builder; a first-class honest `partial` outcome |
| Deterministic gates beat advisory instructions | Claude Code: "hooks are deterministic … CLAUDE.md is advisory"; Stop hook blocks until green [D] https://code.claude.com/docs/en/hooks | Move cheap gates to the build exit and re-enter build on RED (G5) |
| Treat the agent as adversarial to the verifier | Berkeley RDI: a 10-line `conftest.py` "resolves" 100 % of SWE-bench Verified; isolate the eval environment [D] https://rdi.berkeley.edu/blog/trustworthy-benchmarks-cont/ | Gates read the committed tree in a worktree the builder cannot write |
| Scope drift is a framework property | Permissive frameworks 5.4–27.7 % out-of-scope actions vs 0.2–4.5 % ask-to-continue [P] https://arxiv.org/abs/2605.18583 | File allowlist from the plan enforced by the harness, not by prose |
| Best-of-N works only with a real verifier | DeepSeek-Coder-V2 15.9 % → 56 % with unit-test selection; voting plateaus [P] https://arxiv.org/abs/2407.21787 · verifier-free scaling suboptimal [P] https://arxiv.org/abs/2502.12118 | Worth trying for build only once ACS predicates are trusted end-to-end; not before |
| Effort dials are non-monotonic for small models | scoring study: nano/mini best with no reasoning [P] https://arxiv.org/abs/2604.26954 | Measure effort per phase/tier before pinning; the dashboard is the instrument |

## 4. Ranked proposals

Ranking = (impact on the measured failure buckets × confidence the mechanism transfers) ÷ implementation cost. S < 1 day, M 1–3 days, L > 3 days.

| # | Proposal | Closes | Cost | Status |
|---|---|---|---|---|
| **R1** | **Arm repair-round escalation** — `core.repairRoundTier` raises a tdd/build re-dispatch inside an audit-repair round to the profile's declared `audit_retry_2plus` tier at the ADR-0076 D seam (the production tier path — the `subagent.ResolveModelTier` override table is not on it); `effort_overrides` keyed by resolved tier make effort follow. | G1; the 3-round grind | S | **landed — ADR-0096** |
| **R2** | **Repair brief carries the auditor's findings + reflection memory** — `core.composeRepairBrief` appends the rejecting round's findings (`reportdoc.Findings`, CRITICAL/HIGH/MEDIUM) to the gate reasons, marks findings that persisted from the previous round (`reportdoc.FindingKey`), and archives `<phase>-prompt.round<N>.txt`. | G2, G10 | S–M | **landed — ADR-0096** |
| R3 | **Build-exit deterministic floor** — apicover, explanation-SHA, Closes-Inbox proof, production-caller scan, clean-tree check at build→audit; RED re-enters build (≤ 2 micro-rounds) without an audit dispatch. Extend `buildGraduationCheck`. | G5; buckets (a)(c)(d)(e) | M | inbox 0.90 |
| R4 | **Inline the contract** — acceptance criteria verbatim + ACS predicate names (`go test -list`) + schema-validated `handoff-tdd.json`. | G3, G4 | M | inbox 0.88 |
| R5 | **Completion minted by the harness** — per-criterion `{id, status, evidence}`; checker writes `Closes-Inbox`; honest `partial` routes to carryover without penalty. | bucket (a) | M–L | inbox 0.88 |
| R6 | **Capability-aware scaffolding** — per-tier `prompt_scaffold`; numbered DoD with evidence lines for balanced/fast tiers; measured effort per phase. | G8 | S–M | inbox 0.80 |
| R7 | **Learning re-entry** — produce `instinctSummary` (top-k lessons by file/keyword overlap) and inject where personas already look. | G6 | M | inbox 0.78 |
| R8 | **Verifier isolation audit** — enumerate builder-writable seams the gates read; close them. | bucket (e) | M–L | inbox 0.75 |
| — | Durable outcome ledger | — | — | **dropped**: `knowledge-base/cycles/cycle-N.json` already carries verdict + ship SHA + fingerprint (1840 files); `evolve dashboard` computes the rate from it |

## 5. Instrumenting the SLO

Nothing computed the ship rate before this work. `evolve dashboard` (ADR-0095) reads the committed dossiers and renders: ship rate last-20 / last-50 / all-time, per-cycle verdict strip, audit-round count per cycle, failure-category and fingerprint recurrence tables (first/last seen, regressed). The ≥ 60 % target in `cyclehealth/outcome.go` now has a live reading. Convergence of the repair loop is visible as the round histogram; if R1+R2 work, the 3-round column should stop being a graveyard.

## 6. Skeptic's notes

- Ship-rate-by-build-CLI is confounded by task difficulty (the claude-build cycles all drew the same hard carryover); do not read it as a model signal.
- Several 2026 preprints cited are arXiv-only; their direction is consistent across papers but individual percentages are indicative.
- FrugalGPT/MoT numbers are from non-code tasks; the *mechanism* (escalate on failure) transfers, the figures do not.
- Best-of-N is deferred deliberately: with `evalgate` failing open (G7) and 1597's feature-deletable-with-suite-green, the verifier is not yet trustworthy enough to select among samples.

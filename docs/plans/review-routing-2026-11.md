# Plan: every change class gets its review seats and checks from one table

- **Status:** proposed, 2026-11-03.
- **Research:** [review-routing-2026-11](../research/review-routing-2026-11.md). **Decision set:** [architecture-decisions-2026-11](architecture-decisions-2026-11.md).
- **Convergence rule:** [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) and [the review loop index](../architecture/review-loop-and-quality-index.md). **Commit gate:** [package note](../architecture/packages/internal-commitgate.md). **Protected surface:** [guards package note](../architecture/packages/internal-guards.md) and `go/internal/guards/integrity_surface.go`.
- **Related ADRs:** [ADR-0124](../architecture/adr/0124-code-review-phase.md) (lane code-review phase) and [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md) (held proposals).

**Terms used throughout:**

- **Seat:** one review run by one reviewer persona.
- **Change class:** a name for a kind of diff.
- **T0:** the protected surface.
- **Open blocking:** open CRITICAL plus open HIGH findings.
- **Stage:** `off`, `shadow` or `enforce`.

## 1. Request

The operator asked on 2026-11-03:

> "Are those code changes applied full review before sending to PR?"

> "I want to make sure the process is defined what changes will always triggering particular review and fix all the needed defect and issues"

The answer to the first question is no (F2.8, F2.9 of the research). This plan defines the rule and the code that enforces it.

## 2. Facts

| Fact | Research id |
|---|---|
| The gate requires two seat names and trusts plain tokens. | F2.1, F2.4 |
| A WARN verdict with open HIGH passes the gate. | F2.5, F2.6 |
| The gate never reads the protected manifest. | F2.7, F2.18 |
| The rule "architecture-reviewer in parallel" is text only. | F2.8 |
| Main holds 0 review trailers in 547 console commits. | F2.9 |
| The persona strip check is missing (H9 incident). | F2.14 |
| A code-reviewer seat passed an inbox commit that deleted a pending item (2026-11-03). | F2.15 |
| The adversarial seat has `allowed: claude` only on 2026-11-03. | F2.16 |
| The lanes are stricter than the console on T0. | F2.17, F2.19 |
| The gate, CI and `gateclass` hold three class definitions. | F2.13, F2.17 |

## 3. Goals and non-goals

**Goals.**

- G-1: one change-class table decides the seats and the checks. The commit gate, CI and the lane code-review phase read it.
- G-2: a protected-surface change always gets the full seat set before it lands.
- G-3: a seat counts only with 0 open CRITICAL and 0 open HIGH on the final tree.
- G-4: an inbox-data-only commit gets deterministic checks and no LLM seat.
- G-5: main records which seats ran, so CI can check it.
- G-6: the adversarial and security seats run on a family other than the author's, or the record states the fallback.

**Non-goals.**

- No ADR and no spec edit in this change. The ADR comes with the decision set (`docs/plans/architecture-decisions-2026-11.md`).
- No feature flag. One policy key, `review_routing.stage`, selects the stage.
- No GitHub teams or CODEOWNERS layer (research §4, option B).
- No new human approval step, except the stage flip.

## 4. Candidate designs

| Criterion | A matrix registry | B CODEOWNERS and statuses | C skill checklist | D risk score |
|---|---|---|---|---|
| One class definition for gate, CI and lanes | **5** | 2 | 1 | 3 |
| Enforced without console discipline; blocks on HIGH | **5** | 3 | 1 | 4 |
| LLM cost balance | 4 | 3 | 3 | **5** |
| Build size | 3 | 3 | **5** | 3 |
| Reversibility | **5** | 3 | 5 | 4 |
| Fit with the project rules | **5** | 2 | 2 | 3 |
| **Total** | **27** | 16 | 17 | 22 |

A wins. It fixes the root cause: three places define the classes (F2.13, F2.17). B puts a GitHub token in an LLM cell and adds a fourth classifier. C is the rule that failed (F2.8). D uses thresholds that drift, and it cannot see the inbox or persona kinds. The full scoring is in [research §4](../research/review-routing-2026-11.md#4-options-and-scores).

## 5. Decisions

| Id | Decision | Research id | Rejected |
|---|---|---|---|
| D1 | One protected Go registry `reviewclass` maps class to seats and checks. The gate, CI and the lane code-review phase read it. Docs and the commit skill are generated from it. The stage key is `review_routing.stage`: shadow, then enforce. | F2.1 to F2.7, F2.13, F2.17 | Table in `policy.json`: a larger schema surface, no gain. |
| D2 | A protected-surface change gets the full seat set, then lands normally. The full set is the seat list of the C11 row in section 6. The operator approves only stage flips. | F2.17 to F2.19 | A held proposal for each T0 console commit: safest, slowest. |
| D3 | A seat counts with 0 open CRITICAL and 0 open HIGH on the final tree. ADR-0126 sets the rounds. The commit gate has no round 4. The code-review phase of a lane keeps its own budget (ADR-0124). The change escalates as a held record. | F2.5, F2.6, F2.21 | Block only CRITICAL: today's behavior, and the cause of F2.6. |
| D4 | An inbox-data-only commit gets deterministic checks only: `inbox lint` and `inbox integrity`. No LLM seat. | F2.12, F2.15 | Keep one code-reviewer: it passed the bad commit. |
| D5 | The adversarial and security seats route to a family other than the author's. Routing data holds the rule. While one family is available, the fallback is the deep tier of the same family, and the record says so. | F2.16, F2.19 | Hard-code a family in the gate: it breaks the routing table. |
| D6 | The rules of the table: R1 a diff gets the union of the seats and checks of every class it touches. R2 the highest tier wins for shared seats. R3 the floor only rises: a seat or a size rule can raise a class and never lower it. R4 the waiver is narrow: a proven comment-only change, or a generated region whose `derived` check passes. | F2.2, F2.3; P3.1 | Whole-diff single class (today): it lets a mixed diff dodge a seat. |

## 6. The change-class matrix

This is the final table. The registry holds it as data. Each row has a test that pins it.

| # | Class (how it is detected) | Required seats | Required deterministic checks | Cap |
|---|---|---|---|---|
| C0 | Comment-only, proven (`commentaudit.Equivalent`). | none | comment equivalence (exists) | $0 |
| C1 | Docs: `*.md` outside `agents/`, `skills/`, `guidance/`. | `docs-review` | `ste-lint --strict --added`; `docs index check`; committed-doc deletion guard | $1 |
| C2 | Inbox data: `.evolve/inbox/**`. | none | `inbox lint`; new `inbox integrity`: each removed pending item has a consumed, claimed or moved copy, or the commit names it in `Closes-Inbox` | $0 |
| C3 | Generated outputs: each `derived.Catalog()` output. | none for the generated hunks | the `Check` verb of that entry | $0 |
| C4 | Gate inputs: `go/.apicover-enforce`, `go/.cover-strict`, `go/.darwin-set`, `go/.test-latency.json`. | `code-reviewer` | floor-list regression (the logic of `core/floor_lists_regression.go`): no package dropped, no floor lowered | $1 |
| C5 | Go tests only: `*_test.go`, `testdata/`; not a guard test. | `go-reviewer` with the `golang-test-review` lens | gofmt, vet, lint, `go test` of changed packages; `gateclass` flags `removed-guard-test` and `added-skip` are 0 | $1 |
| C6 | Go T2 (ordinary code). | `code-simplifier` and (`go-reviewer` or `code-reviewer`) | lanes; `cover-changed` of the changed packages; apicover | $2 |
| C7 | Go T1: `core`, `bridge`, `cliroute`, `gc`. | C6 seats and `architecture-reviewer` | C6 checks | $4 |
| C8 | Persona or prompt: `agents/*.md`, `skills/**`, `guidance/rules/**`. | `architecture-reviewer` and `docs-review` | new `persona strip` check; `skills check`; `guidance check`; ste-lint | $3 |
| C9 | Manifests and profiles: `.evolve/profiles/*.json`, `.evolve/phases/*.json`, CLI manifests, `.claude-plugin/`. | `code-reviewer` and security seat | profile load; `skills check`; permission-widening trace: a wider `allowed_tools`, `write_subpaths` or `env_passthrough` names a consumer | $2 |
| C10 | CI workflows: `.github/workflows/**`. | `code-reviewer` and `architecture-reviewer` | each new job is in the `required` aggregator `needs`; the path routes are generated from the registry | $3 |
| C11 | T0 protected: `guards.IsProtectedSurface`. | C7 seats; adversarial review on another family at the deep tier; security seat when the guarantee class is sandbox, dispatch-credential or policy | all C6 checks; `gateclass.Classify` flags are 0 or each is acknowledged by `Gate-change: <pattern> <reason>`; diff-scoped mutation of the changed guard lines, with the signal sender injected | about $5.6 to $6.8 |
| C12 | Stage flip: `gateclass` flag `stage-off-enforce` in any policy file. | C11 seats and an operator decision record | refuse unless the commit has `Operator-approved: <item>` | C11 |
| C13 | Default: a changed path that matches no other class (for example `Makefile`, `go.mod`, a shell script or `knowledge-base/**`). | C6 seats | C6 checks; the `reviewclass` table test fails when a tracked path matches no class and the default is not listed for it | $2 |

**Finding rule for every class with a seat.**

- **F1:** a seat run counts when its report has 0 open CRITICAL and 0 open HIGH. The bridge writes the field `open_blocking` in the launch record from the fixed verdict block of the report. The gate reads the field, not the prose.
- **F2:** a run counts only on the final tree. `BoundTo(tree)` already enforces this. A fix changes the tree, so a fix needs a new review.
- **F3:** rounds follow ADR-0126. Round 3 blocks on HIGH and above, and MEDIUM and LOW go to the inbox. At round 3 the change can also accept a certified fail-safe residue or split. If a HIGH stays open after round 3, the change becomes a held record for the operator. There is no round 4.

**Worked example (the 2026-11-03 case).** The commit deletes `.evolve/inbox/pending/x.json`. The registry finds class C2 and requires no seat.
`inbox integrity` compares the pending set of the base and the head.
It finds no consumed copy and no `Closes-Inbox` line, so the gate refuses.

A mixed diff adds a `go/internal/guards/` edit to this deletion. By R1 it is C2 and C11. It gets the C11 seats and both checks.

## 7. Components

Size is an estimate (synthesis). Stage is the value of `review_routing.stage` at which the component acts. "Console" means the console builds it. The item touches the protected surface, and ADR-0157 decision 8 lets a lane work such an item only as a held proposal. A lane takes an item outside the protected surface.

| Id | Component | Files | Stage | Size | Proving test | Route |
|---|---|---|---|---|---|---|
| S0 | Find why console commits carry no `Reviewed-by` trailer. Make each console commit carry `Review-class` and the `run:` ids. | `go/internal/phases/ship/commitgate.go` | none | S, about 60 LOC | A manual ship test: the commit message has both trailers (red before the fix). | Console: ship is a protected gate. |
| S1 | `reviewclass` registry, `evolve review-class [--base B] [--json]`, and the manifest entries of G5. | `go/internal/reviewclass/`, `go/cmd/evolve/`, `go/internal/guards/integrity_surface.go` | data only | M, about 250 LOC | Table test: one path set per class gives the expected seats and checks, with R1 for a mixed diff. A path that matches no class gets C13. | Console: it edits the protected manifest. |
| S2 | Deterministic checks: `inbox integrity`, `persona strip`, permission-widening trace. Each is an `evolve` verb. | `go/internal/inbox/`, `go/internal/prompts/`, `go/cmd/evolve/` | shadow | M, about 300 LOC | Inbox: base has `pending/x.json`, head deletes it with no consumed copy, so refused. Persona: a directive under `## Reference Index` is refused (H9). | Lane: no protected path. Console if a verb file is protected. |
| S3 | `launchrecord` gains `open_blocking`, `duration_ms` and `tokens`. The bridge parses the fixed verdict block. | `go/internal/launchrecord/`, `go/internal/bridge/` | none | S, about 80 LOC | A report with one open HIGH gives `open_blocking=1`. | Console: launchrecord is protected. |
| S4 | The commit gate reads `reviewclass.Requirements`. At shadow it emits `REVIEW_ROUTING_SHADOW`. At enforce it refuses. A plain token no longer satisfies a seat at enforce. | `go/internal/commitgate/`, `go/internal/policy/strict_keys.go` | shadow | M, about 150 LOC | At enforce, a T0 sandbox diff with only simplifier and code-reviewer runs is refused, and the refusal names `architecture-reviewer`, `adversarial-review` and `security`. A T0 diff outside the sandbox, dispatch-credential and policy classes gets no `security` in the refusal. A WARN run with `open_blocking=1` is refused. | Console: gates and policy parser are protected. |
| S5 | CI job `review-class` in the `required` aggregator. `--ci-routes` replaces the bash `case`. | `.github/workflows/required.yml`, `go/cmd/evolve/` | shadow (job warns) | S, about 40 lines of YAML and 40 LOC | Parity test: the CI route output equals `reviewclass` for every path in the tree. | Console: workflows join the protected set at S1. |
| S6 | Diff variants of the seats `security` and `adversarial-review`: two profiles, two personas. | `.evolve/profiles/diff-security-reviewer.json`, `.evolve/profiles/diff-adversarial-reviewer.json`, `agents/evolve-diff-security-reviewer.md`, `agents/evolve-diff-adversarial-reviewer.md` | none | S | Profile load test; the persona strip check passes on both. | Console: reviewer personas join the protected set at S1. |
| S7 | The lane `code-review` phase and the held-proposal gate read the same requirements (the ADR-0124 triggers). | `go/internal/core/`, `go/internal/phases/ship/` | shadow | M, about 150 LOC | A lane diff in class C7 triggers `architecture-reviewer`. | Console: core and ship are protected. |
| S8 | `docs/architecture/review-routing.md` with a generated table region. Edit the lines `CLAUDE.md:52` and `AGENTS.md:53,302-304` to point at it. | `docs/architecture/review-routing.md`, `go/internal/derived/`, `AGENTS.md` | none | S | `derived` check: the region equals the output of `evolve review-class --json`. | Console: `AGENTS.md` is the kernel instruction. |
| S9 | The operator flips `review_routing.stage` to enforce after one wave boundary of shadow records with no false refusal. | `.evolve/policy.json` | enforce | none | The shadow records show 0 false refusals. | Operator: C12 stage flip. |
| S11 | A diff-scoped mutation runner for the changed guard lines of a C11 change: `evolve mutate --changed-guards --base B`. It injects the signal sender, so a kill-guard mutant never sends a real signal. | `go/internal/mutation/` (new), `go/cmd/evolve/` | shadow | M, about 200 LOC | A mutant that deletes a changed guard condition survives the old tests. The new test kills it. A kill-guard mutant runs with the fake sender only. | Console: guards and kill paths are protected, and the HIGHEST rule on live kill guards applies. |
| S10 | Family routing for the adversarial and security seats (D5). | the `cli_routing` block of `.evolve/policy.json` (routing data), `go/internal/cliroute/` only if a rule is missing | none | S | `evolve cli-routing explain adversarial-review` shows an allowed family other than the author's, or prints the stated fallback. | Console: the routing table decides which model sees the code. |

## 8. Order

1. S0 and S3 first. They add the record that the other steps read. Neither changes behavior.
2. S1 next. It makes the table exist and puts the reviewer files in the protected set before S6 adds more.
3. S2, S6, S10 and S11 in parallel. They add checks and seats. Nothing enforces them yet.
4. S4 and S5 at shadow. The gate and CI report the refusals that they will make at enforce.
5. S7 and S8. The lanes and the docs read the table.
6. S9 last. The operator flips the stage after shadow records show no false refusal.

## 9. Verification

| Goal | How we measure it |
|---|---|
| G-1 | The parity test of S5 and the table test of S1 pass. The bash `case` is gone. |
| G-2 | The S4 test refuses a T0 diff with fewer seats. `evolve review-class --base origin/main` on a T0 commit lists the full set. |
| G-3 | The S3 and S4 tests refuse a WARN run with `open_blocking=1`. |
| G-4 | The S2 test refuses the 2026-11-03 deletion. A C2 diff needs no seat. |
| G-5 | The commits of a wave after S0 all carry `Review-class`. Count with `git log --grep`. |
| G-6 | `evolve cli-routing explain adversarial-review` output after S10. |
| Cost | `evolve review-class --cost` reports measured seat cost per class after S3. Compare with the estimate of [research §7](../research/review-routing-2026-11.md#7-limits-and-cost). |

## 10. Risks

| Risk | Control |
|---|---|
| False refusals block honest commits at enforce. | Shadow first. S9 needs one wave of records with no false refusal. |
| The console forges a trailer. | CI closes the "forgot" gap only. The tree binding of `run:` tokens stays local. This is a stated limit. |
| T0 cost rises by about $2.8 on average for each T0 commit. Today a T0 commit costs about $2 (two seats), and the full C11 set caps at $5.6 to $6.8. | The inbox class saves about $108 per two weeks (54 inbox commits). The net is about +$200 per two weeks at cap (synthesis, research §7). |
| One family is the only one available (F2.16). | D5 states the same-family deep-tier fallback, and the record names it. |
| The registry itself is a protected file that a bad edit can weaken. | S1 puts `go/internal/reviewclass/` in the manifest. A change to it is class C11. |
| A path floor misses semantic risk. | Rule R3: a seat or a size rule can raise a class. |

## 11. Open questions

- **Q1:** what is the line count for the size rule G1? The research gives no number. The shadow data of S4 sets it.
- **Q2:** which second family serves the adversarial seat? The answer is routing data (S10). It depends on the families that the operator enables.
- **Q3:** does S2 stay a lane item? It does when no verb file is in the protected manifest. S1 settles this.

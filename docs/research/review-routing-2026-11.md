# Review routing: which change gets which review (research, 2026-11)

> The plan that uses this research: [review-routing-2026-11](../plans/review-routing-2026-11.md). The decision set: [architecture-decisions-2026-11](../plans/architecture-decisions-2026-11.md). The convergence rule: [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md). The commit gate: [package note](../architecture/packages/internal-commitgate.md).

Date: 2026-11-03. Base: `origin/main` at `dc7f1085f`. Method: read the code on main, count the console commits of the last two weeks, and search the web on 2026-11-03. Each local claim names its file or command. "Synthesis" marks our own inference. This study made no repository writes.

**Terms used throughout:**

- **Seat:** one review run by one reviewer persona, for example `code-reviewer`.
- **Change class:** a name for a kind of diff, found from its paths and its content.
- **T0, T1, T2:** the surface tiers of `gateclass.TierOf`. T0 is the protected surface.
- **Open blocking:** the count of open CRITICAL and open HIGH findings in a seat report.

## Table of contents

1. Request
2. Local evidence
3. Outside practice
4. Options and scores
5. Winner and grafts
6. Operator picks
7. Limits and cost
8. What changes our mind

## 1. Request

The operator asked on 2026-11-03:

> "Are those code changes applied full review before sending to PR?"

> "I want to make sure the process is defined what changes will always triggering particular review and fix all the needed defect and issues"

Short answer to the first question: no, not by rule. The commit gate requires two seats for each code change. The rest of the fleet runs only when the console remembers to run it. Main keeps no record of what ran (F2.9).

## 2. Local evidence

### Commit gate (console path)

**F2.1** The gate requires one "simplify" token and one "review" token. One `code-review-simplify` token satisfies both (`go/internal/commitgate/reviewers.go:8-30`).
*Implication:* the gate checks two seat names and no more.

**F2.2** A change is docs-only when every path ends in `.md`. It then needs `docs-review` (`go/internal/commitgate/docs_review.go:10-33`). One `.json` file in the diff sends the change back to the code seats.
*Implication:* the class is a property of the whole diff, with no union rule.

**F2.3** A Go file that `commentaudit.Equivalent` proves comment-only needs no reviewer (`go/internal/commitgate/comment_only.go:16-48`).
*Implication:* a narrow waiver already exists and works. Keep it.

**F2.4** `resolveReviewRuns` checks only the tokens that start with `run:`. Every other token passes as a name, with no proof that it ran (`go/internal/commitgate/review_runs.go:31-46`).
*Implication:* a plain token can claim a seat that never ran.

**F2.5** A `run:` token must be a bridge launch, a reader, with a report, and it must have seen the staged tree. Its verdict must be PASS or WARN (`go/internal/commitgate/review_runs.go:50-66`).
*Implication:* the tree binding is sound. The WARN pass is the weak point.

**F2.6** The architecture rubric maps HIGH to "Warn (fix before merge unless the change gives a reason)" (`agents/evolve-diff-architecture-reviewer.md:115`). The code reviewer gives "FAIL on CRITICAL; WARN on MAJOR" (`agents/evolve-diff-code-reviewer.md:74`).
*Implication:* a review with open HIGH findings satisfies the gate today.

**F2.7** The lanes of the gate are per language: gofmt, go vet, golangci-lint and `go test` of the changed packages (`go/internal/commitgate/lanes.go:78-125`). Before the lanes the gate refuses binaries, added comments and rewritten history (`go/internal/commitgate/comments_added.go:16-49`). Command: `git grep "guards\.\|IsProtected" -- go/internal/commitgate` returns nothing.
*Implication:* the gate does not know the protected surface. It has no coverage, STE, skills, derived-sync, inbox or persona check.

**F2.8** The written rule says "code-simplifier, then architecture-reviewer and code-reviewer in parallel" (`CLAUDE.md:52`, `AGENTS.md:53,302-304`). The gate never requires `architecture-reviewer` (F2.1).
*Implication:* the rule is text. No code enforces it.

**F2.9** Main holds 547 console commits since 2026-10-20 (no merges, no `evolve-cycle`). None has a `Reviewed-by:` or `Review-waived:` trailer. The last such trailer is from 2026-10-09 (`96cda0f9e`). The trailer code writes only on the `--class manual` path (`go/internal/phases/ship/commitgate.go:16-37`).

The console does run the gate. Recent `dev/*/.commit-gate/attestation.json` files show `run:` attestations, for example `{"reviewers_run":["code-simplifier","architecture-reviewer","code-reviewer"],"n_runs":3}`. The attestation stays in the worktree. CI and main cannot see it.
*Implication:* nobody can prove from main which seats ran.

### Seats and checks that exist

**F2.10** `evolve agent list` shows the diff seats `code-simplifier`, `code-reviewer`, `go-reviewer`, `architecture-reviewer` and `docs-review`. The cap of `diff-architecture-reviewer` is $2.0 and 30 turns. The others are $1.0 with 12 to 15 turns (`.evolve/profiles/diff-*.json`). All run on `codex-tmux`.

**F2.11** No diff seat exists for security or adversarial review. The cycle profiles `adversarial-review` (claude-tmux, deep, $0.4, 12 turns), `security-scan`, `authz-gap-scan` and `secret-leak-scan` (codex, balanced, $0.4, 12 turns) write into the cycle workspace. `AGENTS.md:303` names only the four diff reviewers.
*Implication:* the plan must add diff variants of these seats.

**F2.12** Deterministic checks exist but run in CI only:

- `make cover-changed` requires 100% of the changed functions (`.github/workflows/go.yml:53-62`).
- `evolve docs ste-lint --strict --changed --added`, `docs index check` and `guidance check` run in `.github/workflows/required.yml:138-166`.
- `inbox lint` runs in `.github/workflows/required.yml:110-136`. It loads only the pending items (`go/cmd/evolve/cmd_inbox.go:243-260`). It cannot see a pending item that a diff deleted.
- `derived.Catalog()` lists each generated output with its `Check` verb (`go/internal/derived/derived.go:39-84`).
- No deterministic mutation tool exists. `mutation-gate` is an LLM profile ($0.8, 15 turns).

**F2.13** The `changes` job in `.github/workflows/required.yml:64-91` sorts paths with a bash `case` into go, landing, docs and inbox.
*Implication:* this is a third path classifier, apart from the gate (F2.1 to F2.3) and `gateclass`.

### The two named incidents

**F2.14** `prompts.StripOnDemandSections` cuts everything from the first `## Reference Index` line (`go/internal/prompts/prompts.go:111-121`). 10 `agents/*.md` files have that heading. No general check exists. Round 1 of H9 put a directive below the marker. Only the architecture reviewer caught it, and that seat is optional (F2.8).
*Implication:* a directive below the marker never reaches the agent, and no gate sees it.

**F2.15** On 2026-11-03 a code-reviewer seat passed an inbox commit that deleted a pending item with no consumed copy. The console caught it from the commit stat before the push (operator report). No check compares the base and head inbox sets (F2.12).
*Implication:* an LLM seat is the wrong tool for this class. A set comparison finds it in milliseconds.

**F2.16** On 2026-11-03 `evolve cli-routing explain adversarial-review` prints `allowed: claude`, `chain: claude-tmux` and the note FLOOR.
*Implication:* the adversarial seat cannot run on another family today. A plan that needs "another family" must change the routing data, or state the stronger-tier fallback while one family is available.

### Lane (cycle) path

**F2.17** `gateclass.TierOf` returns T0 for `guards.IsProtectedSurface`, T1 for `go/internal/{core,bridge,cliroute,gc}/` and `agents/`, and T2 for the rest (`go/internal/gateclass/gateclass.go:33-76`). `gateclass.Classify` flags removed guard tests, added skips, changed thresholds, `stage` off to enforce flips, frozen checks and constitution paths (`go/internal/gateclass/gateclass.go:21-31`). The flags run in the cycle build floor and in ship (`go/internal/core/gate_classifier_floor.go`, `go/internal/phases/ship/gate_classifier_gate.go`), not in the commit gate.

**F2.18** The protected manifest has 206 fragments in 10 guarantee classes (`go/internal/guards/integrity_surface.go:13-24,37-246`). `IsProtectedScope` equals `IsProtectedSurface` (`:270-272`). `.github/workflows/` and the `agents/evolve-diff-*.md` and `diff-*.json` reviewer personas and profiles have no entry. Only `agents/evolve-code-reviewer.md` and `.evolve/profiles/code-reviewer.json` do (`:189,191`).
*Implication:* a change that weakens a reviewer is not a protected change today.

**F2.19** ADR-0157 decisions 8 and 9: a lane works a protected item only as a held proposal. The proposal gets an adversarial audit from another family at the deep tier ([ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md)). `held_proposals.stage` starts at shadow.
*Implication:* the lanes are stricter on T0 than the console. That order is backwards.

**F2.20** The lane `code-review` phase runs in every code cycle that touched files. ADR-0124 chose a lead reviewer with triggered specialists. The triggers do not exist yet ([ADR-0124](../architecture/adr/0124-code-review-phase.md)).

**F2.21** ADR-0126: round 1 is a normal fix. Round 2 changes the feedback. Round 3 changes the strategy, and the bar rises to HIGH. There is no round 4. The best round lands (`docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md:35-51`).

### Class mix of real console commits

**F2.22** A path-prefix count of the 547 console commits (synthesis). The commit list is in the scratchpad of the study. The rows count by prefix, so they overlap, and 4 commits match no row.

| Class | Commits |
|---|---|
| Go T0-like (protected prefixes, mixed ones included) | 113 |
| Go T1 | 81 |
| Go T2 | 104 |
| Go tests only | 56 |
| Other (94 touch `knowledge-base/cycles`; `go/.apicover-enforce` 32; `go/.cover-strict` 28) | 95 |
| Docs only | 40 |
| Inbox only | 35 |
| Docs plus inbox | 19 |
| Touch `agents/`, `skills/` or `guidance/` | 46 |

*Implication:* about a fifth of the console commits touch a protected surface. They get the same two seats as a typo fix.

## 3. Outside practice

Live search, 2026-11-03.

**P3.1** Path rules are the floor. A heuristic can raise the level and never lower it ([TradersPost](https://blog.traderspost.io/article/risk-proportional-review-for-ai-agents), [sharedcontext.ai](https://sharedcontext.ai/skills/external/wook3024/ownership-routing)). A tiered map of risk to approvals: [CodeAnt](https://www.codeant.ai/blogs/ai-based-pr-decision-making-code-review). Rigor by blast radius: [DEV](https://dev.to/pyor/risk-based-code-review-tier-rigor-by-blast-radius-352j).

**P3.2** Paths alone miss semantic risk. GitLab proposes domain tags that join path patterns and semantic signals ([GitLab work item 21610](https://gitlab.com/groups/gitlab-org/-/work_items/21610)).

**P3.3** The GitHub required-reviewer rule (GA 2026-02-17) needs approvals from teams for path patterns ([GitHub changelog](https://github.blog/changelog/2026-02-17-required-reviewer-rule-is-now-generally-available/)). The search found no ruleset feature that makes a required check depend on the changed paths. A workflow that a path filter skips leaves its required check Pending. The documented fix is a twin no-op workflow ([GitHub docs](https://docs.github.com/en/enterprise-server@3.4/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/troubleshooting-required-status-checks)).

**P3.4** A severity gate must sit outside the author. "If your review gate can be bypassed, weakened, or self-approved by the same system that wrote the change, you don't have a gate" ([Tenki](https://tenki.cloud/blog/github-agentic-workflows-review-gate)). Policies that block on unresolved high-severity findings: [Qodo](https://www.qodo.ai/blog/5-ai-code-review-pattern-predictions-in-2026/), [mergewall](https://pypi.org/project/mergewall/). Agents must not merge what they reviewed, and the approved commit must be the merged one ([WorkOS](https://workos.com/blog/ai-coding-agent-github-merge-policies)).

**P3.5** Mutate the changed lines only. Google skips arid lines and picks operators by past usefulness. Developer-rated usefulness rose from 20% to 80% ([Petrović et al., TSE 2021](https://research.google/pubs/practical-mutation-testing-at-scale-a-view-from-google/); [ICSE-SEIP 2018](https://research.google/pubs/state-of-mutation-testing-at-google/)).

Synthesis: the industry pattern is a deterministic path and kind floor, with seats that block on severity, enforced outside the author. The T0 list (F2.18) and the gateclass tiers (F2.17) are that floor already. They are not wired to the console gate or to CI.

## 4. Options and scores

**A. One data-driven review matrix.** A Go registry `reviewclass` maps class to seats and checks. The commit gate, a CI job and the lane code-review phase read it.

- It reuses `gateclass`, `guards.IsProtectedSurface`, `resolveReviewRuns` and `launchrecord.Record`. `derived.Catalog()` is the model for its shape.
- It adds three checks: `inbox integrity`, `persona strip` and a permission-widening trace.
- One policy key, `review_routing.stage` (off, shadow, enforce), controls it. It is not a feature flag.

**B. CODEOWNERS and one required status per seat.** The required-reviewer rule takes team approvals (P3.3). This repository has one human account. A seat is a bridge launch.

- To post a status, a seat needs a GitHub token inside an LLM cell. That breaks the dispatch-credential guarantee (`go/internal/guards/integrity_surface.go:21,135`, ADR-0154 I-3).
- Rulesets cannot make a check depend on paths (P3.3).
- It covers PRs only, while lanes land by fast-forward push.
- It adds a fourth path classifier.
- It cannot say "open HIGH is zero" or check the inbox.

**C. A minimal gate and a full checklist in a skill.** Rejected on facts. The written rule exists (F2.8) and no code enforces it. The inbox deletion (F2.15) and the persona bug (F2.14) passed review with discipline alone. P3.4 gives the same verdict.

**D. A risk score from diff size, surface and kind.** `gateclass.TierOf` and `git diff --numstat` can feed it. A threshold is a number that drifts. A score must not lower a safety floor (P3.1). It still needs A's checks for the kinds that a score cannot see.

Scores: 1 is poor, 5 is best.

| Criterion | A matrix registry | B CODEOWNERS and statuses | C skill checklist | D risk score |
|---|---|---|---|---|
| Fixes the root cause (one class definition for gate, CI and lanes) | **5** | 2 | 1 | 3 |
| Safety (enforced without console discipline; blocks on HIGH) | **5** | 3 | 1 | 4 |
| LLM cost balance | 4 | 3 | 3 | **5** |
| Build size | 3 | 3 | **5** | 3 |
| Reversibility | **5** | 3 | 5 | 4 |
| Fit (evo is the controller, no flags, shadow then enforce, 100% coverage, STE docs) | **5** | 2 | 2 | 3 |
| **Total** | **27** | 16 | 17 | 22 |

## 5. Winner and grafts

A wins with 27 of 30.

- **Fact:** the gate checks two seat names, trusts plain tokens, passes WARN with open HIGH and never reads the protected manifest (F2.1 to F2.7). Main keeps no review record (F2.9). The gate, CI and the lanes each define the classes alone (F2.13, F2.17).
- **Implication:** the review of a protected, persona or inbox change depends on the console. Two real defects passed for that reason (F2.14, F2.15).
- **Decision:** one registry, three readers, one stage key. A seat counts only when it is bound to the tree with zero open CRITICAL and zero open HIGH.
- **Consequence:** "always triggers" becomes a table that tests pin. "Fix all defects" becomes `open_blocking == 0` on the final tree, with ADR-0126 limiting the rounds. Cost moves from inbox and test-only commits to T0 commits.

| Graft | From | What it adds |
|---|---|---|
| G1 | D | A raise-only size rule: a T2 diff over a set line count adds `architecture-reviewer`. A diff of 20 lines or fewer in the test or T2 class can use one `code-review-simplify` seat (F2.1). It never removes a T0 seat. The plan adopts only the raise half (R3), so the one-seat path for a small diff is not part of the plan. |
| G2 | B | CI checks classes and trailers in one job inside the `required` aggregator. This avoids the pending-check trap (P3.3), because the aggregator already routes skipped jobs (`.github/workflows/required.yml:168-193`). |
| G3 | C | The commit skill and `CLAUDE.md` print the table from `evolve review-class --json`. A generated region in the architecture doc keeps it in sync. |
| G4 | A and B | `evolve review-class --ci-routes` replaces the bash `case` (`.github/workflows/required.yml:72-85`). One classifier remains. |
| G5 | F2.18 | The manifest gains `.github/workflows/`, `agents/evolve-diff-*.md`, `.evolve/profiles/diff-*.json` and `go/internal/reviewclass/`. |
| G6 | F2.9 | `launchrecord.Record` gains `duration_ms`, `open_blocking` and `tokens`. The cost line is then measured. |
| G7 | F2.16 | A family-routing rule: the adversarial and security seats go to a family other than the author's. Routing data holds it. While one family is available, the fallback is the deep tier of the same family, and the record says so. |

## 6. Operator picks

The operator picked, by click on 2026-11-03, the recommended option of each question:

1. **Table home:** one protected Go registry `reviewclass`. The commit gate, CI and the lane code-review phase read it. Docs and the commit skill are generated from it. The stage key is `review_routing.stage`, shadow then enforce.
2. **Protected surface:** the full seat set, then a normal landing. The set has six parts.
   - Simplifier.
   - Code or Go reviewer.
   - `architecture-reviewer`.
   - Adversarial review on another family.
   - Security review for sandbox, credential or policy changes.
   - Mutation of the changed guard lines.

   Operator approval is only for stage flips.
3. **Blocking bar:** a seat counts with 0 open CRITICAL and 0 open HIGH on the final tree. ADR-0126 sets the rounds. There is no round 4. The change escalates as a held record.
4. **Inbox-data-only commits:** deterministic checks only. They are `inbox lint` and an integrity check that each removed pending item has a consumed copy or a `Closes-Inbox` landing. No LLM seat.

## 7. Limits and cost

**Limits.**

- CI cannot prove a seat ran. The trailers are forgeable by the console. CI closes the "forgot" gap that F2.8, F2.9 and F2.15 show. The tree binding stays local.
- A path floor misses semantic risk (P3.2). The raise-only rule (R3 of the plan) is the control.
- The family routing (G7) depends on a second family being available.
- Shadow records can show false refusals. The stage flip waits for them.

**Cost line.**

- The caps are the profile budgets (F2.10). The launch record holds one `ts` and no duration, so G6 adds `duration_ms`.
- Estimate (synthesis): 3 to 8 minutes per diff seat. The seats after the simplifier run in parallel. Wall time is 6 to 10 minutes for an ordinary Go change and 8 to 15 minutes for a T0 change.
- Mix-weighted estimate (synthesis, from F2.22): inbox commits drop from 2 seats to 0 (54 commits, about $108 saved per two weeks at cap). T0 adds about $2.8 on average for each of about 113 commits (about $316).
- The T0 base: a T0 commit gets two seats today, about $2 at cap. The full C11 set caps at about $5.6 to $6.8 (synthesis). The seat caps sum to about $5.6. The new adversarial and security profiles have no measured cost yet (G6). The average is lower, because the security seat applies only to the sandbox, dispatch-credential and policy classes. The net is about +$200 per two weeks at cap, and it lands where the incidents were.

## 8. What changes our mind

- A second human reviewer or GitHub teams join the repository: add the required-reviewer rule for T0 paths as an extra layer. Do not replace A.
- Shadow records show CRITICAL or HIGH from `architecture-reviewer` on more than 10% of T2 commits: move that seat into the T2 class (+$2 per commit).
- Measured seat time (G6) is above 15 minutes for T0: run the adversarial seat only for the sandbox, dispatch-credential, policy and gate guarantee classes.
- A deterministic Go mutation tool lands: replace the LLM `mutation-gate` with it, scoped to the changed guard lines.
- A second family never becomes available for the adversarial seat: state the same-family deep-tier fallback as the permanent rule, and re-score option A on safety.

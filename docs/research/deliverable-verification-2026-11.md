# Deliverable verification: static and authentic checks for every deliverable (research, 2026-11)

> The plan that uses this research: [deliverable-verification-2026-11](../plans/deliverable-verification-2026-11.md). The design: [deliverable-verification](../architecture/deliverable-verification.md). The decision record: [ADR-0172](../architecture/adr/0172-every-deliverable-has-a-registered-static-check-and-an-authentic-check.md).

Date: 2026-11-27. Method: a read-only inventory of the tree at `origin/main` (`385a332d8`), counts from `git grep` and perl scripts, and a web search of outside practice. Each local claim names its file or its command. "Synthesis" marks our own inference. The working notes are in `.evolve/evidence/deliverable-verification/`, which git ignores. This dossier is the record.

## Request

The operator asked on 2026-11-27:

1. "Make sure we have static verification for all test deliverables"
2. "I mean all deliverables are required to be properly tested and examed with a static and authentic method"

The operator chose the scope of the first guard on the same day: "Docs of record + ratchet". The operator also chose that the research-loop components C2, C3 and C4 are built in the console.

## Terms used throughout

- **Deliverable:** any artifact that a builder, a console session or a phase agent hands to the pipeline. The matrix in section 1 has 15 classes.
- **Static check:** a deterministic check that code runs: a Go test, a lint, a gate verb or a CI step. A reviewer LLM's probe or a builder's claim is not static.
- **Authentic check:** a check that truly exercises the deliverable. A new or changed test fails on the base and passes on the change (FAIL_TO_PASS), and the old tests still pass (PASS_TO_PASS). A mutant of the changed lines fails a test. A proof drives the production entry point. A cited test, `path:line`, id, link or number exists and matches.
- **Tautological check:** a check that passes whatever the deliverable holds.
- **Shadow stage:** a gate that records its verdict and blocks nothing, until evidence moves it to enforce ([ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md)).

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Outside practice](#2-outside-practice)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)
5. [Gaps in this research](#5-gaps-in-this-research)
6. [Decision entries](#6-decision-entries)

## 1. Local evidence

### 1.1 Where the checks run

The matrix has 15 deliverable classes. D1 to D13 are the producers of the tree. D14 (ratchet files) and D15 (eval definitions) are classes that this research adds.

| Id | Class | Main static checks today | Authentic check today |
|---|---|---|---|
| D1 | Go production code | gofmt, vet, golangci, cover-changed, apicover, sizeratchet, comment ban | None static. No check that a new export has a production caller. |
| D2 | Go tests | testaudit (13 rules), emptytests, testdup, paralleltest, added-tests tags | Anti-tautology rules only. No red-on-base. |
| D3 | ACS predicate packages | TDD predicate floor, evalgate, EGPS host run, acs-durable | EGPS host run. No proof that a predicate can fail. |
| D4 | Docs of record | STE lint on added lines, index check, docgo, incidentkb | Only `guidance` paths and data-file rows. |
| D5 | Plans and research | STE lint on added lines, index check | None. |
| D6, D7 | Skills and personas | `skills check`, plugin manifest, `install --ci` | Entries exist. No STE lint, no citation check. |
| D8, D14 | Data and ratchet files | `guidance check`, data-file rows, shrink-only ratchets | Shrink-only. Named tests and paths exist (rows only). |
| D9 | Generated files | render equals file | Render equals file. |
| D10 | Inbox items | `inbox lint`, `inbox trailer-check`, landed-claim proof | A landed claim needs an ancestor sha. |
| D11 | Cycle artifacts | heading contract, EGPS override, ship audit binding | EGPS sealed to the tree. Report claims unchecked. |
| D12 | Console PRs and commits | commit gate, review-run proof, commit prefix scope | None for "N/N PASS" claims. |
| D13 | Release artifacts | semver, preflight, consistency, release CI | Release CI re-runs the suite. |
| D15 | Eval definitions | evalgate (trivial grader, unsatisfiable, source-text) | Blocks only at `enforce`. |

Sources: `go/internal/commitgate/commitgate.go:206` (the language lanes), `go/internal/stelint/stelint.go:47` (the lint scope), `go/internal/evalgate/reviewer.go:56` (the stage test), `.github/workflows/ci.yml:69` (the ACS job).

**F1.1** The commit gate starts language lanes only for go, py, ts, js and rs files (`go/internal/commitgate/commitgate.go:206`). For a diff of only `.md` files it asks for a docs-review run record and nothing else.

*Implication:* a doc of record has no code check of its citations. The seat of docs-review (an LLM) is the only check.

**F1.2** The TDD phase passes when the report holds the heading `## RED Run Output`. `classify` calls `phasecontract.TDD.Complete` (`go/internal/phases/tdd/tdd.go:72`). It runs no test.

*Implication:* "red first" is a claim of the engineer. An already-green predicate passes the phase.

**F1.3** The base-red floor re-runs reds on the lane base and excuses a red that the base already has (`go/internal/core/base_red_floor.go:77`, `go/internal/phases/ship/base_red.go:52`).

*Implication:* the gate that touches the base run does the inverse of FAIL_TO_PASS. It forgives a base red. Nothing demands a red on the base for a new test.

**F1.4** `MeasureMutation` (`go/internal/qualitypanel/mutation.go:78`) and `MeasureRevertSensitivity` (`go/internal/qualitypanel/revert_sensitivity.go:53`) exist. `git grep -l "MeasureMutation\|MeasureRevertSensitivity" -- go` finds only their own package and two ACS predicate packages, `go/acs/cycle2067` and `go/acs/cycle2074`.

*Implication:* the libraries work and no gate calls them. Mutation is an LLM probe in a reviewer persona (`agents/evolve-diff-architecture-reviewer.md:57`).

**F1.5** The package `go/internal/testrefs` and the guard `TestDocsOfRecord_EveryNamedTestExists` exist on branch `cl/docs-named-tests-exist`, which a pending PR carries. The guard holds a shrink-only `baseline.json` of 381 names in 117 docs.

*Implication:* V1 of the plan is built. It is not on main yet.

**F1.6** Backticked test names that no Go declaration defines (inventory section 4.1, script `cited.pl`, 2026-11-27):

| Doc class | Citations | Hard-missing cites |
|---|---|---|
| Package notes | 7,433 | 62 |
| Architecture specs | 2,359 | 212 |
| ADRs | 387 | 25 |
| Incidents | 1,125 | 16 |
| Plans | 1,619 | 483 |

A subset of 36 package-note lines says that a test "pins", "proves" or "covers" a behavior, and the test does not exist. One example: `docs/architecture/packages/cmd-evolve-cmdutil.md:19` names `TestNewPromptsLoader_HonorsEnvOverride`. The package holds two other tests of that loader (`go/cmd/evolve/cmdutil/filterenv_promptsloader_test.go:55,69`).

*Implication:* a document of record can state a guarantee that points at a renamed or removed test. Plans name tests that are still to write, so their misses are expected. Notes and specs must have no miss.

**F1.7** `path:line` citations: 1,460 across the tracked markdown (script `pathline.pl`). In specs, incidents and research, about one in five does not resolve. A file is absent, or the line is beyond the end of the file. Relative links: about one in 300 links in docs of record is broken (script `links.pl`). No general tool checks either. Two docs have a link test (`go/cmd/evolve/cmd_docs_removal_design_amp_test.go:53`).

*Implication:* a check of existence is cheap and the miss rate is high enough to matter. The content of a line is a harder check, and this research does not measure it.

**F1.8** ACS packages: 957 `go/acs/cycleNNNN` packages exist, all tagged `acs`. The EGPS run executes the current cycle package, `acs/regression/*` and `acs/redteam` (`go/internal/acssuite/acssuite.go:104`). CI runs only `./acs/regression/...` (`.github/workflows/ci.yml:69`). A search with `git grep -- '-tags acs'` finds no other caller (synthesis).

*Implication:* after its cycle, a package is never run again. About 956 packages are weight that protects nothing and can rot. 26 of 957 import no production package and run no command, so they test text only.

**F1.9** Some gates block only at `enforce`, or only warn. They are: evalgate (`go/internal/evalgate/reviewer.go:56`), the manifest gate (`go/internal/phases/ship/manifest.go:51`), the strict audit (`go/internal/phases/ship/audit.go:40`), the top-N gate drift, the flaky-shape lint and docsfloor. Inbox dead paths, coverage below 85 percent and apicover outside the enforce list also only warn.

*Implication:* each flip is a policy decision. The plan lists them as operator questions and does not flip any.

**F1.10** The ship step checks that `tree_state_sha` of the attestation equals the current tree (`go/internal/phases/ship/commitgate.go:85`). It does not re-check `checks_passed`. Since 2026-11-01, 4 of 1,442 commits on main carry a "N/N PASS" line (`git log --since=2026-11-01 --format=%B | grep -cE '[0-9]+/[0-9]+ PASS'`).

*Implication:* a test count in a commit body or a build report is text. Nothing compares it with a recorded run.

**F1.11** The testaudit ratchet tolerates 1,735 baseline rows (196 shards), of which 1,430 are `error-text-match`. A new row is refused.

*Implication:* the ratchet is shrink-only, so it is the right model for a new guard with a backlog.

## 2. Outside practice

All facts in this section come from search summaries, not from fetched pages. Each carries the mark "(search summary)".

**P2.1** SWE-bench splits its tests in two sets. FAIL_TO_PASS tests fail before the fix and pass after it. PASS_TO_PASS tests pass before and after. A candidate edit must pass both sets. (search summary) Sources: [introducing-swe-bench-verified](https://openai.com/index/introducing-swe-bench-verified/), [SWE-bench_Verified dataset](https://huggingface.co/datasets/princeton-nlp/SWE-bench_Verified).

*Implication:* the two sets map to two rules of V2. A new test fails on the base. The old tests stay green.

**P2.2** SWE-bench-Live builds its tests from per-test outcomes before and after the patch. An instance needs at least one FAIL_TO_PASS transition. Runs repeat to drop flaky instances. (search summary) Source: [arXiv 2505.23419](https://arxiv.org/pdf/2505.23419).

*Implication:* a flaky test must not decide the gate. V2 repeats each run.

**P2.3** UTBoost found 36 instances with insufficient tests that let 345 wrong patches pass in the original SWE-bench. Augmented tests flagged 26 of 500 (5.2 percent) of SWE-bench Verified as under-tested. The log parser missed test cases. (search summary) Source: [arXiv 2506.09289](https://arxiv.org/html/2506.09289v1).

*Implication:* a test set that passes does not prove the change. This is the "green but proves nothing" risk of F1.2. The parser point says V2 must read test events by name from `go test -json`, not from text.

**P2.4** "State of Mutation Testing at Google" (ICSE-SEIP 2018) reports more than 70,000 diffs, 1.1 million mutants and about 150,000 actionable findings in code review. (search summary) Source: [Google Research](https://research.google/pubs/state-of-mutation-testing-at-google/).

**P2.5** "Practical Mutation Testing at Scale" (TSE 2021) mutates only changed code during review. About 24,000 developers and more than 1,000 projects took part. The mutant count is far lower. (search summary) Source: [arXiv 2102.11378](https://arxiv.org/pdf/2102.11378v1).

*Implication:* P2.4 and P2.5 support V3. A mutant of the changed functions is cheap enough for each diff.

**P2.6** An LLM test generator that is built to pass can validate faulty code (Mathews and Nagappan). Tests that an LLM generates from faulty code inherit its faults. Detection falls from 25 percent to about 14 percent. (search summary) Sources: [arXiv 2412.14137](https://arxiv.org/html/2412.14137v1), [arXiv 2610.11835](https://arxiv.org/pdf/2610.11835).

**P2.7** assertFlip reaches 43.6 percent fail-to-pass success on SWT-Bench-Verified. (search summary) Source: [arXiv 2507.17542](https://arxiv.org/html/2507.17542v1). A gold-patch gate flagged 65 of 105 decisive LLM-written tests as failing on the gold patch (61.9 percent). An LLM judge alone missed them. This is a single-author preprint of 2026. (search summary) Source: [arXiv 2606.16062](https://arxiv.org/html/2606.16062v1).

*Implication:* a code run on a known-good state finds what a judge does not. Treat the 61.9 percent as weak evidence.

**P2.8** Reinforcement learning on repository repair can learn to reward-hack by disabling tests. (search summary) Source: [arXiv 2511.21654](https://arxiv.org/pdf/2511.21654).

*Implication:* the check of a test must run outside the control of the agent that writes it. V2 and V3 run in the host, on files that the builder cannot change.

**Synthesis:** a passing test is weak evidence by itself. The authentic check is behavioral and code runs it. The test is red on the base and green on the change. The old tests stay green. A mutant of the changed lines fails a test. A repeat run drops the flaky result.

## 3. Adopt or reject

| Id | Practice | Decision | Reason |
|---|---|---|---|
| A1 | FAIL_TO_PASS on each added or changed test (P2.1) | Adopt as V2, shadow first | F1.2 and F1.3: no gate demands red on the base. |
| A2 | PASS_TO_PASS: the old tests stay green (P2.1) | Adopt. The existing floors already run the changed packages | The new part is only the base run of F2P. |
| A3 | Repeat runs to drop flaky results (P2.2) | Adopt in V2 with a fixed repeat count | A flaky test must not decide a gate. |
| A4 | Mutation of changed code only (P2.5) | Adopt as V3, shadow first | F1.4: the library exists and has no caller. |
| A5 | Whole-repository mutation score (P2.4) | Reject | The cost is too high, and a legacy score blocks no diff. |
| A6 | An LLM judge as the authentic check (P2.6, P2.7) | Reject | An LLM probe is not static. Keep the review as a second line. |
| A7 | Gold-patch gating of every test (P2.7) | Adopt in a reduced form: the base is the "gold" state for F2P | We have no gold patch for human work. |
| A8 | Parse test results by name from `go test -json` (P2.3) | Adopt in V2 | A text parser missed test cases in UTBoost. |
| A9 | Local existence check of cited things | Adopt as V1 and V4 | F1.6 and F1.7: the miss rate is high and the check is cheap. |

## 4. Refinements to adopt

**R1** Reuse a guard that exists. The registry (V0) names each check by the id of an existing guard. It adds no second guard for a property that a guard already holds. (F1.1, F1.11)

**R2** Every new guard with a backlog starts as a shrink-only baseline, like testaudit and V1. The baseline may only shrink. (F1.6, F1.11)

**R3** Every new authenticity gate starts in shadow, with a named stage type, and moves to enforce on chained evidence under ADR-0167. (F1.9)

**R4** An exemption from FAIL_TO_PASS (a refactor or a characterization test) needs an explicit marker, a reason, and a count that the ledger records. An unmarked exemption is a failure. (P2.3, P2.8)

**R5** A claim in text (the "N/N PASS" line, the red evidence) must match a run that the host recorded. A heading in the report is not evidence. (F1.2, F1.10)

**R6** The host runs each authentic check on files that the builder cannot change. The builder can overlay only the test file on the base. (P2.8)

## 5. Gaps in this research

- **Content of a cited line.** F1.7 checks that a line exists. It does not check that the line holds what the text says. V4 starts with existence.
- **Build-report claims.** This research did not read `.evolve/runs/*`, so it has no measure of how often a build report claims red first.
- **A Go mutation tool.** The research did not find outside evidence for a Go mutation tool. V3 reuses the code in `go/internal/qualitypanel`, so the choice does not depend on it.
- **Flake rate.** We have no measure of the flake rate of our tests. The repeat count of V2 is an open question in the plan.
- **Stage settings.** The counts of stages come from the local `.evolve/policy.json`, which git does not track. They can differ on another host.
- **Research-loop components.** The components C2, C3 and C4 are named in the operator's request. Their definition is not in this tree. The plan orders them after V6 and does not design them.

## 6. Decision entries

The plan decides. These entries state the proposed decisions with their reasons, so a reader of the dossier sees the chain. The plan numbers them as D1 to D10.

**E1 Registry first.**
- Decision: build V0, a data file that maps each class to its static checks and its authentic checks, and a meta-guard.
- Rationale: F1.1 to F1.10 show that the checks exist in many places and no file lists them. A class with no authentic check is invisible today.
- Alternatives: patch each gate one at a time (rejected: nobody sees the classes that have no gate). Build one new pipeline (rejected: it duplicates the guards that exist, F1.1).

**E2 Existence checks before behavior checks.**
- Decision: build V1 and V4 before V2 and V3.
- Rationale: F1.6 and F1.7 show a high miss rate for a low cost. They need no base run.
- Alternatives: start with V2 (rejected: V2 needs a base worktree and a repeat run, so its cost and its risk are higher).

**E3 Behavior checks start in shadow.**
- Decision: V2 and V3 record a verdict and block nothing until the evidence under ADR-0167 moves them.
- Rationale: P2.2 says flaky results exist. A blocking gate with a flake rate that we have not measured stops good work (gap 4).
- Alternatives: enforce at once (rejected: no flake measure). Never enforce (rejected: a shadow gate protects nothing, F1.9).

**E4 Claims must match a recorded run.**
- Decision: build V5, which compares the red evidence and the "N/N PASS" lines with a host record.
- Rationale: F1.2 and F1.10 show that both are text.
- Alternatives: ask the reviewer LLM to read the claim (rejected: not static).

**E5 Old ACS packages re-run on a schedule.**
- Decision: build V6, a scheduled run of `-tags acs ./acs/...`. Promote each package to regression or retire it with a record.
- Rationale: F1.8 shows that about 956 packages never run again.
- Alternatives: run them on each PR (rejected: the run time is too long for a PR check, and the failures are legacy).

**E6 Gate flips are the operator's decision.**
- Decision: list the stage-gated and advisory gates with their evidence. Flip none.
- Rationale: F1.9. A flip changes what blocks a cycle, and the operator owns that.
- Alternatives: flip the safe ones in this work (rejected: no shadow record is in this research).

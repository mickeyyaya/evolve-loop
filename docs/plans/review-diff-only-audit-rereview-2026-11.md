# Plan: the audit re-reviews only the repair diff from round 2

- **Status:** proposed, 2026-11-21. Review round 3 revision.
- **Decision:** ADR-0168, to land with the build (section 5 gives its text). It amends decision 9 of [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) and keeps decision 4 of ADR-0158. **Research:** [code-review-at-anthropic-2026-10](../research/code-review-at-anthropic-2026-10.md), section 5, rank 1. **Case:** [cycle 2035](../incidents/2026-10-27-cycle-2035-repair-loop.md). **Spec of record:** [convergence-policy](../architecture/convergence-policy.md).
- **Item:** `review-diff-only-audit-rereview` (rank 65, console-owned).

## 1. Request

The inbox item says: "From audit round 2, the auditor reviews the previous findings and the hunks of the repair. It does not re-hunt the whole tree." Its fix says: "A new finding on code unchanged since round 1 is filed, unless a CRITICAL certificate survives a falsification check."

The console decided the conflict between the item text and the accepted decisions. A late CRITICAL or HIGH blocks until its falsification check refutes it. This follows ADR-0126 decision 6, convergence-policy section 4 rule 3 and ADR-0158 decision 4. So this plan files only a late MEDIUM or LOW. The acceptance criteria are restated to match:

1. A synthetic replay of cycle 2035 raises no blocking MEDIUM or LOW on unchanged code. A late HIGH or CRITICAL still blocks.
2. No audit FAIL becomes PASS. The host never raises a verdict. A CRITICAL on changed lines still blocks.
3. The plan and the ADR record the amendment of ADR-0126 decision 9.

**Terms used throughout:**

- **Round r:** the r-th audit run of one cycle (`CycleState.AuditDispatches`).
- **Repair hunks:** the changed line ranges between the tree that round 1 audited and the tree that round r audits. New files count.
- **Host id:** the id of a finding, `defectledger.ID(title)` (`go/internal/core/defectledger/schema.go:109`). The host prints it. The auditor copies it and never renumbers it.
- **Late finding:** a finding in round 2 or later, with a new host id, and with a `path:line` location outside the repair hunks.
- **Scope:** the previous findings with their host ids, plus the repair hunks.

## 2. Facts

**F2.1** ADR-0126 decision 9 reads "The audit is untouched. CRITICAL is never deferred, and no audit FAIL becomes a PASS. The audit keeps its full verdict rules." (`docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md:73`).
*Implication:* the audit has no diff-only rule today. The amendment needs a new ADR.

**F2.2** Decision 6 of the same ADR gives code-review and console judges a verify-only scope. A late finding below HIGH is filed. A late CRITICAL or HIGH is filed only when its falsification check refutes it, and it blocks until then (`:61-66`, the late rule at `:64`). Convergence-policy repeats this (`docs/architecture/convergence-policy.md:293`).

ADR-0158 decision 4 says "A finding blocks if a gate fails on it, the audit proves it red, or it is HIGH or above" (`docs/architecture/adr/0158-*.md:30`). `fixability.isBlockerFinding` enforces it (`go/internal/fixability/fixability.go:152-155`). The mechanism is `Finding.Late` and `Finding.Falsification` (`go/internal/convergence/convergence.go:73`) and `filedByRule` (`go/internal/convergence/findings.go:57-70`).
*Implication:* three accepted decisions agree. A late HIGH blocks until a check refutes it. No production code runs that check (F2.9), so a late HIGH always blocks.

**F2.3** `rulesByLoop` gives `LoopAuditRepair` only `judgeRaise` and `freshContext`. The late branch of `filedByRule` runs only when `filesByRule` is true, and that flag also files INFO and capability findings (`go/internal/convergence/rules.go:36`, `findings.go:62-67`).
*Implication:* the audit needs the late branch without the other filing rules. The change is one data field in the table.

**F2.4** The audit-repair convergence input builds each finding with an id, a severity, a class and a status (`go/internal/core/audit_repair_convergence.go:63-70`). The id is the title first (`:66`). `fixedSince` marks each previous id that is missing from the new report as FIXED (`:80-89`).

The decision is advisory at `shadow`, the default (`go/internal/policy/convergence.go:10`; `audit_repair_convergence.go:97-108`). `convergence.Hunk`, `Judgment.FixHunks` (`convergence.go:78-89`) and `insideHunks` (`findings.go:45-50`) exist, and audit-repair never fills them.
*Implication:* a reworded old finding becomes "fixed", and its copy becomes new. That is the path that this design must close. The types for the hunks exist and are reused.

**F2.5** The verdict comes from the auditor sentinel (`go/internal/phases/audit/classification.go:31`). Each gate can only override a verdict to FAIL (`classification.go:59-67`). The auditor FAIL is a whole judgment: acceptance results, decisive reasoning links and missing evidence can each cause it (`go/internal/phases/audit/audit.go:84-88`). Convergence-policy section 11 says "It never turns an audit FAIL into a PASS".
*Implication:* the host must change a verdict in one direction only, toward FAIL. A host rule that lifts a FAIL can hide a FAIL that rests on another cause.

**F2.6** The audit prompt has one host block of rules, `auditRulesPromptBlock` (`go/internal/phases/audit/audit_rules.go:25-30`). It does not name the round. The persona says "Always run full checklist when `strategy` is `harden`/`repair`" (`agents/evolve-auditor.md:71`). It grounds the audit in `git diff HEAD` (`:52`, `:87`).

`PhaseRequest.AuditRound` carries the round (`go/internal/core/phase.go:72`). Two places set it: `cyclerun_dispatch.go:62` and `resume_execution.go:137`. Both call `supersedePreviousAuditRound` (`cyclerun_dispatch.go:29`, `resume_execution.go:115`).
*Implication:* the auditor has no scope in round 2 and later. The scope must reach both entry points. Persona line 71 must apply to round 1 only.

**F2.7** Each round archives its report as `audit-report.round<n>.md` (`go/internal/core/audit_round_artifacts.go:32-58`, name at `:38`). The archive can be lost: the function can drop it (`:43`). The archive holds no tree SHA.

The audited tree is in the hash-chained auditor ledger binding. `auditBindingEntry` writes `WorktreeTreeSHA` from `shipmanifest.TakeShipTree`, and that tree includes new files (`go/internal/core/phase_bindings.go:131-160`). The SHA is empty for an in-place worktree (`:149`).
*Implication:* `git diff <tree>` against a working tree misses untracked files. The host must diff two ledger trees. When a tree is missing, the host cannot scope.

**F2.8** The research found that about 59% of repeated flags hit code from the first pass (`docs/research/code-review-at-anthropic-2026-10.md:67`). The team added a round cap and a diff-only re-review.
*Implication:* this is the reason for the change. Cycle 2035 is a weak case (F2.10).

**F2.9** No production code writes `Falsification`. `Decision.File` is only printed (`go/cmd/evolve/cmd_convergence.go:158`). `reportdoc.Finding` has only `ID`, `Severity` and `Title` (`go/internal/reportdoc/findings.go:8-12`).

`fixability.parseFinding` and `extractPaths` read the id, the severity and the paths, and drop the line (`fixability.go:78-138`). Core already parses unified diffs: `parseUnifiedDiffToHunks` (`go/internal/core/mergerung2.go:110`).
*Implication:* there is no check runner to reuse. Filing has no consumer. The location needs one projection that keeps the line.

**F2.10** Cycle 2035 ran four audit rounds and used 160.1 LLM minutes. Build used 70% of them. Three of four stops came from deterministic gates. ADR-0158 is the accepted fix (`docs/incidents/2026-10-27-cycle-2035-repair-loop.md:72`, `:129`).

M3 came from the document-only correction of repair round 1, so it is inside the hunks. H1 was a real HIGH, and the lane fixed it in repair round 2 (`:113`, `:124`). Round 3 came after a ship refusal with no repair in between (`:45-46`). The repeated audit text was M1 and M2 re-reported (`:131`).

*Implication:* cycle 2035 sets a low upper bound on the saving, and this rule does not file H1. The real reports are gone: `runtime/.evolve/runs` holds cycle-0 and cycle-2195 and later. Only a synthetic fixture can replay the shape.

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| Round 2 and later do not block on late MEDIUM or LOW | late MEDIUM or LOW findings that block, per fixture | 0 |
| No verdict is raised | fixtures where the host turns FAIL into PASS or WARN | 0 |
| A late HIGH or CRITICAL, and a CRITICAL on changed lines, still block | fixtures | 100% |
| Rounds cost less | audit LLM minutes in round 2 and later | lower than round 1, measured in the first enforce waves |

Non-goals:

- This plan does not change round 1. Round 1 is a full audit.
- This plan does not change the round budget, the rungs or the deferral rule.
- This plan does not change the code-review loop.
- This plan does not add a falsification run (section 11).

## 4. Candidate designs

Criteria: **C1** meets acceptance 1 and 2. **C2** holds in code, not in prose that the auditor can ignore. **C3** has one home for the rule and the setting. **C4** adds no feature flag and reads no control value from the environment. **C5** keeps the protected surface small. **C6** runs the same on Linux and macOS, Go 1.23.

| Design | Idea | C1 | C2 | C3 | C4 | C5 | C6 |
|---|---|---|---|---|---|---|---|
| A. Prompt only | Tell the auditor in the persona to read only the diff from round 2. | no | no | no | yes | one fenced file | yes |
| B. Host scope, one-way check | The host computes the scope from two ledger trees. At enforce, the prompt carries it. After the audit, the host checks each late entry and can only force FAIL. | yes | yes | yes | yes: a stage key, which ADR-0167 allows | five fenced files, small edits | yes |
| C. Reuse the code-review rules | Point `LoopAuditRepair` at the code-review rule set. | partly | yes | yes | yes | small | yes |
| D. Cache the unchanged files | Skip the audit of files with the same blob hash. | no | yes | no | yes | large | yes |
| E. B, plus a host rule that lifts a FAIL | The host removes a filed finding from the blocking count and lifts the verdict. | yes | yes | no | yes | large | yes |

Why B wins: it fixes the cause (F2.6) and enforces the result in code, and it keeps "no audit FAIL becomes PASS" literally true (F2.5).

Why not C: `defers` and `accepts` defer MEDIUM findings and accept a residue. That breaks "no deferrals". `filesByRule` also files INFO and capability findings (F2.3).

Why not E: no blocking count exists to exclude from, and a lifted FAIL can hide another cause (F2.5). It also needs an amendment of decision 9 that says so in words. The operator removed it.

Why not A and D: A leaves the prompt as the only guard. D cannot show that a test in a changed file still covers an unchanged function.

## 5. Decisions

| Id | Decision | Because | Rejected |
|---|---|---|---|
| D1 | Round 1 stays a full audit. Round 2 and later get the scope. | F2.6, F2.8 | whole-tree re-hunt every round |
| D2 | The host computes the hunks as `git diff-tree -U0 <round-1 tree> <round-r tree>`. The round-1 tree comes from the auditor ledger binding. | F2.4, F2.7, F2.9 | `git diff` of a working tree: it drops new files |
| D2b | The round-r tree does not exist in the ledger when the prompt is built. `auditBindingEntry` runs after the audit (`go/internal/core/phase_bindings.go:85`, `:122`; `reaudit.go:101`). So at dispatch the host computes the tree with `worktreeContentSHA`, the function that the binding uses. It runs `git add -u`, so new files count. After the audit, the host checks that the round-r binding `WorktreeTreeSHA` equals that tree. It uses D3 when they differ. | F2.7 | read both trees from the ledger: every round becomes "no scope" |
| D2c | The hunks use the post-image side. The host converts the `+c,d` of each hunk header into `convergence.Hunk{File, From: c, To: c+d-1}`. A pure deletion (`+c,0`) becomes the range `[c,c]`, which fails closed. `parseUnifiedDiffToHunks` reads only the pre-image numbers (`go/internal/core/mergerung2.go:110`). So K1 reads the numbers from `MergeHunk.Header`, or adds a new-side parser beside it. | F2.9 | pre-image numbers: an insertion gets an empty range and each later hunk shifts |
| D2d | The cumulative hunks stay in the round scope, which the late test reads. `Judgment.FixHunks` keeps the hunks of fix r alone (round r-1 tree to round r tree). `effectOf` counts damage from `FixHunks` (`go/internal/convergence/findings.go:120`), and convergence-policy section 4 rule 2 says "fix r's own hunks". | F2.4 | cumulative hunks in `FixHunks`: a new finding in fix 1 then counts as damage of fix 2 |
| D3 | When the scope is not available, the host treats the round as round 1: a full audit, nothing filed, and a signal. | F2.7 | file with a partial scope: fails open |
| D3b | A scope is not available in these cases. The round-1 binding is missing. A tree SHA is empty. The round-r binding tree differs from the dispatch tree. The archive is lost. Git fails. | F2.7 | none |
| D3c | After a rebase the scope widens, which is safe. `ReauditCycle` (`go/internal/core/reaudit.go:87`) has no `AuditRound`, so a re-audit stays a full audit. | F2.7 | none |
| D4 | One identity: the host id. The host prints each previous finding with its host id. The auditor copies the ids and writes the disposition of each one: FIXED with resolvable evidence, or still open. A previous id with no disposition stays open, and `fixedSince` no longer marks it fixed. A finding with no stable id is never late. | F2.4 | three keys (title, `FindingKey`, ledger hash) |
| D4b | The dispositions use the shape of the defect-ledger entry and its `Resolver` (`go/internal/core/defectledger/defectledger.go:74`), in a separate round-disposition file. The continuation file `defect-dispositions.json` (`defectledger/schema.go:17`) is written in full each cycle, so a second block must not write it. DEFERRED counts as open in audit rounds, because ADR-0126 decision 9 keeps "no deferrals". | F2.4 | reuse the continuation file: two writers, one file |
| D5 | A late CRITICAL or HIGH blocks, because no falsification check runs (F2.2, F2.9). A late MEDIUM or LOW is filed. A finding with no `path:line` is in scope and never late. | ADR-0126 decision 6, convergence-policy section 4 rule 3, ADR-0158 decision 4 | file a late HIGH without a check |
| D6 | The verdict side changes in one direction. At enforce, the round-2+ prompt says: put a late finding below HIGH under `## Filed (late)`, and it does not decide the verdict. After the audit, the host checks each entry: a new host id, a location outside the hunks and a severity below HIGH. An entry that fails the check forces FAIL through the existing `fail()` path. The host never lifts a verdict. | F2.5 | design E |
| D7 | A data field `lateFileBar Severity` in `loopRules` replaces any bool. Code-review, console and audit-repair all use HIGH. The late branch of `filedByRule` reads it and no longer needs `filesByRule`. The late branch applies only when `lateFileBar` is set, so a loop without it never files a refuted late CRITICAL. | F2.2, F2.3 | a second threshold beside "below HIGH" |
| D7b | One late-filing predicate (`filedByRule` with `lateFileBar`) lives in the leaf package `convergence`. Core late classification (K4) and the audit check (K7) call it. The convergence `strict()` bar (MEDIUM by default, `findings.go:74-81`) and the fixability bar (HIGH or red, `fixability.go:152-163`) stay separate. | F2.2, F2.3 | one predicate for all blocking: it changes the code-review loop and ADR-0158. A home in `fixability`: it makes an import cycle through `guards` and `core` |
| D8 | The rule ships behind a dedicated stage key `workflow.convergence.audit_rereview.stage`, not behind `workflow.convergence.stage`. At `enforce`, the scope block and the check turn on together. The flip verb is `evolve stage verify <key>`. | ADR-0167 decisions 2, 9-11, 17 | the convergence key: a flip also flips the Stop of every loop |
| D8b | At `shadow` the prompt does not change, and the convergence input and the `CONVERGENCE_RUNG` fields stay as today. K4 marks, the `Late` and `Location` marks and the K5 audit-repair `lateFileBar` act only at `audit_rereview` enforce with a scope. At shadow and in each D3 fallback, `judgmentsOf` and `fixedSince` stay unchanged. | ADR-0167 decisions 2, 9-11, 17 | no gate: no previous finding gets a FIXED mark at shadow, `effectOf` counts repairs as 0 and every audit-repair round reads as spent (`findings.go:108-123`, `ladder.go:67`) |
| D8c | At shadow the host computes the late set on the unscoped report. It writes the set only to the `AUDIT_REREVIEW_SHADOW` record. It also counts there each new id at the location of an open previous finding, so that rewording does not inflate `would_file`. | ADR-0167 decision 17 | title-hash identity: a reworded re-report counts as `would_file` |
| D9 | A filed finding becomes one deduplicated follow-up through the existing inbox writer or the ADR-0158 hand-off path. A filing failure writes the finding to the landing notes and emits a signal. | F2.9, convergence-policy section 10 | print only |
| D10 | The text of the amendment: ADR-0126 decision 9 gets a pointer and one sentence. ADR-0168 holds the rule. | acceptance 3 | edit decision 9 in place |

**Text of ADR-0168** (title: "From round 2 the audit re-reviews the repair diff, and a late finding is filed"). Decision items, each 20 words or fewer. Relates to ADR-0126, ADR-0158 and ADR-0167.

1. Round 1 of the audit is a full judgment.
2. From round 2, the host gives the auditor the previous findings and the repair hunks.
3. A finding with a new id, outside the repair hunks, is late.
4. A late MEDIUM or LOW is filed and does not block.
5. A late CRITICAL or HIGH blocks until a falsification check refutes it.
6. The host never turns an audit FAIL into PASS or WARN.

The ADR amends two sentences of ADR-0126 decision 9: "The audit is untouched" and "The audit keeps its full verdict rules" (`:73`). From round 2, a late MEDIUM or LOW no longer decides the verdict. The rest of decision 9 stands: "CRITICAL is never deferred", "no audit FAIL becomes a PASS" and "no deferrals".

## 6. Components

| Id | Component | Home | Red test first |
|---|---|---|---|
| K1 | Scope: the round-1 tree from the ledger, the round-r tree from `worktreeContentSHA`, post-image hunks, host ids of previous findings. Fills `AuditRoundScope.Hunks`, and `FixHunks` for fix r alone. Returns "no scope" in the D3 cases. | new `go/internal/core/audit_round_scope.go` (core owns `AuditDispatches`, the archives and the ledger) | T1, T2 |
| K2 | One finding projection (id, severity, title and `path:line`) and the late-filing predicate (D7), in a leaf package. `fixability`, core K4 and `phases/audit` K7 all import it. `fixability` imports `guards` (`fixability.go:8`) and `guards` imports `core`, so the home cannot be `fixability`. | `go/internal/convergence` (imports only `modelcatalog` and `policy`), with `insideHunks` (`findings.go:45`) exported for K7. `reportdoc` is also safe but is fenced (`integrity_surface.go:88`) | T3 |
| K3 | Typed scope field on `PhaseRequest`, next to `AuditRound`, set at both entry points. | `go/internal/core/phase.go:72`, `cyclerun_dispatch.go:62`, `resume_execution.go:137` | T4 |
| K4 | Late classification and the identity rule: late flag, `Location`, no "fixed" without a disposition. All of it runs only at `audit_rereview` enforce with a scope. A new id at the location of an open previous finding is not late. | `go/internal/core/audit_repair_convergence.go` | T5, T6 |
| K5 | Rule data `lateFileBar`. Reuses `insideHunks`. | `go/internal/convergence/rules.go`, `findings.go` | T7 |
| K6 | Scope block in the prompt, and the persona line. The line reads "Run the full checklist unless the host gives a Round scope block", so that it depends on the block and the shadow prompt does not change. | `go/internal/phases/audit/audit_rules.go`, `agents/evolve-auditor.md` (fenced) | T8, T9 |
| K7 | Post-audit check of `## Filed (late)` entries. It only calls `fail()`. | `go/internal/phases/audit/classification.go`, `audit.go` (fenced) | T10, T11 |
| K8 | Stage key (its one home is `ConvergencePolicy` in `go/internal/policy/convergence.go`, because the strict policy refuses unknown keys), signal `AUDIT_REREVIEW_SHADOW` with `would_file` and `would_block`, closure and scorer. Filer for D9. The filer reads the K7-checked Filed entries directly. It does not call `consultAuditRepairConvergence`, which returns when `workflow.convergence.stage` is empty. | `go/internal/policy/convergence.go`, `go/internal/gateregistry/seed_stages.go`, `core/` | T12, T13 |
| K9 | Docs: ADR-0168 and the amendment line in ADR-0126. Convergence-policy: the preamble of section 4 ("The audit keeps its full verdict rules"), section 6 and the section 11 bullet. Also the ADR-0158 cross-reference, the stage table of `shadow-stage-contract.md`, the package notes `internal-phases-audit.md` and `internal-convergence.md`, and `signal-codes.md` (run `evolve signals codes generate`). | `docs/` | `evolve docs ste-lint --strict`, T14 |

Each new code name is a proposal. The builder must read the module and list its real exports before it calls them, in particular the defect-ledger disposition reader and the inbox writer. The code index is one commit behind `origin/main`, so refresh it first.

The protected surface is the manifest in `go/internal/guards/integrity_surface.go`. These files are fenced: `classification.go` (`:152`), `audit.go` (`:151`), `reportdoc/` (`:88`), `core/defectledger/` (`:116`), `agents/evolve-auditor.md` (`:171`), `cyclerun_dispatch.go` (`:104`), `resume_execution.go` (`:128`) and `gateregistry/` (`:238`). `audit_rules.go` is not fenced. The console builds K6 and K7 and any edit of the other fenced files, and the integration batch merges them. A lane cannot change a protected path.

## 7. Order

1. K9 first for the ADR draft, so that the review sees the contract before code.
2. K5, K2 and their tests. They are pure data and have no live effect.
3. K1, K3 and K4. They compute the scope and mark the findings.
4. K8 and the shadow stage. The prompt and the verdict do not change here.
5. K6 and K7. They run only at `enforce`.

Each step is inert until the next one wires it. The verdict stays unchanged until step 5.

## 8. Tests (red first)

| Test | Package | What it proves | Acceptance |
|---|---|---|---|
| T1 `TestScopeHunksIncludeNewFile` | `core` | A repair that adds a file has a hunk for it. A new finding in it is not late. Also: an insertion that shifts lines (a finding at the new line number is inside), a pure deletion, and a second hunk after an insertion. Also: the round-1 tree comes from the ledger, and the round-r tree equals the later binding tree. | 1 |
| T2 `TestScopeUnavailableIsRound1` | `core` | A missing binding, an empty tree SHA, a lost archive and a git error each give "no scope", no filing and a signal. Zero cumulative hunks also give no late finding. | 2 |
| T3 `TestProjectionKeepsLine` | `fixability` | `path:line` is kept. A finding with no line is in scope and never late. | 1 |
| T4 `TestScopeReachesAuditOnDispatchAndResume` | `core` | Round 2 reaches the audit phase through the dispatch entry point and the resume entry point. | 1 |
| T5 `TestRewordedOldFindingStaysOpen` | `core` | An old finding that the auditor rewords outside the hunks stays open. A new id at the location of an open previous finding is not late. | 2 |
| T6 `TestMissingDispositionIsOpen` | `core` | A previous id with no disposition is open, not fixed. | 2 |
| T7 `TestLateBarFilesMediumBlocksHigh` | `convergence` | With `lateFileBar` HIGH, a late MEDIUM is filed and a late HIGH and a late CRITICAL block. INFO and capability are not filed for audit-repair. | 1, 2 |
| T8 `TestPromptScopeBlockFromRound2AtEnforce` | `audit` | The prompt lists the host ids and the hunks in round 2 at enforce. It has no block in round 1 and no block at shadow. | 1 |
| T9 `TestPersonaScopesFullChecklistToRound1` | `audit` | The persona has one pointer line: "Run the full checklist unless the host gives a Round scope block". | 1 |
| T10 `TestCheckOnlyForcesFail` | `audit` | A late entry with a known id, a location in the hunks, or a severity of HIGH or above turns a PASS into FAIL. No input turns a FAIL into PASS or WARN. A table of 8 FAIL fixtures all stay FAIL. | 2 |
| T11 `TestReplayCycle2035Shape` | `core` | Synthetic fixture from the incident table (`:117-127`): round 2 adds M3 inside the hunks, round 3 adds H1 and a late MEDIUM outside them. Round 3 follows a ship-to-audit edge with zero cumulative hunks. M3 and H1 block. The late MEDIUM is filed. | 1 |
| T12 `TestShadowDoesNotChangePrompt` | `audit` | At shadow the prompt equals the unscoped prompt, and the signal carries `would_file` and `would_block`. This is a guard: it passes today. | 2 |
| T12b `TestShadowConvergenceInputUnchanged` | `core` | At shadow, the convergence input and the `CONVERGENCE_RUNG` fields are byte-equal to today's. | 2 |
| T13 `TestStageActorAtEnforce` | `core` | A `StageActor` spy fires at enforce through the production entry point (ADR-0167 S3). A filed finding makes exactly one follow-up. | 2 |
| T14 docs check | `docs` | ADR-0126 decision 9 links ADR-0168, and convergence-policy section 11 states the amended rule. Both amended sentences of decision 9 are checked, and the three kept phrases stay. | 3 |

T12 and T12b are regression guards. The other tests are red before the build.

## 9. Rollout

- **Stage:** the key `workflow.convergence.audit_rereview.stage`, default `shadow`. It has its own signal, closure and scorer, so its evidence does not mix with the convergence decider (ADR-0167).
- **Shadow:** the prompt does not change. Each round from 2 on gets one record. It holds the late counts by severity, the hunk count and the planned filings. It also holds the round minutes and the verdict.
- **Evidence:** records follow [ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md): hash-chained, and one verb recomputes the verdict. The shadow late counts over the first N repaired cycles replace the replay of real reports.
- **Flip rule:** flip to `enforce` with `evolve stage verify <key>` after 2 shadow waves with a verified chain. The enforce path (K6, K7) must exist, and T13 must pass. T13 needs the core `StageActor` port of ADR-0167 S3. Today only `integration.StageActors` exists (`go/internal/integration/driver.go:527`), and the open item `shadow-contract-k1b-stage-actor-ports` builds the port.
- **CI parity:** all tests run with `go test -count=1` on Linux and macOS, with Go 1.23. The git fixtures use a temp repository and no system `ps`.

**What changes our mind:** if the shadow data shows fewer than 1 late MEDIUM or LOW per repaired cycle, the rule saves little. Then keep K1 to K5 and drop K6 and K7. The weak case of cycle 2035 (F2.10) already predicts a small gain.

## 10. Risks

| Risk | Control |
|---|---|
| A real defect in old code is filed and never fixed | Only MEDIUM and LOW are filed, as a follow-up with its location (D9). A late HIGH and CRITICAL block. |
| The repair moves old code into a hunk, so old findings count as in scope | This is safe: it blocks more. T10 covers it. |
| The hunks are wrong after a rebase | K1 reads two tree SHAs from the ledger, not a branch name. T1 and T2 cover it. |
| The auditor rewords an old finding to make it look new | D4 and T5: an undisposed old id stays open, and its copy is not late. |
| The change lifts a verdict through a bug | The host calls only `fail()` (D6). T10 holds 8 FAIL fixtures. |
| Fenced files need the console | Route K6, K7 and every edit of a fenced file to the console. |
| Dependencies: `cli-neutral-task-assignment-and-guard`, `isolation-dispatch-request-channel` and `selfheal-action-catalog-validator-budgets` are in `runtime/.evolve/inbox/consumed/`. `cli-capability-negotiation`, `cli-neutral-profiles-and-personas` and `selfheal-repair-cycle-path` are still open. | The build waits for the three open dependencies, as the operator note says, and for `shadow-contract-k1b-stage-actor-ports` (the core `StageActor` port, ADR-0167 S3). Add it to the item deps. |

## 11. Follow-up (not in this item)

- **Falsification run.** A fresh read of the cited location and the certificate, with its own persona, `cli_routing` row, cell read grants and an isolation test. With it, a late HIGH can be filed when the check refutes it. Until it exists, a late HIGH blocks.
- **Lift rule.** A host rule that lifts a FAIL needs its own decision, and decision 9 must then say so in words.

## 12. Open questions

- **Q1 (answered):** the audit rounds use a separate round-disposition file with the shape of the defect-ledger entry and the same `Resolver`. DEFERRED counts as open (D4). The builder reads the exports first.
- **Q2:** which existing writer files the follow-up of D9: the inbox writer or the ADR-0158 hand-off path? The builder lists both exports and picks the one that deduplicates.

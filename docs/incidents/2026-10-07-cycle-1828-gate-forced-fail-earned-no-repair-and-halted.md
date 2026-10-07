# Cycle 1828: a gate-forced audit FAIL earned no repair round, and the retro halted the loop (wave 76, 2026-10-07)

## What happened

Cycle 1828 (`generated-skill-command-shadows-its-skill`) changed `RenderCommandStub` in `go/internal/skillcheck/commands.go` and regenerated the 30 `commands/*.md` stubs. Its auditor wrote **PASS** (confidence 0.88; ACS 170 green, 0 red). The in-process CI-parity gate **skills-drift** then forced the verdict to FAIL. The cycle got no repair round, and its retrospective halted the whole loop.

### Timeline

| Time (UTC) | Step | What happened |
|---|---|---|
| 11:43:59 | Cycle start | Scout, triage, TDD and Build ran normally. |
| ~12:20 | Audit | The auditor declared PASS. `skillsDriftCheckDefault` reported 30 drifted artifacts, and `Classify` recorded two error diagnostics (`audit-fail-reason.json`): ``skill projection drift: 30 artifact(s) stale vs their SSOTs … Run `evolve skills generate`. Drifted: commands/adversarial-testing.md, …`` and `verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [skills-drift] …`. |
| 12:20:05 | Audit-FAIL disposition | `ORCHESTRATOR_AUDIT_REPAIR_DECLINED`: "audit FAIL earned no repair round: audit declared no failure class; nothing to base a retry on". The cycle went to retro. |
| ~12:23 | Retro | The retrospective wrote `failure-decision.json` with category `infra-systemic`: the in-process gate grades with the base binary's generator, so the rejection is the pipeline's version skew, not a task defect. |
| 12:23:51 | Halt | `applyFailureDecisionFloor` treated the prose floor category as a system halt: `ORCHESTRATOR_SYSTEM_FAILURE` and `LOOP_SYSTEM_FAILURE_HALT`, `retro_decision=system-failure-floor: infra-systemic`. |

## Root cause

### 1. A gate-forced FAIL declares no failure class

`computeRetryEnvelope` (`go/internal/core/retry_envelope.go`) keys the retry decision on the audit's own declared class, which `auditFailEnvelope` read only from the verdict sentinel's failure block in `audit-report.md` (`phasecontract.ReadFailureBlock`). A PASS narrative carries no failure block. The gates that override it write error diagnostics, which the orchestrator persists as `CycleState.AuditFailReasons`, but they never declare a class. So every FAIL that deterministic gates forced over a PASS or WARN narrative declined with "audit declared no failure class". The gate had already named its one-command remedy in the diagnostic, and nothing read it.

Only one shape of verdict conflict had a recovery route: the bookkeeping regrade (`core/bookkeeping_regrade.go`, ADR-0086), for conflicts whose other reasons are all defect-ledger or closure-claim bookkeeping.

### 2. The retro's prose floor claim had one overrule, for one category

`applyFailureDecisionFloor` halts on a floor category in the retro's `failure-decision.json` unless deterministic evidence contradicts it. The only contradiction it knew was for `verdict-incoherence`: a recorded FAIL with persisted substantive fail reasons is a diagnosed downgrade, not a forgery. An `infra-systemic` claim had no such check, so a FAIL whose every persisted reason was a deterministic gate diagnosis halted the batch on the retro's word.

### History this respects

The skills-drift storm (`core/cyclerun.go`, `recordFloorVerdictFailure`): gate FAILs on the success path once never fed failure learning, and a self-defeating task retried forever. The fix made every gate FAIL a `FailedRecord` carrying its remediation. This change leaves failure learning as it is, and the new repair round is bounded by the same audit-repair envelope as every other one.

## The fix

**One table, in `core`** (`go/internal/core/audit_gate_remedy.go`, `auditGateRemedies`). Each row maps a deterministic audit gate to the stable prefix of its diagnostic, the one-step remediation the repair brief leads with, and whether the gate reads the tree statically (`isStaticReading`) or executes code:

| Gate | Reason prefix | Remediation | Static reading |
|---|---|---|---|
| gofmt | `gofmt: ` | run `gofmt -w -s .` in go/ | yes |
| solution-contract | `solution contract: ` | fix each listed violation, then self-check with `evolve solution check` | yes |
| skills-drift | `skill projection drift: ` | regenerate with the worktree's own generator: `EVOLVE_WORKTREE_ROOT=<worktree> go run ./cmd/evolve skills generate` in <worktree>/go | yes |
| go vet gate | `go vet ./... reported ` | fix each offender `go vet ./...` reports in go/ | no |
| acs-durable gate | `acs-durable (-tags acs) FAILED ` | make each offender pass under `make -C go test-acs-durable` | no |
| integration-tier gate | ``the integration tier (`go test -tags integration`) reported `` | make each offender pass under `go test -tags integration` in go/ | no |
| apicover-enforce gate | `apicover -enforce flagged ` | name each flagged export in a test of its package | no |
| apicover new-package graduation gate | `apicover new-package graduation: ` | add each new package to go/.apicover-enforce with an apicover_named_test.go | yes |
| EGPS red_count>0 | `EGPS: red_count=` | turn every red ACS predicate green | no |

Every derived block declares `code-audit-fail` (`gateDerivedClass`), the class whose policy row is task-level `retry-with-fix`. A reason that carries the harness-red clause (`HarnessRedClauseMarker`, the EGPS reds the harness itself could not run) matches no row.

- **The derived failure block** (`gateFailureBlock`). Only when deterministic gates forced the FAIL over a PASS or WARN narrative (a verdict-conflict record exists), every other persisted audit fail reason is a gate diagnosis, and one or more is, does the audit FAIL have a failure block: class `code-audit-fail`, and one defect per diagnosis, written `<remediation> — <diagnosis>`. A reason outside the table (a host failure, a bookkeeping reason, a CLI wall), or no conflict record (a FAIL narrative beside the gate), means no derived block.
- **The envelope** (`auditFailEnvelope`). The auditor's own declared block still wins. Only when the report declares none does the gate block supply the class, and then the granted envelope narrows to `retry@build` or `decline` (`gateRemediationEnvelope`, through the `narrowRetryEnvelope` helper the explanation-correction route now shares): a mechanical remediation needs no TDD round. The `code-audit-fail` budget bounds it.
- **The brief** (`auditRejectionReasons`). When the report declares no block, the gate block renders first, as `audit defects (the deterministic gates' failure block, class code-audit-fail):` followed by ``- regenerate with the worktree's own generator: … — skill projection drift: …``, so the remediation leads and survives truncation. A declared block keeps the brief it had. The adjudicator reads the same function.
- **The floor** (`floorClaimRefutation`, generalizing the verdict-incoherence overrule into one evidence rule). A prose floor claim is overruled when the persisted evidence refutes it: the existing rule (a `verdict-incoherence` claim against substantive fail reasons) and the new one (any claim, when only static gate readings forced the FAIL over a PASS or WARN narrative and no ship fail reason is recorded, `staticGateReadingsForcedTheFail`). The evidence is orchestrator memory, never a workspace file an agent could write. A reason outside the table, a gate that executes code, a ship reason, a missing conflict record, or a conflict record with no diagnosis keeps the halt. The deterministic dossier floor and the disposition-corroboration rule run first and are unchanged.
- **The producer side.** The new-package graduation diagnostic now starts with a stable prefix (`apicover new-package graduation: `); before, it started with a count and no matcher could anchor on it. The four diagnostics that state a remedy render it from the table (`core.AuditGateRemedy`), and the EGPS harness-red clause renders `core.HarnessRedClauseMarker`, so neither text has a second home. gofmt reports an I/O error as an error, not a `gofmt parse error:` offender. The integration tier is unchanged: when its serialized retake cannot start, it still FAILs on attempt 1's offenders, because a real red is never laundered by retake trouble; that FAIL earns the bounded repair round, and as an executed gate it cannot refute a halt.

### Design decisions

- **The table lives in `core`, not `phases/audit`.** The floor overrule reads it, and `core` cannot import `phases/audit` (the import runs the other way). This follows the bookkeeping precedent: `core` owns the reason classifier, and a `phases/audit` test feeds every real producer's diagnostic through it (`audit_gate_remedy_binding_test.go`). That test walks every check seam of the audit's `hooks` by reflection, so a gate added without a table row fails it.
- **Derived from orchestrator memory, not written into the report.** Appending a host sentinel to `audit-report.md` would overwrite the auditor's narrative reading, which the verdict-conflict record exists to keep, and would change what the coherence floor reads. A separate workspace file would be agent-writable and could talk the floor out of a halt.
- **The regrade wins by construction.** A regrade-eligible reason set holds only bookkeeping reasons and a conflict record; a gate-derived block needs one or more gate diagnoses and nothing else. The table test pins that no gate prefix is also a bookkeeping reason, so no FAIL can take both routes. A gate beside a bookkeeping reason takes neither, as before.
- **Excluded from the table.** `EGPS: acs-verdict.json ship_eligible=false` (the producer sets it from `red_count`, so it disagrees only on an inconsistent or written verdict file), an unreadable `acs-verdict.json`, host predicate execution or evidence failures, the explanation gates (their own correction route), the bookkeeping gates (the regrade), and the `strict_audit` promotion. Each still declines and still keeps the floor.

### The bound

The `code-audit-fail` row's `MaxRetries` is 2 in the compiled policy, and `.evolve/policy.json` sets no `failure_policy`. So with the shipped default, identical gate FAILs earn two repair rounds and the third declines with the envelope's own reason, `retry budget spent for code-audit-fail (2/2)`. The brief asked for "a second identical gate FAIL after the repair is declined", which holds at a budget of 1. The test drives both budgets, and asserts grants equal to the budget and then that exact decline.

## What this does not fix: 1828's gate RED was version skew

Cycle 1828's retro was right that the gate, not the code, was wrong. `skillsDriftCheckDefault` runs `skillcheck.Check` in-process, so the expected `commands/*.md` bytes come from the `RenderCommandStub` compiled into the orchestrator binary, the lane's base. 1828 changed that renderer. The base binary's `evolve skills check` on the worktree returned rc=2 with 30 drifted files; the worktree-built one returned rc=0 (the retro's `failure-decision.json` evidence).

So for 1828's exact shape, the repair round this fix grants cannot succeed: `evolve skills generate` from the base binary would revert the change (and the cycle's own predicates would then go red), and from the worktree build it changes nothing, which is why the remedy names the worktree's own generator (`EVOLVE_WORKTREE_ROOT=<worktree> go run ./cmd/evolve skills generate` in `<worktree>/go`, the form `core/ship_recovery.go` regenerates derived artifacts with). The round is bounded waste, two Build rounds at most, then a decline, and the floor now ends it as a task-level FAIL instead of a loop halt. The cure is the gate fix the retro filed as carryover `skills-drift-gate-grades-with-worktree-generator`: grade with the worktree's generator when the lane diff touches `go/internal/skillcheck/**`. It is out of this change's scope. For every gate RED that is not self-referential (a SKILL.md edited without regenerating, a gofmt-dirty file), the round applies the named remedy.

## And halted: why the floor overrule is safe

The halt is the second half of the defect: a task-level, deterministically diagnosed FAIL stopped the batch. After the fix, the 1828 shape (PASS narrative, a skills-drift diagnosis, a retro decision of `infra-systemic`) ends as a task-level FAIL, and the failure adapter disposes of it with the cycle's history. The overrule needs a verdict-conflict record (a PASS or WARN narrative the gates overrode) and every other persisted audit fail reason to be a diagnosis from a static row of the table, read from `CycleState.AuditFailReasons`, which the orchestrator writes at the `recordFloorVerdictFailure` chokepoint.

The first version of this fix counted every row, and review fix round 1 found that a host failure could reach the table as a gate's offenders: `go vet` turns any non-zero exit into offenders, so a full disk reads as a vet issue; the integration tier's retake that could not start keeps attempt 1's offenders (by design: a real red is never laundered by retake trouble); an EGPS reason counted even when its reds were predicates the harness could not run; and gofmt reported an unreadable file as a `gofmt parse error:` offender. Each would have refuted a correct `infra-systemic` halt. So only the rows whose diagnosis is a static reading of the tree refute a prose claim: gofmt, solution-contract, skills-drift and new-package graduation. gofmt counts because `codequality.UnformattedGoFiles` now returns an error (the gate warns and is skipped) unless every stderr line is a positioned parse diagnostic. The rows that execute code (go vet, acs-durable, the integration tier, apicover-enforce, EGPS) keep their bounded repair grant but never refute. So the tier keeps its no-laundering rule, and its retake-failure FAIL earns the bounded repair round while the halt stands, because the row is not static. An EGPS reason carrying the harness-red clause matches no row at all, so it neither repairs nor refutes.

A genuine systemic failure therefore keeps the halt: a CLI wall ends the dispatch before the audit's gates run; a disk or host failure surfaces as a gate that could not run (a warning, never a reason), as a host-predicate reason outside the table, or as an executed gate's offenders, which do not refute. The deterministic floor candidate (an audit-declared system class) halts before any prose is read, as before.

## Tests (red first)

Red logs are in the lane's scratch directory (`red-core-behavior.log`, `red-table-binding.log`, `red-base-overlay-core.log`).

| Test | Before the fix | After |
|---|---|---|
| `core/audit_gate_remedy_test.go::TestAuditGateFail_SkillsDriftOverPassNarrativeGrantsOneBuildRepairLedByItsRemediation` (drives `recordAndBranch` at the audit chokepoint with 1828's two reasons) | **red**: `decline="audit declared no failure class; nothing to base a retry on"` | green: one round at build; the brief leads with ``- run `evolve skills generate` `` |
| `::TestAuditGateFail_IdenticalGateFailsAreBoundedByTheEnvelopeBudget` (budgets 1 and the shipped 2) | **red**: 0 grants | green: grants equal the budget, then `retry budget spent for code-audit-fail (N/N)` |
| `::TestResumePath_GateForcedAuditFailRepairsAtBuildWithinTheBudget` (drives `RunCycleFromPhase`) | **red**: `phases=[audit retro]` | green: build twice, never tdd |
| `::TestAuditGateFail_ProseInfraSystemicRetroDoesNotHaltAGateDiagnosedFail` (the addendum's 1828 shape, audit then retro through `recordAndBranch`) | **red**: no grant, and `system-failure-floor: infra-systemic` | green: the grant, then no halt |
| `::TestDecideAfterRetroFloor_StaticGateDiagnosisRefutesProseInfraSystemic` (skills-drift over PASS; gofmt over WARN; all four static gates) | **red** (named `GateDiagnosedFailRefutesProseInfraSystemic` then) | green |
| `::TestDecideAfterRetroFloor_AGateThatExecutesCodeKeepsTheHalt` (fix round 1, probes P4, P5: a `go vet` diagnosis of a full disk; an integration tier whose retake could not run; acs-durable; apicover-enforce; an EGPS red beside skills-drift) | **red** on the round-0 lane: all five refuted the halt | green |
| `::TestAuditGateFail_AHarnessRedEGPSEarnsNoRepairAndKeepsTheHalt` (fix round 1, probe P11) | **red**: `next=build`, and no halt | green |
| `::TestAuditGateFail_AFailNarrativeBesideAGateDiagnosisDerivesNothingAndKeepsTheFloor` (fix round 1, probes P9 and PA2) | **red**: `next=build`, and no halt | green |
| `::TestAuditGateFail_TheDerivedRouteIsRetryAtBuild` (fix round 1, kills mutant N8) | green; red against N8 | green |
| `::TestAuditRejectionReasons_AnAuditorDeclaredBlockKeepsTheDerivedGateBlockOut` (fix round 1) | **red**: the derived block spoke over the declared one | green |
| `::TestDecideAfterRetroFloor_ProseInfraSystemicWithAnyNonGateReasonStillHalts` (a CLI wall; a gate beside a CLI wall; a gate beside a ship reason; a conflict record alone; `ship_eligible=false`; a host predicate failure; no reason) | green (preservation) | green |
| `::TestAuditGateFail_BookkeepingConflictStillTakesTheRegrade` and `::TestAuditGateFail_GateBesideBookkeepingEarnsNeitherRoute` | green (preservation) | green |
| `::TestAuditFailEnvelope_AnAuditorDeclaredClassKeepsItsFullEnvelope` | green (preservation) | green |
| `::TestAuditGateRemedies_EveryGateDeclaresARepairableClassAndARemediation` (walks every row: a class the envelope grants at build, a remediation, a prefix that classifies to its own row, no bookkeeping overlap) | **red** on the empty skeleton | green |
| `::TestGateFailureBlock_DerivesTheClassOnlyWhenGatesAloneForcedTheFail` | **red** on the skeleton | green |
| `phases/audit/audit_gate_remedy_binding_test.go::TestAuditGateRemedies_ClassifyEveryGateProducer` (every `hooks` check seam by reflection, plus EGPS red; each real diagnostic classifies as the gate the conflict record names) | **red**: 9 of 9 unclassified | green |
| `::TestAuditGateRemedies_HostAndBookkeepingFailuresAreNoGateDiagnosis` (fix round 1 adds EGPS reds the harness could not run, probe PA1) | green (preservation); the harness case **red** on the round-0 lane | green |
| `::TestAuditGateRemedies_ProducersRenderTheTableRemedy` (fix round 1: gofmt, solution-contract, skills-drift, graduation) | **red**: each message stated its own remedy text | green |
| `codequality/gofmt_test.go::TestUnformattedGoFiles_AnUnreadableFileIsAnErrorNotAnOffender` (fix round 1) | **red**: `gofmt parse error: open …: permission denied` offender | green |
| `codequality/gofmt_test.go::TestUnformattedGoFiles_AGofmtThatDiesSilentlyIsAnErrorNotAnOffender` (fix round 1) | **red**: `gofmt parse error: exit status 2` offender | green |
| `core/audit_gate_remedy_test.go::TestAuditGateFail_AnIntegrationTierRedKeepsItsRepairAndTheProseInfraSystemicHalt` (fix round 1, probe P5 end to end: a tier reason as the producer renders it when the retake could not run, audit then retro through `recordAndBranch`) | **red** on the round-0 lane: no halt | green: the bounded build repair, then the infra-systemic halt |

The verdict-incoherence behaviour is unchanged: `TestDecideAfterRetroFloor_Cycle1603DiagnosedDowngradeIsNotForgery`, `TestDecideAfterRetroFloor_ProseIncoherenceWithoutDiagnosedReasonsStillHalts` and `TestDecideAfterRetroFloor_DiagnosedShipReasonsAlsoContradictProse` pass before and after.

Round 0 claimed 17 overlay mutants killed; the review's rerun found two survivors, N8 (the derived route re-entering as `retry@explanation`, which also lands on build) and N21 (prefix matching after `TrimSpace`). Fix round 1 kills N8 with `TestAuditGateFail_TheDerivedRouteIsRetryAtBuild` (red against the reviewer's exact overlay on the round-0 tree, green on its unmutated control), keeps C0, N14 and N17 killed on the new code, and adds 12 mutants of its own hunks, all killed: the conflict requirement dropped, the harness-red exclusion dropped, go vet marked static, the static check skipped, skills-drift marked executed, the brief's declared-block guard dropped, the integration tier marked static, the remedy lookup returning the first row, gofmt's empty-stderr and non-parse-line guards dropped, the producers' table remedy replaced by a literal, and the harness clause reworded away from the marker. N21 is out of this round's scope. The round-0 list: They cover the conflict skip deleted, a non-gate reason skipped instead of refusing the block, an empty block allowed, the defect's order reversed, prefix matching widened to substring matching, a drifted prefix key, the wrong re-entry, the declared-block guard inverted or deleted, the ship-reason guard deleted, the gate refutation deleted, the verdict-incoherence rule widened, the brief's gate branch deleted, the narrowing guard inverted, the new-package prefix dropped, and a non-repairable class on a row.

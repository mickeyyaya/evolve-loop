# internal/coherence

> Covered elsewhere: the verdict-coherence floor and its halt policy ([ADR-0072](../adr/0072-system-failure-policy-and-halt.md)), the clean-exit false-FAIL storm that motivated it ([chronicle](../../chronicle/2026-07-false-fail-storm.md), [recovery ledger 862–899](../../operations/false-fail-recovery-862-899.md)), the landing record of the cross-artifact invariant stack ([deliverable-alignment §6.4](../../research/deliverable-alignment-2026-08/README.md)), and where `FailReasons` came from ([internal-cyclestate](internal-cyclestate.md)). This page keeps the package-level detail those do not.

## Purpose

`internal/coherence` runs two deterministic checks over one cycle's on-disk artifacts. `CheckVerdictCoherence` decides whether a recorded FAIL or WARN is contradicted by a green audit report and a green ACS verdict. That contradiction is the ADR-0072 `verdict-incoherence` halt, or a reconcile. `ReadCycleVerdicts` feeds it. `CheckCrossArtifactInvariants` runs four weak, advisory verifiers over the same workspace.

## Design

- **Pure check, thin reader.** `CheckVerdictCoherence` does no I/O and is table-tested. `ReadCycleVerdicts` is the adapter that reads the two verdicts. The callers are `core.detectVerdictIncoherence` (`system_failure.go`), its twin `buildFailureDossier` (`failure_dossier.go`), and the runner's verdict package (`phases/runner/verdict/artifact.go`).
- **Decision order.** A recorded verdict other than FAIL or WARN is coherent. So is a cycle whose audit never ran (a genuine incomplete), and a negative verdict with `SubstantiveError` set. The check only fires when both the audit and the ACS verdict are present and PASS. There, `DeliverableValid` picks the result: `Reconciled` (category `verdict-reconciled`) when true, `Incoherent` (category `verdict-incoherence`) when false.
- **`DeliverableValid` comes from the caller.** It means the audit report passed the full `deliverable.Verify` chain: challenge token, required sections and the ADR-0039 failure context. The cheap sentinel read in `ReadCycleVerdicts` is not enough. `core` gets the verifier by injection, because `deliverable` imports `core`. A nil verifier or a Verify error leaves it false, which keeps the conservative halt.
- **Verdicts are read through `phasecontract`.** The audit file name comes from `phasecontract.ArtifactFilename("audit")`. The verdict comes from `ParseVerdictSentinel` or `ParseVerdictSentinelFull`, which is anchored to the `<!-- evolve-verdict -->` sentinel and carries the placeholder-echo guard. A bespoke regex or grep would read the contract's printed example, or any "PASS" in prose, as a real verdict.
- **The cross-artifact stack.** Four weak deterministic verifiers run over one cycle's artifacts and each reports on its own:

  | Invariant | Holds when |
  |---|---|
  | `verdict-agreement` | the embedded evolve-verdict sentinel agrees with the verdict in `acs-verdict.json` |
  | `test-count-agreement` | the claimed green, red and skip counts and `predicate_suite.total` match an independent recount of the file's own `results[]` |
  | `referenced-paths-exist` | every `evidence_path` the audit sentinel's failure block cites resolves under the workspace or the lane worktree |
  | `provenance-phase-order` | `phase-timing.json` runs forward, and the first audit does not start before the first build |

  A stack of weak deterministic verifiers approaches the power of a strong verifier at near-zero cost, and it is immune to LLM-judge bias (Weaver, arXiv:2506.18203). The stack complements the adversarial audit and never replaces it.
- **Three statuses.** Every invariant reports `ok`, `violated` or `indeterminate`. `indeterminate` means the artifacts were absent, malformed or silent.
- **`acs-verdict.json` is decoded locally.** `acsArtifact` holds only the fields the stack compares. Importing `internal/acssuite` would add `gitexec`, `sysexec`, `changedpkgs` and `verifylock` to this package's dependencies. `ReadCycleVerdicts` decodes the same file locally too.
- **Phase order.** Entries without both timestamps are skipped, since timestamps are optional in the wire shape. Fewer than two timed entries, or a timestamp that is not RFC3339, is `indeterminate`. An entry that ends before it starts, or starts before its predecessor, is `violated`. Only the first build and the first audit are compared, so re-dispatched build and audit rounds stay `ok`.
- **The production caller.** `core.recordCrossArtifactInvariants` (`internal/core/crossartifact_invariants.go`) writes the report to `<workspace>/crossartifact-invariants.json` on every cycle from `finalizeCycle`, before the ADR-0072 floor runs, and passes `cs.ActiveWorktree` as the worktree.

## Invariants

- **The recorded verdict is never trusted alone.** The whole check compares it with the artifacts the phases wrote.
- **An absent verdict cannot prove forgery.** When the audit or ACS verdict is missing or not PASS, the result is coherent, because a false halt costs the whole batch. The code keeps a one-line why. Pinned by the `audit PASS but ACS artifact absent` case of `TestCheckVerdictCoherence`.
- **An audit that never ran is a genuine incomplete, not forgery.** The code keeps a one-line why. Pinned by the `audit did not run` case of `TestCheckVerdictCoherence`.
- **An explained negative is never forgery.** `SubstantiveError` covers a substantive (non-teardown) bridge error and a diagnosed runner-side gate downgrade, which is persisted as error-severity reasons. Pinned by the `substantive (non-infra) error` case of `TestCheckVerdictCoherence`.
- **`Incoherent` and `Reconciled` are mutually exclusive.** `DeliverableValid` only downgrades a would-be halt. It never turns a case that is already coherent into a reconcile. Pinned by `TestCheckVerdictCoherence_Reconcile`.
- **The anti-laundering boundary.** A malformed report that only carries a PASS sentinel has `DeliverableValid` false and stays `Incoherent`, so a forgery is never laundered into a PASS. Pinned by `TestCheckVerdictCoherence_ForgedStillHalts`.
- **`ReadCycleVerdicts` never errors and never guesses.** An absent or unparseable artifact yields `""`. `auditRan` is true whenever the audit report file exists, even when it has no parseable sentinel. A reader that guessed would defeat the check. The absent case is pinned by `TestReadCycleVerdicts_Fixture`. The exists-but-unparseable case has no unit test.
- **Every cross-artifact invariant fails safe into `indeterminate`.** An absent or malformed artifact never reads as a verified match, because that is how a presence-only check gets gamed. It is never a violation either, because an advisory that fires on absence earns a false-positive rate and gets switched off. Pinned by `TestCrossArtifactInvariants_AbsentArtifactsAreIndeterminateNotOK` and `TestCrossArtifactInvariants_MalformedArtifactsAreIndeterminateNotOK`.
- **Prose never satisfies `verdict-agreement`.** Pinned by `TestCrossArtifactInvariants_ProseVerdictCannotSatisfyTheSentinelInvariant`, whose report is full of "PASS" and a placeholder echo but has no real sentinel.
- **`Violations()` never includes an indeterminate.** Pinned by `TestCrossArtifactInvariants_ViolationsReturnsOnlyTheViolated`.
- **The report is deterministic.** It holds exactly the four invariants in the declared order, and two evaluations of an unchanged workspace are identical. The report must be diffable across cycles, because that diff is the only way the advisory's false-positive rate becomes measurable. Pinned by `TestCrossArtifactInvariants_ReportShapeIsDeterministic`.
- **A non-ok status always carries evidence.** A finding nobody can act on is not a finding. Every cross-artifact test asserts it through `xaWantStatus`.
- **The stack is advisory.** `InvariantReport.Advisory` is always true and no invariant blocks a cycle. Graduating an invariant to blocking is a separate decision that needs a measured false-positive rate near zero. The no-blocking half is pinned at the seam by `internal/core/crossartifact_invariants_wiring_test.go`.
- **Containment is checked before the stat.** `filepath.Join` cleans `../` segments, so an escaping citation could otherwise land on a real file outside both roots and read as resolved. A path the lane did not write is not evidence the lane produced. The code keeps a one-line why. Pinned by `TestCrossArtifactInvariants_EscapingReferencedPathIsViolatedNotResolved`.
- **Only the lane's own tree counts as evidence.** Cited paths resolve under the workspace, then the lane worktree, never the project root. The unit half is pinned by `TestCrossArtifactInvariants_ReferencedPathsResolveUnderWorkspaceThenWorktree`. That the caller passes the lane worktree is pinned in `internal/core/crossartifact_invariants_wiring_test.go`.

## Findings

- **cycles 862–899**: the clean-exit deliverable-authority bug recorded FAIL while the on-disk audit report and ACS verdict were PASS, and the loop retried the same features for 38 cycles. This package's coherence signal is the deterministic answer ([ADR-0072](../adr/0072-system-failure-policy-and-halt.md), [chronicle](../../chronicle/2026-07-false-fail-storm.md)).
- **cycles 930/931/932**: `SubstantiveError` was left unpopulated at the only call site while the explanation sat in the response diagnostics. A diagnosed FAIL with green artifacts was labeled forged and halted the batch ([chronicle](../../chronicle/2026-07-false-fail-storm.md)).
- **Clean-exit late write**: the bridge can declare a phase's clean exit before Claude Code finishes its post-turn async writes. The runner's settle window was seconds, while the observed write tail was 60–90 s (cycles 930/931/932 and cycle 3). The runner then records FAIL while a valid audit report is still landing. That produced `DeliverableValid` and the `verdict-reconciled` self-heal. `go/acs/cycle1590` locks the acceptance after the cycle-1582 halt.
- **cycle-603**: a printed placeholder example was read as a real verdict. That is why every verdict read here goes through the `phasecontract` sentinel parser ([ADR-0039](../adr/0039-failure-floor-and-failure-signal-contract.md)).
- **cycle-1022**: `SubstantiveError` proved that a reason existed while every operator surface stayed silent about what it was. That added `FailReasons` ([internal-cyclestate](internal-cyclestate.md)).
- **cycle-1673 (M1)**: a document claimed zero red predicates beside an `acs-verdict.json` that recorded `red_count=3`. That is the shape `test-count-agreement` catches.
- **Measured before pinning**: the `predicate_suite.total` check ran on three real artifacts (cycles 1659, 1666 and 1673). All had `total == len(results)` and claimed counts equal to the recount, a measured false-positive rate of 0/3.
- **cycle-1676 audit (L1)**: the first `resolvesUnder` joined a citation to a root without rejecting `../` escapes, so a citation that climbed far enough resolved to `/etc/hosts`. The containment check fixed it ([deliverable-alignment §6.4](../../research/deliverable-alignment-2026-08/README.md)).
- **#612**: a project-root snapshot names a tree the lane did not write, so it is not evidence of what the lane produced.
- **The 1054/1060 breaker lesson**: a check that gains teeth before its false-positive rate is measured becomes a flake generator. That is why the stack ships advisory and records its report on every cycle.

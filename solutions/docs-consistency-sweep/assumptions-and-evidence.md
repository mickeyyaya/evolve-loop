# Docs consistency sweep — assumptions and evidence

Every number in the options and the recommendation cites an entry here: `A<n>` for an assumption,
`E<n>` for evidence. Each entry was checked against the worktree at base `4304168f` on 2026-09-27.

## Assumptions

- **A1** — A living doc (an ADR's decision text, an architecture page, the runtime reference) is read
  as current truth, so it must match the code. A dated record (an incident, a disposition log, a
  chronicle) is read as history, so it is annotated as superseded, with a pointer, and its original
  text is kept. Source: the inbox acceptance "corrected or explicitly marked historical with a link to
  its superseding doc" (operator-stated).
- **A2** — A renumber costs roughly one edit per inbound reference that means the moved ADR. So the
  cheaper move is the ADR with fewer inbound references. Source: builder estimate, measured in E2.
- **A3** — Files under `go/acs/` are TDD-owned and a build phase may not edit them (builder profile
  `disallowed_tools`). A stale ADR number in an ACS predicate comment therefore stays until a TDD
  cycle touches it. Source: `agents/evolve-builder.md` "EGPS Predicate Authoring".
- **A4** — No code mechanizes the operator's two-zero-ship-cycles halt. A compiled breaker for it is
  a control-plane change (`go/internal/core/` blocker breaker). That is out of scope for a document
  cycle (ADR-0099). Source: E6.

## Evidence

- **E1** — Two ADR numbers were shared: `0076-convergence-architecture.md` /
  `0076-failure-disposition-boundary-escalation.md` (both added 2026-07-23) and
  `0082-regression-test-impact-selection-shadow.md` / `0082-shared-worktree-add-retry.md` (both added
  2026-08-04). The highest free number was 0107. Citation: `ls docs/architecture/adr`, and
  `.evolve/runs/cycle-1719/test-report.md` finding 2.
- **E2** — Inbound references outside CHANGELOG and `.evolve`: 105 for ADR-0076 in its convergence
  sense (24 of them in `go/acs/` predicate comments, which a build may not edit, A3), and 3 for the
  failure-disposition ADR (`runtime-reference.md`, `go/internal/policy/policy_disposition.go`,
  `docs/research/factory-pipeline-findings-2026-09.md`). For ADR-0082: 24 for regression TIA
  (including `go/internal/policy/regressiontia.go`), and 10 editable references for the worktree-add
  retry (ADR-0083 ×3, `internal-swarm.md` ×2, the worktree-provisioning chronicle ×5), plus 1 in
  `go/acs/regression/cycle1270`. Citation: `git grep -c` at base.
- **E3** — `docs/architecture/adr/0092-audit-repair-loop.md:21` linked `0076-continuation.md`, a file
  that never existed. Citation: DCS-03 RED output in `test-report.md`.
- **E4** — ADR-0101 gave `[orchestrator]` 284 (line 29) and 287 (line 139). Both are correct
  measurements of different units: the inventory reports 287 prefix-literal occurrences, 284 of them
  `Fprint*(os.Stderr, …)` sites. Its slices S1, S2a, S2b, S3 and S4a are recorded as landed.
  Citation: `docs/research/signal-center-inventory-2026-09-13.md:132`;
  `docs/architecture/signal-center-design.md` §15.1–15.5.
- **E5** — `EVOLVE_PHASE_RECOVERY` is absent from `go/internal/flagregistry/registry_table.go`. The
  live dial is the policy key `recovery.phase_recovery` (`go/internal/config/policy_stages.go:29`).
  `docs/architecture/packages/internal-config.md:66` records the env flag's retirement.
- **E6** — There is no Go implementation of the two-zero-ship halt. The nearest compiled rule is the
  blocker breaker's `consecutive-failures` rule, default ceiling 3 failing cycles
  (`go/internal/core/blocker_breaker.go:188-214`, `go/internal/policy/policy_failure.go:83`). The
  goal-stall escalation defaults to 3 empty and 5 non-progress cycles
  (`go/internal/policy/budgets_recovery.go:50-64`).
- **E7** — The coverage map had 57 rows, one of them a stale duplicate of the cycles 1634/1636 row.
  The newer copy carries `TestRunCycle_TriageFailIsAWarnOutcomeNamingTheReason`. After the dedupe, and
  with the ledger chain-safety row flipped, the tally is 56 rows: 43 ✅, 10 🟡, 3 ❌, 0 ⛔. Before the
  edit the summary still said 73 / 40 / 20 / 10 / 3 (the 2026-05-29 sweep). Citation: DCS-15/16
  probes in `test-report.md`.
- **E8** — The ledger chain-safety fix is `1845945a` (#450), pinned by
  `TestFileLedger_AppendLifecycle_ChainsAndMapsRecord`, `TestLedgerWrites_AreChainValid` and
  `TestRebaseline_ForeignUnchainedTailLine_SealsAndVerifies`. The backward-anchor half is pinned by
  `TestAnchor_RejectsAmbiguousSeq` (cycle-1433). The retro completion cutoff was fixed by
  `a812b5c3` (#432), and its index row was flipped in `003242fb` (#434).
- **E9** — Code truths: `aggregatePriority` lists `LivenessExhausted` first
  (`go/internal/bridge/panestream/livenesscenter.go:139-145`). codex 0.139's `esc to interrupt` is
  matched by `busyAffordanceRE` (`panedelta.go:48`) and pinned by `TestPaneBusy_Codex0_139_Working`.
  `profiles.go:112` expands `AllowedTools`, and has since `302cc611` (before the 2026-06-07 rescue
  record was written). Peer cycle 1718 (`10e1a8f4`) rewrote that rescue row on main as "drop —
  **superseded**" after this cycle's base, so this cycle takes main's blob `271b580e` for the file
  verbatim, and the fleet rebase has nothing to conflict on. ADR-0100's e2e proof declares `handoff-build.json` only in a test-local
  `fixtureCatalog` (`declared_deliverables_e2e_test.go:59-66`).
- **E10** — The ship-streak goal was 5 consecutive ships in the 2026-09-14 verification wave
  (`docs/research/verification-wave-findings-2026-09-14.md:12`). It was a "six-consecutive-ships
  campaign" by 2026-09-26 (`docs/incidents/2026-09-26-triage-claim-left-to-the-agent.md:5`). No
  operations doc recorded either (`docs/research/factory-pipeline-findings-2026-09.md:260-263`).
- **E11** — The inbox lists nine items (`.evolve/evals/docs-consistency-sweep.md:65-66`). Option 1,
  as staged in this cycle, edits 20 files: 19 docs and 1 Go comment
  (`go/internal/policy/policy_disposition.go`). 7 of the 20 change only because of the renumber:
  - the 2 moved ADRs (0107, 0108);
  - 5 files whose only edit is an ADR-0076/0082 reference: ADR-0083, `internal-swarm.md`, the
    worktree-provisioning chronicle, `factory-pipeline-findings-2026-09.md` and
    `policy_disposition.go`.

  The other 13 carry content corrections: `CLAUDE.md`, ADRs 0044, 0068, 0092, 0100 and 0101,
  `bidirectional-channel.md`, the 2026-08-09 incident, `REGRESSION-COVERAGE-INDEX.md`,
  `operating-policy.md`, `pipeline-factory-rules.md`, the 2026-06-07 rescue-branch record and
  `runtime-reference.md`. No edit time was measured for any option. Citation:
  `git diff --cached --name-status -M 4304168f`, excluding `solutions/`, `.evolve/evals/` and
  `docs/explain/`; the per-file split is from `git diff --cached --word-diff=porcelain 4304168f`.
  After the fleet rebase onto `2a455b0d`, whose main already holds peer cycle 1718's identical
  rescue-row edit, the landed diff has 19 of the 20 files (12 content corrections). Citation:
  `git diff --cached --name-status 2a455b0d`, with the same exclusions.
- **E12** — At base the two-zero-ship halt was stated in three homes, none linking to another:
  `docs/operations/pipeline-factory-rules.md:58` (§3.8, listed as a breaker), `CLAUDE.md:9` (an
  operator guardrail) and `docs/incidents/2026-08-10-continuation-absorbing-fail.md:80` ("2-zero-ship
  halt … now repo policy"). `operating-policy.md` did not state it. The only other match,
  `docs/research/factory-pipeline-findings-2026-09.md:191,293`, reports those three statements and
  does not set the rule. After this cycle, `operating-policy.md` §4.1 is the one canonical statement.
  `pipeline-factory-rules.md:58` and `CLAUDE.md:9` link to it. The incident is a dated record and
  keeps its text (A1). Citation: `git grep -n -i -E "2 consecutive cycles produce zero|two
  consecutive zero-ship|2-zero-ship" 4304168f -- docs CLAUDE.md AGENTS.md`.
- **E13** — The TDD-authored eval `.evolve/evals/docs-consistency-sweep.md` holds 22 checks, DCS-00
  to DCS-21. All 22 are GREEN under both bash and zsh on the handoff tree. Citation:
  `grep -c '^  - criterion: ' .evolve/evals/docs-consistency-sweep.md`, and
  `.evolve/runs/cycle-1719/tdd-red-runner.sh . .evolve/evals/docs-consistency-sweep.md`.

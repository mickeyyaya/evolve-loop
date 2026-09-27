# Build Explanation — Cycle 1719

## Build Binding
- Cycle: 1719
- Base SHA: 2a455b0d0f0a15a96c75ff4a477e80bfef69b50f

## Summary
This is a documents-only sweep of nine findings where docs disagreed with each other or with the
code. Two ADR number collisions are resolved: the failure-disposition ADR moves from 0076 to 0107,
and the `git worktree add` retry ADR moves from 0082 to 0108. Their inbound links are repointed, and
ADR-0092's dangling `0076-continuation.md` link is fixed. ADR-0101 is marked Accepted and states one
`[orchestrator]` count. ADR-0044 names its live policy dial. ADR-0068's aggregation rule lists
`Exhausted` first. ADR-0100's Verification credits the test-local fixture catalog. The codex busy
signal is corrected wherever it was called weak-signal. The rescue-branch `AllowedTools` row already
reads superseded on this base, so the sweep leaves it alone. The regression coverage index loses
its stale duplicate row, and its ledger row flips to covered. Its summary is recounted from its rows. The zero-ship halt now has one canonical
statement, which also records the six-ship streak goal. The document-cycle solution records the
options weighed.

## Rationale
The inbox acceptance allows "corrected or explicitly marked historical with a link". Living docs
(ADRs, architecture pages, the runtime reference) are read as current truth, so they are corrected to
match the code. Each correction was re-derived from code or git, not copied from the inbox. Dated
records (the 2026-08-09 incident) are annotated with a pointer, and their original text is kept. The rescue-branch `AllowedTools` row is not in this diff. Peer cycle 1718 (`10e1a8f4`) made
the same correction on main while this cycle was in flight, and this cycle took main's wording byte
for byte. On base `2a455b0d` the file is identical to main, so the fleet rebase has nothing to
conflict on. In each colliding pair, the ADR with fewer inbound references moved: 3 and 10 edits, against
more than 100 for the convergence ADR. The zero-ship halt's home is `operating-policy.md`, which is
already the canonical operating-policy document that `CLAUDE.md` points to.

## Changed Areas
- `docs/architecture/adr/0076-failure-disposition-boundary-escalation.md` — removed by the rename to 0107, so ADR number 0076 names one file.
- `docs/architecture/adr/0107-failure-disposition-boundary-escalation.md` — the renamed ADR: H1 now ADR-0107, plus a `Renumbered:` line naming the old number and separating it from ADR-0076 slice D.
- `docs/architecture/adr/0082-shared-worktree-add-retry.md` — removed by the rename to 0108, so ADR number 0082 names one file.
- `docs/architecture/adr/0108-shared-worktree-add-retry.md` — the renamed ADR: H1 now ADR-0108, plus a `Renumbered:` line and a real link to ADR-0076.
- `docs/architecture/adr/0083-worktree-add-transience-classification.md` — its amendment link and prose now name ADR-0108, because the ADR it amends moved.
- `docs/architecture/adr/0092-audit-repair-loop.md` — the ADR-0076 link now targets `0076-convergence-architecture.md`, because `0076-continuation.md` never existed.
- `docs/architecture/packages/internal-swarm.md` — the two worktree-retry links point to ADR-0108.
- `docs/chronicle/2026-08-worktree-provisioning-retry.md` — five prose citations of the worktree-retry ADR now say ADR-0108.
- `docs/research/factory-pipeline-findings-2026-09.md` — the failure-disposition ADR path now uses its 0107 filename.
- `go/internal/policy/policy_disposition.go` — the `FailureDispositionPolicy` doc comment says "See ADR-0107", because its ADR moved (comment only, no behaviour change).
- `docs/architecture/adr/0101-signal-center.md` — status changes from Proposed to Accepted, naming the landed slices (S2b as re-scoped in the design's §15.5). The count is reconciled to one `[orchestrator]` figure (287 prefix literals), with its measurement point, and 284 is kept as the stderr-site subset.
- `docs/architecture/adr/0044-unified-phase-recovery-protocol.md` — the header names the policy dial `recovery.phase_recovery` and marks `EVOLVE_PHASE_RECOVERY` retired, with a link to its retirement record.
- `docs/architecture/adr/0068-bridge-signal-center-concurrency.md` — the aggregation rule now lists `LivenessExhausted` first, matching `aggregatePriority`, with the ADR-0070 rationale and the pinning test.
- `docs/architecture/adr/0100-declared-deliverables-gate.md` — Verification now says the e2e proof declares `handoff-build.json` in a test-local `fixtureCatalog`, not in the registry (Decision 1 removed it there).
- `docs/architecture/bidirectional-channel.md` — the codex table row, §2.3 and §6 now say codex ≥0.139 reads busy through `esc to interrupt`, pinned by `TestPaneBusy_Codex0_139_Working`. The `EVOLVE_CHANNEL` row names the policy dial.
- `docs/operations/runtime-reference.md` — the channel row's codex weak-signal claim and its env-dial wording are corrected. The consecutive-failures row gets its real `failure_policy.` key prefix and a zero-ship link to the canonical halt rule. The failure-disposition row links ADR-0107.
- `docs/operations/operating-policy.md` — new §4.1 is the one canonical statement of the two-zero-ship-cycles halt. It says the halt is an operator guardrail, not a compiled breaker, and names its compiled neighbours. It also records the ship-streak goal moving from five to six.
- `docs/operations/pipeline-factory-rules.md` — §3.8 no longer implies the zero-ship halt is compiled, and it links §4.1.
- `CLAUDE.md` — the zero-ship guardrail line links the canonical §4.1 statement.
- `docs/incidents/REGRESSION-COVERAGE-INDEX.md` — the older 1634/1636 duplicate row is dropped. The ledger chain-safety row flips to ✅, citing the #450 and cycle-1433 tests. The summary is recounted from the map, and the 2026-05-29 numbers move to a prose baseline.
- `docs/incidents/2026-08-09-zero-ship-batch.md` — the retro completion cutoff moves from open follow-ups to a new "Follow-ups closed" list citing #432, matching the index.
- `solutions/docs-consistency-sweep/assumptions-and-evidence.md` — the document-cycle evidence file (A1–A4, E1–E13) that every option cites. E11 carries the measured edit set (20 files, 7 of them renumber-only), which the option cost figures cite. E12 traces the zero-ship halt's three base homes, and E13 the eval's check count.
- `solutions/docs-consistency-sweep/options/1-correct-living-docs-annotate-records.md` — the chosen strategy, as implemented.
- `solutions/docs-consistency-sweep/options/2-historical-banners-only.md` — the rejected banner-only alternative and why it leaves contradictions.
- `solutions/docs-consistency-sweep/options/3-mechanize-drift-checks.md` — the CI-lint and compiled-breaker follow-up, out of scope for a document cycle.
- `solutions/docs-consistency-sweep/recommendation.md` — the comparison matrix, the winner (Option 1), the runner-up (Option 3) and the flip evidence.
- `.evolve/evals/docs-consistency-sweep.md` — the TDD-authored eval (22 doc-state checks), shipped unchanged with the cycle.

## Design Decisions
Option 1 moves the less-cited ADR of each pair and leaves a `Renumbered:` line, so old citations
still resolve for a reader. The ADR-0101 count keeps both true numbers, each labelled with its unit,
instead of picking one arbitrarily. The coverage summary drops ⛔ to 0, because no map row is ⛔;
untestable modes live in their own section. The halt text says plainly that no Go code enforces it.

## Verification
- `tdd-red-runner.sh` over `.evolve/evals/docs-consistency-sweep.md`: 22/22 GREEN under both bash and
  zsh. This includes the code-truth pins `TestPaneBusy_Codex0_139_Working`,
  `TestSignalCenter_ExhaustedDominatesAggregate` and `TestDeclaredDeliverables_*`.
- DCS-13 passes on main's wording of the rescue-branch row (it accepts "superseded").
- The change is pended on base `2a455b0d`, which is main's tip, and the rescue-branch record is
  byte-identical to main (no diff against the base).
- `evolve solution check docs-consistency-sweep`: OK.
- `go test -tags acs ./acs/cycle433/` (the ADR-0068 guard) passes.
- `go test ./internal/policy/` passes, and `gofmt` and `go vet` are clean on the edited Go file.

## Compatibility
No behaviour, flag, schema or API changes. The only Go edit is a doc comment. ADR files 0076 and
0082 keep their primary topics. External links to the two moved filenames break; in-repo links are
all repointed.

## Limitations
`go/acs/regression/cycle1270/predicates_test.go:32` still cites "ADR-0082:83" for the worktree retry,
now ADR-0108. Builders may not edit ACS predicates, so that comment is left for a TDD cycle. The
coverage index's "Incidents mapped" figure (43) is a hand count and no check pins it. The zero-ship
halt remains operator-enforced.

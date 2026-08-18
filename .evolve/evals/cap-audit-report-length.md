---
score_cap:
  - criterion: "auditReportMaxBytes bounds audit-report.md and the size contract is green against the real Classify path"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestAuditReportLength ./internal/phases/audit"
  - criterion: "Over-cap emits exactly one warning-severity diagnostic naming size and cap; under-cap and the exact boundary stay silent"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run 'TestAuditReportLength/(over_cap_warns_once|under_cap_silent|exact_boundary_silent)' ./internal/phases/audit"
  - criterion: "The cap is diagnostic-only: it never flips the verdict and never mutates the SHA-bound artifact on disk"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run 'TestAuditReportLength/(over_cap_does_not_flip_verdict|over_cap_does_not_mutate_artifact)' ./internal/phases/audit"
  - criterion: "Existing verdict-conflict semantics in Classify are unregressed by the size check"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestVerdictConflict ./internal/phases/audit"
  - criterion: "The documented auditor budget matches the code constant — no prompt/gate drift"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run TestC1525_001_DocCapMatchesCodeCap ./acs/cycle1525"
---

# Eval: Cap audit-report.md length (warning-only overflow diagnostic)

> Pins the bounded-report contract introduced in cycle 1522 for the fleet-scoped
> task `cap-audit-report-length`. `audit-report.md` previously had no upper bound
> on total size: the `## Issues` table grows one row per finding, the ship phase
> re-reads the entire body and SHA-binds it
> (`go/internal/phases/ship/audit.go:83`), and the next cycle's handoff carries the
> prior audit forward — so an unbounded report compounds token cost on every
> downstream read, which is exactly the per-agent token cost this cycle's goal
> targets. The fix follows the established in-package idiom from
> `go/internal/phases/audit/defect_ledger.go:56-61` (`defectLedgerMaxEntries` /
> `defectTextMaxRunes`): bound it, RECORD the overflow, never silently drop.
>
> Two asymmetries are load-bearing and are pinned permanently here. First,
> severity is wiring, not taste: `core.errorSeverityMessages` keys off
> `Severity=="error"` to build `AuditFailReasons`, so an error-severity size
> diagnostic would convert a merely verbose report into a dossier-visible
> failure. The size diagnostic must be `warning`. Second, the check must never
> touch the file on disk — ship SHA-binds those exact bytes, so a truncating cap
> would break the ship-time integrity check it was supposed to protect.
>
> Source incident: cycle 1522 (goal: cut per-agent token usage across phase
> agents); precedent incident cycle-1282 DEF-6, cited in `defect_ledger.go`.
>
> **Cycle-1525 continuation note.** This eval and the whole-report cap itself
> (`auditReportMaxBytes`, `audit.go`) were salvaged intact from cycle 1522
> (git commit `d3d32df6`); criteria 1-4 were already live and GREEN when
> cycle-1525's TDD phase picked this task back up — `TestAuditReportLength`
> in `go/internal/phases/audit/audit_report_length_test.go` covers all four
> without modification. Only criterion 5 (doc-code-sync) had no materialized
> predicate. Its evidence command below was re-pointed from the original
> `./acs/cycle1522` to `./acs/cycle1525`: `evolve acs suite` only ever scopes
> to the *current* cycle's `go/acs/cycle<N>/` directory plus the regression
> and redteam sets (see `go/acs/README.md`), so a predicate left under
> `cycle1522` would never run under the live cycle-1525 gate.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cap-exists | `auditReportMaxBytes` declared and the size contract runs against the real `hooks.Classify` seam | 7/10 | `go test -run TestAuditReportLength ./internal/phases/audit` |
| overflow-recorded | Exactly one warning naming actual size and cap; silent under cap and at the exact boundary (`== cap` silent, `> cap` warns) | 6/10 | `go test -v -run 'TestAuditReportLength/(over_cap_warns_once\|under_cap_silent\|exact_boundary_silent)' ./internal/phases/audit` |
| non-lossy | Verdict never flips on size; the on-disk artifact is byte-identical after `Classify` (ship SHA-binding) | 8/10 | `go test -v -run 'TestAuditReportLength/(over_cap_does_not_flip_verdict\|over_cap_does_not_mutate_artifact)' ./internal/phases/audit` |
| no-regression | The EGPS/verdict-conflict override semantics sharing `Classify` stay green | 6/10 | `go test -run TestVerdictConflict ./internal/phases/audit` |
| doc-code-sync | `agents/evolve-auditor-reference.md` documents the SAME numeric budget the gate enforces | 5/10 | `go test -tags acs -run TestC1525_001_DocCapMatchesCodeCap ./acs/cycle1525` |

# Build Explanation — Cycle 1652

## Build Binding
- Cycle: 1652
- Base SHA: 287aa81ca87ab626203db464ea455af05d8e9502

## Summary
An empty triage commitment now stops before implementation without being credited as legitimate no-work when claimable inbox work remains. Cycle health detects historical dossiers with that invalid shape, and the dossier producer now accepts one named parameter value instead of ten positional payload arguments.

## Rationale
The orchestrator already owns the empty-commitment decision, the inbox loader already distinguishes dispatchable from console-routed items, and committed dossiers already preserve the difference between an absent task set and an explicit empty task set. Reusing those seams keeps the fix deterministic and avoids a second queue parser or a new public API.

## Changed Areas
- `.evolve/evals/triage-empty-commitment-still-dispatches-spine.md` — defines the executable acceptance contract for fresh and resumed dispatch, legitimate no-work, committed work, and historical anomaly detection.
- `.evolve/evals/dossier-producer-params-struct.md` — defines the named-parameter and byte-preservation acceptance contract.
- `go/acs/cycle1652/predicates_test.go` — binds both task contracts to production-path and source-shape checks.
- `go/cmd/evolve/cmd_cycle_health_dossier_commitment_test.go` — exercises historical dossier detection through the real cycle-health command.
- `go/internal/core/empty_commitment_claimable_test.go` — proves fresh/resumed claimable-work stops and protects legitimate no-work and committed-work behavior.
- `go/internal/core/dossier_producer_params_test.go` — executes keyed partial construction and golden byte equivalence.
- `go/internal/core/testdata/dossierparams/cycle-4242.golden.json` — preserves the pre-refactor JSON bytes for a fixed dossier input.
- `go/internal/core/testdata/dossierparams/cycle-4242.golden.md` — preserves the pre-refactor Markdown bytes for the same input.
- `go/internal/core/triage_termination.go` — classifies an explicit empty commitment against the live inbox and assigns a distinct claim-failure reason when work remains.
- `go/internal/core/cyclerun_select.go` — supplies the project root to the fresh-cycle termination check.
- `go/internal/core/resume_cursor.go` — applies the same project-root-aware termination check on resume.
- `go/internal/core/resume_execution.go` — threads the request project root into the resume cursor.
- `go/internal/core/cycle_closeout.go` — converts claimable-work empty commitments to FAIL and constructs normal dossier input with keyed fields.
- `go/internal/core/dossier_producer.go` — introduces the internal named parameter value and preserves its projection into `dossier.BuildOpts`.
- `go/internal/core/cyclerun_epilogue.go` — constructs abnormal-closeout dossier input with keyed fields.
- `go/internal/core/dossier_producer_test.go` — migrates existing producer behavior tests to the named input.
- `go/internal/core/dossier_producer_failure_test.go` — preserves failure-evidence tests through the signature change.
- `go/internal/core/dossier_producer_lock_test.go` — preserves lock and concurrency tests through the signature change.
- `go/internal/core/dossier_verdict_not_adopted_test.go` — preserves skipped and non-adopted verdict projection tests through the signature change.
- `go/internal/core/empty_commitment_dispatch_test.go` — supplies the project root argument in the existing resume-cursor unit seam.
- `go/internal/cyclehealth/cyclehealth.go` — adds the dossier commitment signal using committed dossier task and phase evidence.
- `go/internal/cyclehealth/cyclehealth_test.go` — updates the signal-roster contract from twelve to thirteen and names the new signal.

## Design Decisions
Inbox read failures and malformed inbox warnings are treated as evidence that legitimate no-work is unproven, so they cannot silently earn the no-work disposition. The cycle-health signal is fatal because implementation after an explicit empty commitment is a pipeline-integrity breach. The dossier parameter type remains package-private because only the two core closeout paths use it.

## Verification
Focused tests cover both dispatch roots, three legitimate no-work shapes, the committed-work anti-no-op case, historical and healthy dossier shapes, omitted optional evidence, and exact JSON/Markdown bytes. The cycle ACS suite and repository build selfcheck provide the final regression evidence.

## Compatibility
Normal empty-inbox cycles retain `triage-empty-commitment` and the skipped outcome, committed triage work still advances, legacy dossiers without a `tasks` field remain quiet, and dossier output bytes remain unchanged for fixed input.

## Limitations
The claimability check shares the inbox loader and explicit console-route classifier but cannot import the protected-surface guard because that package depends on core; protected-surface enforcement remains at the existing composition and ship boundaries.

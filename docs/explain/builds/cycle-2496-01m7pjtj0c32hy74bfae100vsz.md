# Build Explanation — Cycle 2496

## Build Binding
- Cycle: 2496
- Base SHA: b27e13c68c27e597197d10e5254a751284b51eff

## Summary
The repository now has one redaction leaf, `internal/redact`, and an evidence bundle with `evolve issue evidence add` and `evolve issue check`. This is component I3 of the incident knowledge base plan.

## Rationale
The pane trust package held the only secret pattern list. The bundle copy needs the same list, and two lists drift. The patterns moved to a leaf that both packages import. The bundle needs a tamper check, so each manifest entry holds the digest of the copy and the digest of the raw source.

## Changed Areas
- `go/internal/redact/redact.go` — the pattern list moved here from `panetrust`, with the new entropy rule `EntropyRuns`.
- `go/internal/redact/apicover_named_test.go` — names each export of the leaf in a real assertion.
- `go/internal/panetrust/panetrust.go` — `redactSecrets` calls `redact.Secrets`, and the local pattern list is gone.
- `go/internal/incidentbundle/bundle.go` — `Add`, `Check` and `CheckAll` for the bundle, its manifest and its budget.
- `go/internal/incidentbundle/apicover_named_test.go` — names each export and covers tamper, redaction, budget and entropy.
- `go/internal/policy/incident.go` — the policy key `incident.evidence.budget_bytes` with a default of 256 KB.
- `go/internal/policy/incident_test.go` — tests the default and the set value.
- `go/internal/policy/policy.go` — the `Incident` field of `Policy`.
- `go/cmd/evolve/cmd_issue.go` — the `evolve issue` verb with `evidence add` and `check`.
- `go/cmd/evolve/cmd_issue_test.go` — drives the verb through its entry point.
- `go/cmd/evolve/registry.go` — registers the `issue` verb.
- `go/cmd/evolve/main.go` — adds the verb to the help text.
- `go/.apicover-enforce` — enrols `internal/redact` and `internal/incidentbundle` in the repo-wide gate.
- `docs/architecture/packages/internal-redact.md` — the package page of the leaf.
- `docs/architecture/packages/internal-incidentbundle.md` — the package page of the bundle.
- `docs/architecture/packages/internal-panetrust.md` — states that `panetrust` now imports the leaf.
- `docs/architecture/packages/README.md` — lists the two new pages.

## Design Decisions
The oldest repeat source becomes a pointer only when another full entry holds each of its error templates. This keeps every error template in the bundle. When no source repeats, the bundle stays over budget, because the sweep must never refuse a source. The budget is policy data, so no flag exists.

## Verification
The ACS predicates of cycle 2496 drive the real binary. The unit tests of `incidentbundle`, `redact` and `policy` and the tests of the verb cover the same four clauses.

## Compatibility
`panetrust.RedactSecrets` returns the same text as before for the same input. The new policy key is optional.

## Limitations
The size ratchet on the whole `knowledge-base/incidents/` tree, the `extract` pipeline (I12) and the check in the commit gate (I10) are later components.

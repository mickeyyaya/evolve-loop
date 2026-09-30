# Comment history: `acs/cycle806`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle806/predicates_test.go:3` — above `package cycle806`

```text
// Package cycle806 materializes the cycle-806 acceptance criteria for this
// fleet lane's sole committed defect, fix-fleet-soak-red-ci (triage top_n:
// sweep-tombstone-attribution, soak-invariants-reconcile, ciparity-integration-
// tier). Per R9.3 no predicate binds to any deferred/dropped item
// (ciparity-jobmatrix-superset-pin is out of scope).
//
// Every predicate EXECUTES the system under test as a subprocess (`go test` of
// a named behavioral unit/integration test) and requires an explicit
// `--- PASS: <name>` marker — exit 0 alone would also cover the "0 tests
// matched" case (a renamed/removed test), which must fail the predicate, not
// pass it. No source-grep predicates over logic files (cycle-85 rule).
//
// AC map (1:1, from scout-report.md Selected Tasks + Acceptance Criteria):
//
//	Task sweep-tombstone-attribution
//	  AC1.1 tombstone-aware resolver reads a reaped registry
//	        → C806_001 sessionrecord.TestReadAllResolving_ReadsTombstoneAfterReap
//	  AC1.2 (negative) missing live+tombstone → zero records, no fabrication
//	        → C806_002 sessionrecord.TestReadAllResolving_MissingBothIsZeroNoFabrication
//	  AC1.3 (edge) live+tombstone same session → no double-count (union dedup)
//	        → C806_003 sessionrecord.TestReadAllResolving_LiveAndTombstoneNoDoubleCount
//	  AC1.4 attribution survives a real ReapOrphans tombstone end-to-end
//	        → C806_004 sessionreaper.TestReapOrphans_AttributionDiscoverableAfterTombstone
//	Task soak-invariants-reconcile
//	  AC2.1 the integration-tier soak test is GREEN under the new contract
//	        (N tombstones + idempotent second reap + attribution via resolver;
//	        UNKNOWN/cross-run branch kept) → C806_005 runs the real
//	        TestFleetSoak_AllFourInvariants with -tags integration
//	Task ciparity-integration-tier
//	  AC3.1 integration-tier gate offenders FAIL audit
//	        → C806_006 audit.TestRun_IntegrationTierGate_Offenders_FAILsAudit
//	  AC3.2 (edge) default gate no-ops without a go module
//	        → C806_007 audit.TestIntegrationTierCheckDefault_NoOpWithoutGoModule
//	  AC3.3 (anti-drift) NewDefault wires a gate that truly runs -tags integration
//	        → C806_008 audit.TestNewDefault_WiresIntegrationTierGate
//
// Adversarial axes: negative (AC1.2 no-fabrication), edge (AC1.3 dedup, AC3.2
// no-op), semantic (resolver read vs end-to-end reaper attribution vs soak
// invariants vs gate FAIL vs tag membership are distinct behaviors, not one
// restated).
```

# Build Explanation — Cycle 2212

## Build Binding
- Cycle: 2212
- Base SHA: 75affaae020a3cbf63f28922a3c1a583833268df

## Summary
A failed inbox item now records the retry-ladder rung of its last try in the field `last_failure_rung`. A reader returns the value. Quarantine release clears it.

## Rationale
The D10 rule needs to know if an item failed at rung R1 or higher. The failure record held the class and the fingerprint but no rung. The change adds the rung beside the class, with the same lifecycle.

## Changed Areas
- `go/internal/retryladder/decide.go` — adds `ParseRung`, the one validator of a recorded rung.
- `go/internal/cycleoutcome/cycleoutcome.go` — reads the rung from the ladder state of the cycle workspace and passes it into the failure closeout.
- `go/internal/inboxmover/outcome.go` — carries the rung from the cycle outcome into the quarantine policy.
- `go/internal/inboxmover/lifecycle/quarantine.go` — adds the rung to the policy and clears it on release from quarantine.
- `go/internal/inboxmover/lifecycle/release.go` — passes the rung into the failure record when an item is parked or released.
- `go/internal/inboxmover/lifecycle/item.go` — stamps or clears the rung with each charged failure and adds `ReadLastFailureRung`.
- `go/internal/inboxbatch/fieldowner.go` — declares the key as a loop stamp so a curated edit cannot forge it.
- `go/internal/retryladder/apicover_named_test.go` — names and checks `ParseRung`.
- `go/internal/cycleoutcome/failure_record_test.go` — tests the rung read for every branch.
- `go/internal/inboxmover/lifecycle/failure_record_test.go` — tests stamp, clear and read of the rung.
- `go/internal/inboxmover/lifecycle/apicover_named_test.go` — names `ReadLastFailureRung`.
- `go/internal/inboxbatch/fieldowner_test.go` — adds the key to the closed lists of loop stamps.

## Design Decisions
No rung is stamped when the ladder state is absent or holds an unknown value. An absent state does not prove R0, and a false "R1 or higher" claim can raise the next item. Each charged failure replaces the older rung, as it does for the class. Infra failures never charge the item, so they never touch the rung.

## Verification
The predicates TestC2212_001 to TestC2212_011 pass. The package tests of inboxbatch, inboxmover, cycleoutcome and retryladder pass. Every changed function has 100% coverage.

## Compatibility
Existing records without the key read as an empty rung. No existing field changes.

## Limitations
The D10 consumer and the `inboxmover` facade are protected paths. The console wires the reader in a separate item. This change does not update `docs/architecture/retry-ladder.md`: line 241 still says that the item records no rung, which is now stale. A separate documentation item must correct that line.

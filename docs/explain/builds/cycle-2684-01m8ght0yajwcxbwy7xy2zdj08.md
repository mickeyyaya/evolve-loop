# Build Explanation — Cycle 2684

## Build Binding
- Cycle: 2684
- Base SHA: bea93ef80479145e99fe423da52dea1da4aefb5d

## Summary
The subprocess bound message now names the caller deadline when that deadline ended the call. A new test pins that the subprocess cap plus the largest lane sum fits inside the checked-in `acs.go_timeout_s`.

## Rationale
The message always said "the earlier of the lane end and 5m0s". That text hid the cause when the test deadline of the caller was the bound. The fix compares the context deadline with the caller deadlines that built it. This reuses existing data and adds no new setting.

## Changed Areas
- `go/pkg/acsassert/subprocess_bound.go` — `boundLine` takes a flag for the caller deadline, and `boundNote` sets it when a caller deadline equals the context deadline.
- `go/pkg/acsassert/go_tests.go` — `GoTests` passes its caller deadlines to `boundNote`.
- `go/pkg/acsassert/assertions.go` — `SubprocessOutputIn` has no caller deadline and passes false.
- `go/pkg/acsassert/subprocess_cap_fit_test.go` — pins the cap relation against the real policy and the caller-deadline message.
- `go/acs/cycle2684/predicates_test.go` — the two ACS predicates of this cycle.
- `.evolve/evals/acs-subprocess-cap-fits-the-go-timeout.md` — the eval graders of the task.

## Design Decisions
An equality check of deadlines avoids a new field on the context. A lane end that equals the caller deadline reports the caller, which is correct for both.

## Verification
`go test -count=1 ./pkg/acsassert/` passes, and `go test -tags acs -count=1 ./acs/cycle2684` passes.

## Compatibility
The marker text that the lane reads is unchanged. Only the words after it differ when a caller deadline ended the call.

## Limitations
The largest lane sum of 187 s is a constant in the test. It does not read the lane scope at run time.

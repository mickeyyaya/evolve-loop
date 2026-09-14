package loopchain

// The seven chain stop reasons — the chain summary's wire vocabulary
// (chain_stop_reason in the stdout JSON).
const (
	StopOperatorBrake         = "chain_operator_brake"
	StopInboxEmpty            = "chain_inbox_empty"
	StopMaxBatches            = "chain_max_batches"
	StopQuotaDefer            = "chain_quota_defer"
	StopBatchError            = "chain_batch_error"
	StopInboxUnreadable       = "chain_inbox_unreadable"
	StopBoundaryRefreshReexec = "chain_boundary_refresh_reexec"
)

// StartDecision decides whether batch n (0-based) may start, and names the
// reason when it may not. Pure, so the precedence between the three
// pre-batch stop conditions is testable without running a batch: the
// operator brake outranks everything (an explicit instruction), then a
// drained inbox AFTER at least one batch (the success exit), then the runaway
// cap. The `n > 0` scope on the drained-inbox exit is the min-one-batch
// guarantee (cycle 1098): a drained inbox is a CONTINUE condition, not a
// START condition, so opting into chaining is never weaker than the
// pre-chain contract; it never widens the cap (a non-positive cap still runs
// nothing — no cap+1).
func StartDecision(n, maxBatches, inboxPending int, brake bool) (reason string, stop bool) {
	switch {
	case brake:
		return StopOperatorBrake, true
	case inboxPending == 0 && n > 0:
		return StopInboxEmpty, true
	case n >= maxBatches:
		return StopMaxBatches, true
	}
	return "", false
}

// ContinueDecision maps a finished batch's exit code onto the chain's next
// move. rc 0 (clean) and rc 3 (completed with absorbed failures) both ran to
// completion — the queue is never halted for them. rc 5 is the QUOTA-PAUSE
// contract the batch derives from the all-families-exhausted sequence:
// relaunching would only burn the next batch into the same drained families,
// so the chain defers with the checkpoint intact. Every other code is a fatal
// batch outcome and propagates unchanged.
func ContinueDecision(rc int) (reason string, exit int, stop bool) {
	switch rc {
	case 0, 3:
		return "", rc, false
	case 5:
		return StopQuotaDefer, 5, true
	default:
		return StopBatchError, rc, true
	}
}

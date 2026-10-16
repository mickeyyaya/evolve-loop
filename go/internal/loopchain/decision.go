package loopchain

const (
	StopOperatorBrake         = "chain_operator_brake"
	StopInboxEmpty            = "chain_inbox_empty"
	StopMaxBatches            = "chain_max_batches"
	StopQuotaDefer            = "chain_quota_defer"
	StopBatchError            = "chain_batch_error"
	StopInboxUnreadable       = "chain_inbox_unreadable"
	StopBoundaryRefreshReexec = "chain_boundary_refresh_reexec"
)

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

const (
	exitCompletedWithAbsorbed = 3
	exitQuotaPause            = 5
)

func ContinueDecision(rc int) (reason string, exit int, stop bool) {
	switch rc {
	case 0, exitCompletedWithAbsorbed:
		return "", rc, false
	case exitQuotaPause:
		return StopQuotaDefer, exitQuotaPause, true
	default:
		return StopBatchError, rc, true
	}
}

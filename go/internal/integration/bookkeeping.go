package integration

func SkipsL2(holdsCode bool) bool { return !holdsCode }

func FlushDue(pending int, codeFlush FlushTrigger) bool {
	return pending > 0 && codeFlush != TriggerNone
}

package core

func (cr *cycleRun) endAfterDebugger() (loopAction, error) {
	if len(cr.cs.ShipFailReasons) == 0 {
		cr.cs.ShipFailReasons = []string{"the debugger ended the " + cr.cs.ShipRecoveryCode + " ship recovery without a reship"}
	}
	cr.result.FinalVerdict = VerdictFAIL
	return loopBreak, nil
}

package wavehealth

const notEvaluated = "not evaluated by this verb"

func gapStep(id, reading, action string) Step {
	return Step{ID: id, Reading: reading, Threshold: notEvaluated, Result: ResultGap, Action: action}
}

func usageStep() Step {
	return gapStep("A7", "usage is read only when a seat is blocked or benched is not empty",
		"run evolve clihealth usage --json <family> when a seat is blocked or benched is not empty")
}

func seatsStep() Step {
	return gapStep("A8-seats", "no record of console seats and suites",
		"keep one line for each started seat in the check-in note; item console-full-suite-guard-during-a-wave")
}

func watchesStep() Step {
	return gapStep("A9", "the console watch processes are not read",
		"read ps -axo pid=,etime=,command= and find the four watches of SOP step A9")
}

func inboxStep() Step {
	return gapStep("A10", "the console-routed tasks are not read",
		"run evolve inbox list --status console --json and take each id that a wake line names")
}

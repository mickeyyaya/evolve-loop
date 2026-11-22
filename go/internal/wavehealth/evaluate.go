package wavehealth

func Evaluate(s Snapshot) Report {
	loop, loopKnown := s.Loop.Value, s.Loop.Known
	steps := []Step{loopStep(s)}
	if loopKnown && !loop.Running {
		steps = append(steps, skippedSteps()...)
		return Report{Steps: steps, Outcome: OutcomeContinue}
	}
	steps = append(steps, waveStep(s), failuresStep(s), abnormalStep(s), stallsStep(s), ciStep(s), usageStep(), diskStep(s), loadStep(s), seatsStep(), watchesStep(), inboxStep())
	if loopKnown && loop.BrakeHeld {
		steps = holdBrakes(steps)
	}
	return Report{Steps: steps, Outcome: outcomeOf(steps)}
}

func outcomeOf(steps []Step) Outcome {
	outcome := OutcomeContinue
	for _, st := range steps {
		switch st.Result {
		case ResultBrake:
			return OutcomeBrake
		case ResultFixLocal:
			outcome = OutcomeFixLocal
		}
	}
	return outcome
}

const heldAction = "the brake is held: do not run the BRAKE procedure again"

func holdBrakes(steps []Step) []Step {
	out := make([]Step, len(steps))
	for i, st := range steps {
		if st.Result == ResultBrake {
			st.Result, st.Action = ResultHeld, heldAction
		}
		out[i] = st
	}
	return out
}

func notGathered(id, threshold string) Step {
	return Step{ID: id, Reading: "not gathered", Threshold: threshold, Result: ResultGap, Action: "gather the reading and check again"}
}

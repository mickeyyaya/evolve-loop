package core

const PhaseAborted Phase = "aborted"

func IsTerminalPhase(phase string) bool {
	return phase == string(PhaseEnd) || phase == string(PhaseAborted)
}

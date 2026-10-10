package attemptpostmortem

import "time"

func findSuspect(commands []Command, end time.Time, window time.Duration) *Suspect {
	windowStart := end.Add(-window)
	for i := len(commands) - 1; i >= 0; i-- {
		if reason, ok := suspectReason(commands[i], windowStart); ok {
			return &Suspect{Command: commands[i], Reason: reason}
		}
	}
	return nil
}

func suspectReason(c Command, windowStart time.Time) (SuspectReason, bool) {
	switch {
	case c.Status == StatusNoResult:
		return ReasonNoResult, true
	case c.Status == StatusSignal:
		return ReasonSignalExit, true
	case !c.StartedAt.Before(windowStart):
		return ReasonEndWindow, true
	}
	return "", false
}

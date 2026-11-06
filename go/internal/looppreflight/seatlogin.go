package looppreflight

import "github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"

const seatLoginCheck = "seat-login"

func CheckSeatLogin(inspect func() seatlogin.Health) CheckResult {
	if inspect == nil {
		return CheckResult{Name: seatLoginCheck, Level: LevelPass, Message: "seat login not measured"}
	}
	h := inspect()
	switch h.State {
	case seatlogin.StateBad:
		return CheckResult{Name: seatLoginCheck, Level: LevelHalt, Message: "the seat token is bad, so each claude seat would refuse", Detail: h.Message}
	case seatlogin.StateOrphan, seatlogin.StateFault:
		return CheckResult{Name: seatLoginCheck, Level: LevelWarn, Message: "the seat directory and the holder disagree", Detail: h.Message}
	}
	return CheckResult{Name: seatLoginCheck, Level: LevelPass, Message: h.Message}
}

func checkSeatLogin(o resolved) CheckResult { return CheckSeatLogin(o.seatInspect) }

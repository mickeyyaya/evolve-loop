package looppreflight

import "github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"

const seatLoginCheck = "seat-login"

var seatLoginLevel = map[seatlogin.Severity]CheckLevel{
	seatlogin.SeverityOK:   LevelPass,
	seatlogin.SeverityWarn: LevelWarn,
	seatlogin.SeverityHalt: LevelHalt,
}

var seatLoginHeadline = map[seatlogin.State]string{
	seatlogin.StateBad:      "the seat token is bad, so each claude seat would refuse",
	seatlogin.StateOrphan:   "the seat directory and the holder disagree",
	seatlogin.StateFault:    "the seat directory and the holder disagree",
	seatlogin.StateExpiring: "the seat token is near its renewal date",
}

func CheckSeatLogin(inspect func() seatlogin.Health) CheckResult {
	if inspect == nil {
		return CheckResult{Name: seatLoginCheck, Level: LevelPass, Message: "seat login not measured"}
	}
	h := inspect()
	severity := h.State.Severity()
	if severity == seatlogin.SeverityUnknown {
		return CheckResult{Name: seatLoginCheck, Level: LevelHalt, Message: "unknown seat state, so the check cannot judge the holder", Detail: h.Message}
	}
	level := seatLoginLevel[severity]
	if headline, ok := seatLoginHeadline[h.State]; ok {
		return CheckResult{Name: seatLoginCheck, Level: level, Message: headline, Detail: h.Message}
	}
	return CheckResult{Name: seatLoginCheck, Level: level, Message: h.Message}
}

func checkSeatLogin(o resolved) CheckResult { return CheckSeatLogin(o.seatInspect) }

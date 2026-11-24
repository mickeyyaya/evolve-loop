package seatlogin

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type State int

const (
	StateNone State = iota
	StateIsolated
	StateBad
	StateOrphan
	StateFault
	StateExpiring
)

type Health struct {
	State   State
	Message string
}

func Inspect(home string, deps Deps) Health {
	if home == "" {
		return Health{State: StateFault, Message: "the home directory is empty, so the seat token cannot be found."}
	}
	root := paths.SeatRoot(home)
	info, err := deps.Stat(root)
	rootPresent := err == nil
	switch {
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return Health{State: StateFault, Message: fmt.Sprintf("the seat directory %s cannot be read: %v.", root, err)}
	case rootPresent && !info.IsDir():
		return Health{State: StateFault, Message: fmt.Sprintf("the seat directory %s is not a directory, so no claude seat can use it. Next: remove it.", root)}
	}
	d, mtime := resolveAt(home, "", deps)
	switch {
	case d.Refusal != "":
		return Health{State: StateBad, Message: d.Refusal}
	case d.HostIsolated:
		return isolatedHealth(home, holderAgeDays(mtime, deps))
	case rootPresent && armed(home, deps):
		return Health{State: StateOrphan, Message: fmt.Sprintf("the seat directory %s exists but the holder is absent, so each claude seat uses the stored login. A parameter seat can leave this state. Next: run evolve auth seat-token clear.", root)}
	}
	return Health{State: StateNone, Message: "no seat token is set, so each claude seat uses the stored login. Next: run claude setup-token, then evolve auth seat-token set."}
}

func armed(home string, deps Deps) bool {
	_, err := deps.Stat(paths.SeatSettingsPath(home))
	return err == nil
}

const hoursPerDay = 24

func holderAgeDays(mtime time.Time, deps Deps) int {
	now := time.Now
	if deps.Now != nil {
		now = deps.Now
	}
	age := now().Sub(mtime)
	if age < 0 {
		return 0
	}
	return int(age / (hoursPerDay * time.Hour))
}

func isolatedHealth(home string, ageDays int) Health {
	path := paths.SeatTokenPath(home)
	if ageDays >= TokenWarnAgeDays {
		return Health{State: StateExpiring, Message: fmt.Sprintf("the seat token at %s is %d days old and the token lasts one year. Next: run claude setup-token, then evolve auth seat-token set.", path, ageDays)}
	}
	return Health{State: StateIsolated, Message: fmt.Sprintf("the claude seats are isolated, with the token at %s (%d days old).", path, ageDays)}
}

type Severity int

const (
	SeverityUnknown Severity = iota
	SeverityOK
	SeverityWarn
	SeverityHalt
)

var stateSeverity = map[State]Severity{
	StateNone:     SeverityOK,
	StateIsolated: SeverityOK,
	StateBad:      SeverityHalt,
	StateOrphan:   SeverityWarn,
	StateFault:    SeverityWarn,
	StateExpiring: SeverityWarn,
}

func (s State) Severity() Severity { return stateSeverity[s] }

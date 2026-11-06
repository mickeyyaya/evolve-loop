package seatlogin

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type State int

const (
	StateNone State = iota
	StateIsolated
	StateBad
	StateOrphan
	StateFault
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
	d := Resolve(home, "", deps)
	switch {
	case d.Refusal != "":
		return Health{State: StateBad, Message: d.Refusal}
	case d.HostIsolated:
		return Health{State: StateIsolated, Message: fmt.Sprintf("the claude seats are isolated, with the token at %s.", paths.SeatTokenPath(home))}
	case rootPresent && armed(home, deps):
		return Health{State: StateOrphan, Message: fmt.Sprintf("the seat directory %s exists but the holder is absent, so each claude seat uses the stored login. A parameter seat can leave this state. Next: run evolve auth seat-token clear.", root)}
	}
	return Health{State: StateNone, Message: "no seat token is set, so each claude seat uses the stored login."}
}

func armed(home string, deps Deps) bool {
	_, err := deps.Stat(paths.SeatSettingsPath(home))
	return err == nil
}

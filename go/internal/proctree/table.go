package proctree

import "time"

type Process struct {
	Pid     int
	Ppid    int
	Pgid    int
	Started time.Time
	Comm    string
	Args    []string
	Env     map[string]string
}

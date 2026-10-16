package swarm

import "github.com/mickeyyaya/evolve-loop/go/internal/runlease"

func ExecPidAlive(pid int) bool { return runlease.PIDAlive(pid) }

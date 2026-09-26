package swarm

import "github.com/mickeyyaya/evolve-loop/go/internal/runlease"

// ExecPidAlive is the production PidLiveness, delegating to runlease.PIDAlive.
func ExecPidAlive(pid int) bool { return runlease.PIDAlive(pid) }

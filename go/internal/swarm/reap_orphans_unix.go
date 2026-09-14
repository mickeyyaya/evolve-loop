// reap_orphans_unix.go — the reaper's pid-liveness probe. The signal-0 probe
// itself lives in runlease (PIDAlive, with its own build-tagged files); this
// file keeps the reaper's name and injected-probe seam intact.
package swarm

import "github.com/mickeyyaya/evolve-loop/go/internal/runlease"

// ExecPidAlive delegates to runlease.PIDAlive — the ONE owner-liveness probe
// (nil/EPERM ⇒ alive, ESRCH ⇒ dead, pid ≤ 0 never signalled).
func ExecPidAlive(pid int) bool { return runlease.PIDAlive(pid) }

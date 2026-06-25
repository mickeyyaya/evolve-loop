// reap_orphans_unix.go — pid-liveness probe via signal 0 (unix). The repo
// targets macOS/Linux only (bash 3.2 / CLAUDE.md; same constraint as
// swarmrunner/kill_unix.go); no Windows build, so no portable fallback is
// needed and the _unix filename keeps the syscall out of any non-unix build.
package swarm

import "syscall"

// ExecPidAlive reports whether a PID is alive without affecting it. Signal 0
// performs error checking only: ESRCH ⇒ no such process (dead); nil ⇒ alive and
// signalable; EPERM ⇒ alive but owned by another user (still alive, so its
// session must NOT be reaped). PID ≤ 0 is treated as dead — never signal 0
// (whole group), -1 (everything), or negatives (process groups) here.
func ExecPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

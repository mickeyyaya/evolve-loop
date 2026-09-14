//go:build unix

package runlease

import "syscall"

// PIDAlive reports whether pid is a live process without affecting it —
// signal 0 performs error checking only: nil ⇒ alive and signalable, EPERM ⇒
// alive but owned by another user (still alive), ESRCH ⇒ gone. PID ≤ 0 is
// never probed (0 = the caller's group, -1 = everything, negatives = groups)
// and reports dead. This is the ONE owner-liveness probe the lease readers
// hand OwnerLive; the swarm reaper delegates here.
func PIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

//go:build !unix

package runlease

// PIDAlive cannot probe a process on this platform; it reports alive so a
// fresh lease keeps its conservative meaning (OwnerLive falls back to the
// heartbeat alone).
func PIDAlive(pid int) bool { return pid > 0 }

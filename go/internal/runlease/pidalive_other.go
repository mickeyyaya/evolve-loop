//go:build !unix

package runlease

func PIDAlive(pid int) bool { return pid > 0 }

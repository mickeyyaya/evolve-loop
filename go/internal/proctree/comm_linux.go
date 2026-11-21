//go:build linux

package proctree

func commFrom(p Process) string {
	return p.Comm
}

func readComm(pid int) (string, error) {
	return commOfAt("/proc", pid)
}

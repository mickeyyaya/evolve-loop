package proctree

func CommOf(pid int) (string, error) {
	return readComm(pid)
}

package proctree

func StartOf(pid int) (string, error) {
	return readProcStart(pid)
}

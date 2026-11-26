//go:build !darwin && !linux

package proctree

func commFrom(p Process) string {
	return p.Comm
}

func readComm(int) (string, error) {
	return "", errTableUnsupported()
}

//go:build linux

package proctree

import (
	"os"
	"strconv"
)

func readProcArgs(pid int) ([]string, map[string]string, error) {
	dir := "/proc/" + strconv.Itoa(pid)
	cmdline, err := os.ReadFile(dir + "/cmdline")
	if err != nil {
		return nil, nil, err
	}
	environ, err := os.ReadFile(dir + "/environ")
	if err != nil {
		return nil, nil, err
	}
	args, env := parseProcFiles(cmdline, environ)
	return args, env, nil
}

//go:build linux

package proctree

import (
	"os"
	"strconv"
)

func readProcArgs(pid int) ([]string, map[string]string, error) {
	return readProcArgsAt("/proc", pid)
}

func readProcArgsAt(root string, pid int) ([]string, map[string]string, error) {
	dir := root + "/" + strconv.Itoa(pid)
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

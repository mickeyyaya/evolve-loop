//go:build !unix

package main

import (
	"fmt"
	"runtime"
	"syscall"
)

func detachSysProcAttr() (*syscall.SysProcAttr, error) {
	return nil, fmt.Errorf("--detach is unsupported on %s", runtime.GOOS)
}

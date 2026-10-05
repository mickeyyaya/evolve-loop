//go:build unix

package main

import "syscall"

func detachSysProcAttr() (*syscall.SysProcAttr, error) {
	return &syscall.SysProcAttr{Setsid: true}, nil
}

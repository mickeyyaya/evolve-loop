package main

import "syscall"

func detachSessionID(pid int) (int, error) {
	return syscall.Getsid(pid)
}

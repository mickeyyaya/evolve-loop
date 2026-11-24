//go:build !darwin && !linux

package term

import "io"

func IsFD(int) bool { return false }

func IsTerminal(io.Reader) bool { return false }

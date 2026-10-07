//go:build !unix

package ship

import "os/exec"

func cancelKillsTheProcessGroup(cmd *exec.Cmd) {}

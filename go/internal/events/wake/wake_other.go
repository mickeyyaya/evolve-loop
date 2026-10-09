//go:build !darwin && !linux

package wake

import (
	"os"
	"runtime"
)

func newKernel(*os.File) (kernel, error) {
	return nil, refuseSystem(runtime.GOOS)
}

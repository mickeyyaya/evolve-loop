package main

import (
	"os"
	"testing"
)

// TestMain_DelegatesExitCode verifies main() forwards run()'s exit code via the
// osExit seam. With no --out the command fails fast (no network), so main() must
// exit 1.
func TestMain_DelegatesExitCode(t *testing.T) {
	prevExit, prevArgs := osExit, os.Args
	t.Cleanup(func() { osExit = prevExit; os.Args = prevArgs })
	var got int
	osExit = func(code int) { got = code }
	os.Args = []string{"genimage"} // missing --out → run returns 1

	main()

	if got != 1 {
		t.Errorf("main() exit code = %d, want 1", got)
	}
}

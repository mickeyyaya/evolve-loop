package audit

import (
	"fmt"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

func TestMain(m *testing.M) {
	if err := gotooltest.LimitProcess(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
	}
	runCmd = sharedGeneratorBuilds.runner(runCmd)
	code := m.Run()
	if err := sharedGeneratorBuilds.remove(); err != nil {
		fmt.Fprintln(os.Stderr, "audit tests: remove generator builds:", err)
	}
	if err := gittest.RemoveTemplates(); err != nil {
		fmt.Fprintln(os.Stderr, "audit tests: remove git templates:", err)
	}
	os.Exit(code)
}

package core

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

var seatWorkspace string

func TestMain(m *testing.M) {
	backoffSleep = func(time.Duration) {}
	if err := gotooltest.LimitProcess(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
	}
	ws, err := os.MkdirTemp("", "core-seat-ws-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
	}
	seatWorkspace = ws
	code := m.Run()
	if err := os.RemoveAll(ws); err != nil {
		fmt.Fprintln(os.Stderr, "core tests: remove seat workspace:", err)
	}
	if err := gittest.RemoveTemplates(); err != nil {
		fmt.Fprintln(os.Stderr, "core tests: remove git templates:", err)
	}
	os.Exit(code)
}

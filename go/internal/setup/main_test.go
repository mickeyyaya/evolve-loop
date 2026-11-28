package setup

import (
	"fmt"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestMain(m *testing.M) {
	restore, err := gittest.IsolateHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate HOME: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := restore(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: restore HOME: %v\n", err)
	}
	os.Exit(code)
}

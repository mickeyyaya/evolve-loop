package bridge_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestMain(m *testing.M) {
	codexHome, err := os.MkdirTemp("", "evcodexhome")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate CODEX_HOME: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("CODEX_HOME", codexHome); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: set CODEX_HOME: %v\n", err)
		os.Exit(1)
	}
	code := tmuxtest.Main(m)
	if err := os.RemoveAll(codexHome); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", codexHome, err)
	}
	os.Exit(code)
}

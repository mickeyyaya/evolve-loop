package bridge_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestMain(m *testing.M) {
	codexHome, err := os.MkdirTemp("", "evcodexhome")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate CODEX_HOME: %v\n", err)
		os.Exit(1)
	}
	if err := seedLiveCodexAuthFromOperator(codexHome); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: seed the live codex auth: %v\n", err)
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

func operatorCodexHome() (string, error) {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".codex"), err
}

func seedLiveCodexAuthFromOperator(scratch string) error {
	if os.Getenv("EVOLVE_BRIDGE_LIVE_CLI") != "1" {
		return nil
	}
	operator, err := operatorCodexHome()
	if err != nil {
		return err
	}
	return seedLiveCodexAuth(scratch, operator)
}

func seedLiveCodexAuth(scratch, operator string) error {
	if os.Getenv("EVOLVE_BRIDGE_LIVE_CLI") != "1" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(operator, "auth.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(scratch, "auth.json"), raw, 0o600)
}

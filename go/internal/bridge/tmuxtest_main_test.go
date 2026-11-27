package bridge_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestMain(m *testing.M) {
	var liveOperatorCodex string
	if os.Getenv("EVOLVE_BRIDGE_LIVE_CLI") == "1" {
		var err error
		liveOperatorCodex, err = operatorCodexHome()
		if err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: resolve operator CODEX_HOME: %v\n", err)
			os.Exit(1)
		}
	}
	restoreHome, err := gittest.IsolateHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate HOME: %v\n", err)
		os.Exit(1)
	}
	if liveOperatorCodex != "" {
		if err := seedLiveCodexAuth(os.Getenv("CODEX_HOME"), liveOperatorCodex); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: seed the live codex auth: %v\n", err)
			_ = restoreHome()
			os.Exit(1)
		}
	}
	cellBinRoot, err := os.MkdirTemp("", "evcellbin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate the cell-bin root: %v\n", err)
		_ = restoreHome()
		os.Exit(1)
	}
	bridge.SetCellBinRootForTest(cellBinRoot)
	code := tmuxtest.Main(m)
	bridge.RunTestMainCleanups()
	if err := os.RemoveAll(cellBinRoot); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", cellBinRoot, err)
	}
	if err := restoreHome(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: restore HOME: %v\n", err)
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

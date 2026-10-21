package bridge_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedLiveCodexAuth_OnlyTheLiveTierCopiesTheOperatorAuth(t *testing.T) {
	operator, scratch := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(operator, "auth.json"), []byte(`{"token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_BRIDGE_LIVE_CLI", "")
	if err := seedLiveCodexAuth(scratch, operator); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scratch, "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a default run must not copy the operator auth (stat err %v)", err)
	}
	t.Setenv("EVOLVE_BRIDGE_LIVE_CLI", "1")
	if err := seedLiveCodexAuth(scratch, operator); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(scratch, "auth.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the live tier must get the operator auth at mode 0600 (%v, %v)", info, err)
	}
	if copied, _ := os.ReadFile(filepath.Join(scratch, "auth.json")); string(copied) != `{"token":"x"}` {
		t.Errorf("the copy = %q, want the operator's bytes", copied)
	}
	unreadable := t.TempDir()
	if err := os.Mkdir(filepath.Join(unreadable, "auth.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := seedLiveCodexAuth(t.TempDir(), unreadable); err == nil {
		t.Error("an operator auth that cannot be read must fail loudly")
	}
	if err := seedLiveCodexAuth(scratch, t.TempDir()); err != nil {
		t.Errorf("an operator with no auth file is no error: %v", err)
	}
	if err := seedLiveCodexAuth(filepath.Join(scratch, "missing"), operator); err == nil {
		t.Error("a scratch home that cannot take the file must fail loudly")
	}
}

func TestSeedLiveCodexAuthFromOperator_ReadsTheOperatorHomeOnlyForTheLiveTier(t *testing.T) {
	operator, scratch := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(operator, "auth.json"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", operator)
	t.Setenv("HOME", "")
	t.Setenv("EVOLVE_BRIDGE_LIVE_CLI", "")
	if err := seedLiveCodexAuthFromOperator(scratch); err != nil {
		t.Fatalf("a default run must not resolve the operator home: %v", err)
	}
	t.Setenv("EVOLVE_BRIDGE_LIVE_CLI", "1")
	if err := seedLiveCodexAuthFromOperator(scratch); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scratch, "auth.json")); err != nil {
		t.Errorf("the live tier must copy from CODEX_HOME: %v", err)
	}
	t.Setenv("CODEX_HOME", "")
	if err := seedLiveCodexAuthFromOperator(t.TempDir()); err == nil {
		t.Error("a live run with no home dir must fail loudly")
	}
}

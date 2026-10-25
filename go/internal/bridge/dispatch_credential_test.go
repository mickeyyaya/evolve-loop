package bridge

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
)

func init() {
	dir := filepath.Join(os.TempDir(), "evolve-bridge-test-dispatch")
	defaultDispatchStore = func() (dispatchcred.Store, error) { return dispatchcred.Store{Dir: dir}, nil }
}

func testDispatchStore(t *testing.T) (dispatchcred.Store, func() (dispatchcred.Store, error)) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := dispatchcred.Store{Dir: filepath.Join(dir, "dispatch")}
	return s, func() (dispatchcred.Store, error) { return s, nil }
}

func TestSandboxWrap_TheCellCannotReadTheDispatchCredentialOnMacOS(t *testing.T) {
	t.Parallel()
	s, open := testDispatchStore(t)
	sbpl := wrapForExecDeny(t, Deps{ExecStage: "enforce", DispatchStore: open}, "darwin", "claude-tmux")
	if !strings.Contains(sbpl, `(deny file-read* (subpath "`+s.HeldDir()+`"))`) || !strings.Contains(sbpl, `(deny file-write* (subpath "`+s.Dir+`"))`) {
		t.Fatalf("the profile must deny the read of %s and each write in %s:\n%s", s.HeldDir(), s.Dir, sbpl)
	}
	if got := s.Stage(); got != "enforce" {
		t.Fatalf("the launch must record the stage for the verbs of the cell, got %q", got)
	}
	if err := s.Hold(); err != nil {
		t.Fatalf("the launcher must leave a credential that the host holds: %v", err)
	}
}

func TestSandboxWrap_TheCellCannotReadTheDispatchCredentialOnLinux(t *testing.T) {
	t.Parallel()
	s, open := testDispatchStore(t)
	argv := wrapForExecDeny(t, Deps{ExecStage: "shadow", DispatchStore: open}, "linux", "claude-tmux")
	if !strings.Contains(argv, "--tmpfs "+s.HeldDir()+" --chmod 000 "+s.HeldDir()+" --remount-ro "+s.HeldDir()) || !strings.Contains(argv, "--ro-bind "+s.Dir+" "+s.Dir) {
		t.Fatalf("bwrap must mask the held credential directory %s at shadow too:\n%s", s.HeldDir(), argv)
	}
	if got := s.Stage(); got != "shadow" {
		t.Fatalf("stage = %q, want shadow", got)
	}
}

func TestSandboxWrap_OffGivesNoCredentialDenyAndWritesNoStore(t *testing.T) {
	t.Parallel()
	s, open := testDispatchStore(t)
	sbpl := wrapForExecDeny(t, Deps{ExecStage: "off", DispatchStore: open}, "darwin", "claude-tmux")
	if strings.Contains(sbpl, s.HeldDir()) {
		t.Fatalf("off must not deny the credential:\n%s", sbpl)
	}
	if _, err := os.Stat(s.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("off must not write the store: %v", err)
	}
}

func TestSandboxWrap_AStoreFailureWarnsAndKeepsTheWrap(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	open := func() (dispatchcred.Store, error) { return dispatchcred.Store{}, errors.New("no passwd") }
	sbpl := wrapForExecDeny(t, Deps{ExecStage: "enforce", DispatchStore: open, Stderr: &stderr}, "darwin", "claude-tmux")
	if !strings.Contains(stderr.String(), "WARN dispatch credential: no passwd") || strings.Contains(sbpl, "dispatch") {
		t.Fatalf("a store failure must warn and add no deny: %q\n%s", stderr.String(), sbpl)
	}
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(tmp, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	quiet := Deps{ExecStage: "enforce", DispatchStore: func() (dispatchcred.Store, error) { return dispatchcred.Store{Dir: blocker}, nil }}
	read, write := quiet.dispatchCredentialDeny("enforce")
	if len(read) != 1 || read[0] != filepath.Join(blocker, "held") || len(write) != 1 || write[0] != blocker {
		t.Fatalf("an Ensure failure must still deny the store, so the cell cannot make its own credential: %v %v", read, write)
	}
}

func TestDispatchCredentialDeny_TheDefaultStoreIsTheProcessStore(t *testing.T) {
	t.Parallel()
	held, _ := Deps{}.dispatchCredentialDeny("shadow")
	s, _ := defaultDispatchStore()
	want, err := filepath.EvalSymlinks(s.HeldDir())
	if err != nil || len(held) != 1 || held[0] != want {
		t.Fatalf("a Deps with no store uses the default store: %v, want [%s] (%v)", held, want, err)
	}
}

package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
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
	if !strings.Contains(sbpl, `(deny file-read* (subpath "`+s.HeldDir()+`"))`) || !strings.Contains(sbpl, `(deny file-write* (subpath "`+filepath.Dir(s.Dir)+`"))`) {
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
	if !strings.Contains(argv, "--tmpfs "+s.HeldDir()+" --chmod 000 "+s.HeldDir()+" --remount-ro "+s.HeldDir()) || !strings.Contains(argv, "--ro-bind "+filepath.Dir(s.Dir)+" "+filepath.Dir(s.Dir)) {
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
	got := quiet.dispatchCredentialDeny("enforce")
	if len(got.read) != 1 || got.read[0] != filepath.Join(blocker, "held") || len(got.write) != 1 || got.write[0] != tmp {
		t.Fatalf("an Ensure failure must still deny the store, so the cell cannot make its own credential: %+v", got)
	}
}

func TestDispatchCredentialDeny_TheDefaultStoreIsTheProcessStore(t *testing.T) {
	t.Parallel()
	held := Deps{}.dispatchCredentialDeny("shadow").read
	s, _ := defaultDispatchStore()
	want, err := filepath.EvalSymlinks(s.HeldDir())
	if err != nil || len(held) != 1 || held[0] != want {
		t.Fatalf("a Deps with no store uses the default store: %v, want [%s] (%v)", held, want, err)
	}
}

func unheldDispatchStore(t *testing.T, stage string) func() (dispatchcred.Store, error) {
	t.Helper()
	s, open := testDispatchStore(t)
	if err := s.Ensure(stage); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(s.HeldDir(), "credential"), 0o644); err != nil {
		t.Fatal(err)
	}
	return open
}

type countingLaunchDriver struct{ launches *int }

func (countingLaunchDriver) Name() string { return "counting" }

func (d countingLaunchDriver) Launch(context.Context, *Config, Deps) (int, error) {
	*d.launches++
	return 0, nil
}

func eachLaunchEntry(deps Deps, launches *int) map[string]func() error {
	driver := countingLaunchDriver{launches: launches}
	return map[string]func() error{
		"LaunchLLM": func() error {
			deps := deps
			deps.Runner = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
				*launches++
				return 0, nil
			}
			_, err := LaunchLLM(context.Background(), deps, LLMExec{Purpose: PurposeProbe, Binary: "claude", Args: []string{"--version"}})
			return err
		},
		"launchPane": func() error {
			_, err := launchPane(context.Background(), PurposeSmoke, driver, &Config{CLI: "claude-tmux"}, deps)
			return err
		},
		"launchSwept": func() error {
			deps := deps
			deps.ListProcesses = func(context.Context) ([]proctree.Process, error) { return nil, nil }
			deps.ListCwds = func(context.Context) ([]proctree.Cwd, error) { return nil, nil }
			deps.SignalProcess = func(pid int, _ syscall.Signal) error {
				return fmt.Errorf("the test signals no process, got pid %d", pid)
			}
			_, err := launchSwept(context.Background(), driver, &Config{CLI: "claude-tmux", Agent: "scout"}, deps)
			return err
		},
		"newRecipeDriver": func() error {
			_, _, err := newRecipeDriver(&Config{Workspace: os.TempDir(), Worktree: os.TempDir(), Agent: "recipe"}, deps, "claude-tmux")
			if err == nil {
				*launches++
			}
			return err
		},
	}
}

func TestLaunchAPI_EachEntryRefusesALaunchWithNoDispatchCredentialAtEnforce(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	deps := Deps{DispatchStore: unheldDispatchStore(t, "enforce"), Signals: signals, Stderr: io.Discard, Env: map[string]string{}, LookupEnv: mapLookup(nil), Now: time.Now}
	var launches int
	for name, launch := range eachLaunchEntry(deps, &launches) {
		if err := launch(); !errors.Is(err, ErrNoDispatchCredential) {
			t.Errorf("%s with no credential at enforce = %v, want ErrNoDispatchCredential", name, err)
		}
	}
	if launches != 0 {
		t.Fatalf("a refused entry must start nothing, got %d launches", launches)
	}
	got := 0
	for _, e := range signals.Recent() {
		if e.Code == "ISOLATION_I3_VIOLATION" && e.Fields["rule"] == "launch.credential" && e.Fields["action"] == "refused" {
			got++
		}
	}
	if got != 4 {
		t.Fatalf("each refusal must emit ISOLATION_I3_VIOLATION rule=launch.credential, got %d", got)
	}
}

func TestLaunchAPI_ShadowReportsAndLaunchesAndAHeldCredentialIsSilent(t *testing.T) {
	t.Parallel()
	for stage, wantSignals := range map[string]int{"shadow": 4, "held": 0} {
		open := unheldDispatchStore(t, "shadow")
		if stage == "held" {
			_, open = testDispatchStore(t)
		}
		signals := signalcenter.New()
		deps := Deps{DispatchStore: open, Signals: signals, Stderr: io.Discard, Env: map[string]string{}, LookupEnv: mapLookup(nil), Now: time.Now}
		var launches int
		for name, launch := range eachLaunchEntry(deps, &launches) {
			if err := launch(); errors.Is(err, ErrNoDispatchCredential) {
				t.Errorf("%s/%s must not refuse: %v", stage, name, err)
			}
		}
		credential, logged := 0, 0
		for _, e := range signals.Recent() {
			if e.Fields["rule"] == "launch.credential" {
				credential++
				if e.Fields["action"] == "logged" {
					logged++
				}
			}
		}
		if launches != 4 || (stage == "held" && credential != 0) || logged != credential || logged < wantSignals {
			t.Errorf("%s: launches = %d (want 4), credential signals = %d, logged = %d (want at least %d logged, none when held)", stage, launches, credential, logged, wantSignals)
		}
	}
}

func TestSandboxWrap_TheCellCannotMoveAnAncestorOfTheDispatchStore(t *testing.T) {
	t.Parallel()
	s, open := testDispatchStore(t)
	sbpl := wrapForExecDeny(t, Deps{ExecStage: "enforce", DispatchStore: open}, "darwin", "claude-tmux")
	for dir := filepath.Dir(filepath.Dir(s.Dir)); dir != "/"; dir = filepath.Dir(dir) {
		if !strings.Contains(sbpl, `(deny file-write* (literal "`+dir+`"))`) {
			t.Errorf("the profile must deny a rename of the ancestor %s, so a cell cannot swap in a store of its own:\n%s", dir, sbpl)
		}
	}
	if strings.Contains(sbpl, `(deny file-write* (literal "/"))`) {
		t.Fatal("the root needs no literal deny")
	}
}

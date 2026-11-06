package bridge

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func isolatedSeatProfile(t *testing.T) (string, string) {
	t.Helper()
	home, _ := filepath.EvalSymlinks(seatHome(t))
	t.Setenv("HOME", home)
	root := paths.SeatRoot(home)
	req := SandboxWrapRequest{Phase: "itest", CLI: "claude-tmux", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(), HostIsolated: true, SeatRoot: root}
	prefix, ok := defaultSandboxWrapWithProbe(Deps{SandboxMode: "on"}, fakeProbe("darwin", true))(req)
	if !ok {
		t.Fatal("no wrap")
	}
	body, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	return home, string(body)
}

func TestSandboxWrap_TheHolderIsNotWritableFromAnIsolatedSeat(t *testing.T) {
	home, sbpl := isolatedSeatProfile(t)
	root := paths.SeatRoot(home)
	deny := `(deny file-write* (subpath "` + paths.SeatHolderDir(home) + `"))`
	at, grant := strings.LastIndex(sbpl, deny), strings.Index(sbpl, `(allow file-write* (subpath "`+root+`"))`)
	if grant < 0 || at < grant {
		t.Fatalf("no write deny on the holder after the seat root grant: deny at %d, grant at %d", at, grant)
	}
}

func TestSandboxWrap_TheSeatRootCannotBeRenamedFromAnIsolatedSeat(t *testing.T) {
	home, sbpl := isolatedSeatProfile(t)
	deny := `(deny file-write* (literal "` + paths.SeatRoot(home) + `"))`
	if !strings.Contains(sbpl, deny) {
		t.Fatalf("no literal write deny on the seat root %q", deny)
	}
}

func TestSandboxWrap_ANoTokenHostThatWrappedOnceInspectsAsNone(t *testing.T) {
	home := seatHome(t)
	req := SandboxWrapRequest{Phase: "itest", CLI: "codex-tmux", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir()}
	if _, ok := defaultSandboxWrapWithProbe(Deps{SandboxMode: "on"}, fakeProbe("darwin", true))(req); !ok {
		t.Fatal("no wrap")
	}
	if got := seatlogin.Inspect(home, seatlogin.OS()); got.State != seatlogin.StateNone {
		t.Fatalf("a host with no token reads %v: %s", got.State, got.Message)
	}
}

func TestApplySeatGrants_GrantsTheRootAndDeniesTheHolderAndTheRootLiteral(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	holders := []string{filepath.Join(root, "holder")}
	got := applySeatGrants(sandbox.Config{WritePaths: []string{"/w"}}, SandboxWrapRequest{SeatRoot: root}, holders)
	if !slices.Contains(got.WritePaths, root) || !slices.Contains(got.WritePaths, "/w") {
		t.Errorf("WritePaths=%v", got.WritePaths)
	}
	if !slices.Equal(got.DenyWritePaths, holders) || !slices.Contains(got.DenyLiterals, root) {
		t.Errorf("DenyWritePaths=%v DenyLiterals=%v", got.DenyWritePaths, got.DenyLiterals)
	}
}

func TestApplySeatGrants_NoSeatRootChangesNothing(t *testing.T) {
	t.Parallel()
	in := sandbox.Config{WritePaths: []string{"/w"}}
	if got := applySeatGrants(in, SandboxWrapRequest{}, []string{"/h"}); !reflect.DeepEqual(got, in) {
		t.Fatalf("got %+v", got)
	}
}

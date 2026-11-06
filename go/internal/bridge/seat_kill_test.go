package bridge

import (
	"io/fs"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func TestResolveSeat_ABadHolderRefusesAWorktreeRecipe(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, " ", 0o600)
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Worktree: t.TempDir()}
	if err := (Deps{}).recipeSeatRefusal(cfg); err == nil {
		t.Fatal("a bad holder must refuse a worktree recipe")
	}
}

func TestResolveSeat_AnUnknownCLIDecidesNothingAndStarts(t *testing.T) {
	seatHome(t)
	d := &seatStubDriver{}
	if rc, err := d.deps().resolveSeat(&Config{CLI: "no-such-cli"}); rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
}

func TestSeatLoginDeps_AHalfSeamIsNotUsed(t *testing.T) {
	t.Parallel()
	half := seatlogin.Deps{Stat: func(string) (fs.FileInfo, error) { return nil, os.ErrInvalid }}
	if got := (Deps{SeatLogin: half}).seatLoginDeps(); got.ReadFile == nil {
		t.Fatal("a half seam reached Resolve")
	}
	half = seatlogin.Deps{ReadFile: os.ReadFile}
	if got := (Deps{SeatLogin: half}).seatLoginDeps(); got.Stat == nil {
		t.Fatal("a half seam reached Resolve")
	}
}

func TestStoredLoginDeny_NoClaudeDirectoryNoDeny(t *testing.T) {
	home := seatHome(t)
	if got := storedLoginDeny(home); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestResolveSeat_ASeatTokenFileOnANonIsolatedDoorRefuses(t *testing.T) {
	home := seatHome(t)
	tok := home + "/tok"
	if err := os.WriteFile(tok, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, cli := range []string{"codex-tmux", "claude-cloud", "no-such-cli"} {
		d := &seatStubDriver{}
		rc, err := d.deps().resolveSeat(&Config{CLI: cli, SeatTokenFile: "/nonexistent/tok"})
		if rc != ExitSeatTokenRefused || err == nil || err.Error() != seatTokenFileNotIsolated {
			t.Fatalf("%s: rc=%d err=%v", cli, rc, err)
		}
	}
}

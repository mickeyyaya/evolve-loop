package bridge

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func seatHolderLaunch(t *testing.T, cli, readStage string) string {
	t.Helper()
	deps := Deps{SandboxMode: config.SandboxModeOn, ReadStage: readStage}
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(SandboxWrapRequest{
		CLI: cli, Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if !ok {
		t.Fatal("the wrap was refused")
	}
	body, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func holderDenyLine(t *testing.T, home string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return `(deny file-read* (subpath "` + paths.SeatHolderDir(real) + `"))`
}

func TestSeatHolderDeny_EveryFamilyAtEveryStateIsDeniedTheHolder(t *testing.T) {
	for _, cli := range []string{"claude", "agy", "codex"} {
		for _, stage := range []string{"off", "shadow", "enforce"} {
			t.Run(cli+"/"+stage, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				profile := seatHolderLaunch(t, cli, stage)
				if got := strings.Count(profile, holderDenyLine(t, home)); got != 1 {
					t.Fatalf("the profile holds the holder deny %d times, want exactly once", got)
				}
			})
		}
	}
}

func TestSeatHolderDeny_ALaunchCreatesTheHolderDirectoryWithMode0700(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seatHolderLaunch(t, "claude", "off")
	info, err := os.Stat(paths.SeatHolderDir(home))
	if err != nil || !info.IsDir() {
		t.Fatalf("the holder directory does not exist: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("holder mode = %o, want 700", got)
	}
}

func TestSeatHolderDeny_AddsOnlyTheHolderToTheReadDenials(t *testing.T) {
	t.Parallel()
	base := sandbox.Config{RepoRoot: "/r", WritePaths: []string{"/w"}, DenyPaths: []string{"/d"}, DenyReadPaths: []string{"/a", "/b"}, AllowNetwork: true}
	got := withSeatHolderDeny(base, "/h/.evolve-seat/holder", "/v/h/.evolve-seat/holder")
	want := base
	want.DenyReadPaths = []string{"/a", "/b", "/h/.evolve-seat/holder", "/v/h/.evolve-seat/holder"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the config differs by more than the one holder deny:\n got %+v\nwant %+v", got, want)
	}
	if len(base.DenyReadPaths) != 2 {
		t.Fatalf("the input config was mutated: %v", base.DenyReadPaths)
	}
}

func TestSeatHolderDeny_AnEmptyHomeRefusesWithThePinnedText(t *testing.T) {
	t.Setenv("HOME", "")
	var stderr strings.Builder
	deps := Deps{SandboxMode: config.SandboxModeOn, Stderr: &stderr}
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(SandboxWrapRequest{
		CLI: "claude", Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if ok || prefix != nil {
		t.Fatalf("a launch with no home must be refused, got (%v, %v)", prefix, ok)
	}
	want := "[bridge] the seat token holder cannot be created: the home directory is empty. The launch is refused, because the cell would run with no deny on the holder.\n"
	if stderr.String() != want {
		t.Fatalf("refusal text:\n got %q\nwant %q", stderr.String(), want)
	}
}

func TestSeatHolderDeny_ABlockedDirectoryRefusesWithThePinnedText(t *testing.T) {
	home := t.TempDir()
	blocker := filepath.Join(home, ".evolve-seat")
	if err := os.WriteFile(blocker, []byte("a file where the directory belongs"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	mkErr := os.MkdirAll(paths.SeatHolderDir(home), 0o700)
	if mkErr == nil {
		t.Fatal("the fixture must block the directory")
	}
	var stderr strings.Builder
	deps := Deps{SandboxMode: config.SandboxModeOn, Stderr: &stderr}
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(SandboxWrapRequest{
		CLI: "codex", Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if ok || prefix != nil {
		t.Fatalf("a launch that cannot create the holder must be refused, got (%v, %v)", prefix, ok)
	}
	want := "[bridge] the seat token holder cannot be created: " + mkErr.Error() + ". The launch is refused, because the cell would run with no deny on the holder.\n"
	if stderr.String() != want {
		t.Fatalf("refusal text:\n got %q\nwant %q", stderr.String(), want)
	}
}

func TestSeatHolderDeny_ALaunchOnAnExistingHolderKeepsItsContents(t *testing.T) {
	home := t.TempDir()
	token := paths.SeatTokenPath(home)
	if err := os.MkdirAll(filepath.Dir(token), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("planted-fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	seatHolderLaunch(t, "claude", "off")
	body, err := os.ReadFile(token)
	if err != nil || string(body) != "planted-fixture" {
		t.Fatalf("the launch changed the planted file: %q, %v", body, err)
	}
}

func TestSeatHolderDeny_ALinuxLaunchMasksTheHolderAtBothSpellingsOfASymlinkedHome(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "home-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", link)
	deps := Deps{SandboxMode: config.SandboxModeOn}
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("linux", true))(SandboxWrapRequest{
		CLI: "claude", Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if !ok {
		t.Fatal("the wrap was refused")
	}
	canonical, err := filepath.EvalSymlinks(paths.SeatHolderDir(link))
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{paths.SeatHolderDir(link), canonical} {
		if !slices.Contains(prefix, spelling) || prefix[slices.Index(prefix, spelling)-1] != "--tmpfs" {
			t.Errorf("no --tmpfs mask at %q in %v", spelling, prefix)
		}
	}
}

func TestEnsureSeatHolder_AHomeWithoutASymlinkYieldsOneSpelling(t *testing.T) {
	t.Parallel()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ensureSeatHolder(home)
	if err != nil || len(got) != 1 || got[0] != paths.SeatHolderDir(home) {
		t.Fatalf("ensureSeatHolder = (%v, %v), want exactly [%s]", got, err, paths.SeatHolderDir(home))
	}
}

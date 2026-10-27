//go:build integration

package releasepipeline

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func stubTool(t *testing.T, name, script string) string {
	t.Helper()
	bin := t.TempDir()
	path := filepath.Join(bin, name)
	fakeclitest.Install(t, path, "#!/bin/sh\n"+script+"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path
}

func readRecord(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("record %s: %v", path, err)
	}
	return string(data)
}

func TestDefaultRebuildBinary_StampsTargetAndShortHeadIntoTheBuild(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "head")
	short := repo.Git("rev-parse", "--short=12", "HEAD")
	if err := os.MkdirAll(filepath.Join(repo.Dir, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "go-args")
	stubTool(t, "go", `printf '%s\n' "$PWD" "$@" > '`+record+`'; : > evolve`)

	if err := defaultRebuildBinary(repo.Dir, "7.8.9", false); err != nil {
		t.Fatalf("defaultRebuildBinary: %v", err)
	}

	got := readRecord(t, record)
	for _, want := range []string{
		filepath.Join(repo.Dir, "go") + "\nbuild\n-ldflags\n",
		"pkg/version.version=7.8.9 ",
		"pkg/version.commit=" + short + " ",
		"\n-o\nevolve\n./cmd/evolve\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("go invocation %q lacks %q", got, want)
		}
	}
}

func TestDefaultRebuildBinary_OutsideAGitRepoStampsUnknownCommit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "go-args")
	stubTool(t, "go", `printf '%s\n' "$@" > '`+record+`'; : > evolve`)

	if err := defaultRebuildBinary(root, "1.0.0", false); err != nil {
		t.Fatalf("defaultRebuildBinary: %v", err)
	}
	if got := readRecord(t, record); !strings.Contains(got, "pkg/version.commit=unknown ") {
		t.Fatalf("go invocation %q, want commit=unknown", got)
	}
}

func TestDefaultRebuildBinary_FailuresNameTheirCause(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"build fails with stderr", "echo 'syntax error in main.go' >&2; exit 2", "go build: exit status 2 (stderr: syntax error in main.go)"},
		{"build reports success but writes nothing", "exit 0", "post-build stat "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
				t.Fatal(err)
			}
			stubTool(t, "go", tc.script)

			err := defaultRebuildBinary(root, "1.0.0", false)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("defaultRebuildBinary = %v, want %q", err, tc.want)
			}
			if tc.name == "build reports success but writes nothing" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("err = %v, want it to wrap ErrNotExist", err)
			}
		})
	}
}

func TestDefaultShip_RunsTheReleaseShipAndReturnsTheNewHead(t *testing.T) {
	repo := gittest.Fixture(t)
	record := filepath.Join(t.TempDir(), "ship")
	script := `printf '%s\n' "$@" "notes=$EVOLVE_SHIP_RELEASE_NOTES" "confirm=$EVOLVE_SHIP_AUTO_CONFIRM" > '` + record + `'
git -C '` + repo.Dir + `' commit -q --allow-empty -m shipped`
	installRepoEvolve(t, repo.Dir, "#!/bin/sh\n"+script+"\n")

	head, err := defaultShip(repo.Dir, "release: v1.2.3", "line one\nline two")

	if err != nil || head != repo.Git("rev-parse", "HEAD") || len(head) != 40 {
		t.Fatalf("defaultShip = %q, %v; want the post-ship HEAD", head, err)
	}
	want := "ship\n--class\nrelease\nrelease: v1.2.3\nnotes=line one\nline two\nconfirm=\n"
	if got := readRecord(t, record); got != want {
		t.Fatalf("ship invocation = %q, want %q", got, want)
	}
}

func TestDefaultShip_FailuresAreReturnedWithoutAHead(t *testing.T) {
	t.Run("ship fails", func(t *testing.T) {
		repo := gittest.Fixture(t)
		installRepoEvolve(t, repo.Dir, "#!/bin/sh\necho 'gate refused'\nexit 3\n")

		head, err := defaultShip(repo.Dir, "msg", "")

		if head != "" || err == nil || err.Error() != "evolve ship: exit status 3 (output: gate refused)" {
			t.Fatalf("defaultShip = %q, %v; want the ship failure with its output", head, err)
		}
	})
	t.Run("head unreadable after ship", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
		installRepoEvolve(t, root, "#!/bin/sh\nexit 0\n")

		head, err := defaultShip(root, "msg", "")

		var exitErr *exec.ExitError
		if head != "" || !errors.As(err, &exitErr) {
			t.Fatalf("defaultShip = %q, %v; want the rev-parse exit error", head, err)
		}
	})
}

func TestDefaultReleaseVerify_ACommitWithoutTheBinaryIsTheV1850Defect(t *testing.T) {
	repo, _ := committedBinaryRepo(t, stampScript("1.2.3", 0))
	repo.Git("rm", "-q", "--cached", "go/evolve")
	repo.Git("commit", "-q", "-m", "binary untracked")
	untracked := repo.Git("rev-parse", "HEAD")

	err := defaultReleaseVerify(repo.Dir, "1.2.3", untracked)

	var exitErr *exec.ExitError
	if err == nil || !strings.Contains(err.Error(), "go/evolve not committed in release "+untracked+" (the v18.5.0 defect)") || !errors.As(err, &exitErr) {
		t.Fatalf("defaultReleaseVerify = %v, want the not-committed error wrapping git's exit", err)
	}
}

func installRepoEvolve(t *testing.T, root, script string) {
	t.Helper()
	bin := filepath.Join(root, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeclitest.Install(t, bin, script)
}

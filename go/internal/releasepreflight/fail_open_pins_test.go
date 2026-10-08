package releasepreflight

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

const redRequiredRun = `[{"status":"completed","conclusion":"failure","url":"https://ci/required"}]`

func TestDefaultNameGuard_MissingManifestIsCleanPass(t *testing.T) {
	for _, tc := range []struct {
		name   string
		layout func(t *testing.T, repoRoot string)
	}{
		{"no .evolve directory", func(*testing.T, string) {}},
		{".evolve directory without naming.json", func(t *testing.T, repoRoot string) {
			if err := os.MkdirAll(filepath.Join(repoRoot, ".evolve"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			tc.layout(t, repoRoot)
			violations, err := defaultNameGuard(repoRoot)
			if err != nil || violations != nil {
				t.Fatalf("defaultNameGuard = %v, %v; want nil, nil: a repo without a naming manifest has nothing to guard", violations, err)
			}
		})
	}
}

func TestDefaultNameGuard_StatErrorsOtherThanNotExistFail(t *testing.T) {
	for _, tc := range []struct {
		name           string
		makeUnreadable func(t *testing.T, manifestPath string)
		wantCause      error
	}{
		{"manifest directory without search permission", lockManifestDirectory, fs.ErrPermission},
		{"manifest is a symlink loop", loopManifestSymlink, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			manifestPath := filepath.Join(repoRoot, naminguard.DefaultManifestPath)
			if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
				t.Fatal(err)
			}
			tc.makeUnreadable(t, manifestPath)
			violations, err := defaultNameGuard(repoRoot)
			if err == nil {
				t.Fatalf("defaultNameGuard = %v, nil; want an error: an unreadable manifest is not an absent one", violations)
			}
			if errors.Is(err, fs.ErrNotExist) {
				t.Errorf("err = %v; want a cause other than not-exist", err)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Errorf("err = %v; want it to wrap %v", err, tc.wantCause)
			}
			if !strings.Contains(err.Error(), filepath.Base(manifestPath)) {
				t.Errorf("err = %v; want it to name %s", err, filepath.Base(manifestPath))
			}
		})
	}
}

func lockManifestDirectory(t *testing.T, manifestPath string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if err := os.WriteFile(manifestPath, []byte(`{"forbidden":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(manifestPath)
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("restore %s: %v", dir, err)
		}
	})
}

func loopManifestSymlink(t *testing.T, manifestPath string) {
	t.Helper()
	if err := os.Symlink(filepath.Base(manifestPath), manifestPath); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultCIConclusion_LookupFailuresAreUnavailableNotErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{"not a git repository", func(t *testing.T) string {
			dir := t.TempDir()
			t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
			installFakeGH(t, redRequiredRun)
			return dir
		}},
		{"git and gh absent from PATH", func(t *testing.T) string {
			repo := committedRepo(t)
			t.Setenv("PATH", t.TempDir())
			return repo
		}},
		{"gh absent from PATH", func(t *testing.T) string {
			repo := committedRepo(t)
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			fakeclitest.Install(t, filepath.Join(bin, "git"), "#!/bin/sh\nexec '"+realGit+"' \"$@\"\n")
			t.Setenv("PATH", bin)
			return repo
		}},
		{"gh exits non-zero", func(t *testing.T) string {
			repo := committedRepo(t)
			bin := t.TempDir()
			fakeclitest.Install(t, filepath.Join(bin, "gh"), "#!/bin/sh\necho 'gh: not logged in' >&2\nexit 4\n")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			return repo
		}},
		{"gh prints malformed JSON", func(t *testing.T) string {
			repo := committedRepo(t)
			installFakeGH(t, "not-json")
			return repo
		}},
		{"gh lists no run of the required workflow", func(t *testing.T) string {
			repo := committedRepo(t)
			installFakeGH(t, "[]")
			return repo
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := tc.setup(t)
			got, err := defaultCIConclusion(repoRoot)
			if want := (CIRunStatus{Conclusion: ciConclusionUnavailable}); err != nil || got != want {
				t.Fatalf("defaultCIConclusion = %+v, %v; want %+v, nil: absent CI tooling must not block a release", got, err, want)
			}
		})
	}
}

func committedRepo(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-qm", "release")
	return repo.Dir
}

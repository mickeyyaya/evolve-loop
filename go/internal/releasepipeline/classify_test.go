package releasepipeline

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func initClassifyRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := gittest.Fixture(t).Dir
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go/x.go", "package p\n")
	git("add", "-A")
	git("commit", "-m", "go change")
	git("tag", "v1.0.0")
	write("README.md", "docs only\n")
	git("add", "-A")
	git("commit", "-m", "docs only")
	git("tag", "v1.0.1")
	write("go/y.go", "package p\n\nfunc Y() {}\n")
	git("add", "-A")
	git("commit", "-m", "another go change")
	git("tag", "v1.1.0")
	return dir
}

func TestGitPathsChanged_RealRepo(t *testing.T) {
	repo := initClassifyRepo(t)
	if ch, err := gitPathsChanged(repo, "v1.0.0", "v1.0.1", "go"); err != nil || ch {
		t.Errorf("go changed v1.0.0..v1.0.1 = (%v,%v), want (false,nil) — that commit was docs-only", ch, err)
	}
	if ch, err := gitPathsChanged(repo, "v1.0.1", "v1.1.0", "go"); err != nil || !ch {
		t.Errorf("go changed v1.0.1..v1.1.0 = (%v,%v), want (true,nil) — go/y.go was added", ch, err)
	}
	if _, err := gitPathsChanged(t.TempDir(), "a", "b", "go"); err == nil {
		t.Error("gitPathsChanged on a non-git dir: want error")
	}
}

func TestGitOlderTags_RealRepo(t *testing.T) {
	repo := initClassifyRepo(t)
	got := gitOlderTags(repo, "v1.1.0")
	want := []string{"v1.0.1", "v1.0.0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("gitOlderTags(v1.1.0) = %v, want %v", got, want)
	}
	if got := gitOlderTags(repo, "v1.0.0"); len(got) != 0 {
		t.Errorf("gitOlderTags(v1.0.0) = %v, want empty (nothing older)", got)
	}
}

func probeFromMap(m map[string]bool) goChangedProbe {
	return func(from, to string) (bool, error) {
		return m[from+".."+to], nil
	}
}

func TestClassifyRelease(t *testing.T) {
	cases := []struct {
		name      string
		target    string
		prevTag   string
		older     []string
		changed   map[string]bool
		wantClass ReleaseClass
		wantSince string
	}{
		{
			name:   "binary change since prev tag -> binary-release",
			target: "22.3.0", prevTag: "v22.2.0",
			changed:   map[string]bool{"v22.2.0..HEAD": true},
			wantClass: BinaryRelease, wantSince: "22.3.0",
		},
		{
			name:   "no binary change, prev tag was a binary-release -> config since prev",
			target: "22.2.1", prevTag: "v22.2.0", older: []string{"v22.1.0"},
			changed:   map[string]bool{"v22.1.0..v22.2.0": true},
			wantClass: ConfigRelease, wantSince: "v22.2.0",
		},
		{
			name:   "no binary change across two config releases -> traces to the real binary-release",
			target: "22.2.3", prevTag: "v22.2.2", older: []string{"v22.2.1", "v22.2.0", "v22.1.0"},
			changed:   map[string]bool{"v22.1.0..v22.2.0": true},
			wantClass: ConfigRelease, wantSince: "v22.2.0",
		},
		{
			name:   "first release (no prev tag) -> binary-release",
			target: "1.0.0", prevTag: "",
			changed:   map[string]bool{},
			wantClass: BinaryRelease, wantSince: "1.0.0",
		},
		{
			name:   "config release, only the prev tag known -> traces to prev",
			target: "22.2.1", prevTag: "v22.2.0", older: nil,
			changed:   map[string]bool{},
			wantClass: ConfigRelease, wantSince: "v22.2.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := classifyRelease(tc.target, tc.prevTag, tc.older, probeFromMap(tc.changed))
			if err != nil {
				t.Fatalf("classifyRelease: %v", err)
			}
			if got.Class != tc.wantClass {
				t.Errorf("Class = %q, want %q", got.Class, tc.wantClass)
			}
			if got.SinceVersion != tc.wantSince {
				t.Errorf("SinceVersion = %q, want %q", got.SinceVersion, tc.wantSince)
			}
		})
	}
}

func TestReleaseClassification_Fields(t *testing.T) {
	c := ReleaseClassification{Class: BinaryRelease, SinceVersion: "22.3.0"}
	if c.Class != BinaryRelease {
		t.Errorf("Class = %q, want %q", c.Class, BinaryRelease)
	}
	if c.SinceVersion != "22.3.0" {
		t.Errorf("SinceVersion = %q, want 22.3.0", c.SinceVersion)
	}
}

func TestClassifyRelease_ProbeError(t *testing.T) {
	boom := func(_, _ string) (bool, error) { return false, errors.New("git boom") }
	if _, err := classifyRelease("22.3.0", "v22.2.0", nil, boom); err == nil {
		t.Fatal("want error propagated from the probe, got nil")
	}
}

func TestReleaseClassBanner_Wording(t *testing.T) {
	got := bannerFor(ConfigRelease, "22.1.0")
	if !strings.Contains(got, "config-release") || !strings.Contains(got, "unchanged since v22.1.0") {
		t.Errorf("config banner wrong: %q", got)
	}
	if !strings.Contains(got, "no new corporate approval") {
		t.Errorf("config banner should say no approval needed: %q", got)
	}
	gotB := bannerFor(BinaryRelease, "")
	if !strings.Contains(gotB, "binary-release") || !strings.Contains(gotB, "approval") {
		t.Errorf("binary banner wrong: %q", gotB)
	}
}

func TestReleaseClassBanner_FailsClosed(t *testing.T) {
	nonGitDir := t.TempDir()
	banner, err := releaseClassBanner(nonGitDir, "22.3.0", "v22.2.0")
	if err == nil {
		t.Fatal("want a git error from a non-git dir, got nil")
	}
	if banner == "" || !strings.Contains(banner, "unavailable") || !strings.Contains(banner, "approval") {
		t.Errorf("fail-closed banner must be non-empty and steer to approval; got %q", banner)
	}
}

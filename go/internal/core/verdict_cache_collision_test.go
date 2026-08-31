//go:build integration

package core

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func TestVerdictCacheCollisionRegression(t *testing.T) {
	tests := []struct {
		name        string
		dirty       bool
		baseMissing bool
		cached      bool
		wantSkipped bool
		wantMatched bool
	}{
		{name: "clean cached base is suppressed", cached: true, wantSkipped: true},
		{name: "missing base remains lookup eligible", baseMissing: true, cached: true, wantMatched: true},
		{name: "dirty cache miss remains observable", dirty: true},
		{name: "dirty cache hit remains observable", dirty: true, cached: true, wantMatched: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _ := initVerdictCacheProbeRepo(t)
			if tt.dirty {
				if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("changes"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			sha := worktreeContentSHA(context.Background(), repo)
			if sha == "" {
				t.Fatal("worktree content SHA is empty")
			}

			now := func() time.Time { return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC) }
			if tt.cached {
				if err := verdictcache.NewStore(repo, now).Put(verdictcache.Entry{
					TreeSHA: sha, Cycle: 99, Verdict: VerdictPASS,
					ArtifactSHA256: "test-artifact", ArtifactPath: "test-artifact",
				}); err != nil {
					t.Fatalf("seed verdict cache: %v", err)
				}
			}

			origGitRunner := gitRunner
			if tt.baseMissing {
				gitRunner = missingBaseGitRunner(origGitRunner)
				t.Cleanup(func() { gitRunner = origGitRunner })
			}

			var observations []verdictCacheLookupObservation
			runners := buildRunners(nil)
			o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 100}}, &fakeLedger{}, runners,
				WithWorktreeProvisioner(fixedWorktree{dir: repo}),
				WithVerdictCacheLookupHook(func(sha string, skipped bool, matched bool, entry verdictcache.Entry) {
					observations = append(observations, verdictCacheLookupObservation{sha, skipped, matched})
				}),
			)
			o.now = now
			if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: repo, GoalHash: "g"}); err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			if len(observations) != 1 {
				t.Fatalf("cache probe calls = %d, want 1", len(observations))
			}
			got := observations[0]
			if got.sha != sha || got.skipped != tt.wantSkipped || got.matched != tt.wantMatched {
				t.Fatalf("cache probe = %+v, want sha=%q skipped=%t matched=%t", got, sha, tt.wantSkipped, tt.wantMatched)
			}
			for _, phase := range []Phase{PhaseTDD, PhaseBuild, PhaseAudit} {
				if runners[phase].(*fakeRunner).calls != 1 {
					t.Errorf("%s calls = %d, want 1", phase, runners[phase].(*fakeRunner).calls)
				}
			}
		})
	}
}

type verdictCacheLookupObservation struct {
	sha     string
	skipped bool
	matched bool
}

func initVerdictCacheProbeRepo(t *testing.T) (repo, ws string) {
	t.Helper()
	repo, ws = initBindingRepo(t, "cycle-100")
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".gitignore")
	runGit("commit", "-m", "ignore .evolve")
	return repo, ws
}

func missingBaseGitRunner(orig sysexec.RunFunc) sysexec.RunFunc {
	return sysexec.RunFunc(func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "rev-parse" && args[1] == "HEAD" {
			_, _ = io.WriteString(stdout, "missing-base\n")
			return 0, nil
		}
		if len(args) == 2 && args[0] == "rev-parse" && args[1] == "missing-base^{tree}" {
			_, _ = io.WriteString(stderr, "fatal: Not a valid object name\n")
			return 128, nil
		}
		return orig(ctx, name, dir, args, env, stdin, stdout, stderr)
	})
}

type fixedWorktree struct {
	dir string
}

func (f fixedWorktree) Create(string, int) (string, error) { return f.dir, nil }
func (f fixedWorktree) Cleanup(string, string) error       { return nil }

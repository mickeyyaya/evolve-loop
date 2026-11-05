//go:build acs

package cycle2169

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitRefused    = 3
	quarantinedID  = "quarantined-item"
	quarantinedRel = ".evolve/inbox/quarantined-item.json"
	quarantineRel  = ".evolve/inbox/quarantine/quarantined-item.json"
	bystanderRel   = ".evolve/inbox/bystander.json"
	sourceRel      = "go/x.go"
	trackedItem    = `{"id":"quarantined-item","title":"a tracked item","weight":0.5}`
	loopStampedMv  = `{"id":"quarantined-item","title":"a tracked item","weight":0.5,"failure_count":2,"route":"console-manual","routed_reason":"protected surface"}`
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
	buildDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	buildOnce.Do(func() {
		buildDir, buildErr = os.MkdirTemp("", "cycle2169-evolve-")
		if buildErr != nil {
			return
		}
		builtBin = filepath.Join(buildDir, "evolve")
		out, err := exec.Command("go", "build", "-C", filepath.Join(root, "go"), "-o", builtBin, "./cmd/evolve").CombinedOutput()
		if err != nil {
			buildErr = &buildFailure{err: err, output: string(out)}
		}
	})
	if buildErr != nil {
		t.Fatalf("build the evolve binary: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct {
	err    error
	output string
}

func (b *buildFailure) Error() string { return b.err.Error() + "\n" + b.output }

type plane struct {
	origin  *gittest.Repo
	repo    *gittest.Repo
	console *gittest.Repo
}

func newPlane(t *testing.T) plane {
	t.Helper()
	origin := gittest.Bare(t)
	seed := gittest.Clone(t, origin.Dir)
	seed.Git("branch", "-M", "main")
	seed.Write(quarantinedRel, trackedItem)
	seed.Write(bystanderRel, `{"id":"bystander","weight":0.5}`)
	seed.Write(sourceRel, "package x\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	seed.Git("push", "-q", "origin", "main")
	return plane{origin: origin, repo: gittest.Clone(t, origin.Dir), console: gittest.Clone(t, origin.Dir)}
}

func (p plane) originAdvances(t *testing.T) {
	t.Helper()
	p.console.Write("docs/from-origin.md", "origin moved on\n")
	p.console.Git("add", "-A")
	p.console.Git("commit", "-q", "-m", "origin moves on")
	p.console.Git("push", "-q", "origin", "main")
}

func (p plane) quarantine(t *testing.T, body string) {
	t.Helper()
	if err := os.Remove(filepath.Join(p.repo.Dir, quarantinedRel)); err != nil {
		t.Fatal(err)
	}
	p.repo.Write(quarantineRel, body)
}

func (p plane) syncMain(t *testing.T) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), "sync-main", "--project-root", p.repo.Dir)
	cmd.Dir = p.repo.Dir
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	var sout, serr strings.Builder
	cmd.Stdout, cmd.Stderr = &sout, &serr
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return sout.String(), serr.String(), ee.ExitCode()
	}
	if err != nil {
		t.Fatalf("run evolve sync-main: %v", err)
	}
	return sout.String(), serr.String(), 0
}

func (p plane) head(t *testing.T) string {
	t.Helper()
	return p.repo.Git("rev-parse", "HEAD")
}

func (p plane) headTracks(t *testing.T, rel string) bool {
	t.Helper()
	return exec.Command("git", "-C", p.repo.Dir, "cat-file", "-e", "HEAD:"+rel).Run() == nil
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func exists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, rel))
	return err == nil
}

func TestC2169_001_ASyncMainPassesOnTheLoopsQuarantineMove(t *testing.T) {
	p := newPlane(t)
	p.quarantine(t, loopStampedMv)

	stdout, stderr, code := p.syncMain(t)

	if code != 0 {
		t.Fatalf("sync-main exit=%d, want 0: a quarantine move of a tracked inbox item is the loop's own write\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if got := readFile(t, p.repo.Dir, quarantineRel); got != loopStampedMv {
		t.Errorf("quarantined item after sync-main = %q, want it untouched: %q", got, loopStampedMv)
	}
	if exists(p.repo.Dir, quarantinedRel) {
		t.Errorf("%s came back to the inbox root: the item must stay in quarantine", quarantinedRel)
	}
}

func TestC2169_002_TheMoveIsRecordedInHistoryAndOriginsWorkIsMerged(t *testing.T) {
	p := newPlane(t)
	p.originAdvances(t)
	p.quarantine(t, loopStampedMv)

	stdout, stderr, code := p.syncMain(t)

	if code != 0 {
		t.Fatalf("sync-main exit=%d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if p.headTracks(t, quarantinedRel) {
		t.Errorf("HEAD still tracks %s: the removal half of the move is not recorded", quarantinedRel)
	}
	if !p.headTracks(t, quarantineRel) {
		t.Errorf("HEAD does not track %s: the quarantine half of the move is not recorded", quarantineRel)
	}
	if !p.headTracks(t, "docs/from-origin.md") {
		t.Errorf("origin's commit is not merged into HEAD: the move must not stop the merge")
	}
	if status := p.repo.Git("status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Errorf("tracked tree is dirty after sync-main:\n%s", status)
	}
}

func TestC2169_003_ARealEditOrDeletionIsStillRefused(t *testing.T) {
	cases := []struct {
		name  string
		dirty func(t *testing.T, p plane)
		names string
	}{
		{
			name: "an edit of a tracked source file",
			dirty: func(t *testing.T, p plane) {
				p.repo.Write(sourceRel, "package x\n\nvar Dirty = 1\n")
			},
			names: sourceRel,
		},
		{
			name: "a deleted tracked source file",
			dirty: func(t *testing.T, p plane) {
				if err := os.Remove(filepath.Join(p.repo.Dir, sourceRel)); err != nil {
					t.Fatal(err)
				}
			},
			names: sourceRel,
		},
		{
			name: "a deleted tracked inbox item with no quarantine copy",
			dirty: func(t *testing.T, p plane) {
				if err := os.Remove(filepath.Join(p.repo.Dir, quarantinedRel)); err != nil {
					t.Fatal(err)
				}
			},
			names: quarantinedRel,
		},
		{
			name: "a quarantine copy that differs from the tracked item beyond the loop's stamps",
			dirty: func(t *testing.T, p plane) {
				p.quarantine(t, `{"id":"quarantined-item","title":"an operator rewrote this","weight":0.9}`)
			},
			names: quarantinedRel,
		},
		{
			name: "a quarantine move together with an edit of a source file",
			dirty: func(t *testing.T, p plane) {
				p.quarantine(t, loopStampedMv)
				p.repo.Write(sourceRel, "package x\n\nvar Dirty = 1\n")
			},
			names: sourceRel,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newPlane(t)
			p.originAdvances(t)
			tc.dirty(t, p)
			before := p.head(t)

			_, stderr, code := p.syncMain(t)

			if code != exitRefused {
				t.Errorf("sync-main exit=%d, want %d (refused) for %s\nstderr: %s", code, exitRefused, tc.name, stderr)
			}
			if !strings.Contains(stderr, "refused") || !strings.Contains(stderr, tc.names) {
				t.Errorf("stderr must say refused and name %s:\n%s", tc.names, stderr)
			}
			if after := p.head(t); after != before {
				t.Errorf("a refused sync-main moved HEAD from %s to %s: it must commit nothing", before, after)
			}
			if p.headTracks(t, "docs/from-origin.md") {
				t.Errorf("a refused sync-main merged origin")
			}
		})
	}
}

func TestC2169_004_AMoveAroundAnUntouchedBystanderLeavesTheBystanderAlone(t *testing.T) {
	p := newPlane(t)
	p.quarantine(t, loopStampedMv)
	bystanderBefore := readFile(t, p.repo.Dir, bystanderRel)

	_, stderr, code := p.syncMain(t)

	if code != 0 {
		t.Fatalf("sync-main exit=%d, want 0\nstderr: %s", code, stderr)
	}
	if got := readFile(t, p.repo.Dir, bystanderRel); got != bystanderBefore {
		t.Errorf("an inbox item the loop did not move changed: %q, want %q", got, bystanderBefore)
	}
	if !p.headTracks(t, bystanderRel) {
		t.Errorf("HEAD lost the untouched inbox item %s", bystanderRel)
	}
}

func TestC2169_005_ACleanTreeStillSyncs(t *testing.T) {
	p := newPlane(t)
	p.originAdvances(t)

	stdout, stderr, code := p.syncMain(t)

	if code != 0 {
		t.Fatalf("sync-main exit=%d on a clean tree, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !p.headTracks(t, "docs/from-origin.md") {
		t.Errorf("origin's commit is not merged on a clean tree")
	}
}

func TestC2169_006_EveryFunctionOfInboxstampsStaysFullyCovered(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	if out, err := exec.Command("go", "test", "-C", goDir, "-count=1", "-coverprofile="+profile, "./internal/inboxstamps").CombinedOutput(); err != nil {
		t.Fatalf("go test ./internal/inboxstamps: %v\n%s", err, acsassert.OutputTail(string(out), 20))
	}
	funcs, err := exec.Command("go", "tool", "cover", "-func="+profile).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover -func: %v\n%s", err, funcs)
	}
	var short []string
	for _, line := range strings.Split(string(funcs), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "total:" || fields[len(fields)-1] == "100.0%" {
			continue
		}
		short = append(short, line)
	}
	if len(short) > 0 {
		t.Errorf("functions of internal/inboxstamps below 100%% coverage:\n%s", strings.Join(short, "\n"))
	}
}

func TestC2169_007_ASyncMainOutsideAGitTreeFailsLoudlyNamingTheRead(t *testing.T) {
	notARepo := t.TempDir()
	cmd := exec.Command(evolveBinary(t), "sync-main", "--project-root", notARepo)
	cmd.Dir = notARepo
	var sout, serr strings.Builder
	cmd.Stdout, cmd.Stderr = &sout, &serr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 1 {
		t.Fatalf("sync-main outside a git tree: err=%v, want exit 1\nstderr: %s", err, serr.String())
	}
	if !strings.Contains(serr.String(), "reading the tree's changes failed") {
		t.Errorf("stderr does not name the failed read of the tree:\n%s", serr.String())
	}
}

func TestC2169_008_RefuseDirtBeyondInboxStampsIsFullyCoveredBySyncMainTests(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	if out, err := exec.Command("go", "test", "-C", goDir, "-count=1", "-run", "SyncMain", "-coverprofile="+profile, "./cmd/evolve").CombinedOutput(); err != nil {
		t.Fatalf("go test -run SyncMain ./cmd/evolve: %v\n%s", err, acsassert.OutputTail(string(out), 20))
	}
	funcs, err := exec.Command("go", "tool", "cover", "-func="+profile).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover -func: %v\n%s", err, funcs)
	}
	var measured []string
	for _, line := range strings.Split(string(funcs), "\n") {
		if strings.Contains(line, "cmd_syncmain.go") && strings.Contains(line, "refuseDirtBeyondInboxStamps") {
			measured = append(measured, line)
		}
	}
	if len(measured) != 1 {
		t.Fatalf("want one coverage line for refuseDirtBeyondInboxStamps, got %d:\n%s", len(measured), strings.Join(measured, "\n"))
	}
	fields := strings.Fields(measured[0])
	if fields[len(fields)-1] != "100.0%" {
		t.Errorf("a changed function of AC3 is below 100%% coverage under the sync-main tests: %s", measured[0])
	}
}

func TestC2169_009_OriginEditingTheMovedItemAbortsTheMergeCleanlyAndKeepsTheMove(t *testing.T) {
	p := newPlane(t)
	p.console.Write(quarantinedRel, `{"id":"quarantined-item","title":"edited on origin","weight":0.9}`)
	p.console.Git("add", "-A")
	p.console.Git("commit", "-q", "-m", "origin edits the item")
	p.console.Git("push", "-q", "origin", "main")
	p.quarantine(t, loopStampedMv)

	_, stderr, code := p.syncMain(t)

	if code != 1 {
		t.Fatalf("sync-main exit=%d, want 1: origin's edit of a moved item conflicts\nstderr: %s", code, stderr)
	}
	if exec.Command("git", "-C", p.repo.Dir, "rev-parse", "-q", "--verify", "MERGE_HEAD").Run() == nil {
		t.Errorf("MERGE_HEAD is set: the conflicting merge was not aborted")
	}
	if got := readFile(t, p.repo.Dir, quarantineRel); got != loopStampedMv {
		t.Errorf("quarantine copy = %q, want it kept: %q", got, loopStampedMv)
	}
	if !p.headTracks(t, quarantineRel) || p.headTracks(t, quarantinedRel) {
		t.Errorf("HEAD does not record the move after the aborted merge")
	}
}

//go:build acs

package cycle2538

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	builderReport = "## Changes\nx.go\n## Tests\n1/1 PASS\n## Verdict\nVerdict: PASS\n"
	patchName     = "fence-reverted.patch"
	grantedPath   = "go/x.go"
	tamperedPath  = "go/other.go"
	tamperedBody  = "package x // seat tampered\n"
	baseBody      = "package x // other base\n"
)

var profileFiles = []string{
	"docs/architecture/phase-registry.json",
	".evolve/phases/console-builder/phase.json",
	".evolve/profiles/console-builder.json",
	"agents/evolve-console-builder.md",
}

var printedPatchPath = regexp.MustCompile(`reverted patch: ([^)\s]+)`)

type seatBridge struct {
	writes map[string]string
}

func (b seatBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := os.WriteFile(req.ArtifactPath, []byte(builderReport), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	for rel, body := range b.writes {
		abs := filepath.Join(req.Worktree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return core.BridgeResponse{}, err
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			return core.BridgeResponse{}, err
		}
	}
	return core.BridgeResponse{}, nil
}

func (seatBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func stagedProject(t *testing.T) string {
	t.Helper()
	repoRoot := acsassert.RepoRoot(t)
	root := t.TempDir()
	for _, rel := range profileFiles {
		raw, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func dispatchedWorktree(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(grantedPath, "package x // base\n")
	repo.Write(tamperedPath, baseBody)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	wt := filepath.Join(t.TempDir(), "seat")
	repo.Git("worktree", "add", "-q", "-b", "seat", wt)
	return wt
}

func runBuilder(t *testing.T, root, worktree string, writes map[string]string) (int, string) {
	t.Helper()
	deps := agentcmd.Deps{
		Bridge:    func(string, *signalcenter.Center) core.Bridge { return seatBridge{writes: writes} },
		Signals:   func(string, io.Writer) (*signalcenter.Center, func()) { return signalcenter.New(), func() {} },
		TreeState: func(context.Context, string) (string, error) { return "tree", nil },
		Getenv:    func(string) string { return "" },
		Now:       func() time.Time { return time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC) },
	}
	args := []string{"run", "console-builder", "--project-root", root, "--json", "--worktree", worktree, "--writable", grantedPath}
	var stdout, stderr bytes.Buffer
	code := agentcmd.New(deps)(args, strings.NewReader("build item X"), &stdout, &stderr)
	return code, stderr.String()
}

func fenceRevertingWrites() map[string]string {
	return map[string]string{
		grantedPath:  "package x // seat edit\n",
		tamperedPath: tamperedBody,
	}
}

func patchesUnder(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".patch") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestC2538_001_AFenceRevertedPatchSurvivesAtThePathTheWarnPrints(t *testing.T) {
	root := stagedProject(t)
	wt := dispatchedWorktree(t)

	code, stderr := runBuilder(t, root, wt, fenceRevertingWrites())

	match := printedPatchPath.FindStringSubmatch(stderr)
	if match == nil {
		t.Fatalf("exit %d: the WARN line prints no reverted patch path:\n%s", code, stderr)
	}
	raw, err := os.ReadFile(match[1])
	if err != nil {
		t.Fatalf("RED: the patch the WARN names is not readable after agent run returned: %v", err)
	}
	if !strings.Contains(string(raw), tamperedPath) {
		t.Errorf("the kept patch does not name the reverted %s:\n%s", tamperedPath, raw)
	}
	if filepath.Base(match[1]) != patchName {
		t.Errorf("the kept patch is %q, want the file name %q", filepath.Base(match[1]), patchName)
	}
}

func TestC2538_002_GitApplyOfTheKeptPatchRestoresTheRevertedWrites(t *testing.T) {
	root := stagedProject(t)
	wt := dispatchedWorktree(t)

	_, stderr := runBuilder(t, root, wt, fenceRevertingWrites())

	match := printedPatchPath.FindStringSubmatch(stderr)
	if match == nil {
		t.Fatalf("the WARN line prints no reverted patch path:\n%s", stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(wt, tamperedPath)); string(got) != baseBody {
		t.Fatalf("the fence did not revert %s: %q", tamperedPath, got)
	}
	if out, err := exec.Command("git", "-C", wt, "apply", match[1]).CombinedOutput(); err != nil {
		t.Fatalf("RED: git apply %s: %v: %s", match[1], err, out)
	}
	if got, _ := os.ReadFile(filepath.Join(wt, tamperedPath)); string(got) != tamperedBody {
		t.Errorf("git apply did not restore the reverted write to %s: %q", tamperedPath, got)
	}
}

func TestC2538_003_ARunThatRevertsNothingLeavesNoPatchAndNoWorkspace(t *testing.T) {
	root := stagedProject(t)
	wt := dispatchedWorktree(t)

	code, stderr := runBuilder(t, root, wt, map[string]string{grantedPath: "package x // seat edit\n"})

	if code != 0 {
		t.Fatalf("exit %d, want 0 for a seat inside its grant:\n%s", code, stderr)
	}
	if printedPatchPath.MatchString(stderr) {
		t.Errorf("a run that reverts nothing prints a patch path:\n%s", stderr)
	}
	if kept := patchesUnder(t, filepath.Join(root, ".evolve")); len(kept) != 0 {
		t.Errorf("a run that reverts nothing left patches: %v", kept)
	}
	workspaces, err := os.ReadDir(filepath.Join(root, ".evolve", "ephemeral", "agent-runs"))
	if err == nil && len(workspaces) != 0 {
		t.Errorf("the run workspace was not removed: %d entries left", len(workspaces))
	}
}

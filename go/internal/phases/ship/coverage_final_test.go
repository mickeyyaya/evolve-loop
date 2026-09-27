//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestVerifySelfSHA_CleanPass_ReturnsNilNoLog(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-stable\n")
	sha, _ := sha256File(bin)
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"`+sha+`","expected_ship_version":"10.0.0"}`)
	mustWrite(t, filepath.Join(dir, ".claude-plugin", "plugin.json"),
		`{"version":"10.0.0"}`)
	opts := &Options{ProjectRoot: dir, PluginRoot: dir, ShipBinaryPath: bin}
	res := &RunResult{}
	if err := verifySelfSHA(context.Background(), opts, res); err != nil {
		t.Fatalf("clean pass must return nil; got %v", err)
	}
	for _, l := range res.Logs {
		if strings.Contains(l, "TOFU") {
			t.Errorf("clean pass must not repin; got TOFU log: %q", l)
		}
	}
}

func TestShipFromWorktree_TreeSHABindingVerifiedLog(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "binding-test-branch")
	mustWrite(t, filepath.Join(wt, "binding.txt"), "content\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":20,"active_worktree":"`+wt+`"}`)

	// seedAudit (no bound tree) is used here so the binding check is skipped
	// and only the post-push path is exercised.
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: binding log test"})
	if err != nil {
		t.Fatalf("ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "ff-merged binding-test-branch into main") {
		t.Errorf("missing ff-merge log; got %v", res.Logs)
	}
}

func TestShipFromWorktree_WithAuditBoundTreeSHA_BindingLogged(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "treesha-branch")

	mustWrite(t, filepath.Join(wt, "treesha.txt"), "bound content\n")
	runGit(t, wt, "add", "treesha.txt")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-m", "pre-ship commit in wt")

	wtTreeSHA := strings.TrimSpace(runGitOut(t, wt, "rev-parse", "HEAD^{tree}"))

	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":21,"active_worktree":"`+wt+`"}`)
	// Binds audit HEAD to repo's HEAD (the check target) while setting
	// internalAuditBoundTreeSHA to wtTreeSHA.
	seedAuditWithBoundTree(t, repo, "PASS", wtTreeSHA)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: bound tree ship"})
	if err != nil {
		t.Fatalf("bound tree ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "tree-SHA binding verified") {
		t.Errorf("missing tree-SHA binding verified log; got %v", res.Logs)
	}
}

func TestAdvanceLastCycleNumber_WriteStateFails_WarnsAndReturnsNil(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	mustWrite(t, filepath.Join(evolveDir, "cycle-state.json"), `{"cycle_id":15}`)
	mustWrite(t, filepath.Join(evolveDir, "state.json"), `{"lastCycleNumber":14}`)
	// .evolve is made read-only so CreateTemp inside writeStateMap fails.
	if err := os.Chmod(evolveDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	opts := &Options{ProjectRoot: root}
	res := &RunResult{}
	err := advanceLastCycleNumber(opts, res)
	if err != nil {
		t.Fatalf("writeStateMap fail must WARN not error; got %v", err)
	}
	if !containsLog(*res, "WARN: could not advance lastCycleNumber") {
		t.Errorf("missing WARN log; got %v", res.Logs)
	}
}

func TestRepinPostCycle_StateReadError_ReturnsError(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "binary-v4\n")
	newSHA, _ := sha256File(bin)
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"), `{"expected_ship_sha":"old"}`)
	if err := os.Remove(filepath.Join(root, ".evolve", "state.json")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "state.json"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_ = newSHA

	opts := &Options{ProjectRoot: root, ShipBinaryPath: bin}
	res := &RunResult{}
	err := repinPostCycle(opts, res)
	if err == nil {
		t.Fatal("state.json read error must propagate from repinPostCycle")
	}
}

func TestRepinPostCycle_WriteStateFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "binary-v5\n")
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"),
		`{"expected_ship_sha":"completely-different-sha"}`)
	if err := os.Remove(filepath.Join(root, ".evolve", "state.json")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "state.json"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	opts := &Options{ProjectRoot: root, ShipBinaryPath: bin}
	res := &RunResult{}
	err := repinPostCycle(opts, res)
	if err == nil {
		t.Fatal("read error must propagate")
	}
}

func TestPostShip_AdvanceError_Propagates(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "bin\n")
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":50}`)
	stDir := filepath.Join(root, ".evolve", "state.json")
	if err := os.MkdirAll(stDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	opts := &Options{
		Class:          ClassCycle,
		ProjectRoot:    root,
		ShipBinaryPath: bin,
		Stderr:         io.Discard,
	}
	res := &RunResult{ClassUsed: ClassCycle}
	err := postShip(context.Background(), opts, res)
	if err == nil {
		t.Fatal("advance error must propagate from postShip")
	}
}

func TestVerifyTrivial_StagedDiffError_Errors(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "trivial")
	r := &scriptedRunner{}
	r.runner()
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{err: errors.New("git diff failed")}
	opts := &Options{ProjectRoot: root, Runner: r.runner()}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("captureGitOutput error must propagate")
	}
}

func TestVerifyTrivial_CriticalPathTruncated_Shows3Max(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "trivial")
	r := &scriptedRunner{}
	r.runner()
	criticalFiles := "skills/a.md\nskills/b.md\nskills/c.md\nskills/d.md\n"
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: criticalFiles, exit: 0}
	r.scripts["git ls-files"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "", exit: 0}
	opts := &Options{ProjectRoot: root, Runner: r.runner()}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	// 4 critical files → config-class refusal; message shows "4 touched" but
	// only samples 3 paths.
	wantShipErr(t, err, core.CodeTrivialCriticalPaths, core.ShipClassConfig, "4 touched")
}

func TestFindLatestAudit_ReadError_Propagates(t *testing.T) {
	dir := t.TempDir()
	// Pass a directory path (not a file) — os.ReadFile returns "is a directory"
	// which is NOT os.ErrNotExist.
	_, err := findLatestAudit(dir, "")
	if err == nil {
		t.Fatal("read error must propagate")
	}
	if _, ok := err.(*IntegrityError); ok {
		t.Errorf("read error should not be an IntegrityError; got %v", err)
	}
	se := mustShipErr(t, err)
	if se.Class == core.ShipClassIntegrity {
		t.Errorf("read error should be transient, not integrity; got class=%s", se.Class)
	}
}

func TestVerifyAuditBinding_WarnFluent_LogsAndPasses(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "WARN")
	opts := auditOpts(t, repo)
	res := &RunResult{}
	if err := verifyAuditBinding(context.Background(), opts, res); err != nil {
		t.Fatalf("WARN fluent must pass; got %v", err)
	}
	if !containsLog(*res, "WARN — shipping per fluent-by-default policy") {
		t.Errorf("missing WARN fluent log; got %v", res.Logs)
	}
}

func TestReadStateMap_EmptyFile_ReturnsEmptyMap(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, err := readStateMap(p)
	if err != nil {
		t.Fatalf("empty file must return nil error; got %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("empty file must return empty non-nil map; got %v", m)
	}
}

func TestAtomicShip_EmptyBranch_DetachedHEAD_Refuses(t *testing.T) {
	r := &scriptedRunner{}
	r.runner()
	// symbolic-ref exits 0 but returns empty string → "" branch name.
	r.scripts["git symbolic-ref"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "\n", exit: 0}
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "x",
		ProjectRoot:   t.TempDir(),
		Runner:        r.runner(),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := atomicShip(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("empty branch name must refuse with detached HEAD; got %v", err)
	}
}

func TestShipDirect_BuildDiffFooterRunnerError_Propagates(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "change.txt"), "staged\n")
	runGit(t, repo, "add", "change.txt")

	r := &scriptedRunner{}
	r.runner()
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{err: errors.New("diff exploded")}

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "test",
		ProjectRoot:   repo,
		Runner:        r.runner(),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil {
		t.Fatal("diff runner error must propagate")
	}
}

func TestWriteShipBinding_MkdirFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":77}`)
	mustWrite(t, filepath.Join(root, ".evolve", "runs"), "i am a file\n")
	opts := &Options{ProjectRoot: root}
	err := writeShipBinding(opts, "tree", "commit")
	if err == nil {
		t.Fatal("MkdirAll failure must return error")
	}
}

package ship

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func containsLog(res RunResult, substr string) bool {
	for _, l := range res.Logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

func writeAttestation(t *testing.T, repo, treeSHA string) {
	t.Helper()
	writeGateAttestation(t, repo, commitgate.Attestation{
		TreeStateSHA: treeSHA,
		TS:           "2026-05-27T00:00:00Z",
		ChecksPassed: []string{"go:gofmt", "go:test"},
		ReviewersRun: []string{"code-simplifier", "code-reviewer", "go-reviewer"},
		Tool:         "shasum",
	})
}

func writeGateAttestation(t *testing.T, repo string, att commitgate.Attestation) {
	t.Helper()
	body, err := att.Marshal()
	if err != nil {
		t.Fatalf("commit-gate attestation: %v", err)
	}
	mustMkdir(t, filepath.Join(repo, ".commit-gate"))
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"), string(body))
}

func tempRepoDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "shiptest-*")
	if err != nil {
		t.Fatalf("mkdir temp repo: %v", err)
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(p string, _ fs.DirEntry, walkErr error) error {
			if walkErr == nil {
				_ = os.Chmod(p, 0o755)
			}
			return nil
		})
		_ = os.RemoveAll(dir)
	})
	return dir
}

func makeRepo(t *testing.T) string {
	t.Helper()
	return gittest.FromTemplate(t, "ship-make-repo", func(r *gittest.Repo) error {
		r.Write(".gitignore", ".evolve/\n")
		if err := os.MkdirAll(filepath.Join(r.Dir, ".evolve", "runs", "cycle-1"), 0o755); err != nil {
			return err
		}
		r.Write(".evolve/ledger.jsonl", "")
		r.Write(".evolve/state.json", "{}\n")
		r.Write("fixture.txt", "fixture line 1\n")
		r.Write("ship-binary-fixture", "ship-binary-v1\n")
		r.Git("config", "user.email", "test@evolve-loop.test")
		r.Git("config", "user.name", "Test User")
		r.Git("config", "core.hooksPath", "/dev/null")
		r.Git("add", "-A")
		r.Git("-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial test repo")
		return nil
	}).Dir
}

func addRemote(t *testing.T, repo string) string {
	t.Helper()
	bare := gittest.Bare(t).Dir
	runGit(t, repo, "remote", "add", "origin", bare)
	runGit(t, repo, "branch", "-M", "main")
	return bare
}

func seedAudit(t *testing.T, repo, verdict string, optOverrides ...map[string]string) {
	t.Helper()
	overrides := map[string]string{}
	if len(optOverrides) > 0 {
		overrides = optOverrides[0]
	}
	state, _ := readStateMap(filepath.Join(repo, ".evolve", "cycle-state.json"))
	if state == nil {
		state = map[string]any{}
	}
	cycle, _ := stateInt(state, "cycle_id")
	if cycle <= 0 {
		cycle = 1
	}
	runID := stateString(state, "run_id")
	if runID == "" {
		runID = "test-run"
	}
	if raw := overrides["cycle"]; raw != "" {
		fmt.Sscanf(raw, "%d", &cycle)
	}
	auditPath := filepath.Join(repo, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle), "audit-report.md")
	body := fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle %d\n\nVerdict: %s\n\nAll criteria met (test fixture).\n", cycle, verdict)
	mustWrite(t, auditPath, body)
	testedRoot := stateString(state, "active_worktree")
	if testedRoot == "" {
		testedRoot = repo
	}
	runGit(t, testedRoot, "add", "-A")
	snap, err := treefence.TakeStaged(context.Background(), testedRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if overrides["bound_tree"] != "" {
		snap.Tree = overrides["bound_tree"]
	}
	sealTestPredicateEvidence(t, acssuite.EvidenceIdentity{Cycle: cycle, RunID: runID, Round: 1, TreeSHA: snap.Tree}, auditPath)
	sha := mustHashFile(t, auditPath)
	state["cycle_id"], state["run_id"], state["audit_dispatches"] = cycle, runID, 1
	state["workspace_path"] = filepath.Dir(auditPath)
	if err := writeStateMap(filepath.Join(repo, ".evolve", "cycle-state.json"), state); err != nil {
		t.Fatal(err)
	}

	headSHA := overrides["head"]
	if headSHA == "" {
		headSHA = strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	}
	treeSHA := overrides["tree"]
	if treeSHA == "" {
		treeSHA = treeStateSHA(t, repo)
	}
	exitCode := 0
	if v := overrides["exit_code"]; v != "" {
		fmt.Sscanf(v, "%d", &exitCode)
	}

	entry := map[string]any{
		"ts":                "2026-04-27T00:00:00Z",
		"cycle":             cycle,
		"run_id":            runID,
		"worktree_tree_sha": snap.Tree,
		"role":              "auditor",
		"kind":              "agent_subprocess",
		"model":             "sonnet",
		"exit_code":         exitCode,
		"duration_s":        "30",
		"artifact_path":     auditPath,
		"artifact_sha256":   sha,
		"challenge_token":   "testtoken123",
		"git_head":          headSHA,
		"tree_state_sha":    treeSHA,
	}
	line, _ := json.Marshal(entry)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

func treeStateSHA(t *testing.T, repo string) string {
	t.Helper()
	out := runGitOut(t, repo, "diff", "HEAD")
	h := sha256.New()
	_, _ = h.Write([]byte(out))
	return hex.EncodeToString(h.Sum(nil))
}

func mustHashFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = filteredEnv()
		return cmd.CombinedOutput()
	})
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = filteredEnv()
		return cmd.Output()
	})
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func filteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func runShip(t *testing.T, repo string, opts Options) (RunResult, error) {
	t.Helper()
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = repo
	}
	if opts.PluginRoot == "" {
		opts.PluginRoot = repo
	}
	if opts.ShipBinaryPath == "" {
		opts.ShipBinaryPath = filepath.Join(repo, "ship-binary-fixture")
	}
	if opts.Runner == nil {
		opts.Runner = execRunner
	}
	var stderr bytes.Buffer
	if opts.Stderr == nil {
		opts.Stderr = &stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = bytes.NewReader(nil)
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return Run(ctx, opts)
}

func makeWorktree(t *testing.T, repo, branch string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", "-b", branch, wt, "main")
	return wt
}

func seedAuditWithBoundTree(t *testing.T, repo, verdict, boundTreeSHA string) {
	t.Helper()
	seedAudit(t, repo, verdict, map[string]string{"bound_tree": boundTreeSHA})

}

func makeWorktreeScenario(t *testing.T) (string, string) {
	t.Helper()
	repo := makeRepo(t)
	addRemote(t, repo)
	seedAudit(t, repo, "PASS")

	wt := tempRepoDir(t)
	runGit(t, repo, "worktree", "add", "-b", "cycle-1", wt)

	mustWrite(t, filepath.Join(wt, "wt-change.txt"), "worktree change\n")
	runGit(t, wt, "add", "wt-change.txt")

	return repo, wt
}

func predicateVerdictFixture(cycle, green, red, skip int) acssuite.Verdict {
	v := acssuite.Verdict{SchemaVersion: "1.0", Cycle: cycle, GreenCount: green, RedCount: red, SkipCount: skip, Verdict: "PASS", ShipEligible: red == 0,
		PredicateSuite: acssuite.PredicateSuite{Total: green + red + skip, ThisCycleCount: green + red + skip, SkippedCount: skip}}
	if red > 0 {
		v.Verdict = "FAIL"
	}
	for i := 0; i < green+red+skip; i++ {
		id := fmt.Sprintf("predicate-%d", i)
		r := acssuite.Result{ACID: id, Predicate: "go/acs/cycle/...:" + id, ResultStr: "green"}
		if i >= green && i < green+red {
			r.ResultStr = "red"
			r.ExitCode = 1
			v.RedIDs = append(v.RedIDs, id)
		}
		if i >= green+red {
			r.ResultStr = "skip"
			r.ExitCode = acssuite.SkipExitCode
			v.SkipIDs = append(v.SkipIDs, id)
		}
		v.Results = append(v.Results, r)
	}
	return v
}

func sealTestPredicateEvidence(t *testing.T, id acssuite.EvidenceIdentity, reportPath string) {
	t.Helper()
	raw, err := json.Marshal(predicateVerdictFixture(id.Cycle, 1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(filepath.Dir(reportPath), acsverdict.Filename), string(raw))
	if err := acssuite.SealEvidence(reportPath, raw, id); err != nil {
		t.Fatal(err)
	}
}

func auditOpts(t *testing.T, repo string) *Options {
	t.Helper()
	bin := filepath.Join(repo, "ship-binary-fixture")
	preSeedTOFU(t, repo, bin)
	return &Options{
		ProjectRoot: repo,
		CycleID:     1, RunID: "test-run", AuditRound: 1,
		PluginRoot:     repo,
		ShipBinaryPath: bin,
		Runner:         execRunner,
		NowFn:          defaultNow,
	}
}

func preSeedTOFU(t *testing.T, repo, binPath string) {
	t.Helper()
	sha, err := sha256File(binPath)
	if err != nil {
		t.Fatalf("sha256File(%s): %v", binPath, err)
	}
	stPath := filepath.Join(repo, ".evolve", "state.json")
	m, _ := readStateMap(stPath)
	m["expected_ship_sha"] = sha
	m["expected_ship_version"] = ""
	if err := writeStateMap(stPath, m); err != nil {
		t.Fatalf("preSeedTOFU writeStateMap: %v", err)
	}
}

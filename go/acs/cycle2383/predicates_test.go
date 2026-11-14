//go:build acs

// Package cycle2383 encodes the acceptance of gc-branch-items-are-ref-state-work-and-landed-content-is-landed.
package cycle2383

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	gcRunsPerScenario  = 3
	sweepAppliedMarker = "workspace sweep applied"
	commandTimeout     = 2 * time.Minute
	commitFile         = "feature.txt"
	unlandedFile       = "unlanded.txt"
)

var evolveBin, buildFailure string

func TestMain(m *testing.M) {
	os.Exit(mainWithBinary(m))
}

func mainWithBinary(m *testing.M) int {
	binDir, err := os.MkdirTemp("", "acs-cycle2383-")
	if err != nil {
		buildFailure = err.Error()
		return m.Run()
	}
	defer os.RemoveAll(binDir)
	wd, err := os.Getwd()
	if err != nil {
		buildFailure = err.Error()
		return m.Run()
	}
	moduleDir, err := acsassert.ModuleDir(wd)
	if err != nil {
		buildFailure = err.Error()
		return m.Run()
	}
	target := filepath.Join(binDir, "evolve")
	build := exec.Command("go", "build", "-o", target, "./cmd/evolve")
	build.Dir = moduleDir
	if out, err := build.CombinedOutput(); err != nil {
		buildFailure = fmt.Sprintf("go build ./cmd/evolve: %v\n%s", err, out)
		return m.Run()
	}
	evolveBin = target
	return m.Run()
}

type project struct {
	t    *testing.T
	repo *gittest.Repo
	env  []string
}

func newProject(t *testing.T) project {
	t.Helper()
	if buildFailure != "" {
		t.Fatalf("evolve CLI did not build from this worktree: %s", buildFailure)
	}
	repo := gittest.Fixture(t)
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "seed.txt")
	repo.Git("commit", "-qm", "seed")
	p := project{t: t, repo: repo, env: isolatedEnv(t)}
	p.write(".evolve/policy.json", `{"gc":{}}`)
	p.holdALiveRun()
	return p
}

func isolatedEnv(t *testing.T) []string {
	t.Helper()
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "TMUX") || strings.HasPrefix(name, "GIT_") || name == "GOCACHE" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOCACHE="+t.TempDir(), "TMUX_TMPDIR="+t.TempDir(), "GIT_TERMINAL_PROMPT=0")
}

func (p project) write(rel, body string) {
	p.t.Helper()
	p.repo.Write(rel, body)
}

func (p project) holdALiveRun() {
	p.t.Helper()
	lease, err := json.Marshal(map[string]any{
		"run_id":       "live-run",
		"owner_pid":    os.Getpid(),
		"heartbeat_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/live-run/.lease", string(lease)+"\n")
}

func (p project) run(args ...string) (stdout, stderr string, code int) {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin, args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = p.repo.Dir, p.env, 5*time.Second
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		p.t.Fatalf("evolve %s: %v", strings.Join(args, " "), err)
	}
	return out.String(), errOut.String(), code
}

func (p project) gc() {
	p.t.Helper()
	stdout, stderr, _ := p.run("gc", "--project-root", p.repo.Dir)
	if !strings.Contains(stdout, sweepAppliedMarker) {
		p.t.Fatalf("evolve gc never applied its workspace sweep:\n--- stdout ---\n%s--- stderr ---\n%s", stdout, stderr)
	}
}

func (p project) gcRepeatedly() {
	p.t.Helper()
	for range gcRunsPerScenario {
		p.gc()
	}
}

type filedItem struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

func (p project) inboxFiled() []filedItem {
	p.t.Helper()
	var items []filedItem
	root := filepath.Join(p.repo.Dir, ".evolve", "inbox")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var it filedItem
		if err := json.Unmarshal(raw, &it); err != nil {
			return fmt.Errorf("inbox file %s is not an item: %w", path, err)
		}
		items = append(items, it)
		return nil
	})
	if err != nil {
		p.t.Fatal(err)
	}
	return items
}

func (p project) branchExists(branch string) bool {
	p.t.Helper()
	return gittest.BranchExists(p.t, p.repo.Dir, branch)
}

func (p project) commitOn(branch, rel, body string) {
	p.t.Helper()
	p.repo.Git("checkout", "-q", "-b", branch)
	p.write(rel, body)
	p.repo.Git("add", rel)
	p.repo.Git("commit", "-qm", "work on "+branch)
	p.repo.Git("checkout", "-q", "main")
}

func (p project) revertedBranch(branch string) {
	p.t.Helper()
	p.repo.Git("checkout", "-q", "-b", branch)
	p.write(commitFile, branch+"\n")
	p.repo.Git("add", commitFile)
	p.repo.Git("commit", "-qm", "add "+branch)
	p.repo.Git("revert", "--no-edit", "HEAD")
	p.repo.Git("checkout", "-q", "main")
}

func (p project) advanceMain(rel string) {
	p.t.Helper()
	p.write(rel, "main moved on\n")
	p.repo.Git("add", rel)
	p.repo.Git("commit", "-qm", "advance main with "+rel)
}

func (p project) unlandedWorktree(branch string) string {
	p.t.Helper()
	dir := filepath.Join(p.repo.Dir, ".evolve", "worktrees", branch)
	p.repo.Git("worktree", "add", "-q", "-b", branch, dir)
	if err := os.WriteFile(filepath.Join(dir, unlandedFile), []byte(branch+"\n"), 0o644); err != nil {
		p.t.Fatal(err)
	}
	p.repo.Git("-C", dir, "add", unlandedFile)
	p.repo.Git("-C", dir, "commit", "-qm", "work on "+branch)
	return dir
}

func (p project) requireNoItemFiled(scenario string) {
	p.t.Helper()
	if filed := p.inboxFiled(); len(filed) != 0 {
		p.t.Errorf("RED: gc filed %v for %s", filed, scenario)
	}
}

func TestC2383_001_ABranchWhoseCommitWasRevertedHasNoNetDiffAndGcFilesNoItem(t *testing.T) {
	p := newProject(t)
	p.revertedBranch("cycle-9001")

	p.gcRepeatedly()

	p.requireNoItemFiled("cycle-9001, whose only commit was reverted so its tree equals HEAD")
}

func TestC2383_002_ARevertedBranchBehindAMovedMainContributesNothingAndGcFilesNoItem(t *testing.T) {
	p := newProject(t)
	p.revertedBranch("cycle-9002")
	p.advanceMain("later-main-work.txt")

	p.gcRepeatedly()

	p.requireNoItemFiled("cycle-9002, which adds nothing that main lacks")
}

func TestC2383_003_ABranchWithOnlyAnEmptyCommitHasNoNetDiffAndGcFilesNoItem(t *testing.T) {
	p := newProject(t)
	p.repo.Git("checkout", "-q", "-b", "cycle-9003")
	p.repo.Git("commit", "-q", "--allow-empty", "-m", "empty work")
	p.repo.Git("checkout", "-q", "main")

	p.gcRepeatedly()

	p.requireNoItemFiled("cycle-9003, whose only commit is empty")
}

func TestC2383_004_AWorktreeBranchWithNoNetDiffIsNotFiledAfterRepeatedRuns(t *testing.T) {
	p := newProject(t)
	dir := filepath.Join(p.repo.Dir, ".evolve", "worktrees", "cycle-9004")
	p.repo.Git("worktree", "add", "-q", "-b", "cycle-9004", dir)
	if err := os.WriteFile(filepath.Join(dir, commitFile), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.repo.Git("-C", dir, "add", commitFile)
	p.repo.Git("-C", dir, "commit", "-qm", "add")
	p.repo.Git("-C", dir, "revert", "--no-edit", "HEAD")

	p.gcRepeatedly()

	p.requireNoItemFiled("cycle-9004, a worktree branch whose tree equals HEAD")
}

func TestC2383_005_ABranchWhoseContentMainAlreadyHoldsFilesNoItem(t *testing.T) {
	p := newProject(t)
	p.commitOn("cycle-9005", "landed.txt", "same bytes\n")
	p.write("landed.txt", "same bytes\n")
	p.repo.Git("add", "landed.txt")
	p.repo.Git("commit", "-qm", "land the same content by another route")

	p.gcRepeatedly()

	p.requireNoItemFiled("cycle-9005, whose content is already on main")
}

func TestC2383_006_ABranchWithRealUnlandedWorkStaysFlaggedAndIsFiledOnlyOnTheSecondRunAndIsNeverDeleted(t *testing.T) {
	p := newProject(t)
	p.commitOn("cycle-9006", unlandedFile, "real work\n")
	tip := p.repo.Git("rev-parse", "cycle-9006")

	p.gc()
	if filed := p.inboxFiled(); len(filed) != 0 {
		t.Fatalf("RED: the first gc run filed %v; a flag must be seen twice first", filed)
	}
	p.gc()
	p.gc()

	filed := p.inboxFiled()
	if len(filed) != 1 || !strings.HasPrefix(filed[0].ID, "gc-flag-unmerged-cycle-9006-") {
		t.Fatalf("RED: filed = %v, want exactly one gc-flag-unmerged item for cycle-9006", filed)
	}
	if !p.branchExists("cycle-9006") || p.repo.Git("rev-parse", "cycle-9006") != tip {
		t.Errorf("RED: gc deleted or moved the unlanded branch cycle-9006")
	}
}

type supplyJSON struct {
	Data struct {
		Counts   map[string]int `json:"counts"`
		Refusals []struct {
			ID    string `json:"id"`
			Class string `json:"class"`
		} `json:"refusals"`
	} `json:"data"`
}

func (p project) supply() supplyJSON {
	p.t.Helper()
	stdout, stderr, code := p.run("wave", "supply", "--json", "--project-root", p.repo.Dir)
	var s supplyJSON
	if err := json.Unmarshal([]byte(stdout), &s); err != nil || code != 0 {
		p.t.Fatalf("evolve wave supply --json: exit=%d err=%v\n--- stdout ---\n%s--- stderr ---\n%s", code, err, stdout, stderr)
	}
	return s
}

func TestC2383_007_EveryItemGcFilesAboutABranchIsRefusedAsNonLaneAdmissibleByWaveSupply(t *testing.T) {
	p := newProject(t)
	p.commitOn("cycle-9007", unlandedFile, "orphan work\n")
	p.unlandedWorktree("cycle-9008")

	p.gcRepeatedly()

	filed := p.inboxFiled()
	if len(filed) < 2 {
		t.Fatalf("RED: gc filed %v, want one item for the orphan branch and one for the worktree branch", filed)
	}
	supply := p.supply()
	if supply.Data.Counts["admissible"] != 0 {
		t.Errorf("RED: wave supply counts %d admissible item(s), want 0: %+v", supply.Data.Counts["admissible"], supply)
	}
	refused := map[string]string{}
	for _, r := range supply.Data.Refusals {
		refused[r.ID] = r.Class
	}
	for _, it := range filed {
		if class := refused[it.ID]; !strings.HasPrefix(class, "console_") {
			t.Errorf("RED: item %s is not console-owned in wave supply: class=%q, supply=%+v", it.ID, class, supply)
		}
	}
}

package ship

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

const (
	githubServerError = shiptest.GithubServerError
	githubPolicyError = shiptest.GithubPolicyError
	landingCycle      = shiptest.LandingCycle
	landingRunID      = shiptest.LandingRunID
	landingBranch     = shiptest.LandingBranch
	landingItemID     = shiptest.LandingItemID
	landingItem       = shiptest.LandingItem
)

type (
	pushOutage = shiptest.PushOutage
	sleepLog   = shiptest.SleepLog
)

var (
	mustWrite                 = shiptest.MustWrite
	mustMkdir                 = shiptest.MustMkdir
	tempRepoDir               = shiptest.TempRepoDir
	makeRepo                  = shiptest.MakeRepo
	addRemote                 = shiptest.AddRemote
	makeWorktree              = shiptest.MakeWorktree
	runGit                    = shiptest.RunGit
	runGitOut                 = shiptest.RunGitOut
	filteredEnv               = shiptest.FilteredEnv
	headSHA                   = shiptest.HeadSHA
	remoteHeadSHA             = shiptest.RemoteHeadSHA
	mustHashFile              = shiptest.MustHashFile
	treeStateSHA              = shiptest.TreeStateSHA
	pushOnTopOf               = shiptest.PushOnTopOf
	skipWhenRoot              = shiptest.SkipWhenRoot
	leaseHeld                 = shiptest.LeaseHeld
	writeStrictAuditPolicy    = shiptest.WriteStrictAuditPolicy
	mustShipErr               = shiptest.MustShipErr
	wantShipErr               = shiptest.WantShipErr
	mergedUnpushedFixture     = shiptest.MergedUnpushedFixture
	seedAudit                 = shiptest.SeedAudit
	seedAuditWithBoundTree    = shiptest.SeedAuditWithBoundTree
	sealTestPredicateEvidence = shiptest.SealTestPredicateEvidence
	predicateVerdictFixture   = shiptest.PredicateVerdictFixture
	readIntentAt              = shiptest.ReadIntentAt
)

func containsLog(res RunResult, substr string) bool {
	return shiptest.ContainsLog(res.Logs, substr)
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

func emptyRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	return gittest.FromTemplate(t, "ship-empty-repo", func(*gittest.Repo) error { return nil })
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

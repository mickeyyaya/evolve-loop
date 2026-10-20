package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const transientShipError = `{"code":"GIT_FLEET_REBASE_NEEDED","class":"transient","message":"lane lost the landing race"}`

func shipRanAndFailed(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, "ship-error.json", transientShipError)
	return ws
}

func writeWorkspaceFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func plantBinding(t *testing.T, dir string, b dossier.ShipBinding) {
	t.Helper()
	body, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal binding: %v", err)
	}
	writeWorkspaceFile(t, dir, dossier.ShipBindingFile, string(body))
}

func landedRepo(t *testing.T) (root, commit, tree string) {
	t.Helper()
	root = t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "ci@example.com")
	runGit(t, root, "config", "user.name", "ci")
	runGit(t, root, "config", "commit.gpgsign", "false")
	writeWorkspaceFile(t, root, "base.txt", "main before the cycle started in "+root+"\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "base")
	writeWorkspaceFile(t, root, "delivered.txt", "the work this cycle landed in "+root+"\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "landed")
	return root, gitRev(t, root, "HEAD"), gitRev(t, root, "HEAD^{tree}")
}

func gitRev(t *testing.T, dir, rev string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse %s in %s: %v\n%s", rev, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

const fortyF = "ffffffffffffffffffffffffffffffffffffffff"
const fortyA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDetectLostLanding_PlantedShipBindingDoesNotClearTheFloor(t *testing.T) {
	t.Parallel()

	repo, commit, tree := landedRepo(t)
	const cycle = 1686

	planted := []struct {
		name    string
		binding dossier.ShipBinding
		why     string
	}{
		{
			"commit-does-not-exist",
			dossier.ShipBinding{CommitSHA: fortyF, TreeSHACommitted: fortyA, Cycle: cycle},
			"the cheapest forge: invent a plausible commit id and a plausible tree",
		},
		{
			"real-commit-tree-does-not-match",
			dossier.ShipBinding{CommitSHA: commit, TreeSHACommitted: fortyA, Cycle: cycle},
			"naming a commit that DOES exist is not enough; the binding must be internally true",
		},
		{
			"tree-id-passed-off-as-the-commit",
			dossier.ShipBinding{CommitSHA: tree, TreeSHACommitted: tree, Cycle: cycle},
			"`<tree>^{tree}` resolves to the tree itself, so peeling without ^{commit} would have matched this against itself",
		},
		{
			"ref-name-smuggled-in-as-the-commit",
			dossier.ShipBinding{CommitSHA: "HEAD", TreeSHACommitted: tree, Cycle: cycle},
			"a ref turns the cycle-owned question into the repo-wide one the floor exists to avoid",
		},
		{
			"branch-name-smuggled-in-as-the-commit",
			dossier.ShipBinding{CommitSHA: "main", TreeSHACommitted: tree, Cycle: cycle},
			"same forge as HEAD, through whatever the fleet's last lander left on the branch",
		},
		{
			"an-old-commit-already-on-main-before-the-cycle",
			dossier.ShipBinding{CommitSHA: preCycle(t, repo), TreeSHACommitted: gitRev(t, repo, preCycle(t, repo)+"^{tree}"), Cycle: cycle},
			"any commit on main and its tree are free to copy; only a commit that landed after the cycle started is this cycle's",
		},
		{
			"a-real-commit-and-tree-that-never-landed",
			unlandedCommit(t, repo, cycle),
			"an agent can commit in its own worktree, which shares the object store; only a commit on the project's HEAD landed",
		},
		{
			"abbreviated-commit",
			dossier.ShipBinding{CommitSHA: commit[:12], TreeSHACommitted: tree, Cycle: cycle},
			"an abbreviation is ambiguous and, at short lengths, indistinguishable from a ref name",
		},
	}

	for _, p := range planted {
		t.Run(p.name, func(t *testing.T) {
			ws := shipRanAndFailed(t)
			plantBinding(t, ws, p.binding)
			sig := detectLostLanding(landingEvidence{
				Workspace: ws, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
			})
			if sig == nil {
				t.Fatalf("a planted binding cleared the INTEGRITY floor (%s) — this is defect d22c3331: %s",
					p.name, p.why)
			}
			if sig.Category != "landing-lost" {
				t.Fatalf("category = %q, want landing-lost", sig.Category)
			}
		})
	}
}

func TestDetectLostLanding_ValidatedBindingClearsTheFloor(t *testing.T) {
	t.Parallel()

	repo, commit, tree := landedRepo(t)
	const cycle = 1686

	honest := map[string]dossier.ShipBinding{
		"tree_sha_committed":   {CommitSHA: commit, TreeSHACommitted: tree, Cycle: cycle},
		"audit_bound_tree_sha": {CommitSHA: commit, AuditBoundTreeSHA: tree, Cycle: cycle},
	}
	for field, binding := range honest {
		t.Run(field, func(t *testing.T) {
			ws := shipRanAndFailed(t)
			plantBinding(t, ws, binding)
			if sig := detectLostLanding(landingEvidence{
				Workspace: ws, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
			}); sig != nil {
				t.Fatalf("a binding whose commit and tree git confirms IS a landing; flagging it makes every real"+
					" contended-but-landed cycle a false alarm: %+v", sig)
			}
		})
	}
}

func TestDetectLostLanding_CorruptBindingIsNotAWitness(t *testing.T) {
	t.Parallel()

	repo, commit, _ := landedRepo(t)
	const cycle = 1686

	corrupt := map[string]string{
		"empty-file":          "",
		"truncated-json":      `{"commit_sha":"` + commit,
		"not-json-at-all":     "landed, trust me",
		"json-but-not-object": `["landed"]`,
		"no-fields":           `{}`,
		"empty-commit":        `{"commit_sha":"","tree_sha_committed":"","cycle":1686}`,
		"commit-but-no-tree":  `{"commit_sha":"` + commit + `","cycle":1686}`,
		"null":                `null`,
	}
	for name, body := range corrupt {
		t.Run(name, func(t *testing.T) {
			ws := shipRanAndFailed(t)
			writeWorkspaceFile(t, ws, dossier.ShipBindingFile, body)
			if sig := detectLostLanding(landingEvidence{
				Workspace: ws, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
			}); sig == nil {
				t.Fatalf("%s is not proof of a landing; an unreadable claim must leave the floor up", name)
			}
		})
	}
}

func TestDetectLostLanding_ShippedViaBuildExitIsNotFlagged(t *testing.T) {
	t.Parallel()

	for _, verdict := range []string{CycleOutcomeShippedViaBuild, VerdictPASS} {
		t.Run(verdict, func(t *testing.T) {
			ws := shipRanAndFailed(t)
			if sig := detectLostLanding(landingEvidence{
				Workspace: ws, Cycle: 1686, Shipped: true, FinalVerdict: verdict,
			}); sig != nil {
				t.Fatalf("the host latched this cycle's own ship PASS; flagging it invents a landing loss: %+v", sig)
			}
			if sig := detectLostLanding(landingEvidence{
				Workspace: ws, Cycle: 1686, Shipped: false, FinalVerdict: verdict,
			}); sig == nil {
				t.Fatalf("without the latch this workspace has no witness at all and must be flagged")
			}
		})
	}
}

func TestDetectLostLanding_CleanNoOpExitIsNotFlagged(t *testing.T) {
	t.Parallel()

	for _, shipped := range []bool{true, false} {
		for _, verdict := range []string{CycleOutcomeShippedViaBuild, VerdictPASS} {
			if sig := detectLostLanding(landingEvidence{
				Workspace: t.TempDir(), Cycle: 1686, Shipped: shipped, FinalVerdict: verdict,
			}); sig != nil {
				t.Fatalf("a clean no-op exit (no binding, no ship-error; shipped=%v verdict=%s) lost no landing: %+v",
					shipped, verdict, sig)
			}
		}
	}
}

func TestDetectLostLanding_SiblingCycleBindingDoesNotCreditThisCycle(t *testing.T) {
	t.Parallel()

	ws := realCycleWorkspace(t, "cycle-1536")

	if sig := detectLostLanding(landedOnThisRepo(t, ws, 1535)); sig == nil {
		t.Fatalf("a sibling lane's valid binding sitting in this workspace is not THIS cycle's landing;" +
			" crediting it is the cycle-1535/1536 confusion under a new name")
	}
	if sig := detectLostLanding(landedOnThisRepo(t, ws, 1536)); sig != nil {
		t.Fatalf("the same binding IS cycle-1536's own landing and must clear the floor: %+v", sig)
	}
}

func TestDetectLostLanding_WitnessNeverConsultsRepoHEAD(t *testing.T) {
	t.Parallel()

	repo, commit, tree := landedRepo(t)
	const cycle = 1686

	landed := shipRanAndFailed(t)
	plantBinding(t, landed, dossier.ShipBinding{CommitSHA: commit, TreeSHACommitted: tree, Cycle: cycle})
	lost := shipRanAndFailed(t)

	verdicts := func(stage string) {
		t.Helper()
		if sig := detectLostLanding(landingEvidence{
			Workspace: landed, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
		}); sig != nil {
			t.Fatalf("%s: a cycle with its own validated binding must stay clean: %+v", stage, sig)
		}
		if sig := detectLostLanding(landingEvidence{
			Workspace: lost, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
		}); sig == nil {
			t.Fatalf("%s: a cycle with no witness of its own must stay flagged — a sibling's commit on HEAD"+
				" is not this cycle's landing", stage)
		}
	}

	verdicts("before HEAD moves")

	writeWorkspaceFile(t, repo, "sibling.txt", "another lane's work\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "a sibling lane landed")
	if moved := gitRev(t, repo, "HEAD"); moved == commit {
		t.Fatalf("setup: HEAD did not move")
	}

	verdicts("after a sibling moved HEAD")
}

func TestDetectLostLanding_HostileActiveWorktreeCannotRedirectTheWitness(t *testing.T) {
	t.Parallel()

	repo, commit, tree := landedRepo(t)
	const cycle = 1686
	honest := dossier.ShipBinding{CommitSHA: commit, TreeSHACommitted: tree, Cycle: cycle}

	decoy := t.TempDir()
	plantBinding(t, decoy, honest)

	for _, hostile := range []string{
		decoy,
		filepath.Join(decoy, "."),
		filepath.Join("..", filepath.Base(decoy)),
		"",
	} {
		ws := shipRanAndFailed(t)
		writeWorkspaceFile(t, ws, "cycle-state.json",
			`{"cycle_id":1686,"active_worktree":`+quote(t, hostile)+`}`)
		if sig := detectLostLanding(landingEvidence{
			Workspace: ws, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
		}); sig == nil {
			t.Fatalf("an active_worktree of %q redirected the witness — that is defect d6ed7e1d returning:"+
				" the floor must read the witness from the HOST's workspace argument and nothing else", hostile)
		}
	}

	ws := shipRanAndFailed(t)
	plantBinding(t, ws, honest)
	if sig := detectLostLanding(landingEvidence{
		Workspace: ws, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: cycle, FinalVerdict: VerdictPASS,
	}); sig != nil {
		t.Fatalf("control: the same binding in the host's own workspace must be a witness: %+v", sig)
	}
}

func quote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("quote %q: %v", s, err)
	}
	return string(b)
}

func TestFinalizeCycle_ShipLatchAndProjectRootReachTheFloor(t *testing.T) {
	t.Parallel()

	repo, commit, tree := landedRepo(t)
	const cycle = 1686

	finalize := func(t *testing.T, cs CycleState, projectRoot string) *CycleResult {
		t.Helper()
		o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
		result := &CycleResult{FinalVerdict: VerdictPASS}
		if _, err := o.finalizeCycle(context.Background(), cs, cycle, preCycle(t, repo), projectRoot, result, &State{}, nil); err != nil {
			t.Fatalf("finalizeCycle: %v", err)
		}
		return result
	}

	latched := finalize(t, CycleState{WorkspacePath: shipRanAndFailed(t), Shipped: true}, repo)
	if latched.FinalVerdict != VerdictPASS || latched.SystemFailure != nil {
		t.Fatalf("the ship latch is not reaching the floor through finalizeCycle; got verdict=%q sysfail=%+v",
			latched.FinalVerdict, latched.SystemFailure)
	}

	ws := shipRanAndFailed(t)
	plantBinding(t, ws, dossier.ShipBinding{CommitSHA: commit, TreeSHACommitted: tree, Cycle: cycle})

	inRepo := finalize(t, CycleState{WorkspacePath: ws}, repo)
	if inRepo.FinalVerdict != VerdictPASS || inRepo.SystemFailure != nil {
		t.Fatalf("a binding confirmed by the project repo must close out untouched; got verdict=%q sysfail=%+v",
			inRepo.FinalVerdict, inRepo.SystemFailure)
	}

	elsewhere, _, _ := landedRepo(t)
	foreign := finalize(t, CycleState{WorkspacePath: ws}, elsewhere)
	if foreign.SystemFailure == nil || foreign.SystemFailure.Category != "landing-lost" {
		t.Fatalf("a commit no reachable object store has is not a witness, so projectRoot must be threaded;"+
			" got verdict=%q sysfail=%+v", foreign.FinalVerdict, foreign.SystemFailure)
	}
}

func unlandedCommit(t *testing.T, repo string, cycle int) dossier.ShipBinding {
	t.Helper()
	runGit(t, repo, "checkout", "-q", "-b", "lane-only")
	writeWorkspaceFile(t, repo, "lane.txt", "work that never reached the project HEAD\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "lane only")
	b := dossier.ShipBinding{CommitSHA: gitRev(t, repo, "HEAD"), TreeSHACommitted: gitRev(t, repo, "HEAD^{tree}"), Cycle: cycle}
	runGit(t, repo, "checkout", "-q", "-")
	return b
}

func preCycle(t *testing.T, repo string) string {
	t.Helper()
	return gitOut(t, repo, "rev-list", "--max-parents=0", "HEAD")
}

const cycle1536Commit = "adcbddb28cfef90ca46ac7f24b3f6ceb84e5472f"

func landedOnThisRepo(t *testing.T, workspace string, cycle int) landingEvidence {
	t.Helper()
	root := gitRev(t, ".", "--show-toplevel")
	return landingEvidence{Workspace: workspace, ProjectRoot: root, PreCycleHEAD: gitRev(t, root, cycle1536Commit+"^"), Cycle: cycle, FinalVerdict: VerdictPASS}
}

func TestBindingWitnessesLanding_NeedsTheHostsProjectRootAndAResolvablePreCycleHead(t *testing.T) {
	t.Parallel()

	repo, commit, tree := landedRepo(t)
	ws := shipRanAndFailed(t)
	plantBinding(t, ws, dossier.ShipBinding{CommitSHA: commit, TreeSHACommitted: tree, Cycle: 1686})
	for name, ev := range map[string]landingEvidence{
		"no project root":           {Workspace: ws, PreCycleHEAD: preCycle(t, repo), Cycle: 1686, FinalVerdict: VerdictPASS},
		"no pre-cycle head":         {Workspace: ws, ProjectRoot: repo, Cycle: 1686, FinalVerdict: VerdictPASS},
		"an unknown pre-cycle head": {Workspace: ws, ProjectRoot: repo, PreCycleHEAD: fortyF, Cycle: 1686, FinalVerdict: VerdictPASS},
	} {
		if sig := detectLostLanding(ev); sig == nil {
			t.Errorf("%s: a binding the host cannot bound to this cycle cleared the floor", name)
		}
	}
	if sig := detectLostLanding(landingEvidence{Workspace: ws, ProjectRoot: repo, PreCycleHEAD: preCycle(t, repo), Cycle: 1686, FinalVerdict: VerdictPASS}); sig != nil {
		t.Fatalf("control: the same binding with the host's inputs is a witness: %+v", sig)
	}
	real := landedOnThisRepo(t, realCycleWorkspace(t, "cycle-1536"), 1536)
	real.ProjectRoot = ""
	if sig := detectLostLanding(real); sig == nil {
		t.Error("with no project root the witness read the process working directory, which holds the commit")
	}
}

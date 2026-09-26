package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

const (
	unwindCycle = 42
	unwindRunID = "run-42"
	inboxItem   = ".evolve/inbox/item.json"
	itemBody    = "{\"id\":\"item\"}\n"
)

// shippedLane is a lane in the shape a fleet ship leaves when main moved under it: the Builder's change
// and document on the audited base, audited as auditedTree, committed by ship (with its inbox
// consumption unless built by newCommittedLane), while a peer landed newBase on main.
type shippedLane struct {
	t                         *testing.T
	root, worktree, workspace string
	base, auditedTree         string
	shipCommit, newBase       string
	doc, lanePath             string
}

type laneEdit func(write func(rel, body string))

func addLaneFile(write func(rel, body string)) { write("lane.txt", "lane change\n") }
func addPeerFile(write func(rel, body string)) { write("peer.txt", "peer change\n") }

func editSharedLine(line int, body string) laneEdit {
	return func(write func(rel, body string)) { write("shared.txt", sharedText(line, body)) }
}

func sharedText(line int, body string) string {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "line " + string(rune('a'+i))
	}
	if line > 0 {
		lines[line-1] = body
	}
	return strings.Join(lines, "\n") + "\n"
}

func newShippedLane(t *testing.T, lanePath string, lane, peer laneEdit) *shippedLane {
	t.Helper()
	return newLane(t, lanePath, lane, peer, true)
}

// newCommittedLane is a shipped lane whose ship commit carries no inbox consumption.
func newCommittedLane(t *testing.T, lanePath string, lane, peer laneEdit) *shippedLane {
	t.Helper()
	return newLane(t, lanePath, lane, peer, false)
}

func newLane(t *testing.T, lanePath string, lane, peer laneEdit, consumes bool) *shippedLane {
	t.Helper()
	root := t.TempDir()
	fx := &shippedLane{t: t, root: root, worktree: filepath.Join(root, "worktree"), workspace: filepath.Join(root, ".evolve", "runs", "cycle-42")}
	for _, dir := range []string{fx.worktree, fx.workspace} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fx.git("init", "-q")
	fx.git("config", "user.email", "t@example.com")
	fx.git("config", "user.name", "test")
	fx.git("config", "commit.gpgsign", "false")
	fx.git("checkout", "-q", "-b", "main")
	fx.write("base.txt", "base\n")
	fx.write("shared.txt", sharedText(0, ""))
	fx.write(inboxItem, itemBody)
	fx.commitAll("base")
	fx.base = fx.git("rev-parse", "HEAD")
	fx.git("checkout", "-q", "-b", "cycle")

	fx.lanePath = lanePath
	fx.build(lanePath, lane)
	fx.git("add", "-A")
	fx.auditedTree = fx.git("write-tree")

	if consumes {
		fx.write(".evolve/inbox/consumed/item.json", "{\"id\":\"item\",\"consumed_by\":\"cycle-42\"}\n")
		if err := os.Remove(filepath.Join(fx.worktree, inboxItem)); err != nil {
			t.Fatal(err)
		}
	}
	fx.commitAll("ship cycle 42")
	fx.shipCommit = fx.git("rev-parse", "HEAD")

	fx.git("checkout", "-q", "main")
	peer(fx.write)
	fx.commitAll("peer")
	fx.newBase = fx.git("rev-parse", "HEAD")
	fx.git("checkout", "-q", "cycle")
	return fx
}

// build is the Builder: it changes lanePath, authors the explanation and seals the approved result.
func (fx *shippedLane) build(lanePath string, lane laneEdit) {
	t := fx.t
	activation := fx.bindingAt("")
	activation.Worktree = ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	binding := fx.bindingAt(fx.base)
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	lane(fx.write)
	doc, err := explanationdocs.DocumentPath(unwindCycle, unwindRunID)
	if err != nil {
		t.Fatal(err)
	}
	fx.doc = doc
	fx.write(doc, "# Build Explanation\n\n## Build Binding\n- Cycle: 42\n- Base SHA: "+fx.base+"\n\n"+
		"## Summary\nThe lane behavior changes through one focused source file.\n\n"+
		"## Rationale\nThe direct implementation is the smallest compatible change and adds no configuration surface.\n\n"+
		"## Changed Areas\n- `"+lanePath+"` — carries the lane behavior introduced by this cycle.\n\n"+
		"## Design Decisions\nThe existing file layout remains the single source of behavior.\n\n"+
		"## Verification\nThe recovery tests drive the complete ship-recovery lifecycle.\n\n"+
		"## Compatibility\nNo public API changes.\n\n## Limitations\nNo migration is included.\n")
	if err := os.WriteFile(filepath.Join(fx.workspace, "build-report.md"), []byte("## Explanation Documentation\n- Status: REQUIRED\n- Document: "+doc+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
}

func (fx *shippedLane) git(args ...string) string {
	fx.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", fx.worktree}, args...)...).CombinedOutput()
	if err != nil {
		fx.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (fx *shippedLane) write(rel, body string) {
	fx.t.Helper()
	path := filepath.Join(fx.worktree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fx.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *shippedLane) commitAll(message string) {
	fx.git("add", "-A")
	fx.git("commit", "-q", "-m", message)
}

// landOnMain lands a later peer commit on main from a second worktree, as a peer lane landing after the
// rebase would, and returns it.
func (fx *shippedLane) landOnMain(rel, body string) string {
	t := fx.t
	t.Helper()
	peer := filepath.Join(fx.root, "peer")
	fx.git("worktree", "add", "-q", peer, "main")
	if err := os.WriteFile(filepath.Join(peer, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", peer}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("add", "-A")
	run("commit", "-q", "-m", "later peer")
	return run("rev-parse", "HEAD")
}

func (fx *shippedLane) bindingAt(base string) explanationdocs.CycleBinding {
	return explanationdocs.CycleBinding{
		ProjectRoot: fx.root, Worktree: fx.worktree, Workspace: fx.workspace, BaseSHA: base,
		Cycle: unwindCycle, RunID: unwindRunID, ContractVersion: explanationdocs.CurrentContractVersion,
	}
}

func (fx *shippedLane) cycleState() CycleState {
	return CycleState{
		CycleID: unwindCycle, RunID: unwindRunID, WorkspacePath: fx.workspace, ActiveWorktree: fx.worktree,
		WorktreeBaseSHA: fx.base, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}
}

func (fx *shippedLane) auditRow() LedgerEntry {
	return LedgerEntry{Cycle: unwindCycle, RunID: unwindRunID, Role: "auditor", Kind: "agent_subprocess", WorktreeTreeSHA: fx.auditedTree}
}

func (fx *shippedLane) recover(storage *fakeStorage, rows ...LedgerEntry) (Phase, bool, CycleState) {
	o := NewOrchestrator(storage, &fakeLedger{entries: rows}, buildRunners(nil))
	cs := fx.cycleState()
	next, recovering := o.recoverFromShipError(context.Background(), fx.root, unwindCycle, &cs,
		NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 0, 2)
	return next, recovering, cs
}

// requirePendingLaneChangeOn asserts HEAD is base and exactly the audited lane change is pending on it, with
// ship's consumption undone so the next ship consumes the item again.
func (fx *shippedLane) requirePendingLaneChangeOn(base string) {
	t := fx.t
	t.Helper()
	if head := fx.git("rev-parse", "HEAD"); head != base {
		t.Fatalf("HEAD = %s, want %s", head, base)
	}
	staged := strings.Fields(fx.git("diff", "--cached", "--name-only"))
	if want := []string{fx.doc, fx.lanePath}; strings.Join(staged, " ") != strings.Join(want, " ") {
		t.Fatalf("pending change = %v, want exactly the audited lane change %v", staged, want)
	}
	if body, err := os.ReadFile(filepath.Join(fx.worktree, inboxItem)); err != nil || string(body) != itemBody {
		t.Fatalf("inbox item not back at its audited bytes: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(fx.worktree, ".evolve/inbox/consumed/item.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ship's consumption survived the unwind: %v", err)
	}
}

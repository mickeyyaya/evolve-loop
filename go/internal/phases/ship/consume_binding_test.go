//go:build integration

package ship

// consume_binding_test.go — RED contract for the cycle-1506 pipeline-blocker
// halt (batch-20260817b): #466's in-commit consumption mutates the staged tree
// AFTER the audit bound it, so the pre-commit tree-drift integrity check
// refused EVERY PASS ship of an inbox-claimed item — two individually-correct
// mechanisms, jointly contradictory. (#466's own tests passed because they set
// no audit binding, and the check self-skips.) The fix: a drift whose tree
// delta consists EXACTLY of the sanctioned consumption moves is accepted with
// a loud log; any other path in the delta still refuses. The integrity
// guarantee is not weakened — it is taught about the one mutation the ship
// itself performs by design.

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// preConsumptionTree stages everything in wt and returns its write-tree —
// the tree the audit would have bound (consumption has not run yet).
func preConsumptionTree(t *testing.T, wt string) string {
	t.Helper()
	add := exec.Command("git", "-C", wt, "add", "-A")
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add -A: %v\n%s", err, out)
	}
	wtree := exec.Command("git", "-C", wt, "write-tree")
	out, err := wtree.CombinedOutput()
	if err != nil {
		t.Fatalf("git write-tree: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// The cycle-1506 shape: audit binding set to the pre-consumption tree; the
// ship consumes its item; the resulting drift is exactly the consumption move
// and MUST be accepted.
func TestShipFromWorktree_ConsumptionDriftIsSanctioned(t *testing.T) {
	repo, wt, ws, _ := consumeScenario(t, "PASS")
	bound := preConsumptionTree(t, wt)
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	opts.internalAuditBoundTreeSHA = bound
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("a drift consisting solely of the ship's own sanctioned consumption must not refuse the commit (cycle-1506 class): %v", err)
	}
	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "consumption") || !strings.Contains(logs, "sanctioned") {
		t.Errorf("the accepted drift must be LOUDLY attributed to consumption in the ship logs: %v", res.Logs)
	}
	files := commitFileList(t, wt, "cycle-1")
	if !strings.Contains(files, "consumed/2026-08-15T03-00-00Z-fix-the-widget.json") {
		t.Fatalf("the ship commit must still carry the consumption; files=%q", files)
	}
}

// The integrity control: an UNSANCTIONED extra file in the drift still refuses
// — the tolerance is exactly the consumption set, nothing wider.
func TestShipFromWorktree_UnsanctionedDriftStillRefuses(t *testing.T) {
	repo, wt, ws, _ := consumeScenario(t, "PASS")
	bound := preConsumptionTree(t, wt)
	// A post-audit smuggle, STAGED so it is guaranteed to be in the tree the
	// pre-commit check verifies (whether ship's own staging would pick an
	// undeclared untracked file up is a separate policy — this control pins
	// the drift check itself).
	if err := os.WriteFile(filepath.Join(wt, "smuggled.go"), []byte("package smuggled\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "-C", wt, "add", "smuggled.go")
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("stage smuggle: %v\n%s", err, out)
	}
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	opts.internalAuditBoundTreeSHA = bound
	res := &RunResult{}
	err := shipFromWorktree(context.Background(), opts, res, "main", wt)
	if err == nil {
		t.Fatal("a drift containing an unsanctioned path must still refuse — the consumption tolerance must not become a smuggling channel")
	}
	if !strings.Contains(err.Error(), "INTEGRITY") {
		t.Fatalf("refusal must remain the integrity class: %v", err)
	}
}

// Review M3: a SECOND, unconsumed inbox file staged post-binding must refuse —
// pins that the tolerance is the exact consumption set, never an
// .evolve/inbox/ prefix.
func TestShipFromWorktree_SecondUnconsumedInboxFileRefuses(t *testing.T) {
	repo, wt, ws, _ := consumeScenario(t, "PASS")
	bound := preConsumptionTree(t, wt)
	if err := os.WriteFile(filepath.Join(wt, ".evolve/inbox/2026-08-17T00-00-00Z-planted.json"),
		[]byte(`{"id":"planted","title":"not consumed by this cycle"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "-C", wt, "add", ".evolve/inbox/2026-08-17T00-00-00Z-planted.json")
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("stage planted item: %v\n%s", err, out)
	}
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	opts.internalAuditBoundTreeSHA = bound
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err == nil {
		t.Fatal("an inbox-path drift that is NOT this cycle's consumption must still refuse — exact set, not prefix")
	}
}

// Review HIGH: an item TAMPERED after audit binding must not be sanctioned —
// consumption refuses the move (rolled back, pickable), the ship proceeds
// WITHOUT it, and nothing unaudited rides the commit.
func TestShipFromWorktree_TamperedItemIsNotSanctioned(t *testing.T) {
	repo, wt, ws, itemRel := consumeScenario(t, "PASS")
	bound := preConsumptionTree(t, wt)
	if err := os.WriteFile(filepath.Join(wt, filepath.FromSlash(itemRel)),
		[]byte(`{"id":"fix-the-widget","title":"tampered post-binding","fingerprint":"smuggled-ack"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	opts.internalAuditBoundTreeSHA = bound
	res := &RunResult{}
	err := shipFromWorktree(context.Background(), opts, res, "main", wt)
	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "REFUSED sanctioning") {
		t.Fatalf("tampered item must be loudly refused sanctioning; logs=%v err=%v", res.Logs, err)
	}
	if err != nil {
		// The tampered file differs from the bound tree AND was rolled back to
		// its tampered on-disk state — if ship's own staging then carries the
		// tamper into the tree, the drift check must refuse it as unsanctioned.
		if !strings.Contains(err.Error(), "INTEGRITY") {
			t.Fatalf("if the tamper reaches the tree the refusal must be the integrity class: %v", err)
		}
		return
	}
	// Ship succeeded: then the commit must NOT contain a consumed/ record.
	files := commitFileList(t, wt, "cycle-1")
	if strings.Contains(files, "consumed/") {
		t.Fatalf("tampered item must not ship as consumed; files=%q", files)
	}
}

// No binding set: byte-identical legacy behavior (the check self-skips) —
// pinned so the fix cannot accidentally arm the check where it never ran.
func TestShipFromWorktree_NoBindingStillSkipsCheck(t *testing.T) {
	repo, wt, ws, _ := consumeScenario(t, "PASS")
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: widget fixed",
		ProjectRoot:   repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard,
	}
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("no-binding path must stay byte-identical: %v", err)
	}
}

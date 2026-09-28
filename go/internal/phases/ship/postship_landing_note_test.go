package ship

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ledgerLineFor returns the ledger.jsonl lines mentioning taskID.
func ledgerLinesFor(t *testing.T, root, taskID string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger.jsonl: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, `"`+taskID+`"`) {
			out = append(out, line)
		}
	}
	return out
}

// An unlanded ship (merge-base --is-ancestor exit 1) must still release the
// item back to the inbox root AND leave durable per-item evidence — a ledger
// entry for the released item whose reason notes the unlanded ship — so
// triage/operators can tell a delivery failure from an ordinary residual
// drain.
func TestPromoteInbox_UnlandedReleaseCarriesRetryNote(t *testing.T) {
	root := t.TempDir()
	const cid = 752
	const id = "inbox-promotion-requires-landed-ship"
	writeInboxFixture(t, root, cid, id)

	r := landingScriptedRunner(1) // unlanded: not an ancestor of HEAD or origin
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{
		CommitSHA:     "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		RepairOutcome: "needs-reaudit",
	}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	// The item is back at the inbox root, not stranded in processing/ and not
	// buried in processed/.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", id+".json")); err != nil {
		t.Fatalf("unlanded ship must release the item back to the inbox root: %v", err)
	}

	// The release must carry a durable unlanded note.
	lines := ledgerLinesFor(t, root, id)
	if len(lines) == 0 {
		t.Fatalf("no ledger entry recorded for released item %q", id)
	}
	noted := false
	for _, line := range lines {
		if strings.Contains(line, "unlanded") {
			noted = true
			break
		}
	}
	if !noted {
		t.Errorf("released item %q has no unlanded retry note in any ledger entry — an operator cannot distinguish 'ship never landed' from an ordinary residual drain; got: %v", id, lines)
	}
}

// A LANDED cycle whose processing/ dir holds an extra residual claim (not in
// top_n) drains that residual with the ordinary generic reason — the
// unlanded note must NOT appear (guards against a stub that stamps
// "unlanded" on every release).
func TestPromoteInbox_LandedResidualReleaseKeepsGenericReason(t *testing.T) {
	root := t.TempDir()
	const cid = 752
	const id = "inbox-promotion-requires-landed-ship"
	const residual = "unrelated-residual-claim"
	procDir := writeInboxFixture(t, root, cid, id)

	body, _ := json.Marshal(map[string]any{"id": residual, "title": "residual fixture"})
	if err := os.WriteFile(filepath.Join(procDir, residual+".json"), body, 0o644); err != nil {
		t.Fatalf("write residual item: %v", err)
	}

	r := landingScriptedRunner(0) // landed: ancestor of HEAD
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "cafebabecafebabecafebabecafebabecafebabe"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	// The residual (not in top_n) drains back to the inbox root as before.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", residual+".json")); err != nil {
		t.Fatalf("landed cycle must still drain residual claims to the inbox root: %v", err)
	}
	for _, line := range ledgerLinesFor(t, root, residual) {
		if strings.Contains(line, "unlanded") {
			t.Errorf("landed-cycle residual drain must keep the generic release reason, not the unlanded note: %s", line)
		}
	}
}

package ship

// consume_registry_release_test.go — the ship-side wiring proof for the
// transactional retire (park-consume-releases-continuation-binding). Consuming
// an item takes it out of the batch loader's reach, so its scope-keyed
// continuation binding must go in the SAME operation; otherwise the next wave
// mints a lane straight off the immortal binding (cycles 1487, 1497). The
// pointer must survive the release, and an unrelated live lane's binding must
// not be touched.

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// stubRunner accepts every command (git add) without touching a real repo.
func stubRunner(_ context.Context, _ string, _ string, _ []string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) (int, error) {
	return 0, nil
}

// consumeFixture builds a project root whose workspace declares id in top_n
// with a PASS acs verdict, plus an inbox item carrying id.
func consumeFixture(t *testing.T, id string) (root, workspace, itemBase string) {
	t.Helper()
	root = t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-1507")
	for _, d := range []string{inbox, workspace} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("fixture mkdir %s: %v", d, err)
		}
	}
	itemBase = "2026-08-16T15-10-00Z-" + id + ".json"
	item, err := json.MarshalIndent(map[string]any{"id": id, "kind": "bug", "title": "fixture " + id}, "", "  ")
	if err != nil {
		t.Fatalf("fixture marshal item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inbox, itemBase), append(item, '\n'), 0o644); err != nil {
		t.Fatalf("fixture write item: %v", err)
	}
	decision := []byte(`{"top_n":[{"id":"` + id + `"}]}`)
	if err := os.WriteFile(filepath.Join(workspace, "triage-decision.json"), decision, 0o644); err != nil {
		t.Fatalf("fixture write decision: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "acs-verdict.json"), []byte(`{"verdict":"PASS"}`), 0o644); err != nil {
		t.Fatalf("fixture write verdict: %v", err)
	}
	return root, workspace, itemBase
}

// TestConsumeCommittedItems_ReleasesRegistryBinding drives the real
// consumeCommittedItems against an on-disk fixture and asserts all three halves
// of the contract: the consumed id's binding is released, its VALUE is
// preserved into the consumed item file, and an unrelated live scope's binding
// survives untouched.
func TestConsumeCommittedItems_ReleasesRegistryBinding(t *testing.T) {
	const id = "context-fill-telemetry-and-cap"
	const liveSibling = "some-other-live-todo"
	root, workspace, itemBase := consumeFixture(t, id)

	bound := continuation.Continuation{
		Worktree:     "/tmp/evolve/worktrees/cycle-1484",
		Branch:       "cycle-1484",
		SnapshotSHA:  "9813bc621fe4aa0d55e1c0d3f0e1a2b3c4d5e6f7",
		BaseSHA:      "d3c69cd2aa11bb22cc33dd44ee55ff6600112233",
		FindingsPath: ".evolve/runs/cycle-1484/audit-fail-reason.json",
		Cycle:        1484,
	}
	sibling := bound
	sibling.Cycle = 1490
	sibling.SnapshotSHA = "aaaabbbbccccddddeeeeffff0000111122223333"
	for scope, c := range map[string]continuation.Continuation{id: bound, liveSibling: sibling} {
		if err := continuation.WriteRegistryEntry(root, scope, c); err != nil {
			t.Fatalf("fixture bind %s: %v", scope, err)
		}
	}

	opts := &Options{
		Class:         ClassCycle,
		ProjectRoot:   root,
		WorkspacePath: workspace,
		Runner:        stubRunner,
	}
	res := &RunResult{}
	consumeCommittedItems(context.Background(), opts, res, "")

	logs := strings.Join(res.Logs, "\n")
	if _, ok, err := continuation.ReadRegistryEntry(root, id); err != nil || ok {
		t.Errorf("consuming %q left its continuation-registry binding intact (ok=%v err=%v) — the consumed scope re-arms on the next wave\nlogs:\n%s", id, ok, err, logs)
	}
	if _, ok, err := continuation.ReadRegistryEntry(root, liveSibling); err != nil || !ok {
		t.Errorf("consuming %q also released the UNRELATED live scope %q (ok=%v err=%v) — release must be scoped to the consumed item", id, liveSibling, ok, err)
	}

	consumed := filepath.Join(root, ".evolve", "inbox", "consumed", itemBase)
	raw, err := os.ReadFile(consumed)
	if err != nil {
		t.Fatalf("consumed item %s not written: %v\nlogs:\n%s", consumed, err, logs)
	}
	var doc struct {
		Released []continuation.Continuation `json:"released_continuations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("consumed item is not JSON: %v", err)
	}
	if len(doc.Released) != 1 {
		t.Fatalf("consumed item %s has %d released_continuations[] entries, want 1 — the binding pointer (snapshot %s) is lost\n%s", consumed, len(doc.Released), bound.SnapshotSHA, string(raw))
	}
	if got := doc.Released[0]; got.SnapshotSHA != bound.SnapshotSHA || got.Branch != bound.Branch || got.BaseSHA != bound.BaseSHA {
		t.Errorf("released_continuations[0] does not preserve the binding: got %+v want %+v", got, bound)
	}
}

// TestConsumeCommittedItems_NoBindingIsCleanNoOp is the ordinary case: an item
// that never carried preserved work consumes exactly as before, with no
// released_continuations[] key invented on it.
func TestConsumeCommittedItems_NoBindingIsCleanNoOp(t *testing.T) {
	const id = "plain-todo-with-no-binding"
	root, workspace, itemBase := consumeFixture(t, id)

	opts := &Options{Class: ClassCycle, ProjectRoot: root, WorkspacePath: workspace, Runner: stubRunner}
	res := &RunResult{}
	consumeCommittedItems(context.Background(), opts, res, "")

	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "consumed", itemBase))
	if err != nil {
		t.Fatalf("consumed item not written: %v\nlogs:\n%s", err, strings.Join(res.Logs, "\n"))
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("consumed item is not JSON: %v", err)
	}
	if _, present := doc["released_continuations"]; present {
		t.Errorf("an unbound item gained released_continuations[]: %s", string(raw))
	}
}

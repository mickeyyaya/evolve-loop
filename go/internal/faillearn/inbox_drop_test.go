package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inboxFixture(t *testing.T) (inboxDir string, item InboxItem) {
	t.Helper()
	return t.TempDir(), shaped(InboxItem{
		ID: "retro-1255-stale-worktree", Title: "stale cs.ActiveWorktree survives fleet teardown",
		Weight: 0.9, Kind: "bug", Priority: "H", PriorityClass: "correctness", InjectedBy: "faillearn-failure-floor",
	})[0]
}

func shaped(items ...InboxItem) []InboxItem {
	out := make([]InboxItem, len(items))
	for i, it := range items {
		it.Summary = "summary of " + it.ID
		it.Fix = "fix for " + it.ID
		it.Acceptance = []string{"acceptance of " + it.ID}
		out[i] = it
	}
	return out
}

func filedPath(t *testing.T, dir, id string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*-"+id+".json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("inbox %s holds %v for id %q, want exactly one stamped file (err %v)", dir, matches, id, err)
	}
	return matches[0]
}

func TestWriteInboxItems_AlreadyFiledIDIsASkipNotADrop(t *testing.T) {
	dir, item := inboxFixture(t)
	squatter := filepath.Join(dir, "2020-01-01T00-00-00Z-"+item.ID+".json")
	if err := os.WriteFile(squatter, []byte(`{"id":"retro-1255-stale-worktree","title":"someone else's item"}`), 0o644); err != nil {
		t.Fatalf("seed squatter: %v", err)
	}

	if err := (writeConfig{inboxDir: dir, inboxItems: []InboxItem{item}}).writeInboxItems(); err != nil {
		t.Fatalf("a filed id is an idempotent skip, got %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(matches) != 1 {
		t.Errorf("a skip must file nothing new, inbox holds %v", matches)
	}
}

func TestWriteInboxItems_WriterRefusalNamesTheItem(t *testing.T) {
	dir, item := inboxFixture(t)
	item.Weight = 2

	err := writeConfig{inboxDir: dir, inboxItems: []InboxItem{item}}.writeInboxItems()

	if err == nil || !strings.Contains(err.Error(), item.ID) {
		t.Fatalf("a writer-refused item must fail loudly and name its id; got %v", err)
	}
}

func TestWriteInboxItems_IdenticalItemIsIdempotent(t *testing.T) {
	dir, item := inboxFixture(t)
	cfg := writeConfig{inboxDir: dir, inboxItems: []InboxItem{item}}
	if err := cfg.writeInboxItems(); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := cfg.writeInboxItems(); err != nil {
		t.Fatalf("second write of an identical item must be idempotent; got %v", err)
	}
}

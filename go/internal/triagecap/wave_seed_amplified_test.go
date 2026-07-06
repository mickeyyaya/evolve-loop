package triagecap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// wave_seed_amplified_test.go — cycle-541 test-amplification lane for
// ReadInboxBacklog (extracted this cycle as the single inbox-JSON reader
// shared by SelectWaveSeedTopN and WidenTopNToFleetWidth's caller) and its
// SelectWaveSeedTopN regression path. Authored black-box from the exported
// godoc contract (filename-order tie-break; unreadable/malformed files and
// empty-id todos skipped, best-effort) plus the on-disk .evolve/inbox/*.json
// schema (id/weight/files) — not from wave_seed.go's implementation.

type c541ampInboxTodo struct {
	ID     string   `json:"id"`
	Weight float64  `json:"weight,omitempty"`
	Files  []string `json:"files,omitempty"`
}

func c541ampInboxDir(t *testing.T) (evolveDir, inboxDir string) {
	t.Helper()
	evolveDir = t.TempDir()
	inboxDir = filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return evolveDir, inboxDir
}

func c541ampWriteInboxTodo(t *testing.T, inboxDir, filename string, todo c541ampInboxTodo) {
	t.Helper()
	body, err := json.Marshal(todo)
	if err != nil {
		t.Fatalf("marshal inbox fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, filename), body, 0o644); err != nil {
		t.Fatalf("write inbox fixture %s: %v", filename, err)
	}
}

func TestC541Amp_ReadInboxBacklog_MissingEvolveDir(t *testing.T) {
	got := ReadInboxBacklog(filepath.Join(t.TempDir(), "does-not-exist"))
	if len(got) != 0 {
		t.Fatalf("missing evolveDir must yield an empty backlog (best-effort), got %v", got)
	}
}

func TestC541Amp_ReadInboxBacklog_EmptyInboxDir(t *testing.T) {
	evolveDir, _ := c541ampInboxDir(t)
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 0 {
		t.Fatalf("empty inbox dir must yield an empty backlog, got %v", got)
	}
}

func TestC541Amp_ReadInboxBacklog_SkipsMalformedAndNonJSON(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	c541ampWriteInboxTodo(t, inboxDir, "1-valid.json", c541ampInboxTodo{ID: "valid-item", Weight: 0.5, Files: []string{"a.go"}})
	if err := os.WriteFile(filepath.Join(inboxDir, "2-malformed.json"), []byte(`{"id": "broken", not-json`), 0o644); err != nil {
		t.Fatalf("write malformed fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "3-notes.txt"), []byte("not a todo at all"), 0o644); err != nil {
		t.Fatalf("write non-json fixture: %v", err)
	}
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 1 || got[0].ID != "valid-item" {
		t.Fatalf("malformed JSON and non-.json files must be skipped best-effort, got %v", got)
	}
}

func TestC541Amp_ReadInboxBacklog_SkipsEmptyIDTodos(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	c541ampWriteInboxTodo(t, inboxDir, "1-empty-id.json", c541ampInboxTodo{ID: "", Weight: 0.9, Files: []string{"a.go"}})
	c541ampWriteInboxTodo(t, inboxDir, "2-real.json", c541ampInboxTodo{ID: "real-item", Weight: 0.1, Files: []string{"b.go"}})
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 1 || got[0].ID != "real-item" {
		t.Fatalf("empty-id todos must be skipped, got %v", got)
	}
}

func TestC541Amp_ReadInboxBacklog_FilenameOrderTiesEqualWeight(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	// same weight on all three -> order must follow filename, not insertion/id order
	c541ampWriteInboxTodo(t, inboxDir, "b-second.json", c541ampInboxTodo{ID: "second", Weight: 0.5, Files: []string{"b.go"}})
	c541ampWriteInboxTodo(t, inboxDir, "a-first.json", c541ampInboxTodo{ID: "first", Weight: 0.5, Files: []string{"a.go"}})
	c541ampWriteInboxTodo(t, inboxDir, "c-third.json", c541ampInboxTodo{ID: "third", Weight: 0.5, Files: []string{"c.go"}})
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 3 {
		t.Fatalf("want 3 candidates, got %d: %v", len(got), got)
	}
	wantOrder := []string{"first", "second", "third"}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Fatalf("equal-weight todos must tie-break by filename order, want %v got %v", wantOrder, c541ampIDs(got))
		}
	}
}

func TestC541Amp_ReadInboxBacklog_ParsesWeightAndFilesFaithfully(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	c541ampWriteInboxTodo(t, inboxDir, "1.json", c541ampInboxTodo{ID: "full-item", Weight: 0.73, Files: []string{"pkg/a.go", "pkg/b.go"}})
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 1 {
		t.Fatalf("want 1 candidate, got %d", len(got))
	}
	c := got[0]
	if c.ID != "full-item" || c.Weight != 0.73 || len(c.Files) != 2 || c.Files[0] != "pkg/a.go" || c.Files[1] != "pkg/b.go" {
		t.Fatalf("fields must be parsed faithfully, got %+v", c)
	}
}

func TestC541Amp_ReadInboxBacklog_MissingWeightAndFilesFields(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	if err := os.WriteFile(filepath.Join(inboxDir, "1.json"), []byte(`{"id":"bare-item"}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 1 || got[0].ID != "bare-item" || got[0].Weight != 0 || len(got[0].Files) != 0 {
		t.Fatalf("missing weight/files must default to zero-value, not panic: %+v", got)
	}
}

func TestC541Amp_ReadInboxBacklog_SkipsDirectoryMatchingGlob(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	if err := os.MkdirAll(filepath.Join(inboxDir, "oops-a-directory.json"), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	c541ampWriteInboxTodo(t, inboxDir, "1-real.json", c541ampInboxTodo{ID: "real-item", Weight: 0.4, Files: []string{"a.go"}})
	got := ReadInboxBacklog(evolveDir)
	if len(got) != 1 || got[0].ID != "real-item" {
		t.Fatalf("a directory entry matching *.json must be skipped, not panic: %v", got)
	}
}

func TestC541Amp_ReadInboxBacklog_LargeInboxDirectory(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	const n = 200
	for i := 0; i < n; i++ {
		c541ampWriteInboxTodo(t, inboxDir, fmt.Sprintf("%03d.json", i), c541ampInboxTodo{
			ID: fmt.Sprintf("item-%03d", i), Weight: float64(i), Files: []string{fmt.Sprintf("pkg%d/f.go", i)},
		})
	}
	got := ReadInboxBacklog(evolveDir)
	if len(got) != n {
		t.Fatalf("want %d candidates from a large inbox, got %d", n, len(got))
	}
}

func TestC541Amp_SelectWaveSeedTopN_DelegatesToReadInboxBacklogRegression(t *testing.T) {
	evolveDir, inboxDir := c541ampInboxDir(t)
	c541ampWriteInboxTodo(t, inboxDir, "1-high.json", c541ampInboxTodo{ID: "high", Weight: 0.9, Files: []string{"pkg/a.go"}})
	c541ampWriteInboxTodo(t, inboxDir, "2-mid.json", c541ampInboxTodo{ID: "mid", Weight: 0.5, Files: []string{"pkg/b.go"}})
	c541ampWriteInboxTodo(t, inboxDir, "3-colliding.json", c541ampInboxTodo{ID: "colliding", Weight: 0.8, Files: []string{"pkg/a.go"}})

	got := SelectWaveSeedTopN(evolveDir, 2)
	if len(got) != 2 {
		t.Fatalf("want 2 disjoint lane representatives, got %d: %v", len(got), c541ampIDs(got))
	}
	c541ampAllDisjoint(t, got)
	if c541ampContainsID(got, "high") && c541ampContainsID(got, "colliding") {
		t.Fatalf("high and colliding share pkg/a.go and must never both be selected: %v", c541ampIDs(got))
	}
}

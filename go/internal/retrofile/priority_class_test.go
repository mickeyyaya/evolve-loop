package retrofile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestFileActions_StampsTheActionsPriorityClass(t *testing.T) {
	inbox := t.TempDir()
	written, err := FileActions(inbox, 7, []PreventiveAction{{ID: "gate-demoted", Title: "a gate stopped judging", PriorityClass: "correctness"}}, 0.75, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	if err != nil || len(written) != 1 {
		t.Fatalf("FileActions = %v, %v", written, err)
	}
	body, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	var item struct {
		PriorityClass string `json:"priority_class"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	if item.PriorityClass != "correctness" {
		t.Errorf("the filed item carries the class its action names; priority_class = %q", item.PriorityClass)
	}
}

func TestFileActions_RefusesAnActionWithNoPriorityClassButNotAnAlreadyFiledOne(t *testing.T) {
	inbox := t.TempDir()
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	written, err := FileActions(inbox, 7, []PreventiveAction{{ID: "classless", Title: "t"}}, 0.75, now)
	if !errors.Is(err, inboxbatch.ErrNoPriorityClass) || len(written) != 0 {
		t.Fatalf("FileActions = %v, %v; want ErrNoPriorityClass and nothing written", written, err)
	}
	if entries, _ := os.ReadDir(inbox); len(entries) != 0 {
		t.Errorf("a refused action writes no file: %v", entries)
	}
	if err := os.WriteFile(filepath.Join(inbox, "filed.json"), []byte(`{"id":"filed","priority_class":"correctness"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if written, err := FileActions(inbox, 7, []PreventiveAction{{ID: "filed", Title: "t"}}, 0.75, now); err != nil || len(written) != 0 {
		t.Errorf("an already-filed action is deduplicated before the class check: %v, %v", written, err)
	}
}

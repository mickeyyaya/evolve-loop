package ciwatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWatch_ARedRunFilesACorrectnessClassedItem(t *testing.T) {
	red := func(context.Context, string) (RunStatus, error) {
		return RunStatus{Status: StatusCompleted, Conclusion: "failure", FailingTest: "TestX"}, nil
	}
	opts, inbox, _ := watchOpts(t, red)
	if _, err := Watch(context.Background(), opts); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	names := inboxFiles(t, inbox)
	if len(names) != 1 {
		t.Fatalf("inbox files = %v, want one escalation item", names)
	}
	body, err := os.ReadFile(filepath.Join(inbox, names[0]))
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
		t.Errorf("a red run on main is a correctness defect; priority_class = %q", item.PriorityClass)
	}
}

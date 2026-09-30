package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIsMoverWritten_AnswersForTheLifecycleLeaf(t *testing.T) {
	if !IsMoverWritten("route") || !IsMoverWritten("failure_count") || IsMoverWritten("weight") {
		t.Error("IsMoverWritten must answer the leaf's rule: route and lifecycle fields yes, authored fields no")
	}
}

func TestUpdateItemJSON_RewritesAnItemThroughTheFacade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(path, []byte(`{"id":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := UpdateItemJSON(path, func(item map[string]json.RawMessage) { item["route"] = json.RawMessage(`"console-manual"`) })

	if got, _ := os.ReadFile(path); err != nil || string(got) != `{"id":"x","route":"console-manual"}` {
		t.Errorf("UpdateItemJSON = %v, file %s; want the route written as the mover writes items", err, got)
	}
}

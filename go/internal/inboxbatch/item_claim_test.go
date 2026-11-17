package inboxbatch_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestItem_ClaimPathsAreTheDeclaredAndTheProtectedFiles(t *testing.T) {
	var it inboxbatch.Item
	body := `{"id":"x","files":["go/a.go"],"protected_files":["go/internal/guards/p.go"],"landed_sha":"abc1234","routed_by":"loop"}`
	if err := json.Unmarshal([]byte(body), &it); err != nil {
		t.Fatal(err)
	}
	if got := it.ClaimPaths(); !slices.Equal(got, []string{"go/a.go", "go/internal/guards/p.go"}) {
		t.Errorf("ClaimPaths = %v", got)
	}
	if it.LandedSHAText() != "abc1234" || it.RoutedByText() != inboxbatch.RoutedByLoop {
		t.Errorf("item = %+v: landed_sha and routed_by are read", it)
	}
}

func TestItem_AStampThatIsNotTextReadsAsEmpty(t *testing.T) {
	var it inboxbatch.Item
	if err := json.Unmarshal([]byte(`{"id":"x","landed_sha":{"note":"y"},"routed_by":7}`), &it); err != nil {
		t.Fatal(err)
	}
	if it.LandedSHAText() != "" || it.RoutedByText() != "" {
		t.Errorf("item = %+v: a non-text stamp reads as empty", it)
	}
}

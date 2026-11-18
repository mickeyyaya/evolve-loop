package inboxcmd

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

const rerouteFileItem = `{"id":"%s","kind":"bug","origin":"console","priority_class":"correctness","weight":0.5,"title":"t",` +
	`"route":"console-manual","routed_by":"loop","files":["go/internal/fleet/pool.go"],"summary":%q,"fix":"f","acceptance":["a"]}`

func TestCmd_InboxCurate_AnItemWhoseTextNamesAProtectedFileIsNotProposedForALane(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "shadow")
	f.repo.Write(".evolve/inbox/2026-10-02T00-00-00Z-hidden.json", sprintfItem("hidden", "The fix also changes go/internal/guards/ship.go."))
	f.repo.Write(".evolve/inbox/2026-10-03T00-00-00Z-plain.json", sprintfItem("plain", "The fix also changes go/internal/fleet/queue.go."))

	rc, stdout, stderr := f.curate(t, "", "--apply", "--json")

	var report curateReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || rc != 0 {
		t.Fatalf("rc = %d stdout = %q stderr = %q (%v)", rc, stdout, stderr, err)
	}
	var lane []string
	for _, p := range report.Proposals {
		if p.Kind == "reroute" && p.Target == "lane" {
			lane = append(lane, p.ID)
		}
	}
	if !slices.Equal(lane, []string{"plain"}) {
		t.Errorf("lane reroutes = %v, want only plain: the production curator must read the text of the item, not only its declared files", lane)
	}
}

func sprintfItem(id, summary string) string {
	return fmt.Sprintf(rerouteFileItem, id, summary)
}

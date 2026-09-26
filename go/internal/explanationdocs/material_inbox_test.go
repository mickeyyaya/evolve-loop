package explanationdocs

import (
	"strings"
	"testing"
)

func TestMaterialPaths_InboxLifecycleRecordsAreTheHosts(t *testing.T) {
	t.Parallel()
	got := materialPaths([]string{
		".evolve/inbox/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json",
		".evolve/inbox/consumed/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json",
		".evolve/inbox/processed/cycle-1712/51edfb7f-gittest-fixture-centralize.json",
		".evolve/inbox/quarantine/poison.json",
		".evolve/policy.json",
		"go/internal/modelquery/lineage.go",
	})
	if want := ".evolve/policy.json go/internal/modelquery/lineage.go"; strings.Join(got, " ") != want {
		t.Fatalf("materialPaths = %v, want %q: an inbox record is moved and stamped by the host, never designed by the builder", got, want)
	}
}

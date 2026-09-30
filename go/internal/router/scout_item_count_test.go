package router

import (
	"os"
	"path/filepath"
	"testing"
)

func digestScoutReport(t *testing.T, report string) ScoutSignals {
	t.Helper()
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "scout-report.md", report)
	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	return sig.Scout
}

func TestDigest_ScoutReportFallback_CountsCycle1692DocumentTask(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cycle1692-scout-report.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	scout := digestScoutReport(t, string(raw))
	if scout.ItemCount != 1 {
		t.Errorf("ItemCount = %d, want 1: the replan read item_count=0 and proposed ending a cycle that had one document task", scout.ItemCount)
	}
	if scout.DeliverableKind != DeliverableKindDocument || scout.GoalType != "strategy-options" {
		t.Errorf("headers = (%q, %q), want (document, strategy-options)", scout.DeliverableKind, scout.GoalType)
	}
}

func TestDigest_ScoutReportFallback_ItemCountIsTheSelectedTaskHeadings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		report string
		want   int
	}{
		{"template tasks", "## Selected Tasks\n\n### Task 1: alpha\n- **Slug:** alpha\n#### Notes\n### Task 2: beta\n\n## Acceptance Criteria Summary\n### not a task\n", 2},
		{"proposed-tasks alias", "## Proposed Tasks\n### gamma\n", 1},
		{"legitimate no-work", "## Selected Tasks\n\nNone. The scoped item already shipped.\n\n## Deferred\n", 0},
		{"no tasks section", "# Scout Report\n\n### stray heading\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := digestScoutReport(t, tc.report).ItemCount; got != tc.want {
				t.Errorf("ItemCount = %d, want %d", got, tc.want)
			}
		})
	}
}

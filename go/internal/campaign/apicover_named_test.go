package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFile_NamesCampaignPublicTypesAndFileLoader graduates the campaign
// package into the public-API gate while pinning the file-backed entrypoint.
func TestLoadFile_NamesCampaignPublicTypesAndFileLoader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign-plan.json")
	data := []byte(`{"version":1,"goal":"g","research":{"summary":"s","citations":[{"title":"source"}]},"cycles":[{"id":"c1"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	plan, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	var research Research = plan.Research
	var citation Citation = research.Citations[0]
	if plan.Goal != "g" || citation.Title != "source" {
		t.Fatalf("loaded Plan = %+v, citation = %+v", plan, citation)
	}
}

package routingeval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCase_MissingCaseJSONIsLoudError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "response.txt"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadCase(dir, "no-descriptor")
	if err == nil || !strings.Contains(err.Error(), "read case.json") {
		t.Fatalf("a case directory without case.json must be a loud read error, got %v", err)
	}
}

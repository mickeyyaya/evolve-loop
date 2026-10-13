package research

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLessonFile_ReturnsAnErrorForMalformedYAML(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("- id: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	lessons, err := parseLessonFile(path)

	if err == nil || lessons != nil {
		t.Errorf("parseLessonFile = %v, %v, want an error and no lessons so corpus rot surfaces", lessons, err)
	}
}

func TestParseLessonFile_ReturnsAnErrorForAMissingFile(t *testing.T) {
	t.Parallel()

	lessons, err := parseLessonFile(filepath.Join(t.TempDir(), "absent.yaml"))

	if !os.IsNotExist(err) || lessons != nil {
		t.Errorf("parseLessonFile = %v, %v, want the not-exist error and no lessons", lessons, err)
	}
}

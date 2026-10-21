package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexConfigPath_InTestsIsAScratchFileUnderTheTempDir(t *testing.T) {
	t.Parallel()
	got, err := defaultCodexConfigPath()
	if err != nil || !strings.HasPrefix(got, filepath.Clean(os.TempDir())) {
		t.Fatalf("config path in tests = %q (%v): the TestMain must point CODEX_HOME at a scratch dir", got, err)
	}
}

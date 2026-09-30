//go:build acs

package cycle1716

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1716_001_AcsassertCheckedReadersNameMovedFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "-C", goDir, "test", "-count=1",
		"-run", "TestFileContainsAnyChecked|TestCountOccurrencesAnyChecked|TestLineContainsAllChecked",
		"-v", "./pkg/acsassert/...")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("acsassert checked-reader unit tests failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"TestFileContainsAnyChecked_MissingFileHintsAtRelocation",
		"TestFileContainsAnyChecked_ExistingFileNoHintOnMiss",
		"TestCountOccurrencesAnyChecked_MissingFileHintsAtRelocation",
		"TestCountOccurrencesAnyChecked_ExistingFileNoHintOnZero",
		"TestLineContainsAllChecked_MissingFileHintsAtRelocation",
		"TestLineContainsAllChecked_ExistingFileNoHintOnMiss",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected test %s to have run; output:\n%s", want, out)
		}
	}
}

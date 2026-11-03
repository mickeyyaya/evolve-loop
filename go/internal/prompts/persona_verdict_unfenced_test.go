package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestDiffReviewerPersonas_NeverShowTheVerdictLineInsideACodeFence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "agents", "evolve-diff-*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no diff reviewer persona found (err %v)", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fenced := false
		for n, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
				continue
			}
			if fenced && strings.HasPrefix(strings.TrimSpace(line), "Verdict:") {
				t.Errorf("%s:%d shows the verdict line inside a code fence; a model copies the fence and the run then reads no verdict", filepath.Base(path), n+1)
			}
		}
	}
}

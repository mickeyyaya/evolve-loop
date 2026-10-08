package bridge

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func repoRootForEffort(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// claude-tmux declares effort as pass-through (flag, no values table), so
// realizeScalar appends any string; its rows prove the mechanism is wired,
// not that "max" is a valid claude effort level.
func TestEffortTopRungs_RealizeOnCodexAndClaude(t *testing.T) {
	for _, tc := range []struct {
		manifest string
		effort   string
		want     []string
	}{
		{"codex-tmux", "xhigh", []string{"-c", "model_reasoning_effort=xhigh"}},
		{"claude-tmux", "xhigh", []string{"--effort", "xhigh"}},
		{"codex-tmux", "max", []string{"-c", "model_reasoning_effort=max"}},
		{"claude-tmux", "max", []string{"--effort", "max"}},
	} {
		m, err := LoadManifest(tc.manifest)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", tc.manifest, err)
		}
		r := Realize(m, LaunchIntent{Effort: tc.effort})
		if !containsSubsequence(r.LaunchFlags, tc.want) {
			t.Errorf("%s: %s effort did not realize — flags %v want subsequence %v", tc.manifest, tc.effort, r.LaunchFlags, tc.want)
		}
	}
}

func containsSubsequence(hay, needle []string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if slices.Equal(hay[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

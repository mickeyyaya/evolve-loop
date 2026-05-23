package subagent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteCachePrefix_ByteParityWithBash diffs Go output against a fixture
// rendered by a standalone bash replica of _write_cache_prefix (see
// testdata/cache-prefix.golden). If you change renderCachePrefix's layout,
// regenerate the fixture via the script in this directory's README. A
// drifted golden file means the bash callers and Go callers would write
// different cache prefixes — breaking Anthropic prompt-cache reuse across
// the v11.x bridge period when both implementations may coexist.
func TestWriteCachePrefix_ByteParityWithBash(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "cache.md")
	if err := WriteCachePrefix(CachePrefixRequest{
		Cycle:       42,
		Agent:       "scout",
		Workspace:   "/ws/cycle-42",
		ProjectRoot: tmp,
		OutPath:     out,
	}, CachePrefixOptions{
		ReadOrchestratorPrompt: func(string) (string, error) {
			return "header noise\ngoal: improve dispatch reliability\ntrailing\n", nil
		},
		ReadCycleState: func(string) (string, error) {
			return `{"phase":"scout","active_agent":"scout","completed_phases":["intent"]}`, nil
		},
	}); err != nil {
		t.Fatalf("WriteCachePrefix: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read got: %v", err)
	}
	want, err := os.ReadFile("testdata/cache-prefix.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		// Find first divergence to make the diff legible.
		minLen := len(got)
		if len(want) < minLen {
			minLen = len(want)
		}
		var divIdx int
		for divIdx = 0; divIdx < minLen; divIdx++ {
			if got[divIdx] != want[divIdx] {
				break
			}
		}
		t.Fatalf("byte parity failed at offset %d\nlen(got)=%d len(want)=%d\ngot:\n%s\nwant:\n%s",
			divIdx, len(got), len(want), got, want)
	}
}

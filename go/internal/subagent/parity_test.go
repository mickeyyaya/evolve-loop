package subagent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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

package policy

import "testing"

func TestGCFor_ReadsTheProjectGCBlockAndDegradesToTheZeroPolicy(t *testing.T) {
	root := writePolicyJSON(t, `{"gc":{"verdict_cache_keep":7}}`)
	if got := GCFor(root).VerdictCacheKeep(); got != 7 {
		t.Errorf("GCFor(root).VerdictCacheKeep() = %d, want 7", got)
	}
	for name, root := range map[string]string{"no gc block": writePolicyJSON(t, `{}`), "malformed": writePolicyJSON(t, `{"gc":`), "no file": t.TempDir()} {
		if got := GCFor(root).VerdictCacheKeep(); got != 50 {
			t.Errorf("%s: VerdictCacheKeep() = %d, want the default", name, got)
		}
	}
}

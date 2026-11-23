package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeStaleLedger(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "handoff-tdd-build.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOutputDigestStale_NamesTheRecordedDigestOnlyWhenItDiffersAndWasStamped(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		body, tree, want string
	}{
		"the recorded digest equals the tree":     {`{"output_digest":"t1","round":1,"stamp":{"round":1}}`, "t1", ""},
		"the recorded digest names an old tree":   {`{"output_digest":"t0","round":1,"stamp":{"round":1}}`, "t1", "t0"},
		"a newer round restamps and is not stale": {`{"output_digest":"t0","round":2,"stamp":{"round":1}}`, "t1", ""},
		"no digest was recorded":                  {`{"round":1}`, "t1", ""},
		"the tree is unknown":                     {`{"output_digest":"t0"}`, "", ""},
		"the file holds only the check record":    {`{"schema_version":1,"pair":"tdd-build","check":{"outcome":"pass"}}`, "t1", ""},
		"the file is not JSON":                    {`{not json`, "t1", ""},
	}
	for name, tc := range tests {
		if got := outputDigestStaleAt(writeStaleLedger(t, tc.body), tc.tree); got != tc.want {
			t.Errorf("%s: stale = %q, want %q", name, got, tc.want)
		}
	}
	if got := outputDigestStaleAt(filepath.Join(t.TempDir(), "absent.json"), "t1"); got != "" {
		t.Errorf("an absent ledger is not stale, got %q", got)
	}
}

func TestOutputDigestStale_AnUnknownWorktreeIsNeverStale(t *testing.T) {
	t.Parallel()
	path := writeStaleLedger(t, `{"output_digest":"t0","round":1,"stamp":{"round":1}}`)
	if got := OutputDigestStale(context.Background(), "", "", "", path); got != "" {
		t.Errorf("stale = %q, want empty when no worktree digest exists", got)
	}
}

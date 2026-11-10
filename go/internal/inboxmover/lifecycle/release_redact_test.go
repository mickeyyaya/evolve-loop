package lifecycle

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMover_Release_ContinuationStampRedactsHostPaths(t *testing.T) {
	home := filepath.Join(t.TempDir(), "operator-home")
	t.Setenv("HOME", home)
	inbox := newInbox(t)
	workspace := func(c int) string { return filepath.Join(inbox, "..", "runs", "cycle-"+strconv.Itoa(c)) }
	manifest := `{"worktree":"` + home + `/wt/cycle-8","findings_path":"` + home + `/findings.md","snapshot_sha":"abc123","cycle":8}`
	writeItem(t, filepath.Join(workspace(8), "continuation-manifest.json"), manifest)
	writeItem(t, procPath(inbox, 8, "t.json"), `{"id":"t"}`)
	writeItem(t, procPath(inbox, 8, "kept.json"), `{"id":"kept"}`)
	m := New(inbox, nil, WithRunWorkspace(workspace))

	if _, err := m.Release(8, "", nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"t.json", "kept.json"} {
		stamp := string(readJSON(t, filepath.Join(inbox, name))["continuation"])
		if !strings.Contains(stamp, `"snapshot_sha": "abc123"`) {
			t.Fatalf("%s: stamp missing its snapshot, nothing to redact: %s", name, stamp)
		}
		if strings.Contains(stamp, home) {
			t.Errorf("%s: stamp leaks the operator home path into the shared inbox: %s", name, stamp)
		}
		if !strings.Contains(stamp, `"worktree": "~/wt/cycle-8"`) || !strings.Contains(stamp, `"findings_path": "~/findings.md"`) {
			t.Errorf("%s: host paths are not tilde-redacted: %s", name, stamp)
		}
	}
}

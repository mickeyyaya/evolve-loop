package inboxmover

import (
	"path/filepath"
	"testing"
)

func TestReadLastFailureClass_ReadsTheItemOfTheProjectInbox(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeFile(t, filepath.Join(inbox, "task-1.json"), `{"id":"task-1","failure_count":1,"last_failure_class":"spec"}`)
	if got := ReadLastFailureClass(Options{ProjectRoot: root}, "task-1"); got != "spec" {
		t.Fatalf("ReadLastFailureClass = %q, want spec", got)
	}
	if got := ReadLastFailureClass(Options{ProjectRoot: root}, "absent"); got != "" {
		t.Fatalf("an absent item read class %q", got)
	}
}

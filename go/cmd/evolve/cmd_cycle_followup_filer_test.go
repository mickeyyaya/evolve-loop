package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestHostFollowupFiler_FilesIntoTheProjectInboxAndRefusesASecondCopy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	file := hostFollowupFiler(nil, nil, func(string) bool { return false }, []string{"correctness"})
	raw := []byte(`{"id":"x-ac-1-console","kind":"feature","priority_class":"correctness","weight":0.5,"route":"console-manual","title":"t","summary":"s","fix":"f","acceptance":["a"]}`)

	if err := file(root, raw); err != nil {
		t.Fatalf("first filing: %v", err)
	}
	if got, _ := filepath.Glob(filepath.Join(inbox, "*-x-ac-1-console.json")); len(got) != 1 {
		t.Fatalf("inbox holds %v; want the one companion", got)
	}
	if err := file(root, raw); !errors.Is(err, inboxmover.ErrAlreadyFiled) {
		t.Fatalf("second filing = %v; want ErrAlreadyFiled so the gate can treat it as done", err)
	}
}

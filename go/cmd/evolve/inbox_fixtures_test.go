package main

import (
	"bytes"
	"errors"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

const cliInboxItem = `{"id":"cli-inbox-show","kind":"feature","priority_class":"debuggability","weight":0.5,` +
	`"title":"evolve inbox show prints one item","files":["go/cmd/evolve/cmd_inbox.go"],` +
	`"summary":"No verb prints one item.","fix":"Add evolve inbox show <id>.",` +
	`"acceptance":["evolve inbox show <id> prints the item"]}`

func fileCLIItem(t *testing.T) (root, path string) {
	t.Helper()
	root = inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Fatalf("add: rc = %d stderr = %q", rc, stderr.String())
	}
	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-cli-inbox-show.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the filed item: %v %v", matches, err)
	}
	return root, matches[0]
}

func writeBinding(t *testing.T, root, id string, cycle int) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, continuation.Continuation{
		Worktree: "/tmp/wt", Branch: "cycle-x", SnapshotSHA: "abc", BaseSHA: "def",
		FindingsPath: "/tmp/f.json", Cycle: cycle,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBlockerBreakerBootPath_RunsBindingReconcile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	consumed := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumed, "boot-item.json"),
		[]byte(`{"id":"boot-item"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "boot-item", 3)
	var errb bytes.Buffer
	_, _ = blockerBreakerHalt(evolveDir, root, 1, &errb, testRootSignals(t, &errb))
	if _, ok, _ := continuation.ReadRegistryEntry(root, "boot-item"); ok {
		t.Fatalf("breaker boot path did not release the stray binding (stderr=%s)", errb.String())
	}
}

type errReaderE2 struct{}

func (errReaderE2) Read([]byte) (int, error) { return 0, errors.New("stdin torn") }

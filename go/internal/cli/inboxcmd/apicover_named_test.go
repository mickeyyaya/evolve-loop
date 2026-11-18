package inboxcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApicover_NamesEveryExport(t *testing.T) {
	t.Run("Deps and Run", func(t *testing.T) {
		deps := testDeps()
		if deps.QueueGate == nil || deps.LaneForbidden == nil || deps.PrintVerbJSON == nil ||
			deps.HumanWriter == nil || deps.CheckRootHasEvolve == nil || deps.NewSignalCenter == nil {
			t.Fatalf("a Deps field is nil: %+v", deps)
		}
		var out, errb bytes.Buffer
		if rc := Run([]string{"--help"}, strings.NewReader(""), &out, &errb, deps); rc != 0 || !strings.Contains(out.String(), "usage: evolve inbox <verb>") {
			t.Fatalf("Run --help: rc %d stdout %q stderr %q", rc, out.String(), errb.String())
		}
	})
	t.Run("RunMover", func(t *testing.T) {
		var out, errb bytes.Buffer
		if rc := RunMover(nil, strings.NewReader(""), &out, &errb, testDeps()); rc != 1 || errb.Len() == 0 {
			t.Fatalf("RunMover with no subcommand: rc %d stderr %q, want 1 and a usage", rc, errb.String())
		}
	})
	t.Run("ReadInboxItemObject", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "item.json")
		if err := os.WriteFile(path, []byte(`{"id":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		doc, err := ReadInboxItemObject(path)
		if err != nil || string(doc["id"]) != `"x"` {
			t.Fatalf("ReadInboxItemObject = %v, %v", doc, err)
		}
	})
	t.Run("SplitForbidden", func(t *testing.T) {
		forbidden := SplitForbidden(func(p string) bool { return p == "lane-only" })
		if !forbidden("lane-only") || forbidden("go/internal/cli/inboxcmd/doc.go") {
			t.Fatal("SplitForbidden must forbid what the lane forbids and no ordinary path")
		}
	})
	t.Run("CheckInboxTrailerBytes", func(t *testing.T) {
		_, _, violations := CheckInboxTrailerBytes(t.TempDir(), []byte("feat: x\n"), "commit abc")
		if len(violations) != 1 || !strings.Contains(violations[0], "no Closes-Inbox or Refs-Inbox trailer in commit abc") {
			t.Fatalf("violations = %v", violations)
		}
	})
	t.Run("Reconcile", func(t *testing.T) {
		root := t.TempDir()
		var errb bytes.Buffer
		ReconcileConsumedFingerprints(filepath.Join(root, ".evolve"), &errb)
		ReconcileConsumedBindings(root, filepath.Join(root, ".evolve"), &errb)
		if errb.Len() != 0 {
			t.Fatalf("reconcile on an empty root wrote %q", errb.String())
		}
	})
}

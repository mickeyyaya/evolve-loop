package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeInboxItem(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTriageComposePrompt_InjectsInboxBatches(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"alpha","weight":0.9,"campaign":"camp-x"}`)
	writeInboxItem(t, root, "b.json", `{"id":"beta","weight":0.4,"campaign":"camp-x"}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	if !strings.Contains(out, "inbox_batches") {
		t.Fatalf("triage prompt has no inbox_batches section:\n%s", out)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("batch section must carry the grouped ids:\n%s", out)
	}
	if !strings.Contains(out, "whole batch") {
		t.Errorf("section must instruct triage to prefer selecting a WHOLE batch as top_n:\n%s", out)
	}
}

// Same root for both renders so BaseCycleContext's path lines cannot differ.
func TestTriageComposePrompt_EmptyInboxIsByteIdentical(t *testing.T) {
	root := t.TempDir()
	req := core.PhaseRequest{ProjectRoot: root}

	a := hooks{}.ComposePrompt("BODY", req)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := hooks{}.ComposePrompt("BODY", req)
	if a != b {
		t.Errorf("missing vs empty inbox must be byte-identical:\n--- missing ---\n%s\n--- empty ---\n%s", a, b)
	}
	if strings.Contains(a, "inbox_batches") {
		t.Errorf("prompt without a backlog must not carry an inbox_batches section:\n%s", a)
	}
}

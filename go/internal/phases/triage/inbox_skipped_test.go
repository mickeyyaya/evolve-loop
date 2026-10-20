package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestTriageComposePrompt_AMalformedItemIsNamedNotSwallowed(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"alpha","weight":0.9,"campaign":"camp-x"}`)
	writeInboxItem(t, root, "b.json", `{"id":"beta","priority":0.86,"campaign":"camp-x"}`)
	writeInboxItem(t, root, "bad.json", `{"id":"gamma","weight":"high"}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})

	section := out[strings.Index(out, "inbox_batches"):]
	if !strings.Contains(section, "(campaign camp-x): alpha, beta\n") || strings.Contains(out, "gamma") {
		t.Errorf("candidates: want alpha and beta (the numeric priority read as a weight) and no gamma:\n%s", out)
	}
	if !strings.Contains(out, "- skipped_inbox_files: 1 file(s)") || !strings.Contains(out, "bad.json") {
		t.Errorf("the malformed file must be named in the prompt:\n%s", out)
	}
}

func TestTriageComposePrompt_OnlyAMalformedItemStillReportsIt(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "bad.json", `not json`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})

	if !strings.Contains(out, "- skipped_inbox_files: 1 file(s)") || !strings.Contains(out, "bad.json") {
		t.Errorf("an inbox of one malformed file is not an empty queue:\n%s", out)
	}
}

func TestInboxBatchesSection_AnUnreadableInboxDirAddsNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("a file where the inbox belongs"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := inboxBatchesSection(root, nil); got != "" {
		t.Errorf("section %q, want nothing when the inbox dir cannot be read", got)
	}
}

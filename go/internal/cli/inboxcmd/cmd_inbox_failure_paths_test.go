package inboxcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestRunInboxLint_TextNamesTheCitedProtectedBasenamesAndFailsOnEachLoadOrWriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractUnitWrite(t, root, "go/internal/core/disposition_gate.go", "package core\n")
	contractUnitWrite(t, root, ".evolve/inbox/cites.json", `{"id":"cites","kind":"bug","weight":0.9,"priority_class":"correctness","title":"disposition_gate.go misreads a row","summary":"s"}`)
	contractUnitWrite(t, root, ".evolve/inbox/plain.json", `{"id":"plain","kind":"bug","weight":0.8,"priority_class":"correctness","title":"the queue renders slowly","summary":"s"}`)
	var stdout, stderr bytes.Buffer
	rc := runInboxLint(root, nil, &stdout, &stderr)
	want := []string{"cites: undeclared surface (cites protected basename: disposition_gate.go)", "plain: undeclared surface"}
	if rc != exitFailed || !strings.Contains(stdout.String(), want[0]) || !strings.Contains(stdout.String(), want[1]) {
		t.Fatalf("rc %d stdout %q, want exit 1 and %q", rc, stdout.String(), want)
	}
	if rc := runInboxLint(root, []string{"--json"}, &failingWriter{}, &stderr); rc != exitFailed || !strings.Contains(stderr.String(), "inbox lint: encode: ") {
		t.Errorf("an unwritable stdout: rc %d stderr %q", rc, stderr.String())
	}
	broken := t.TempDir()
	contractUnitWrite(t, broken, ".evolve/inbox", "a file\n")
	if rc := runInboxLint(broken, nil, &stdout, &stderr); rc != exitFailed || !strings.Contains(stderr.String(), "inbox lint: ") {
		t.Errorf("an unreadable inbox: rc %d stderr %q", rc, stderr.String())
	}
}

func TestPrintLoadWarnings(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	printLoadWarnings(&stderr, "inbox x", []inboxbatch.LoadWarning{
		{Text: "a.json: not JSON", Unreadable: true},
		{Text: "b.json: sanitized control characters in title"},
	})
	printLoadWarnings(&stderr, "inbox x", nil)

	want := "inbox x: WARN skipped a.json: not JSON\ninbox x: WARN b.json: sanitized control characters in title\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunInboxQuarantineList_FailurePaths(t *testing.T) {
	t.Run("a quarantine path that is a file", func(t *testing.T) {
		root := t.TempDir()
		q := filepath.Join(root, ".evolve", "inbox", "quarantine")
		if err := os.MkdirAll(filepath.Dir(q), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(q, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EVOLVE_PROJECT_ROOT", root)
		var stdout, stderr bytes.Buffer

		code := runInboxQuarantine(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), []string{"list"}, nil, &stdout, &stderr)

		if code != 1 || !strings.HasPrefix(stderr.String(), "inbox quarantine list: ") || stdout.Len() != 0 {
			t.Errorf("code=%d stdout=%q stderr=%q, want 1 and a scan error", code, stdout.String(), stderr.String())
		}
	})
	t.Run("a broken stdout fails the JSON encode", func(t *testing.T) {
		root := t.TempDir()
		q := filepath.Join(root, ".evolve", "inbox", "quarantine")
		if err := os.MkdirAll(q, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(q, "x.json"), []byte(`{"id":"x","title":"t"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EVOLVE_PROJECT_ROOT", root)
		var stderr bytes.Buffer

		code := runInboxQuarantine(cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), []string{"list", "--json"}, nil, &failingWriter{}, &stderr)

		if code != 1 || !strings.Contains(stderr.String(), "inbox quarantine list: encode: broken pipe") {
			t.Errorf("code=%d stderr=%q, want 1 and the encode failure", code, stderr.String())
		}
	})
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dossierRoot(t *testing.T, dir, body string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(dir), "cycle-12.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const shownDossier = `{"cycle":12,"goal":"fix the parser","final_verdict":"PASS"}`

func TestDossierShow_PrintsTheDossierOfOneCycle(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"knowledge-base/cycles", ".evolve/dossiers-pending"} {
		t.Run(dir, func(t *testing.T) {
			root := dossierRoot(t, dir, shownDossier)
			var stdout, stderr bytes.Buffer
			if rc := runDossier([]string{"show", "12", "--json", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
				t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
			}
			var got dossierShowResult
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("stdout is not one dossier document: %v\n%s", err, stdout.String())
			}
			if got.Cycle != 12 || got.Dossier == nil || got.Dossier.FinalVerdict != "PASS" || got.Path != filepath.Join(root, filepath.FromSlash(dir), "cycle-12.json") {
				t.Errorf("result = %+v", got)
			}
		})
	}
}

func TestDossierShow_TheTextFormNamesTheVerdictAndTheGoal(t *testing.T) {
	t.Parallel()
	root := dossierRoot(t, "knowledge-base/cycles", shownDossier)
	var stdout, stderr bytes.Buffer
	if rc := runDossier([]string{"show", "12", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	if out := stdout.String(); !strings.Contains(out, "cycle 12: PASS") || !strings.Contains(out, "fix the parser") {
		t.Errorf("stdout = %q", out)
	}
}

func TestDossierShow_EachFaultHasItsExitCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		root string
		args []string
		want int
	}{
		{"no cycle", t.TempDir(), []string{"show"}, cliExitUsage},
		{"a word for a cycle", t.TempDir(), []string{"show", "x"}, cliExitUsage},
		{"cycle zero", t.TempDir(), []string{"show", "0"}, cliExitUsage},
		{"two cycles", t.TempDir(), []string{"show", "1", "2"}, cliExitUsage},
		{"an unknown flag", t.TempDir(), []string{"show", "12", "--bogus"}, cliExitUsage},
		{"no dossier", t.TempDir(), []string{"show", "12"}, cliExitFailed},
		{"not JSON", dossierRoot(t, "knowledge-base/cycles", "{bad"), []string{"show", "12"}, cliExitInput},
		{"unreadable", unreadableDossierRoot(t), []string{"show", "12"}, cliExitIO},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if rc := runDossier(append(c.args, "--project-root", c.root), nil, &stdout, &stderr); rc != c.want || stderr.Len() == 0 {
				t.Errorf("rc = %d stderr = %q, want %d and a message", rc, stderr.String(), c.want)
			}
		})
	}
}

func unreadableDossierRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "knowledge-base", "cycles", "cycle-12.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDossierShow_AnUnreadableWorkingDirectoryIsAnIOError(t *testing.T) {
	breakWorkingDirectory(t)
	var stdout, stderr bytes.Buffer
	if rc := runDossier([]string{"show", "12"}, nil, &stdout, &stderr); rc != cliExitIO {
		t.Errorf("rc = %d, want 2", rc)
	}
}

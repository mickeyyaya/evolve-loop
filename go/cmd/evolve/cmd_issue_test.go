package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const issueTestID = "2026-11-18-cmd-fixture"

func runIssueArgs(args ...string) (string, int) {
	var out, errOut bytes.Buffer
	code := runIssue(args, nil, &out, &errOut)
	return out.String() + errOut.String(), code
}

func TestRunIssue_EvidenceAddThenCheckThenTamper(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "run.log")
	if err := os.WriteFile(src, []byte("INFO a\nERROR boom 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code := runIssueArgs("evidence", "add", "--project-root", root, issueTestID, src); code != 0 {
		t.Fatalf("evidence add: exit %d\n%s", code, out)
	}
	if out, code := runIssueArgs("check", "--project-root", root); code != 0 || !strings.Contains(out, "ok "+issueTestID) {
		t.Fatalf("check all: exit %d\n%s", code, out)
	}
	copyPath := filepath.Join(root, "knowledge-base", "incidents", issueTestID, "src-01-run.log")
	f, err := os.OpenFile(copyPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("extra\n")
	f.Close()
	if out, code := runIssueArgs("check", "--project-root", root, issueTestID); code != 1 || !strings.Contains(out, "changed since capture") {
		t.Errorf("tampered check: exit %d\n%s", code, out)
	}
}

func TestRunIssue_UsageErrorsExitTen(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{}, {"evidence"}, {"evidence", "add", "only-one"}, {"check", "a", "b"}} {
		if out, code := runIssueArgs(args...); code != 10 {
			t.Errorf("runIssue(%v) = %d, want 10\n%s", args, code, out)
		}
	}
}

func TestRunIssue_BudgetComesFromPolicy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"incident":{"evidence":{"budget_bytes":30}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"ERROR same 1\n" + strings.Repeat("a ", 20), "ERROR same 2\n" + strings.Repeat("b ", 20)} {
		src := filepath.Join(t.TempDir(), "s.log")
		os.WriteFile(src, []byte(body), 0o600)
		if out, code := runIssueArgs("evidence", "add", "--project-root", root, issueTestID, src); code != 0 {
			t.Fatalf("add: %d %s", code, out)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(root, "knowledge-base", "incidents", issueTestID, "manifest.json"))
	if !strings.Contains(string(raw), `"state": "pointer"`) && !strings.Contains(string(raw), `"state":"pointer"`) {
		t.Errorf("the policy budget did not turn the oldest repeat into a pointer:\n%s", raw)
	}
}

func TestRunIssue_BadFlagAndBadIDExitCodes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "run.log")
	if err := os.WriteFile(src, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"check", "--bogus"}, 10},
		{[]string{"evidence", "add", "--bogus"}, 10},
		{[]string{"check", "--project-root", root, "not-an-id"}, 1},
		{[]string{"evidence", "add", "--project-root", root, "not-an-id", src}, 1},
		{[]string{"check", "--project-root", root, issueTestID}, 1},
	}
	for _, c := range cases {
		if out, code := runIssueArgs(c.args...); code != c.want {
			t.Errorf("runIssue(%v) = %d, want %d\n%s", c.args, code, c.want, out)
		}
	}
}

func TestRunIssue_CheckAllFailsWhenAnIncidentRootIsNotADirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "knowledge-base"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code := runIssueArgs("check", "--project-root", root); code != 1 {
		t.Errorf("exit %d\n%s", code, out)
	}
}

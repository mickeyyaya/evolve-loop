package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
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

func TestRunIssue_RecordCheckAndList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := incidentkb.Dir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	raw := `---
schema: incident-record/1
id: 2026-11-19-cmd-record
title: cmd record
level: record
status: fixed
actions:
  - {id: A1, type: prevent, owner: console, item: item-1, pr: 12, done: "done", state: done}
effectiveness:
  metric: {code: CODE_ONE, filters: [], unit: wave, since: abc}
  baseline: {value: 2, units: 1}
  target: 0
  checks: 1
  results: []
verified_at: ""
---

## Summary

<!-- GENERATED:status BEGIN -->
<!-- GENERATED:status END -->

## Trigger and level

## Timeline

## Key logs

## Root cause and contributing factors

## Analysis

## Approaches taken

## Actions

<!-- GENERATED:actions BEGIN -->
<!-- GENERATED:actions END -->

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
<!-- GENERATED:effectiveness END -->

## Lessons
`
	rendered, err := incidentkb.Render([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-19-cmd-record.md"), rendered, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-18-old.md"), []byte("# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "REGRESSION-COVERAGE-INDEX.md"), []byte("# index\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code := runIssueArgs("check", "--project-root", root); code != 0 || !strings.Contains(out, "legacy 2026-11-18-old") || !strings.Contains(out, "ok 2026-11-19-cmd-record") {
		t.Fatalf("check = %d\n%s", code, out)
	}
	if out, code := runIssueArgs("check", "--project-root", root, "2026-11-19-cmd-record"); code != 0 || !strings.Contains(out, "ok 2026-11-19-cmd-record") {
		t.Fatalf("check single = %d\n%s", code, out)
	}
	var stdout, stderr bytes.Buffer
	if code := runIssue([]string{"list", "--project-root", root, "--json"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("list = %d\n%s", code, stderr.String())
	}
	var rows []incidentkb.ListEntry
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("list json = %v, rows=%+v\n%s", err, rows, stdout.String())
	}
	stdout.Reset()
	if code := runIssue([]string{"list", "--project-root", root, "--status", "fixed", "--code", "CODE_ONE", "--json"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("filtered list = %d", code)
	}
	rows = nil
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].ID != "2026-11-19-cmd-record" {
		t.Fatalf("filtered rows = %+v, err=%v", rows, err)
	}
	if out, code := runIssueArgs("list", "--project-root", root, "extra"); code != 10 {
		t.Errorf("list extra arg = %d\n%s", code, out)
	}
	if out, code := runIssueArgs("list", "--project-root", root); code != 0 || !strings.Contains(out, "2026-11-19-cmd-record") {
		t.Errorf("list plain = %d\n%s", code, out)
	}
	if out, code := runIssueArgs("check", "--project-root", root, "2026-11-18-old"); code != 0 || !strings.Contains(out, "legacy 2026-11-18-old") {
		t.Errorf("check legacy single = %d\n%s", code, out)
	}
	corruptBundleDir := filepath.Join(root, "knowledge-base", "incidents", "2026-11-19-cmd-record")
	if err := os.MkdirAll(corruptBundleDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corruptBundleDir, "manifest.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code := runIssueArgs("check", "--project-root", root, "2026-11-19-cmd-record"); code != 1 {
		t.Errorf("corrupt bundle check = %d\n%s", code, out)
	}

	badRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(badRoot, "docs"), 0o750)
	_ = os.WriteFile(filepath.Join(badRoot, "docs", "incidents"), []byte("x"), 0o600)
	if out, code := runIssueArgs("check", "--project-root", badRoot); code != 1 {
		t.Errorf("check non-dir docs/incidents = %d\n%s", code, out)
	}
	if out, code := runIssueArgs("list", "--project-root", badRoot); code != 1 {
		t.Errorf("list non-dir docs/incidents = %d\n%s", code, out)
	}
}

const (
	renderTestID  = "2026-11-25-render-fixture"
	renderTestDoc = `---
schema: incident-record/1
id: 2026-11-25-render-fixture
title: render fixture
level: record
status: fixed
actions:
  - {id: A1, type: prevent, owner: console, item: item-1, pr: 12, done: "done", state: done}
---

## Summary

<!-- GENERATED:status BEGIN -->
<!-- GENERATED:status END -->

## Trigger and level

## Timeline

## Key logs

## Root cause and contributing factors

## Analysis

## Approaches taken

## Actions

<!-- GENERATED:actions BEGIN -->
<!-- GENERATED:actions END -->

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
<!-- GENERATED:effectiveness END -->

## Lessons
`
)

func writeRenderFixture(t *testing.T, doc string) (root, path string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(incidentkb.Dir(root), 0o750); err != nil {
		t.Fatal(err)
	}
	path = incidentkb.RecordPath(root, renderTestID)
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestRunIssue_RenderFillsTheMarkersAndTheCheckThenPasses(t *testing.T) {
	t.Parallel()
	root, path := writeRenderFixture(t, renderTestDoc)
	if out, code := runIssueArgs("check", "--project-root", root, renderTestID); code != 1 || !strings.Contains(out, "rendered status section does not match frontmatter") {
		t.Fatalf("before render: exit %d\n%s", code, out)
	}
	if out, code := runIssueArgs("render", "--project-root", root, renderTestID); code != 0 || !strings.Contains(out, "rendered "+renderTestID) {
		t.Fatalf("render: exit %d\n%s", code, out)
	}
	want, err := incidentkb.Render([]byte(renderTestDoc))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Fatalf("the file is not the render of the frontmatter: %v", err)
	}
	if out, code := runIssueArgs("check", "--project-root", root, renderTestID); code != 0 {
		t.Fatalf("after render: exit %d\n%s", code, out)
	}
}

func TestRunIssue_RenderLeavesARenderedFileUntouched(t *testing.T) {
	t.Parallel()
	root, path := writeRenderFixture(t, renderTestDoc)
	if _, code := runIssueArgs("render", "--project-root", root, renderTestID); code != 0 {
		t.Fatal("first render failed")
	}
	past := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	if _, code := runIssueArgs("render", "--project-root", root, renderTestID); code != 0 {
		t.Fatal("second render failed")
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(past) {
		t.Fatalf("a second render rewrote the file: %v %v", info.ModTime(), err)
	}
}

func TestRunIssue_RenderRefusesAndLeavesTheFileAlone(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		doc  string
		id   string
		want string
	}{
		"a bad level":      {strings.Replace(renderTestDoc, "level: record", "level: loud", 1), renderTestID, "level"},
		"an id mismatch":   {strings.Replace(renderTestDoc, "id: "+renderTestID, "id: 2026-11-25-typo", 1), renderTestID, "does not match filename"},
		"a missing marker": {strings.Replace(renderTestDoc, "<!-- GENERATED:status BEGIN -->\n", "", 1), renderTestID, "missing generated marker"},
		"a missing file":   {renderTestDoc, "2026-11-25-absent", "no such file"},
		"a bad id":         {renderTestDoc, "not-an-id", "must look like"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, path := writeRenderFixture(t, c.doc)
			out, code := runIssueArgs("render", "--project-root", root, c.id)
			if code != 1 || !strings.Contains(out, c.want) {
				t.Fatalf("exit %d, want 1 naming %q\n%s", code, c.want, out)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != c.doc {
				t.Fatalf("a refused render changed the file: %v", err)
			}
		})
	}
}

func TestRunIssue_RenderUsageErrorsExitTen(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"render"}, {"render", "a", "b"}, {"render", "--bogus"}} {
		if out, code := runIssueArgs(args...); code != 10 {
			t.Errorf("runIssue(%v) = %d, want 10\n%s", args, code, out)
		}
	}
	if out, _ := runIssueArgs("nonsense"); !strings.Contains(out, "render") {
		t.Errorf("the unknown-verb line omits render:\n%s", out)
	}
}

func TestRunIssue_RenderReportsAWriteThatFailsAndKeepsTheFile(t *testing.T) {
	t.Parallel()
	root, path := writeRenderFixture(t, renderTestDoc)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	out, code := runIssueArgs("render", "--project-root", root, renderTestID)
	if code != 1 || !strings.Contains(out, "evolve issue render:") {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != renderTestDoc {
		t.Fatalf("a failed write changed the file: %v", err)
	}
}

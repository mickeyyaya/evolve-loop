package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var runNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

const (
	waivedFinding   = `{"finding":{"osv":"GO-2026-0001","trace":[{"function":"F"}]}}` + "\n"
	unlistedFinding = `{"finding":{"osv":"GO-2026-9999","trace":[{"function":"F"}]}}` + "\n"
	validAllowlist  = `[{"id":"GO-2026-0001","expires":"2026-11-01","reason":"mitigated"}]`
)

func writeAllowlist(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "allowlist.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRun(t *testing.T) {
	summary := filepath.Join(t.TempDir(), "summary.md")
	cases := []struct {
		name     string
		args     []string
		allow    string
		stdin    string
		environ  []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{"bad flag", []string{"-nope"}, "", "", nil, 2, "", "flag provided but not defined"},
		{"missing allowlist", []string{"-allowlist", filepath.Join(t.TempDir(), "absent.json")}, "", "", nil, 1, "", "failed to open allowlist"},
		{"malformed allowlist", nil, "[nope", "", nil, 1, "", "failed to parse allowlist"},
		{"malformed stream", nil, validAllowlist, `{"finding":{` + "\n", nil, 1, "", "stream decode error"},
		{"waived finding passes", nil, validAllowlist, waivedFinding, nil, 0, "WAIVED GO-2026-0001", ""},
		{"unlisted finding fails", nil, validAllowlist, unlistedFinding, nil, 1, "", "UNLISTED VULNERABILITY: GO-2026-9999"},
		{"expired entry fails", nil, `[{"id":"GO-2026-0002","expires":"2026-01-01","reason":"old"}]`, "", nil, 1, "", "EXPIRED ALLOWLIST ENTRY: GO-2026-0002"},
		{"invalid entry fails", nil, `[{"id":"GO-2026-0003","expires":"2026-11-01","reason":""}]`, "", nil, 1, "", "INVALID ALLOWLIST ENTRY"},
		{"step summary written", nil, validAllowlist, "", []string{"X=1", "GITHUB_STEP_SUMMARY=" + summary}, 0, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args
			if args == nil {
				args = []string{"-allowlist", writeAllowlist(t, c.allow)}
			}
			var stdout, stderr bytes.Buffer
			code := run(args, strings.NewReader(c.stdin), &stdout, &stderr, c.environ, runNow)
			if code != c.wantCode {
				t.Fatalf("exit = %d, want %d; stderr=%s", code, c.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), c.wantOut) || !strings.Contains(stderr.String(), c.wantErr) {
				t.Fatalf("stdout=%q stderr=%q, want %q and %q", stdout.String(), stderr.String(), c.wantOut, c.wantErr)
			}
		})
	}
	got, err := os.ReadFile(summary)
	if err != nil || !strings.Contains(string(got), "No vulnerabilities found.") {
		t.Fatalf("step summary = %q err=%v, want the clean summary", got, err)
	}
}

func TestMainExitsWithTheRunCode(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	defer func() { os.Args, exit = oldArgs, oldExit }()
	os.Args = []string{"vulnfilter", "-allowlist", filepath.Join(t.TempDir(), "absent.json")}
	code := -1
	exit = func(c int) { code = c }
	main()
	if code != 1 {
		t.Fatalf("main exit code = %d, want 1", code)
	}
}

func TestStepSummaryPath(t *testing.T) {
	if got := stepSummaryPath([]string{"A=1", "GITHUB_STEP_SUMMARY=/x/y"}); got != "/x/y" {
		t.Fatalf("path = %q", got)
	}
	if got := stepSummaryPath([]string{"A=1"}); got != "" {
		t.Fatalf("path = %q, want empty", got)
	}
}

func TestAppendStepSummaryOpenError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "no-dir", "s.md")
	if err := AppendStepSummary(bad, &Result{Passed: true}, nil); err == nil {
		t.Fatal("expected an error when the summary file cannot be opened")
	}
}

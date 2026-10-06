package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func scanTestKey() string { return "AKIA" + "IOSFODNN7EXAMPLE" }

func scanTestRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	scanTestWrite(t, r.Dir, "README.md", "fixture\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func scanTestWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scanTestStageKey(t *testing.T, r *gittest.Repo, rel string) {
	t.Helper()
	scanTestWrite(t, r.Dir, rel, "package config\nconst id = \""+scanTestKey()+"\"\n")
	r.Git("add", rel)
}

func runScanCapture(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = runScan(args, nil, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestScanSecretsStagedKeyExitsOneWithAMaskedFinding(t *testing.T) {
	r := scanTestRepo(t)
	scanTestStageKey(t, r, "config/aws.go")
	code, stdout, stderr := runScanCapture("secrets", "--project-root", r.Dir)
	want := "config/aws.go:2: aws-access-key-id: AKIA****************\nscan secrets: FAIL: 1 finding(s) in the staged diff\n"
	if code != exitSecretFound || stdout != want {
		t.Errorf("exit %d stdout %q, want exit %d stdout %q (stderr %q)", code, stdout, exitSecretFound, want, stderr)
	}
	if strings.Contains(stdout+stderr, scanTestKey()) {
		t.Errorf("the raw key leaked into the output")
	}
}

func TestScanSecretsCleanAndEmptyStagedDiffsPass(t *testing.T) {
	r := scanTestRepo(t)
	if code, stdout, _ := runScanCapture("secrets", "--project-root", r.Dir); code != 0 || stdout != "scan secrets: PASS: 0 finding(s) in the staged diff\n" {
		t.Errorf("nothing staged: exit %d stdout %q", code, stdout)
	}
	scanTestWrite(t, r.Dir, "main.go", "package main\n")
	r.Git("add", "main.go")
	if code, stdout, _ := runScanCapture("secrets", "--staged", "--project-root", r.Dir); code != 0 || !strings.HasPrefix(stdout, "scan secrets: PASS") {
		t.Errorf("clean staged change: exit %d stdout %q", code, stdout)
	}
}

func TestScanSecretsDiffRangeScansCommitsNotTheIndex(t *testing.T) {
	r := scanTestRepo(t)
	r.Git("checkout", "-q", "-b", "feature")
	scanTestWrite(t, r.Dir, "leak.txt", "key="+scanTestKey()+"\n")
	r.Git("add", "leak.txt")
	r.Git("commit", "-q", "-m", "leak")
	code, stdout, _ := runScanCapture("secrets", "--diff", "main...HEAD", "--project-root", r.Dir)
	if code != exitSecretFound || !strings.HasPrefix(stdout, "leak.txt:1: aws-access-key-id: AKIA") || !strings.Contains(stdout, "in diff main...HEAD") {
		t.Errorf("committed key in range: exit %d stdout %q", code, stdout)
	}

	clean := scanTestRepo(t)
	clean.Git("checkout", "-q", "-b", "feature")
	scanTestStageKey(t, clean, "pending.go")
	if code, stdout, _ := runScanCapture("secrets", "--diff=main...HEAD", "--project-root", clean.Dir); code != 0 {
		t.Errorf("a staged-only key must not be in the range: exit %d stdout %q", code, stdout)
	}
}

func TestScanSecretsGitFailuresExitTwoWithNoVerdict(t *testing.T) {
	notARepo := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(notARepo))
	r := scanTestRepo(t)
	cases := map[string][]string{
		"not a repo":       {"secrets", "--project-root", notARepo},
		"missing root":     {"secrets", "--project-root", filepath.Join(notARepo, "missing")},
		"unknown revision": {"secrets", "--diff", "nosuchref", "--project-root", r.Dir},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runScanCapture(args...)
			if code != exitIO || stdout != "" || !strings.HasPrefix(stderr, scanSecretsPrefix+"git diff: ") {
				t.Errorf("exit %d stdout %q stderr %q, want exit %d, empty stdout, a git diff error", code, stdout, stderr, exitIO)
			}
		})
	}
}

func TestScanSecretsUsageErrorsExitTenAndHelpExitsZero(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cases := map[string][]string{
		"no subcommand":      {},
		"unknown subcommand": {"bogus"},
		"extra operand":      {"secrets", "extra", "--project-root", missing},
		"staged with diff":   {"secrets", "--staged", "--diff", "main", "--project-root", missing},
		"flag as revision":   {"secrets", "--diff", "--output=x", "--project-root", missing},
		"empty revision":     {"secrets", "--diff=", "--project-root", missing},
		"diff twice":         {"secrets", "--diff", "a", "--diff", "b", "--project-root", missing},
		"diff without value": {"secrets", "--diff"},
		"unknown flag":       {"secrets", "--bogus", "--project-root", missing},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code, stdout, stderr := runScanCapture(args...); code != exitUsage || stdout != "" || !strings.Contains(stderr, scanUsage) {
				t.Errorf("exit %d stdout %q stderr %q, want exit %d with the usage on stderr", code, stdout, stderr, exitUsage)
			}
		})
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"secrets", "--help"}, {"secrets", "-h"}} {
		if code, stdout, _ := runScanCapture(args...); code != 0 || stdout != scanUsage+"\n" {
			t.Errorf("%v: exit %d stdout %q, want the usage and exit 0", args, code, stdout)
		}
	}
}

func TestScanSecretsMaskKeepsFourRunesAndTheRuneCount(t *testing.T) {
	cases := map[string]string{
		"":               "",
		"abcd":           "****",
		"abcde":          "abcd*",
		"ſecret_key=xyz": "ſecr**********",
	}
	for in, want := range cases {
		if got := maskSecret(in); got != want {
			t.Errorf("maskSecret(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanSecretsIsARegisteredCommand(t *testing.T) {
	c := lookupCommand("scan")
	if c == nil || c.Run == nil {
		t.Fatalf("lookupCommand(\"scan\") = %v, want a runnable command", c)
	}
	if code := c.Run([]string{"--help"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("scan --help through the registry: exit %d", code)
	}
}

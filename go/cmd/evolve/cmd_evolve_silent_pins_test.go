package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeMalformedPolicy(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func TestConsensusDispatch_MalformedPolicyIsReportedNotSwallowed(t *testing.T) {
	root := t.TempDir()
	writeMalformedPolicy(t, root)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	rc := runConsensusDispatch(nil, nil, &stdout, &stderr)

	if rc == 0 {
		t.Errorf("a malformed policy.json must not dispatch under a silent default; rc=%d", rc)
	}
	if !strings.Contains(stderr.String(), "policy: parse "+filepath.Join(root, ".evolve", "policy.json")) {
		t.Errorf("stderr must carry the policy load error naming the file; stderr=%q", stderr.String())
	}
}

func TestConsensusDispatch_HelpNeedsNoPolicy(t *testing.T) {
	root := t.TempDir()
	writeMalformedPolicy(t, root)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runConsensusDispatch([]string{"--help"}, nil, &stdout, &stderr); rc != 0 {
		t.Errorf("--help must stay usable beside a broken policy.json; rc=%d stderr=%q", rc, stderr.String())
	}
}

func TestCycleHealth_RelativeProjectRootIsAbsolutized(t *testing.T) {
	for name, tc := range map[string]struct{ env, workspace string }{
		"dot root":      {".", filepath.Join(".evolve", "runs", "cycle-1")},
		"empty root":    {"", filepath.Join(".evolve", "runs", "cycle-1")},
		"relative name": {"proj", filepath.Join("proj", ".evolve", "runs", "cycle-1")},
	} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			chdirForTest(t, base)
			projectDir := base
			if tc.env == "proj" {
				projectDir = filepath.Join(base, "proj")
				if err := os.MkdirAll(projectDir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeMalformedPolicy(t, projectDir)
			t.Setenv("EVOLVE_PROJECT_ROOT", tc.env)
			var stdout, stderr bytes.Buffer

			rc := runCycleHealth([]string{"1", tc.workspace}, nil, &stdout, &stderr)

			m := regexp.MustCompile(`policy: parse (\S+?policy\.json)`).FindStringSubmatch(stderr.String())
			if rc != 1 || m == nil {
				t.Fatalf("a malformed policy must fail cycle-health naming the file; rc=%d stderr=%q", rc, stderr.String())
			}
			if !filepath.IsAbs(m[1]) {
				t.Errorf("the project root must be absolutized before any path is derived; policy path %q is relative", m[1])
			}
		})
	}
}

func TestWireOrchestratorDeps_MalformedPolicyWarnsOnTheGivenConsole(t *testing.T) {
	root := t.TempDir()
	writeMalformedPolicy(t, root)
	console := captureConsole(func(w io.Writer) {
		wireOrchestratorDeps(root, filepath.Join(root, ".evolve"), w)
	})
	if !strings.Contains(console, "policy") {
		t.Errorf("the composition root writes its policy WARN to the console it is given, not os.Stderr; console=%q", console)
	}
}

func TestCountCallExprs_CountsCallsNotText(t *testing.T) {
	for name, tc := range map[string]struct {
		src  string
		want int
	}{
		"comment only":       {"package p\n// WithSignalCenter( is wired here\nfunc f() {}\n", 0},
		"string only":        {"package p\nvar s = \"WithSignalCenter(\"\n", 0},
		"selector call":      {"package p\nfunc f() { _ = core.NewOrchestrator(a, core.WithSignalCenter(c)) }\n", 1},
		"bare call":          {"package p\nfunc f() { WithSignalCenter(c) }\n", 1},
		"two calls":          {"package p\nfunc f() { core.WithSignalCenter(a); core.WithSignalCenter(b) }\n", 2},
		"reference not call": {"package p\nvar f = core.WithSignalCenter\n", 0},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := countCallExprs([]byte(tc.src), "WithSignalCenter")
			if err != nil || got != tc.want {
				t.Errorf("countCallExprs = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
	if _, err := countCallExprs([]byte("not go"), "WithSignalCenter"); err == nil {
		t.Errorf("unparseable source must be an error, never a silent zero")
	}
}

func TestNilSignalCenterRootsPin_UsesTheASTCount(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle_signal_center_test.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if strings.Contains(body, `strings.Count(string(src), "WithSignalCenter(")`) {
		t.Errorf("TestNilSignalCenterRootsArePinned still counts comment text; a comment spelling the call would hide a missing real one")
	}
	if !strings.Contains(body, "countCallExprs(") {
		t.Errorf("TestNilSignalCenterRootsArePinned must count call expressions through countCallExprs")
	}
}

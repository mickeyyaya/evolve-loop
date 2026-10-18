package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

func TestRatchetVerbs_AreNamedInTheUsageAndTheRegistrySummary(t *testing.T) {
	t.Parallel()
	var summary string
	for _, c := range commands {
		if c.Name == "ratchet" {
			summary = c.Summary
		}
	}
	for verb := range ratchetVerbs {
		if !strings.Contains(ratchetUsage, "ratchet "+verb+" ") || !strings.Contains(summary, verb+" [") {
			t.Errorf("verb %q is dispatched but the usage %q or the registry summary %q does not name it", verb, ratchetUsage, summary)
		}
	}
}

func TestRatchet_CheckPassesOnTheRealModuleAndRejectsOtherVerbs(t *testing.T) {
	t.Parallel()
	if _, errOut, code := runDispatch("ratchet", "check", "--root", "../.."); code != 0 {
		t.Errorf("module must satisfy both ratchets: exit=%d stderr=%s", code, errOut)
	}
	for _, args := range [][]string{{"ratchet"}, {"ratchet", "loosen"}} {
		if _, errOut, code := runDispatch(args...); code != 2 || !strings.Contains(errOut, "usage") {
			t.Errorf("%v: exit=%d stderr=%q", args, code, errOut)
		}
	}
}

func TestRatchet_TightenRewritesTheListToTheModulesSlackAndCheckOnlyReports(t *testing.T) {
	root := ratchetFixtureRoot(t, 60, `{"internal/fixture.Big": 90, "internal/fixture.Gone": 60}`)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	listPath := filepath.Join(root, filepath.FromSlash(sizeratchet.OffendersRelPath))
	wantLines := "internal/fixture.Big: 90 -> 64\ninternal/fixture.Gone: 60 removed\n"

	out, _, code := runDispatch("ratchet", "tighten", "--check", "--root", root)

	if body, err := os.ReadFile(listPath); code != 1 || out != wantLines || err != nil || !strings.Contains(string(body), "Gone") {
		t.Fatalf("--check: exit=%d out=%q list=%s err=%v, want exit 1, the slack named and the list untouched", code, out, body, err)
	}
	if out, _, code := runDispatch("ratchet", "tighten", "--root", root, "--project-root", root); code != 0 || out != wantLines {
		t.Fatalf("tighten: exit=%d out=%q", code, out)
	}
	if body, err := os.ReadFile(listPath); err != nil || string(body) != "{\n  \"internal/fixture.Big\": 64\n}\n" {
		t.Fatalf("the list after tighten = %q, %v", body, err)
	}
	if out, _, code := runDispatch("ratchet", "tighten", "--check", "--root", root); code != 0 || !strings.Contains(out, "tight") {
		t.Errorf("a tight list: exit=%d out=%q", code, out)
	}
	cases := []struct {
		args     []string
		wantCode int
		wantErr  string
	}{
		{[]string{"ratchet", "tighten", "--root", filepath.Join(root, "absent")}, 1, "sizeratchet: walk"},
		{[]string{"ratchet", "tighten", "--bogus-flag"}, 2, "bogus-flag"},
		{[]string{"ratchet", "tighten", "--root", root, "junk"}, 2, "unexpected arguments"},
	}
	for _, c := range cases {
		if _, errOut, code := runDispatch(c.args...); code != c.wantCode || !strings.Contains(errOut, c.wantErr) {
			t.Errorf("%v: exit=%d stderr=%q, want exit %d naming %q", c.args, code, errOut, c.wantCode, c.wantErr)
		}
	}
}

func TestRatchet_TightenIsRefusedInsideAPhaseAndWhileACycleRuns(t *testing.T) {
	root := ratchetFixtureRoot(t, 60, `{"internal/fixture.Big": 90}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	listPath := filepath.Join(root, filepath.FromSlash(sizeratchet.OffendersRelPath))
	before, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "1")
	if _, errOut, code := runDispatch("ratchet", "tighten", "--root", root, "--project-root", root); code != 1 || !strings.Contains(errOut, "inside a phase") {
		t.Fatalf("in a phase: exit=%d stderr=%q, want the write refused", code, errOut)
	}
	if out, _, code := runDispatch("ratchet", "tighten", "--check", "--root", root); code != 1 || !strings.Contains(out, "90 -> 64") {
		t.Errorf("--check in a phase: exit=%d out=%q, want the slack reported", code, out)
	}
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runDispatch("ratchet", "tighten", "--root", root, "--project-root", root); code != 1 || !strings.Contains(errOut, "cycle-7") {
		t.Fatalf("under a live lease: exit=%d stderr=%q, want the write refused naming the run", code, errOut)
	}
	if _, errOut, code := runDispatch("ratchet", "tighten", "--root", root); code != 1 || !strings.Contains(errOut, "cycle-7") {
		t.Fatalf("the project root defaults to EVOLVE_PROJECT_ROOT: exit=%d stderr=%q", code, errOut)
	}
	if after, err := os.ReadFile(listPath); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("a refused write leaves the list unchanged: %q, %v", after, err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	defer func(getwd func() (string, error)) { processGetwd = getwd }(processGetwd)
	processGetwd = func() (string, error) { return "", errors.New("no cwd") }
	if _, errOut, code := runDispatch("ratchet", "tighten", "--root", root); code != 1 || !strings.Contains(errOut, "pass --project-root") {
		t.Errorf("an unreadable working directory: exit=%d stderr=%q", code, errOut)
	}
}

func TestRatchet_TightenRefusesAListItCannotWrite(t *testing.T) {
	root := ratchetFixtureRoot(t, 60, `{"internal/fixture.Big": 90}`)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	listPath := filepath.Join(root, filepath.FromSlash(sizeratchet.OffendersRelPath))
	if err := os.Chmod(listPath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(listPath, 0o644) })
	if f, err := os.OpenFile(listPath, os.O_WRONLY, 0); err == nil {
		_ = f.Close()
		t.Skip("file permissions are not enforced for this user")
	}

	out, errOut, code := runDispatch("ratchet", "tighten", "--root", root)

	if code != 1 || !strings.Contains(errOut, "offenders.json") || !strings.Contains(out, "90 -> 64") {
		t.Fatalf("exit=%d out=%q stderr=%q, want exit 1 naming the list after the slack report", code, out, errOut)
	}
}

func ratchetFixtureRoot(t *testing.T, funcLines int, offenders string) string {
	t.Helper()
	root := t.TempDir()
	var src strings.Builder
	src.WriteString("package fixture\n\nfunc Big() int {\n\ttotal := 0\n")
	for i := 0; i < funcLines; i++ {
		fmt.Fprintf(&src, "\ttotal += %d\n", i)
	}
	src.WriteString("\treturn total\n}\n")
	files := map[string]string{
		"internal/fixture/big.go":      src.String(),
		"internal/fixture/raw_test.go": "package fixture\n\nimport \"os/exec\"\n\nfunc raw() { exec.Command(\"git\", \"init\").Run() }\n",
		sizeratchet.OffendersRelPath:   offenders,
		rawgitratchet.BaselineRelPath:  "{}",
	}
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRatchet_SelectorScansOnlyTheNamedRatchetUnderRoot(t *testing.T) {
	t.Parallel()
	root := ratchetFixtureRoot(t, 80, `{"internal/fixture.Big": 60}`)
	cases := []struct {
		args     []string
		wantCode int
		wantErr  string
	}{
		{[]string{"ratchet", "check", "size", "--root", root}, 1, "internal/fixture.Big grew to"},
		{[]string{"ratchet", "check", "--root", root, "size"}, 1, "internal/fixture.Big grew to"},
		{[]string{"ratchet", "check", "rawgit", "--root", root}, 1, "internal/fixture/raw_test.go"},
		{[]string{"ratchet", "check", "--root", root}, 1, "raw_test.go"},
		{[]string{"ratchet", "check", "bogus", "--root", root}, 2, "unknown ratchet"},
		{[]string{"ratchet", "check", "--root", root, "junk"}, 2, "unknown ratchet"},
		{[]string{"ratchet", "check", "size", "--root", root, "junk"}, 2, "unexpected arguments"},
		{[]string{"ratchet", "check", "--bogus-flag"}, 2, "bogus-flag"},
	}
	for _, c := range cases {
		out, errOut, code := runDispatch(c.args...)
		if code != c.wantCode || !strings.Contains(errOut, c.wantErr) || strings.Contains(out, "clean") {
			t.Errorf("%v: exit=%d out=%q stderr=%q, want exit %d naming %q", c.args, code, out, errOut, c.wantCode, c.wantErr)
		}
	}
	if _, errOut, code := runDispatch("ratchet", "check", "size", "--root", ratchetFixtureRoot(t, 80, `{"internal/fixture.Big": 200}`)); code != 0 {
		t.Errorf("check size must not run the raw-git ratchet: exit=%d stderr=%q", code, errOut)
	}
}

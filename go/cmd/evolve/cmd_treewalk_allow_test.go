package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type treewalkFixture struct {
	root     string
	baseline string
}

func newTreewalkFixture(t *testing.T) treewalkFixture {
	t.Helper()
	f := treewalkFixture{root: t.TempDir(), baseline: filepath.Join(t.TempDir(), "baseline.json")}
	if err := os.MkdirAll(filepath.Join(f.root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runTreewalk([]string{"snapshot", "--root", f.root, "--out", f.baseline}, nil, &out, &errb); code != 0 {
		t.Fatalf("snapshot = %d: %s", code, errb.String())
	}
	return f
}

func (f treewalkFixture) plant(t *testing.T, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		full := filepath.Join(f.root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func (f treewalkFixture) check(extra ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	args := append([]string{"check", "--root", f.root, "--baseline", f.baseline}, extra...)
	code = runTreewalk(args, nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunTreewalkCheckAllowSkipsOnlyTheListedPrefixes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		planted    []string
		allow      []string
		wantCode   int
		wantLeaked []string
		wantAbsent []string
	}{
		{"every new path allowed", []string{"bin/evolve", "coverage.txt"}, []string{"--allow", "bin", "--allow", "coverage.txt"}, 0, nil, []string{"LEAK"}},
		{"leaked evolve directory outside the allow list", []string{"bin/evolve", ".evolve/ledger.ndjson"}, []string{"--allow", "bin"}, 1, []string{"LEAK .evolve", filepath.Join("LEAK .evolve", "ledger.ndjson")}, []string{"LEAK bin"}},
		{"leak nested in a package directory", []string{"pkg/.evolve/state.json"}, []string{"--allow", "bin"}, 1, []string{filepath.Join("LEAK pkg", ".evolve")}, nil},
		{"allow entry equal to a file name does not allow a sibling", []string{"coverage.txt", "coverage.txt.bak"}, []string{"--allow", "coverage.txt"}, 1, []string{"LEAK coverage.txt.bak"}, []string{"LEAK coverage.txt\n"}},
		{"allow prefix stops at a path component", []string{"binary/x"}, []string{"--allow", "bin"}, 1, []string{"LEAK binary"}, nil},
		{"equals form of the flag", []string{"bin/evolve"}, []string{"--allow=bin"}, 0, nil, []string{"LEAK"}},
		{"no allow flag keeps every leak", []string{"bin/evolve"}, nil, 1, []string{"LEAK bin"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newTreewalkFixture(t)
			f.plant(t, c.planted...)
			code, stdout, stderr := f.check(c.allow...)
			if code != c.wantCode {
				t.Fatalf("check = %d, want %d\nstdout: %s\nstderr: %s", code, c.wantCode, stdout, stderr)
			}
			for _, want := range c.wantLeaked {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout lacks %q:\n%s", want, stdout)
				}
			}
			for _, absent := range c.wantAbsent {
				if strings.Contains(stdout, absent) {
					t.Errorf("stdout has %q, which the allow list covers:\n%s", absent, stdout)
				}
			}
			if c.wantCode == 0 && !strings.Contains(stdout, "clean") {
				t.Errorf("a passing check does not say clean:\n%s", stdout)
			}
		})
	}
}

func TestRunTreewalkCheckCountsOnlyTheLeaksThatRemainAfterTheAllowList(t *testing.T) {
	t.Parallel()
	f := newTreewalkFixture(t)
	f.plant(t, "bin/a", "bin/b", "bin/c", ".evolve/ledger.ndjson")
	code, _, stderr := f.check("--allow", "bin")
	if code != 1 || !strings.Contains(stderr, "2 path(s)") {
		t.Errorf("check = %d, stderr = %q; want exit 1 and 2 path(s) (.evolve and its file)", code, stderr)
	}
}

func TestRunTreewalkCheckRejectsAnEmptyAllowEntry(t *testing.T) {
	t.Parallel()
	f := newTreewalkFixture(t)
	f.plant(t, ".evolve/ledger.ndjson")
	for _, extra := range [][]string{{"--allow", ""}, {"--allow="}, {"--allow", "bin", "--allow", ""}} {
		code, stdout, stderr := f.check(extra...)
		if code != 10 {
			t.Errorf("check %v = %d, want usage exit 10 (an empty prefix would allow every path)\nstdout: %s\nstderr: %s", extra, code, stdout, stderr)
		}
		if strings.Contains(stdout, "clean") {
			t.Errorf("check %v reported a clean tree over a leaked .evolve:\n%s", extra, stdout)
		}
	}
}

func TestRunTreewalkCheckRejectsAFlagWithoutAValueAndHelpListsAllow(t *testing.T) {
	t.Parallel()
	f := newTreewalkFixture(t)
	if code, _, _ := f.check("--allow"); code != 10 {
		t.Errorf("check with a bare --allow = %d, want usage exit 10", code)
	}
	var out, errb bytes.Buffer
	if code := runTreewalk([]string{"check", "-h"}, nil, &out, &errb); code != 10 {
		t.Errorf("check -h = %d, want usage exit 10", code)
	}
	if !strings.Contains(errb.String(), "-allow") {
		t.Errorf("check -h does not describe -allow:\n%s", errb.String())
	}
	out.Reset()
	if code := runTreewalk([]string{"help"}, nil, &out, &errb); code != 0 || !strings.Contains(out.String(), "--allow") {
		t.Errorf("treewalk help = %d, usage lacks --allow:\n%s", code, out.String())
	}
}

func TestRunTreewalkCheckReportsAnUnreadableBaselineOrRoot(t *testing.T) {
	t.Parallel()
	f := newTreewalkFixture(t)
	malformed := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"missing baseline file", []string{"check", "--root", f.root, "--baseline", filepath.Join(t.TempDir(), "absent.json"), "--allow", "bin"}},
		{"malformed baseline file", []string{"check", "--root", f.root, "--baseline", malformed, "--allow", "bin"}},
		{"root that no longer exists", []string{"check", "--root", filepath.Join(t.TempDir(), "absent"), "--baseline", f.baseline, "--allow", "bin"}},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if code := runTreewalk(c.args, nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "evolve treewalk check:") {
			t.Errorf("%s: code=%d stderr=%q, want exit 1 with the check prefix", c.name, code, errb.String())
		}
	}
}

func TestRunTreewalkCheckRejectsAPositionalArgument(t *testing.T) {
	t.Parallel()
	f := newTreewalkFixture(t)
	if code, _, _ := f.check("--allow", "bin", "extra"); code != 10 {
		t.Errorf("check with a positional argument = %d, want usage exit 10", code)
	}
}

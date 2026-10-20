package rawgitratchet

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var (
	gitLit  = strconv.Quote("gi" + "t")
	initLit = strconv.Quote("in" + "it")
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, src := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func noGitAbove(t *testing.T, dir string) {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(real))
}

const passTest = "package p\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) {}\n"

func TestBoundTestFiles_BindsTrackedOnly(t *testing.T) {
	r := gittest.Fixture(t)
	writeFiles(t, r.Dir, map[string]string{
		"a/tracked_test.go":    passTest,
		"b/untracked_test.go":  passTest,
		"a/testdata/x_test.go": passTest,
		".hidden/x_test.go":    passTest,
		"a/code.go":            "package p\n",
	})
	r.Git("add", "a", ".hidden")
	files, note, err := BoundTestFiles(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" {
		t.Errorf("note = %q in a work tree, want empty", note)
	}
	if strings.Join(files, ",") != "a/tracked_test.go" {
		t.Errorf("BoundTestFiles = %v, want only the staged a/tracked_test.go", files)
	}
}

func TestBoundTestFiles_OutsideGitBindsEveryFile(t *testing.T) {
	root := t.TempDir()
	noGitAbove(t, root)
	writeFiles(t, root, map[string]string{"a/x_test.go": passTest, "b/c/y_test.go": passTest, "_skip/z_test.go": passTest})
	files, note, err := BoundTestFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(files, ",") != "a/x_test.go,b/c/y_test.go" {
		t.Errorf("BoundTestFiles = %v, want every on-disk test file", files)
	}
	if !strings.Contains(note, "binding every on-disk test file") {
		t.Errorf("note = %q, want the bind-all fallback named", note)
	}

	empty := t.TempDir()
	noGitAbove(t, empty)
	if _, _, err := BoundTestFiles(empty); err == nil || !strings.Contains(err.Error(), "no test files bound") {
		t.Errorf("BoundTestFiles on a module without tests: err = %v, want the empty-set error", err)
	}
}

func TestSites_CountsEachShapeOutsideTheOwner(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a/literal_test.go":             "package a\nimport \"os/exec\"\nfunc f() { exec.Command(" + gitLit + ", " + initLit + ").Run() }\n",
		"b/helper_test.go":              "package b\nimport \"os/exec\"\nfunc run(args ...string) { exec.Command(" + gitLit + ", args...).Run() }\n",
		"b/caller_test.go":              "package b\nfunc g() { run(" + initLit + ", \"-q\"); run(" + initLit + ") }\n",
		"c/table_test.go":               "package c\nvar steps = [][]string{{" + initLit + "}, {" + gitLit + "}}\n",
		"d/nogit_test.go":               "package d\nfunc h(string) {}\nfunc g() { h(" + initLit + ") }\n",
		"e/noinit_test.go":              "package e\nimport \"os/exec\"\nfunc f() { exec.Command(" + gitLit + ", \"status\").Run() }\n",
		"f/switch_test.go":              "package f\nvar bin = " + gitLit + "\nfunc g(s string) bool { switch s { case " + initLit + ": return true }; return false }\n",
		OwnerDir + "/fixture_test.go":   "package gittest\nimport \"os/exec\"\nfunc f() { exec.Command(" + gitLit + ", " + initLit + ").Run() }\n",
		OwnerDir + "/sub/extra_test.go": "package sub\nimport \"os/exec\"\nfunc f() { exec.Command(" + gitLit + ", " + initLit + ").Run() }\n",
	}
	writeFiles(t, root, files)
	var rels []string
	for rel := range files {
		rels = append(rels, rel)
	}
	rels = append(rels, "g/gone_test.go")
	got, err := Sites(root, rels)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"a/literal_test.go": 1, "b/caller_test.go": 2, "c/table_test.go": 1}
	if len(got) != len(want) {
		t.Errorf("Sites = %v, want %v", got, want)
	}
	for rel, n := range want {
		if got[rel] != n {
			t.Errorf("Sites[%s] = %d, want %d (all: %v)", rel, got[rel], n, got)
		}
	}
}

func TestSites_UnparsableFileFails(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"x/broken_test.go": "package x\nfunc {"})
	if _, err := Sites(root, []string{"x/broken_test.go"}); err == nil || !strings.Contains(err.Error(), "x/broken_test.go") {
		t.Errorf("Sites on an unparsable file: err = %v, want a parse error naming x/broken_test.go", err)
	}
}

func TestLoadBaseline_ReadsCountsAndRejectsBadLists(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, body, wantErr string
	}{
		{"valid", `{"a/x_test.go": 2}`, ""},
		{"zero count", `{"a/x_test.go": 0}`, "remove the entry"},
		{"not an object", `null`, "not a JSON object"},
		{"malformed", `{`, "parse baseline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".json")
			if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := LoadBaseline(p)
			if tc.wantErr == "" {
				if err != nil || got["a/x_test.go"] != 2 {
					t.Errorf("LoadBaseline = %v, %v; want {a/x_test.go: 2}", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("LoadBaseline: err = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
	if _, err := LoadBaseline(filepath.Join(dir, "absent.json")); err == nil {
		t.Error("LoadBaseline of a missing file must fail")
	}
}

func TestCheck_TheBaselineIsACeiling(t *testing.T) {
	baseline := map[string]int{"a_test.go": 2, "b_test.go": 1}
	for name, sites := range map[string]map[string]int{
		"exact match":   {"a_test.go": 2, "b_test.go": 1},
		"shrunk count":  {"a_test.go": 1, "b_test.go": 1},
		"migrated file": {"a_test.go": 2},
	} {
		if err := Check(sites, baseline); err != nil {
			t.Errorf("%s: %v; a lane that removes sites passes without an edit to baseline.json", name, err)
		}
	}
	cases := []struct {
		name  string
		sites map[string]int
		want  string
	}{
		{"unlisted file", map[string]int{"a_test.go": 2, "b_test.go": 1, "new_test.go": 1}, "new_test.go builds a raw git repo"},
		{"grown count", map[string]int{"a_test.go": 3, "b_test.go": 1}, "a_test.go grew to 3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Check(tc.sites, baseline)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check: err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

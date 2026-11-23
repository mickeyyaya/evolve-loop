package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func archiveLeafFixture(t *testing.T) (root, leaf, head string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("f.txt", "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	head = repo.Git("rev-parse", "HEAD")
	leaf = "cycle-cd3ae73e-77"
	dir := filepath.Join(repo.Dir, ".evolve", "operator-salvage", leaf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	headBody := head + " " + leaf + "\n"
	if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(headBody), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(headBody))
	body, err := json.Marshal(map[string]any{"cycle": 77, "branch": leaf, "head": head, "ref": "refs/archive/cycles/" + leaf,
		"files": map[string]string{"HEAD": hex.EncodeToString(sum[:])}})
	if err != nil {
		t.Fatal(err)
	}
	msum := sha256.Sum256(body)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.sha256"), []byte(hex.EncodeToString(msum[:])+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo.Dir, leaf, head
}

func TestRunArchive_RestoresACycleAndRefusesBadInput(t *testing.T) {
	t.Parallel()
	root, leaf, head := archiveLeafFixture(t)
	dest := filepath.Join(t.TempDir(), "restored")

	var out, errOut bytes.Buffer
	code := runArchive([]string{"restore", "77", "--project-root", root, "--worktree", dest}, nil, &out, &errOut)

	if code != 0 || !strings.Contains(out.String(), leaf) || !strings.Contains(out.String(), head) {
		t.Fatalf("code %d\n%s%s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "f.txt")); err != nil {
		t.Errorf("restored worktree lacks f.txt: %v", err)
	}
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no subcommand", nil, 10, "usage"},
		{"unknown subcommand", []string{"nope"}, 10, "unknown subcommand"},
		{"no cycle", []string{"restore", "--project-root", root}, 10, "one cycle number"},
		{"not a number", []string{"restore", "abc", "--project-root", root}, 10, "one cycle number"},
		{"extra argument", []string{"restore", "77", "extra", "--project-root", root}, 10, "one cycle number"},
		{"bad flag", []string{"restore", "77", "--nope"}, 10, ""},
		{"unknown cycle", []string{"restore", "5", "--project-root", root}, 1, "cycle 5"},
		{"existing branch", []string{"restore", "77", "--project-root", root, "--worktree", filepath.Join(t.TempDir(), "again")}, 1, "already exists"},
	}
	for _, c := range cases {
		var o, e bytes.Buffer
		if got := runArchive(c.args, nil, &o, &e); got != c.code || !strings.Contains(e.String(), c.want) {
			t.Errorf("%s: code %d stderr %q, want %d containing %q", c.name, got, e.String(), c.code, c.want)
		}
	}
}

func TestSplitLeadingArg_TakesTheFirstWordOnlyWhenItIsNotAFlag(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   []string
		arg  string
		rest []string
	}{
		{nil, "", nil},
		{[]string{"7", "--x"}, "7", []string{"--x"}},
		{[]string{"--x", "7"}, "", []string{"--x", "7"}},
		{[]string{""}, "", []string{""}},
	} {
		arg, rest := splitLeadingArg(c.in)
		if arg != c.arg || !reflect.DeepEqual(rest, c.rest) {
			t.Errorf("splitLeadingArg(%q) = %q, %q; want %q, %q", c.in, arg, rest, c.arg, c.rest)
		}
	}
}

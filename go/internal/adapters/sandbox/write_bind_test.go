package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBwrapPrefix_AMissingWritePathIsBoundWithBindTry(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	argv := BwrapPrefix(Config{RepoRoot: "/repo", ReadOnlyRepo: true, WritePaths: []string{missing}})
	mustContainArg(t, argv, "--bind-try", missing, missing)
	assertNoArgv(t, argv, "--bind", missing, missing)
}

func TestBwrapPrefix_AnExistingWritePathKeepsStrictBind(t *testing.T) {
	present := t.TempDir()
	argv := BwrapPrefix(Config{RepoRoot: "/repo", ReadOnlyRepo: true, WritePaths: []string{present}})
	mustContainArg(t, argv, "--bind", present, present)
	assertNoArgv(t, argv, "--bind-try", present, present)
}

func TestBwrapPrefix_AGlobWithAMissingParentWidensToABindTry(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "runs")
	argv := BwrapPrefix(Config{RepoRoot: "/repo", WritePaths: []string{filepath.Join(parent, "cycle-*")}})
	mustContainArg(t, argv, "--bind-try", parent, parent)
	assertNoArgv(t, argv, "--bind", parent, parent)
}

func TestBwrapPrefix_AGlobWithAnExistingParentWidensToAStrictBind(t *testing.T) {
	parent := t.TempDir()
	argv := BwrapPrefix(Config{RepoRoot: "/repo", WritePaths: []string{filepath.Join(parent, "cycle-*")}})
	mustContainArg(t, argv, "--bind", parent, parent)
}

func TestBwrapPrefix_AMissingPathThatHoldsTheRepoIsBoundWithBindTry(t *testing.T) {
	holder := filepath.Join(t.TempDir(), "absent")
	repo := filepath.Join(holder, "repo")
	argv := BwrapPrefix(Config{RepoRoot: repo, WritePaths: []string{holder}})
	mustContainArg(t, argv, "--bind-try", holder, holder)
	assertNoArgv(t, argv, "--bind", holder, holder)
}

func TestBwrapPrefix_NoStrictBindNamesASourceThatDoesNotExist(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "a", "b")
	cfg := Config{RepoRoot: root, ReadOnlyRepo: true, WritePaths: []string{missing, filepath.Join(root, "g", "x-*"), root}}
	argv := BwrapPrefix(cfg)
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] != "--bind" {
			continue
		}
		if _, err := os.Stat(argv[i+1]); err != nil {
			t.Errorf("bwrap rejects --bind on a missing source %q: %v", argv[i+1], err)
		}
	}
}

func assertNoArgv(t *testing.T, args []string, parts ...string) {
	t.Helper()
	if containsArgSequence(args, parts...) {
		t.Errorf("argv must not contain %v\n  full: %v", parts, args)
	}
}

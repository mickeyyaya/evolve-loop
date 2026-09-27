package core

import (
	"context"
	"strings"
	"testing"
)

func TestBuildGraduationCheck_MessageEmitsTheExactTwoEdits(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")

	got := buildGraduationCheck(context.Background(), wt)
	if got == "" {
		t.Fatal("premise broken: an ungraduated new package must still abort")
	}
	for _, want := range []string{
		// Edit 1: the file to append to, and the verbatim line to append.
		"go/.apicover-enforce",
		"./internal/brandnew",
		// Edit 2: the exact test file path to create, derived from the package.
		"go/internal/brandnew/apicover_named_test.go",
		// The obligation the test file must discharge — a bare empty file passes
		// the enforce list and then fails the repo-wide unnamed-export gate.
		"every exported symbol",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("graduation abort reason must be self-serving and contain %q;\ngot: %s", want, got)
		}
	}
}

func TestBuildGraduationCheck_MessageIsPerPackage(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/alpha/a.go", "package alpha\n")
	gradWrite(t, wt, "go/internal/beta/b.go", "package beta\n")

	got := buildGraduationCheck(context.Background(), wt)
	for _, want := range []string{
		"./internal/alpha",
		"go/internal/alpha/apicover_named_test.go",
		"./internal/beta",
		"go/internal/beta/apicover_named_test.go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("per-package prescription missing %q;\ngot: %s", want, got)
		}
	}
}

func TestBuildGraduationCheck_MessageDistinguishesTheTwoApicoverGates(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")

	got := buildGraduationCheck(context.Background(), wt)
	if !strings.Contains(got, "repo-wide") {
		t.Errorf("reason must name the repo-wide enforce gate (not the per-cycle ACS gate — ADR-0069);\ngot: %s", got)
	}
}

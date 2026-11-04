package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const finalRedOutput = "--- FAIL: TestRedUnderTag (0.01s)\n    x_test.go:9: open %s: no such file\nFAIL\texample.com/m/p\t0.2s\n"

func finalRedWith(dir string) string {
	return fmt.Sprintf(finalRedOutput, dir)
}

func TestCoverChangedFailures_ARedTheRetakeConfirmedIsNotCancelledByAGreenUnitRun(t *testing.T) {
	repo, base := coverChangedRepo(t)
	stubCoverChangedProfiler(t, coverChangedProfile, finalRedWith("/tmp/a"), coverStatusTestsFailed)
	unitCalls := stubUnitRunner(t, false, true, true, true)

	failures := coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base), []string{"go/p/a.go"})

	if len(failures) != 1 || !strings.Contains(failures[0], "TestRedUnderTag") {
		t.Fatalf("failures = %v after %d unit run(s), want the confirmed red to stand", failures, len(*unitCalls))
	}
	if len(*unitCalls) != 0 {
		t.Errorf("unit runs = %d, want 0: no later run may overrule a confirmed red", len(*unitCalls))
	}
}

func TestFindingIdentity_OneGoTestRedIsOneIdentityOnAllThreeRenderers(t *testing.T) {
	t.Parallel()

	render := func(dir string) [3]string {
		out := finalRedWith(dir)
		return [3]string{
			floorLinesFor([]selfCheckFailure{{Pkg: "./p", Output: out}}, nil, nil)[0],
			fmt.Sprintf("enforced package tests FAIL (coverage run doubles as their selfcheck):\n%s", out),
			coverChangedTestsFailedHeadline + ":\n    " + floorFailureDiagnostic(out),
		}
	}
	first, second := render("/var/folders/aa/T/TestX1/001"), render("/var/folders/bb/T/TestX2/001")

	for i := range first {
		if floorFindingsConverge([]string{first[i]}, []string{second[i]}) {
			t.Errorf("renderer %d: the same red under another temp dir reads as converging", i)
		}
		if findingIdentity(first[i]) != findingIdentity(first[0]) {
			t.Errorf("renderer %d identity = %q, want %q", i, findingIdentity(first[i]), findingIdentity(first[0]))
		}
	}
}

func TestAddedTaggedTestFailures_NeverRunsAnEnvExclusivePackage(t *testing.T) {
	in, _ := retakeRepo(t)
	writeFile(t, filepath.Join(in.Worktree, "go", "internal", "bridge", "b_test.go"), "//go:build integration\n\npackage bridge\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n")
	var ran []string
	prev := buildSelfCheckTaggedRunner
	buildSelfCheckTaggedRunner = func(_ context.Context, _, pkg string, _ []string) (string, bool) {
		ran = append(ran, pkg)
		return finalRedWith("/tmp/a"), false
	}
	t.Cleanup(func() { buildSelfCheckTaggedRunner = prev })

	got := addedTaggedTestFailures(context.Background(), in, filepath.Join(in.Worktree, "go"))

	if len(got) != 0 || len(ran) != 0 {
		t.Fatalf("failures = %v, runs = %v, want the env-exclusive internal/bridge left to its backstop", got, ran)
	}
}

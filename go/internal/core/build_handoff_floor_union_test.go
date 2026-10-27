package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func unionFloorWorktree(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	standard, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(stelint.StandardPath)))
	if err != nil {
		t.Fatalf("read the standard: %v", err)
	}
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(stelint.StandardPath)), string(standard))
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\treturn x\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n")
	writeFile(t, filepath.Join(repo.Dir, "docs", "new.md"), "# New\n\nIntro.\n\n"+longSentence(30)+"\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build")
	return repo, base
}

func docsAndCoverFloor(ctx context.Context, in ReviewInput) []string {
	paths := changedFloorPaths(ctx, in)
	return append(docsFloorChecks(ctx, in, paths), coverChangedFailures(ctx, in, paths)...)
}

func TestExplanationGap_OneRejectionAlsoListsTheSteAndCoverageGaps(t *testing.T) {
	repo, base := unionFloorWorktree(t)
	stubCoverChangedProfiler(t, coverChangedProfile, "", coverStatusOK)
	root := floorRoot(t, `{"docs_floor":{"ste_stage":"enforce"},"cover_changed":{"stage":"enforce"}}`)
	in := floorInput(t, root, repo, base)
	in.ExplanationDocumentationVersion = explanationdocs.CurrentContractVersion
	var sink injectedStderr
	in.Stderr = &sink
	r := withMandatoryExplanationReviewer(BuildHandoffFloor{{Name: "production", Run: docsAndCoverFloor}})

	got := r.Review(context.Background(), in)

	if got.Approve || !got.Retry || len(got.Failures) < 3 {
		t.Fatalf("one rejection must carry all three gaps: got %+v", got)
	}
	wants := map[string]string{
		"explanation": "Explanation Documentation",
		"ste":         "[ste-lint] the changed documents fail the STE lint: 1 finding(s) in 1 file(s) (first: docs/new.md:5 STE-SENTENCE)",
		"cover":       "p/a.go:3 p.F 66.7% (2/3)",
	}
	for kind, want := range wants {
		if !anyFailureContains(got.Failures, want) {
			t.Errorf("the %s gap is missing from the one rejection; want %q in %q", kind, want, got.Failures)
		}
	}
	if !strings.HasPrefix(got.Reason, "build handoff floor: ") {
		t.Errorf("reason = %q, want the build handoff floor rejection", got.Reason)
	}
}

func anyFailureContains(failures []string, want string) bool {
	for _, f := range failures {
		if strings.Contains(f, want) {
			return true
		}
	}
	return false
}

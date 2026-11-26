package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
)

func init() {
	RoundEvidenceResolver = existingFileResolver
}

func existingFileResolver(evidence string, req defectledger.Request) (bool, string) {
	path, _, _ := strings.Cut(strings.TrimSpace(evidence), ":")
	if path == "" || strings.ContainsAny(path, " \t") || strings.Contains(path, "..") {
		return false, "no resolvable citation"
	}
	if _, err := os.Stat(filepath.Join(req.Worktree, path)); err != nil {
		return false, "the cited file cannot be read"
	}
	return true, ""
}

func TestClaimsFix_WithNoResolverWiredNoClaimIsFixed(t *testing.T) {
	saved := RoundEvidenceResolver
	RoundEvidenceResolver = nil
	t.Cleanup(func() { RoundEvidenceResolver = saved })
	claim := defectledger.Entry{ID: "x", Status: defectledger.StatusFixed, Evidence: "b.go:1"}
	if claimsFix(claim, t.TempDir()) {
		t.Fatal("a FIXED claim counted as fixed with no evidence resolver wired")
	}
}

func TestClaimsFix_AsksTheWiredResolverAboutTheWorktree(t *testing.T) {
	saved := RoundEvidenceResolver
	var got defectledger.Request
	RoundEvidenceResolver = func(_ string, req defectledger.Request) (bool, string) {
		got = req
		return true, ""
	}
	t.Cleanup(func() { RoundEvidenceResolver = saved })
	wt := t.TempDir()
	if !claimsFix(defectledger.Entry{Status: defectledger.StatusFixed, Evidence: "b.go:1"}, wt) {
		t.Fatal("a FIXED claim the resolver accepts must count as fixed")
	}
	if got.ProjectRoot != wt || got.Worktree != wt {
		t.Fatalf("resolver request = %+v, want the worktree as the root", got)
	}
}

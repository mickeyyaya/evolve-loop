package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

const issueCoverageFixture = "# Index\n\n## Summary\n\n<!-- GENERATED:coverage-summary BEGIN -->\nold\n<!-- GENERATED:coverage-summary END -->\n\n## Coverage map\n\n| Incident | Failure mode | Suspect file | Coverage | Pinning test / proposal |\n|---|---|---|---|---|\n| [one](a.md) | mode | `x.go` | ✅ | `TestA` |\n| [two](b.md) | mode | `y.go` | ❌ | item `z` |\n"

func writeCoverageIndex(t *testing.T, body string) (root, path string) {
	t.Helper()
	root = t.TempDir()
	path = filepath.Join(root, filepath.FromSlash(incidentkb.CoverageIndexRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestRunIssueCoverage_CheckFailsOnAStaleSummaryAndPassesAfterGenerate(t *testing.T) {
	t.Parallel()
	root, path := writeCoverageIndex(t, issueCoverageFixture)
	if out, code := runIssueArgs("coverage", "check", "--project-root", root); code != 1 || !strings.Contains(out, "evolve issue coverage generate") {
		t.Fatalf("check of a stale summary = %d, want 1 naming the generate verb:\n%s", code, out)
	}
	if out, code := runIssueArgs("coverage", "generate", "--project-root", root); code != 0 || !strings.Contains(out, "rows=2") {
		t.Fatalf("generate = %d, want 0 with the counts:\n%s", code, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), "| Incidents mapped | 2 |") || strings.Contains(string(raw), "\nold\n") {
		t.Fatalf("generate did not write the summary: %v\n%s", err, raw)
	}
	if out, code := runIssueArgs("coverage", "check", "--project-root", root); code != 0 {
		t.Fatalf("check after generate = %d, want 0:\n%s", code, out)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, code := runIssueArgs("coverage", "generate", "--project-root", root); code != 0 {
		t.Fatal("a second generate must pass")
	}
	if again, err := os.Stat(path); err != nil || !again.ModTime().Equal(info.ModTime()) {
		t.Fatalf("a generate with nothing to change must not rewrite the file: %v", err)
	}
}

func TestRunIssueCoverage_ABadRowFailsBothVerbsWithItsLine(t *testing.T) {
	t.Parallel()
	root, _ := writeCoverageIndex(t, issueCoverageFixture+"| plain | mode | `x.go` | ✅ | `TestC` |\n")
	for _, verb := range []string{"check", "generate"} {
		if out, code := runIssueArgs("coverage", verb, "--project-root", root); code != 1 || !strings.Contains(out, "line 15") {
			t.Errorf("%s with a plain incident cell = %d, want 1 naming line 15:\n%s", verb, code, out)
		}
	}
}

func TestRunIssueCoverage_AMissingRegionAndAnUnwritableIndexFailGenerate(t *testing.T) {
	t.Parallel()
	noRegion := strings.Replace(issueCoverageFixture, "<!-- GENERATED:coverage-summary END -->", "", 1)
	root, _ := writeCoverageIndex(t, noRegion)
	if out, code := runIssueArgs("coverage", "generate", "--project-root", root); code != 1 || !strings.Contains(out, "missing generated marker") {
		t.Fatalf("generate without the region end = %d, want 1 naming the marker:\n%s", code, out)
	}
	root, path := writeCoverageIndex(t, issueCoverageFixture)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if out, code := runIssueArgs("coverage", "generate", "--project-root", root); code != 1 {
		t.Fatalf("generate into a read-only directory = %d, want 1:\n%s", code, out)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != issueCoverageFixture {
		t.Fatalf("a failed write must leave the index unchanged: %v", err)
	}
}

func TestRunIssueCoverage_AMissingIndexFailsAndABadCallIsUsage(t *testing.T) {
	t.Parallel()
	if out, code := runIssueArgs("coverage", "check", "--project-root", t.TempDir()); code != 1 {
		t.Fatalf("check without the index = %d, want 1:\n%s", code, out)
	}
	for _, args := range [][]string{{"coverage"}, {"coverage", "render"}, {"coverage", "check", "extra"}, {"coverage", "check", "--bogus"}} {
		if out, code := runIssueArgs(args...); code != 10 {
			t.Errorf("runIssue(%v) = %d, want 10:\n%s", args, code, out)
		}
	}
}

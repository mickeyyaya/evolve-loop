package reportdoc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSection_UsesOnlyVisibleUniqueHeadings(t *testing.T) {
	report := `<!--
## Explanation Documentation
- Status: VERIFIED
-->

~~~~markdown
## Explanation Documentation
- Status: VERIFIED
## Hidden Boundary
~~~~

    ## Explanation Documentation
    - Status: VERIFIED

## Explanation Documentation
- Status: NEEDS_CORRECTION
- Evidence: visible
`
	body, ok, err := Section(report, "Explanation Documentation")
	if err != nil || !ok {
		t.Fatalf("Section ok=%v err=%v", ok, err)
	}
	if strings.Contains(body, "VERIFIED") || !strings.Contains(body, "NEEDS_CORRECTION") || !strings.Contains(body, "visible") {
		t.Fatalf("Section returned hidden or lost visible content: %q", body)
	}
}

func TestSection_FencedHeadingDoesNotTerminateSection(t *testing.T) {
	report := `## Explanation Documentation
- Status: VERIFIED
~~~markdown
## Fake Next Section
~~~
- Evidence: checked after the fenced example

## Real Next Section
done
`
	body, ok, err := Section(report, "Explanation Documentation")
	if err != nil || !ok || !strings.Contains(body, "checked after") {
		t.Fatalf("Section body=%q ok=%v err=%v", body, ok, err)
	}
}

func TestSection_RejectsDuplicateVisibleHeading(t *testing.T) {
	_, ok, err := Section("## Review\none\n## Review\ntwo\n", "Review")
	if !ok || err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate Section ok=%v err=%v", ok, err)
	}
}

func TestSection_LevelOneHeadingTerminatesContractSection(t *testing.T) {
	report := "## Explanation Documentation\n- Status: VERIFIED\n# Different Report\n- Evidence: must not leak backward\n"
	body, ok, err := Section(report, "Explanation Documentation")
	if err != nil || !ok {
		t.Fatalf("Section ok=%v err=%v", ok, err)
	}
	if strings.Contains(body, "Evidence") || strings.Contains(body, "Different Report") {
		t.Fatalf("level-one heading did not terminate section: %q", body)
	}
}

func TestFields_ParsesOnlyAllowedVisibleMetadata(t *testing.T) {
	body := `
- Status: VERIFIED
- Finding: first narrative item
- Finding: second narrative item
<!-- - Evidence: hidden -->
` + "```\n- Evidence: fenced\n```\n" + `
- Evidence: visible evidence
`
	fields, err := Fields(body, "Status", "Evidence")
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	if fields["status"] != "VERIFIED" || fields["evidence"] != "visible evidence" || len(fields) != 2 {
		t.Fatalf("Fields=%v", fields)
	}
}

func TestFields_RejectsDuplicateRecognizedField(t *testing.T) {
	_, err := Fields("- Status: VERIFIED\n- Status: NEEDS_CORRECTION\n", "Status")
	if err == nil || !strings.Contains(err.Error(), "duplicate status") {
		t.Fatalf("duplicate Fields err=%v", err)
	}
}

func TestRequirePathLineEvidence_RequiresEveryReference(t *testing.T) {
	if err := RequirePathLineEvidence("checked docs/explain/a.md and go/app.go", "docs/explain/a.md", "go/app.go"); err == nil {
		t.Fatal("path-only evidence was accepted")
	}
	if err := RequirePathLineEvidence("checked docs/explain/a.md:1 against `go/app.go`:19", "docs/explain/a.md", "go/app.go"); err != nil {
		t.Fatalf("path:line evidence rejected: %v", err)
	}
	if err := RequirePathLineEvidence("checked notgo/app.go:12 in the generated output", "go/app.go"); err == nil {
		t.Fatal("reference embedded in a longer path was accepted")
	}
}

func TestRequirePathLineEvidenceAt_RejectsImpossibleCurrentLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go", "app.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package app\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RequirePathLineEvidenceAt(context.Background(), root, "", "reviewed go/app.go:99 against the contract", "go/app.go"); err == nil {
		t.Fatal("out-of-range current-file citation was accepted")
	}
}

func TestRequirePathLineEvidenceAt_AcceptsDeletedPathLineFromBase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	path := filepath.Join(root, "go", "removed.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package removed\n\nvar Enabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "go/removed.go"}, {"commit", "-q", "-m", "base"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	baseOut, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := RequirePathLineEvidenceAt(context.Background(), root, strings.TrimSpace(string(baseOut)), "confirmed the removed behavior at go/removed.go:3", "go/removed.go"); err != nil {
		t.Fatalf("deleted base-blob citation rejected: %v", err)
	}
}

func TestRequirePathLineEvidenceAt_AcceptsCurrentSpecialPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("empty.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "module")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "test"}, {"commit", "--allow-empty", "-q", "-m", "module"}} {
		if out, err := exec.Command("git", append([]string{"-C", module}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git module %v: %v\n%s", args, err, out)
		}
	}
	evidence := "reviewed empty.txt:1, link.txt:1, and module:1 against the explanation"
	if err := RequirePathLineEvidenceAt(context.Background(), root, "", evidence, "empty.txt", "link.txt", "module"); err != nil {
		t.Fatalf("current special-path citations rejected: %v", err)
	}
}

func TestRequirePathLineEvidenceAt_AcceptsDeletedSpecialPathsFromBase(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "test"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "removed-link")); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "removed-module")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "test"}, {"commit", "--allow-empty", "-q", "-m", "module"}} {
		if out, err := exec.Command("git", append([]string{"-C", module}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git module %v: %v\n%s", args, err, out)
		}
	}
	for _, args := range [][]string{{"add", "removed-link", "removed-module"}, {"commit", "-q", "-m", "base"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	baseOut, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "removed-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(module); err != nil {
		t.Fatal(err)
	}
	evidence := "reviewed removed-link:1 and removed-module:1 before deletion"
	if err := RequirePathLineEvidenceAt(context.Background(), root, strings.TrimSpace(string(baseOut)), evidence, "removed-link", "removed-module"); err != nil {
		t.Fatalf("deleted special-path citations rejected: %v", err)
	}
}

func TestRequirePathLineEvidenceAt_RejectsOversizedDeletedBaseBlob(t *testing.T) {
	if testing.Short() {
		t.Skip("large-blob boundary")
	}
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "test"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	path := filepath.Join(root, "large.txt")
	if err := os.WriteFile(path, make([]byte, maxCitationBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "large.txt"}, {"commit", "-q", "-m", "large"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	baseOut, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := RequirePathLineEvidenceAt(context.Background(), root, strings.TrimSpace(string(baseOut)), "reviewed large.txt:1 before deletion", "large.txt"); err == nil {
		t.Fatal("oversized deleted base blob was read without a limit")
	}
}

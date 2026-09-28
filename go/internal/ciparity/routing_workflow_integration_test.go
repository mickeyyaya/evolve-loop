//go:build integration

package ciparity

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type routingCase struct {
	name                string
	paths               []string
	remove              string
	wantGo, wantLanding bool
}

func TestWorkflowGitHelpers_KeepAmbientRoutingOutsideFixtures(t *testing.T) {
	for _, helper := range []string{"fixture", "workflow"} {
		for _, routing := range []string{"clean", "repository", "index", "objects"} {
			t.Run(helper+"/"+routing, func(t *testing.T) {
				assertGitStaysInFixture(t, helper, routing)
			})
		}
	}
}

func assertGitStaysInFixture(t *testing.T, helper, routing string) {
	t.Helper()
	dir, _ := routingRepo(t)
	wantTop, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	external, _ := routingRepo(t)
	routingFile(t, external, "untracked.txt")
	if err := os.WriteFile(filepath.Join(dir, "go", "new.go"), []byte("unique routing object\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blob := routingGit(t, dir, "hash-object", "go/new.go")
	externalGit := filepath.Join(external, ".git")
	index := filepath.Join(externalGit, "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	redirectAmbientGit(t, routing, external, externalGit)
	if top := stageThroughHelper(t, helper, dir); top != wantTop {
		t.Errorf("Git escaped fixture: got %q, want %q", top, wantTop)
	}
	after, err := os.ReadFile(index)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("Git changed the external index: %v", err)
	}
	if _, err := os.Stat(filepath.Join(externalGit, "objects", blob[:2], blob[2:])); !os.IsNotExist(err) {
		t.Errorf("Git wrote the fixture blob to external objects: %v", err)
	}
}

func redirectAmbientGit(t *testing.T, routing, external, externalGit string) {
	t.Helper()
	switch routing {
	case "repository":
		t.Setenv("GIT_DIR", externalGit)
		t.Setenv("GIT_WORK_TREE", external)
		t.Setenv("GIT_COMMON_DIR", externalGit)
	case "index":
		t.Setenv("GIT_INDEX_FILE", filepath.Join(externalGit, "index"))
	case "objects":
		t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(externalGit, "objects"))
	}
}

func stageThroughHelper(t *testing.T, helper, dir string) string {
	t.Helper()
	if helper == "fixture" {
		routingGit(t, dir, "add", "--all")
		return routingGit(t, dir, "rev-parse", "--show-toplevel")
	}
	out, err := runWorkflowShell(t, dir, "git add --all\ngit rev-parse --show-toplevel", nil)
	if err != nil {
		t.Fatalf("workflow Git control failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(out)
}

func routingPathCases() []routingCase {
	cases := []routingCase{
		{"report", []string{"docs/reports/review.md"}, "", false, false},
		{"research", []string{"docs/research/testing/note.md"}, "", false, false},
		{"archive", []string{"docs/private/archive/note.md"}, "", false, false},
		{"non_markdown_research_fixture", []string{"docs/research/testing/cases.json"}, "", true, true},
		{"go", []string{"go/internal/core/new.go"}, "", true, false},
		{"go_module", []string{"go/go.mod"}, "", true, false},
		{"go_fixture", []string{"go/test/fixtures/input.json"}, "", true, false},
		{"go_gate", []string{"go/.cover-strict"}, "", true, false},
		{"skill", []string{"skills/a/SKILL.md"}, "", true, false},
		{"agent", []string{"agents/a.md"}, "", true, false},
		{"landing", []string{"landing/cmd/build/main.go"}, "", false, true},
		{"landing_module", []string{"landing/go.mod"}, "", false, true},
		{"landing_template", []string{"landing/templates/page.html"}, "", false, true},
		{"explanation", []string{"docs/explain/topic.md"}, "", false, true},
		{"mixed", []string{"go/pkg/version/version.go", "landing/static/site.css", "docs/reports/review.md"}, "", true, true},
		{"deleted_source", nil, "go/existing.go", true, false},
		{"deleted_document", nil, "docs/reports/existing.md", false, false},
		{"renamed_source_to_document", []string{"docs/reports/renamed.md"}, "go/existing.go", true, false},
		{"renamed_document_to_source", []string{"go/renamed.go"}, "docs/reports/existing.md", true, false},
		{"unusual_names", []string{"landing/a\nb\tc file.txt", "docs/reports/note with spaces.md"}, "", false, true},
	}
	for _, path := range []string{".github/workflows/go.yml", ".github/workflows/required.yml", ".github/workflows/ci.yml", ".github/workflows/release.yml", ".github/workflows/landing-pages.yml", ".github/workflows/landing-validation.yml", ".goreleaser.yml", ".evolve/policy.json", ".evolve/phases/a.json", ".evolve/profiles/a.json", ".claude-plugin/plugin.json", "install.sh", "README.md", "docs/architecture/phase-registry.json", "docs/architecture/note.md", "docs/incidents/note.md", "config/new.yaml", "future-module/new.rs"} {
		cases = append(cases, routingCase{path, []string{path}, "", true, true})
	}
	return cases
}

func TestRequiredRouting_ConsumedPathsAndExplicitDocumentationSkips(t *testing.T) {
	step := workflowStep(t, "required.yml", "changes", "paths")
	for _, tc := range routingPathCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir, base := routingRepo(t)
			if tc.remove != "" {
				if err := os.Remove(filepath.Join(dir, tc.remove)); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range tc.paths {
				routingFile(t, dir, path)
			}
			head := routingCommit(t, dir)
			got, err := routingOutput(t, dir, step.Run, "push", base, head)
			if err != nil {
				t.Fatalf("routing failed: %v\n%s", err, got)
			}
			want := "go=" + strconv.FormatBool(tc.wantGo) + "\nlanding=" + strconv.FormatBool(tc.wantLanding) + "\n"
			if got != want {
				t.Fatalf("route = %q, want %q", got, want)
			}
		})
	}
}

func TestRequiredRouting_MergeBaseExcludesBaseBranchOnlyChanges(t *testing.T) {
	dir, _ := routingRepo(t)
	routingGit(t, dir, "checkout", "-qb", "feature")
	routingFile(t, dir, "go/pr.go")
	head := routingCommit(t, dir)
	routingGit(t, dir, "checkout", "-q", "main")
	routingFile(t, dir, "landing/base-only.html")
	base := routingCommit(t, dir)
	got, err := routingOutput(t, dir, workflowStep(t, "required.yml", "changes", "paths").Run, "pull_request", base, head)
	if err != nil || got != "go=true\nlanding=false\n" {
		t.Fatalf("PR route included unrelated base changes: %q, %v", got, err)
	}
}

func TestRequiredRouting_MissingEvidenceCannotBecomeDocumentationSkip(t *testing.T) {
	dir, head := routingRepo(t)
	step := workflowStep(t, "required.yml", "changes", "paths")
	cases := []struct {
		name, event, base, head string
		pass                    bool
	}{
		{"manual", "workflow_dispatch", "", head, true},
		{"initial_push", "push", strings.Repeat("0", 40), head, true},
		{"empty_diff", "push", head, head, true},
		{"missing_base", "pull_request", "", head, false},
		{"invalid_base", "push", "--output=bad", head, false},
		{"unknown_base", "push", strings.Repeat("f", 40), head, false},
		{"invalid_head", "push", head, "not-a-sha", false},
		{"unknown_head", "push", head, strings.Repeat("f", 40), false},
		{"unknown_event", "unknown", head, head, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := routingOutput(t, dir, step.Run, tc.event, tc.base, tc.head)
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v want %v: %v\n%s", err == nil, tc.pass, err, got)
			}
			if tc.pass && got != "go=true\nlanding=true\n" {
				t.Fatalf("missing diff evidence weakened routing: %q", got)
			}
			if !tc.pass && strings.Contains(got, "=false") {
				t.Fatalf("failed routing emitted a skip decision: %q", got)
			}
		})
	}
}

func TestRequiredRouting_GitFailuresStopWithoutShellErrexit(t *testing.T) {
	dir, head := routingRepo(t)
	step := workflowStep(t, "required.yml", "changes", "paths")
	for _, tc := range []struct{ name, prefix, event, base, head string }{
		{"diff_missing_base", "", "push", strings.Repeat("f", 40), head},
		{"diff_missing_head", "", "push", head, strings.Repeat("f", 40)},
		{"merge_base_failure", "", "pull_request", strings.Repeat("f", 40), head},
		{"git_unavailable", "PATH=/nonexistent\n", "push", head, head},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := routingOutput(t, dir, "set +e\n"+tc.prefix+step.Run, tc.event, tc.base, tc.head)
			if err == nil || strings.Contains(out, "go=") || strings.Contains(out, "landing=") {
				t.Fatalf("Git failure must stop routing without emitting decisions: err=%v output=%q", err, out)
			}
		})
	}
}

func routingOutput(t *testing.T, dir, script, event, base, head string) (string, error) {
	t.Helper()
	tmp := t.TempDir()
	output := filepath.Join(tmp, "output")
	out, err := runWorkflowShell(t, dir, script, map[string]string{"EVENT_NAME": event, "BASE_SHA": base, "HEAD_SHA": head, "GITHUB_OUTPUT": output, "RUNNER_TEMP": tmp})
	b, readErr := os.ReadFile(output)
	if err != nil {
		return out + string(b), err
	}
	if readErr != nil {
		t.Fatalf("successful routing did not produce outputs: %v\n%s", readErr, out)
	}
	return string(b), nil
}

func routingRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := gittest.Fixture(t).Dir
	for _, path := range []string{"go/existing.go", "docs/reports/existing.md"} {
		routingFile(t, dir, path)
	}
	return dir, routingCommit(t, dir)
}

func routingFile(t *testing.T, dir, path string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("same content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func routingCommit(t *testing.T, dir string) string {
	t.Helper()
	routingGit(t, dir, "add", "--all")
	routingGit(t, dir, "commit", "-qm", "routing fixture")
	return routingGit(t, dir, "rev-parse", "HEAD")
}

func routingGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	args = append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = workflowTestEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git fixture: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

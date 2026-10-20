package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func TestDefaultBuildFloorChecks_AtEnforceAFailingDocGoesBackToTheBuild(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": "# New\n\nIntro.\n\n" + longSentence(30) + "\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	failures := DefaultBuildFloorChecks(context.Background(), in)

	var ste []string
	for _, f := range failures {
		if strings.Contains(f, "[ste-lint]") {
			ste = append(ste, f)
		}
	}
	if len(ste) != 1 || !strings.Contains(ste[0], "1 finding(s) in 1 file(s) (first: docs/new.md:5 STE-SENTENCE)") || !strings.Contains(ste[0], "evolve docs ste-lint --strict") {
		t.Fatalf("floor failures = %q, want one ste-lint failure that names the first finding and the command", failures)
	}
	if strings.Contains(sink.String(), "[ste-lint] WARN: 1 finding") {
		t.Errorf("at enforce the finding is a failure, not a WARN: %q", sink.String())
	}
}

func TestDefaultBuildFloorChecks_AtEnforceACleanDocPasses(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": "Short and clean.\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	for _, f := range DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(f, "[ste-lint]") {
			t.Fatalf("a clean doc must pass the enforced floor: %q", f)
		}
	}
}

func TestDefaultBuildFloorChecks_AtEnforceALintThatDidNotRunOnlyWarns(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{stelint.StandardPath: "# The standard\n\nNo word table.\n", "docs/new.md": longSentence(30) + "\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	for _, f := range DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(f, "[ste-lint]") {
			t.Fatalf("a lint that could not run is a WARN, not a reason to send the doc back: %q", f)
		}
	}
	if !strings.Contains(sink.String(), "did not run") {
		t.Errorf("stderr = %q, want the WARN that the lint did not run", sink.String())
	}
}

func TestDefaultBuildFloorChecks_AtEnforceAnUnreadableDocWarnsBesideTheFailure(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
	if err := os.MkdirAll(filepath.Join(in.Worktree, "docs", "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	failures := docsFloorChecks(in, append(changedFloorPaths(context.Background(), in), "docs/dir.md"))

	if len(failures) != 1 || !strings.Contains(failures[0], "docs/new.md:1 STE-SENTENCE") {
		t.Fatalf("failures = %q, want the one finding of docs/new.md", failures)
	}
	if !strings.Contains(sink.String(), "[ste-lint] WARN: docs/dir.md was not checked") {
		t.Errorf("stderr = %q, want a WARN that names the unread doc", sink.String())
	}
}

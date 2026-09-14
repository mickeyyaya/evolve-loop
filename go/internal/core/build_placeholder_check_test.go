package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cycle 1679 (2026-09-15): round 5's build-report.md carried the literal
// template token FULLSUITE_PLACEHOLDER where the mandatory full-suite result
// belongs, inside a section headed "every number below was executed this
// round"; the handoff floor passed it and the audit spent a round naming it
// (M3 verification-gap). An unsubstituted template token is a deterministic
// fact about the report, so the floor refuses it before handoff.
func TestPlaceholderTokenFailures_RefusesAnUnsubstitutedTemplateToken(t *testing.T) {
	ws := t.TempDir()
	body := "# Build Report\n\n## Full suite\n`go test -count=1 ./...` → FULLSUITE_PLACEHOLDER\n\nok: ACS_RESULT_PLACEHOLDER\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: ws, Worktree: t.TempDir()})
	if len(got) != 2 {
		t.Fatalf("two unsubstituted tokens on two lines → two failures, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "FULLSUITE_PLACEHOLDER") || !strings.Contains(got[0], "build-report.md:4") {
		t.Errorf("a failure names the token and its report line so the builder can substitute it exactly: %q", got[0])
	}
	if !strings.Contains(got[1], "ACS_RESULT_PLACEHOLDER") || !strings.Contains(got[1], "build-report.md:6") {
		t.Errorf("second token, own line: %q", got[1])
	}
}

func TestPlaceholderTokenFailures_CleanReportAndProseAreNotTokens(t *testing.T) {
	ws := t.TempDir()
	body := "# Build Report\n\nThe `_PLACEHOLDER` suffix rule is documented; a placeholder image and PLACEHOLDER_ALPHA are prose, and lowercase full_suite_placeholder is a variable name.\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: ws, Worktree: t.TempDir()}); len(got) != 0 {
		t.Errorf("only an UPPER_SNAKE token ending in _PLACEHOLDER is a template token: %v", got)
	}
	if got := PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: t.TempDir(), Worktree: t.TempDir()}); len(got) != 0 {
		t.Errorf("no report → nothing to refuse (the deliverables gate owns absence): %v", got)
	}
}

func TestDefaultBuildFloorChecks_IncludesThePlaceholderTokenCheck(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("result: FULLSUITE_PLACEHOLDER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	got := DefaultBuildFloorChecks(context.Background(), ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: wt, ProjectRoot: wt})
	found := false
	for _, f := range got {
		if strings.Contains(f, "FULLSUITE_PLACEHOLDER") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the default floor engine runs the placeholder-token check (wiring proof): %v", got)
	}
}

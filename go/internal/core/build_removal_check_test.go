package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// removalFixture writes report (unless empty) and creates each present path in the worktree.
func removalFixture(t *testing.T, report string, present []string) ReviewInput {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, "workspace")
	wt := filepath.Join(root, "worktree")
	for _, d := range []string{ws, wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if report != "" {
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(report), 0o644); err != nil {
			t.Fatalf("write build-report.md: %v", err)
		}
	}
	for _, p := range present {
		abs := filepath.Join(wt, p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", p, err)
		}
		if err := os.WriteFile(abs, []byte("still here\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: wt}
}

func claimBlock(paths ...string) string {
	quoted := make([]string, 0, len(paths))
	for _, p := range paths {
		quoted = append(quoted, `"`+p+`"`)
	}
	return "# Build Report\n\nCleanup performed.\n\n```json\n{\"removedPaths\": [" +
		strings.Join(quoted, ", ") + "]}\n```\n"
}

func TestRemovalClaimFailures(t *testing.T) {
	tests := []struct {
		name        string
		report      string
		present     []string
		wantCount   int
		wantMention []string
	}{
		{
			name:        "false claim — path still present — BLOCKS",
			report:      claimBlock("go/acs/cycle660/predicates_test.go"),
			present:     []string{"go/acs/cycle660/predicates_test.go"},
			wantCount:   1,
			wantMention: []string{"go/acs/cycle660/predicates_test.go"},
		},
		{
			name:      "honest claim — path genuinely absent — passes",
			report:    claimBlock("go/acs/cycle660/predicates_test.go"),
			present:   nil,
			wantCount: 0,
		},
		{
			name:        "mixed claims — only the false one is reported",
			report:      claimBlock("a/gone.go", "b/still-here.go", "c/gone-too.go"),
			present:     []string{"b/still-here.go"},
			wantCount:   1,
			wantMention: []string{"b/still-here.go"},
		},
		{
			name:        "every claim false — one failure per path",
			report:      claimBlock("x.go", "y.go"),
			present:     []string{"x.go", "y.go"},
			wantCount:   2,
			wantMention: []string{"x.go", "y.go"},
		},
		{
			name:      "no claim block — fail-open",
			report:    "# Build Report\n\nRemoved the stale scaffold (prose only).\n",
			present:   []string{"go/acs/cycle660/predicates_test.go"},
			wantCount: 0,
		},
		{
			name:      "empty removedPaths — fail-open",
			report:    "```json\n{\"removedPaths\": []}\n```\n",
			present:   []string{"x.go"},
			wantCount: 0,
		},
		{
			name:      "malformed JSON block — fail-open",
			report:    "```json\n{\"removedPaths\": [\"x.go\",,,}\n```\n",
			present:   []string{"x.go"},
			wantCount: 0,
		},
		{
			name:      "unrelated JSON block — fail-open",
			report:    "```json\n{\"testFiles\": [\"x.go\"]}\n```\n",
			present:   []string{"x.go"},
			wantCount: 0,
		},
		{
			name:      "missing build-report.md — fail-open",
			report:    "",
			present:   []string{"x.go"},
			wantCount: 0,
		},
		{
			name:      "path escaping the worktree — ignored, fail-open",
			report:    claimBlock("../outside.go"),
			present:   nil,
			wantCount: 0,
		},
		{
			name:      "absolute path claim — ignored, fail-open",
			report:    claimBlock("/etc/hosts"),
			present:   nil,
			wantCount: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := removalFixture(t, tc.report, tc.present)
			got := RemovalClaimFailures(context.Background(), in)
			if len(got) != tc.wantCount {
				t.Fatalf("failures = %d, want %d: %v", len(got), tc.wantCount, got)
			}
			joined := strings.Join(got, "\n")
			for _, m := range tc.wantMention {
				if !strings.Contains(joined, m) {
					t.Errorf("failure text does not mention %q:\n%s", m, joined)
				}
			}
		})
	}
}

func TestRemovalClaimFailures_MissingRootsFailOpen(t *testing.T) {
	for _, in := range []ReviewInput{
		{Phase: string(PhaseBuild)},
		{Phase: string(PhaseBuild), Workspace: t.TempDir()},
		{Phase: string(PhaseBuild), Worktree: t.TempDir()},
	} {
		if got := RemovalClaimFailures(context.Background(), in); len(got) != 0 {
			t.Errorf("ReviewInput{ws=%q wt=%q}: want fail-open, got %v", in.Workspace, in.Worktree, got)
		}
	}
}

func TestRemovalClaimFailures_DeliverablesFallback(t *testing.T) {
	in := removalFixture(t, "", []string{"x.go"})
	dir := filepath.Join(in.Workspace, "deliverables")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build-report.md"), []byte(claimBlock("x.go")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := RemovalClaimFailures(context.Background(), in)
	if len(got) != 1 || !strings.Contains(got[0], "x.go") {
		t.Fatalf("promoted report not consulted: got %v", got)
	}
}

// A non-git worktree derives zero changed packages, the early return a false claim could hide behind.
func TestDefaultBuildFloorChecks_IncludesRemovalClaimCheck(t *testing.T) {
	in := removalFixture(t, claimBlock("go/acs/cycle660/predicates_test.go"), []string{"go/acs/cycle660/predicates_test.go"})
	got := DefaultBuildFloorChecks(context.Background(), in)
	if len(got) == 0 {
		t.Fatalf("DefaultBuildFloorChecks did not surface the false removal claim — check is unwired")
	}
	if !strings.Contains(strings.Join(got, "\n"), "go/acs/cycle660/predicates_test.go") {
		t.Fatalf("failures do not name the falsely-claimed path: %v", got)
	}
}

func TestBuildFloorReviewer_RemovalClaimNotActuallyRemoved(t *testing.T) {
	r := NewBuildFloorReviewer(DefaultBuildFloorChecks)

	bad := removalFixture(t, claimBlock("stale/scaffold.go"), []string{"stale/scaffold.go"})
	if res := r.Review(context.Background(), bad); res.Approve {
		t.Fatalf("false removal claim was APPROVED — acceptance criterion 2 unmet")
	} else if !strings.Contains(res.Reason, "stale/scaffold.go") {
		t.Errorf("reject reason does not name the path: %q", res.Reason)
	}

	good := removalFixture(t, claimBlock("stale/scaffold.go"), nil)
	if res := r.Review(context.Background(), good); !res.Approve {
		t.Fatalf("honest removal claim was REJECTED: %q", res.Reason)
	}
}

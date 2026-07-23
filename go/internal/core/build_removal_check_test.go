package core

// build_removal_check_test.go — RED contract for the cycle-1076 task
// `build-selfcheck-removal-claim-check` (inbox item `tdd-topn-binding-gate`,
// acceptance criterion 2: "a build report claiming a removal that did not
// happen fails build-selfcheck deterministically").
//
// The cycle-660 incident this pins: build-report.md asserted that orphaned RED
// scaffolds were "already removed by a concurrent actor" while the files were
// still present in the worktree, and the false claim passed review undetected.
//
// Contract under test (production code is Builder's job — none of it exists at
// RED time):
//
//	func RemovalClaimFailures(ctx context.Context, in ReviewInput) []string
//
// It is a BuildFloorCheckFn: it reads the build report from
// <workspace>/build-report.md (falling back to
// <workspace>/deliverables/build-report.md), parses every fenced ```json block
// for an object carrying a "removedPaths" string array, and returns one failure
// line per claimed path that STILL EXISTS under the worktree. Every ambiguity
// is fail-open (nil): no workspace/worktree, no report, no parseable block,
// malformed JSON, empty list, or a path escaping the worktree. It must also be
// composed into DefaultBuildFloorChecks so a false claim actually REJECTs the
// build deliverable — the wiring proof, not just the unit.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// removalFixture materialises a workspace + worktree pair: report is written to
// <workspace>/build-report.md (unless empty), and each entry of present is
// created as a real file under the worktree.
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

// TestRemovalClaimFailures covers the whole disposition surface: the negative
// case (false claim MUST produce a failure — the anti-no-op signal), the
// positive case (honest removal is silent), and every fail-open edge.
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

// TestRemovalClaimFailures_MissingRootsFailOpen pins the plumbing floor: an
// unset workspace or worktree can never produce a finding (the reviewer must
// not false-block a build over its own wiring).
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

// TestRemovalClaimFailures_DeliverablesFallback pins the second lookup location:
// after the correction ladder promotes the report, it lives under
// <workspace>/deliverables/build-report.md.
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

// TestDefaultBuildFloorChecks_IncludesRemovalClaimCheck is the WIRING proof: the
// check must run inside the PRODUCTION engine, and must not be short-circuited
// by the changed-package early returns (a non-git fixture worktree derives zero
// changed packages, which is exactly the path a false claim would hide behind).
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

// TestBuildFloorReviewer_RemovalClaimNotActuallyRemoved is the end-to-end
// deterministic-failure requirement from the acceptance text: a false removal
// claim REJECTs the build deliverable, an honest report approves.
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

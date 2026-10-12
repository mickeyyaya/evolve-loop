package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func statusRunnerF1(out string, code int, errOut string, runErr error) CmdRunner {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, out)
		_, _ = io.WriteString(stderr, errOut)
		return code, runErr
	}
}

func TestWorktreeStatusPathsBackfillF1_ReportsRunnerFailuresAndParsesRecords(t *testing.T) {
	t.Parallel()
	boom := errors.New("spawn failed")
	cases := []struct {
		name      string
		runner    CmdRunner
		want      []string
		wantErr   string
		wantWraps error
	}{
		{name: "runner error is wrapped", runner: statusRunnerF1("", 0, "", boom), wantErr: "git status: spawn failed", wantWraps: boom},
		{name: "non-zero exit carries trimmed stderr", runner: statusRunnerF1("", 128, "  fatal: not a repo \n", nil), wantErr: "git status exit 128: fatal: not a repo"},
		{name: "empty output yields no paths", runner: statusRunnerF1("", 0, "", nil), want: nil},
		{name: "records past the XY prefix are kept", runner: statusRunnerF1("?? a.go\x00 M dir/b.go\x00", 0, "", nil), want: []string{"a.go", "dir/b.go"}},
		{name: "records no longer than the prefix are dropped", runner: statusRunnerF1("XY \x00ab\x00?? c\x00", 0, "", nil), want: []string{"c"}},
		{name: "unicode path survives intact", runner: statusRunnerF1("?? テスト/é.go\x00", 0, "", nil), want: []string{"テスト/é.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := worktreeStatusPaths(context.Background(), "/wt", Deps{Runner: tc.runner})
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				if tc.wantWraps != nil && !errors.Is(err, tc.wantWraps) {
					t.Fatalf("err %v does not wrap %v", err, tc.wantWraps)
				}
				if got != nil {
					t.Fatalf("paths = %v on error, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") || len(got) != len(tc.want) {
				t.Fatalf("paths = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWorktreeStatusPathsBackfillF1_PassesTheWorktreeToGit(t *testing.T) {
	t.Parallel()
	var gotName string
	var gotArgs []string
	runner := func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		gotName, gotArgs = name, args
		return 0, nil
	}
	if _, err := worktreeStatusPaths(context.Background(), "/the/wt", Deps{Runner: runner}); err != nil {
		t.Fatal(err)
	}
	if gotName != "git" || strings.Join(gotArgs, " ") != "--no-optional-locks -C /the/wt status --porcelain=v1 -z --untracked-files=all --no-renames" {
		t.Fatalf("git invocation = %s %v", gotName, gotArgs)
	}
}

func TestWithinDirBackfillF1(t *testing.T) {
	t.Parallel()
	sep := string(filepath.Separator)
	cases := []struct {
		name      string
		dir, path string
		want      bool
	}{
		{"empty dir never contains", "", "/a/b", false},
		{"the dir itself", "/a", "/a", true},
		{"a child", "/a", "/a/b/c", true},
		{"a sibling with a shared prefix", "/a", "/ab", false},
		{"the parent", "/a/b", "/a", false},
		{"unrelated", "/a", "/z/y", false},
		{"dotdot-named child is inside", "/a", "/a/..x", true},
		{"relative pair that cannot be related", "rel", "/abs", false},
		{"escape via dotdot segment", "/a/b", "/a/b" + sep + ".." + sep + "c", false},
	}
	for _, tc := range cases {
		if got := withinDir(tc.dir, tc.path); got != tc.want {
			t.Errorf("%s: withinDir(%q,%q) = %v, want %v", tc.name, tc.dir, tc.path, got, tc.want)
		}
	}
}

func TestDeliverableUnrewrittenBackfillF1_NoDeliverableAnywhereIsNotCarried(t *testing.T) {
	t.Parallel()
	cfg := &Config{Workspace: t.TempDir(), Worktree: t.TempDir(), Artifact: filepath.Join(t.TempDir(), "build-report.md")}
	d := &worktreeEvidenceDetector{artifactDetector: &artifactDetector{cfg: cfg}}
	if d.deliverableUnrewritten() {
		t.Fatal("with no deliverable at any candidate path there is nothing carried to complete on")
	}
}

func TestReportedPathsBackfillF1(t *testing.T) {
	t.Parallel()
	mk := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("p%d", i)
		}
		return out
	}
	cases := []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "p0"},
		{worktreeEvidenceReportedPaths, "p0,p1,p2,p3,p4,p5,p6,p7"},
		{worktreeEvidenceReportedPaths + 1, "p0,p1,p2,p3,p4,p5,p6,p7,+1 more"},
		{worktreeEvidenceReportedPaths + 12, "p0,p1,p2,p3,p4,p5,p6,p7,+12 more"},
	}
	for _, tc := range cases {
		if got := reportedPaths(mk(tc.n)); got != tc.want {
			t.Errorf("reportedPaths(%d paths) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestWorktreeEvidenceNoteBackfillF1_NamesTheRelocationOnlyWhenOneHappened(t *testing.T) {
	t.Parallel()
	ev := completionEvidence{CarriedDeliverable: "/ws/build-report.md", WorktreeChanges: []string{"a", "b"}}
	plain := worktreeEvidenceNote(ev, "")
	if strings.Contains(plain, "relocated") || !strings.Contains(plain, "changed 2 worktree path(s) since dispatch (a,b)") {
		t.Fatalf("plain note = %q", plain)
	}
	moved := worktreeEvidenceNote(ev, "/wt/build-report.md")
	if !strings.HasSuffix(moved, "; relocated from non-canonical /wt/build-report.md") || !strings.HasPrefix(moved, plain) {
		t.Fatalf("relocated note = %q, want the plain note plus the relocation suffix", moved)
	}
}

type fallbackEvidenceF1 struct {
	cfg      *Config
	detector *worktreeEvidenceDetector
}

func fallbackDeliverableF1(t *testing.T, canonicalParent func(ws string) string) fallbackEvidenceF1 {
	t.Helper()
	ws := t.TempDir()
	cfg := &Config{
		Workspace: ws, Worktree: committedWorktree(t),
		Artifact:   filepath.Join(canonicalParent(ws), "build-report.md"),
		Completion: core.CompletionWorktreeEvidence,
	}
	writeArtifact(t, filepath.Join(cfg.Worktree, "build-report.md"), staleBuildReport, fixedMTime)
	writeFile(t, filepath.Join(cfg.Worktree, "src", "feature.go"), "package feature\n\nvar Dirty = 1\n")
	d := detectorOver(t, cfg)
	writeFile(t, filepath.Join(cfg.Worktree, "docs", "explain.md"), "# Explanation\n- what and why\n")
	return fallbackEvidenceF1{cfg: cfg, detector: d}
}

func TestWorktreeEvidenceDetectorBackfillF1_ACarriedDeliverableAtAFallbackIsRelocatedAndReported(t *testing.T) {
	t.Parallel()
	fx := fallbackDeliverableF1(t, func(ws string) string { return filepath.Join(ws, "fresh") })
	ready, evidence, note, err := fx.detector.completeOnIdle(context.Background())
	if err != nil || !ready {
		t.Fatalf("ready=%v err=%v, want a completion", ready, err)
	}
	if !strings.Contains(note, "; relocated from non-canonical "+filepath.Join(fx.cfg.Worktree, "build-report.md")) {
		t.Fatalf("note %q must name the fallback the deliverable was moved from", note)
	}
	if got, rerr := os.ReadFile(fx.cfg.Artifact); rerr != nil || string(got) != staleBuildReport {
		t.Fatalf("the carried deliverable must arrive at the canonical path untouched: %q (%v)", got, rerr)
	}
	if evidence.CarriedDeliverable != fx.cfg.Artifact || strings.Join(evidence.WorktreeChanges, ",") != "docs/explain.md" {
		t.Fatalf("evidence = %+v", evidence)
	}
}

func TestWorktreeEvidenceDetectorBackfillF1_AnUnrelocatableFallbackSurfacesTheErrorInsteadOfCompleting(t *testing.T) {
	t.Parallel()
	fx := fallbackDeliverableF1(t, func(ws string) string {
		blocker := filepath.Join(ws, "blocker")
		writeFile(t, blocker, "a file where the canonical directory must go")
		return blocker
	})
	ready, evidence, note, err := fx.detector.completeOnIdle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "relocate") {
		t.Fatalf("err = %v, want the relocation failure", err)
	}
	if ready || note != "" || len(evidence.WorktreeChanges) != 0 {
		t.Fatalf("a failed relocation must not complete: ready=%v note=%q evidence=%+v", ready, note, evidence)
	}
}

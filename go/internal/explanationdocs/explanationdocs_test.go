package explanationdocs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

type fixture struct {
	root      string
	worktree  string
	workspace string
	base      string
	cycle     int
	runID     string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	f := fixture{
		root:     t.TempDir(),
		worktree: t.TempDir(),
		cycle:    42,
		runID:    "run-42",
	}
	f.workspace = filepath.Join(f.root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(f.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	f.git(t, "init", "-q")
	f.git(t, "config", "user.email", "t@t")
	f.git(t, "config", "user.name", "t")
	f.write(t, "config/app.yaml", "enabled: false\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "base")
	f.base = f.git(t, "rev-parse", "HEAD")
	return f
}

func (f fixture) binding() CycleBinding {
	return CycleBinding{
		ProjectRoot:     f.root,
		Worktree:        f.worktree,
		Workspace:       f.workspace,
		BaseSHA:         f.base,
		Cycle:           f.cycle,
		RunID:           f.runID,
		ContractVersion: CurrentContractVersion,
	}
}

func (f fixture) activate(t *testing.T) {
	t.Helper()
	binding := f.binding()
	binding.Worktree = ""
	binding.BaseSHA = ""
	if err := Activate(binding); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := SealBuild(f.binding()); err != nil {
		t.Fatalf("SealBuild: %v", err)
	}
}

func (f fixture) write(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(f.worktree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) writeWorkspace(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(f.workspace, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.worktree}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f fixture) validDocument() string {
	return "# Build Explanation — Cycle 42\n\n" +
		"## Build Binding\n- Cycle: 42\n- Base SHA: " + f.base + "\n\n" +
		"## Summary\nEnable the application through its existing configuration field.\n\n" +
		"## Rationale\nUsing the existing field is the smallest compatible behavior change and avoids a second configuration surface.\n\n" +
		"## Changed Areas\n- `config/app.yaml` — flips the existing runtime setting while preserving its schema.\n\n" +
		"## Design Decisions\nThe existing YAML setting remains the only public control for this behavior.\n\n" +
		"## Verification\nThe targeted configuration tests exercise both enabled and disabled behavior.\n\n" +
		"## Compatibility\nThe schema and setting name remain unchanged.\n\n" +
		"## Limitations\nThis does not add per-user configuration.\n"
}

func (f fixture) prepareRequired(t *testing.T) {
	t.Helper()
	f.write(t, "config/app.yaml", "enabled: true\n")
	f.write(t, cycleDocumentPath(f.cycle, f.runID), f.validDocument())
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Explanation Documentation\n- Status: REQUIRED\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n")
}

func (f fixture) check(t *testing.T) []string {
	t.Helper()
	return CheckBuild(context.Background(), f.binding())
}

func TestRequiredCycleDocument_FullLifecycle(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	view, err := Load(f.workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.ContractVersion != CurrentContractVersion || view.Status != statusRequired || view.DocumentPath != cycleDocumentPath(f.cycle, f.runID) || view.DocumentSHA256 == "" || !samePaths(view.MaterialPaths, []string{"config/app.yaml"}) {
		t.Fatalf("derived view=%+v", view)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	snapshot, err := LoadSnapshot(f.binding())
	if err != nil || !SameView(snapshot, view) {
		t.Fatalf("LoadSnapshot=%+v err=%v", snapshot, err)
	}
	verified, active, err := Verify(context.Background(), f.binding())
	if err != nil || !active || !SameView(verified, view) {
		t.Fatalf("Verify=(%+v,%v,%v)", verified, active, err)
	}
}

func TestVerifyLanded_RechecksCommittedBuildAgainstSealedSnapshot(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(filepath.Join(f.worktree, ".git", "info", "exclude"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.workspace = filepath.Join(f.worktree, ".evolve", "runs", "cycle-42")
	binding := f.binding()
	binding.ProjectRoot = f.worktree
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	f.prepareRequired(t)
	if failures := CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	f.git(t, "add", "config/app.yaml", cycleDocumentPath(f.cycle, f.runID))
	f.git(t, "commit", "-q", "-m", "land Build explanation")
	landedCommit := f.git(t, "rev-parse", "HEAD")

	view, active, err := VerifyLanded(context.Background(), binding, landedCommit)
	if err != nil || !active || view.Status != statusRequired {
		t.Fatalf("VerifyLanded=(%+v,%v,%v), want required active document", view, active, err)
	}
}

func TestValidateDocument_ReportsSectionsInContractOrder(t *testing.T) {
	body := "## Build Binding\n- Cycle: 42\n- Base SHA: " + strings.Repeat("a", 40) + "\n"
	want := []string{
		"Explanation Documentation: missing ## Summary",
		"Explanation Documentation: missing ## Rationale",
		"Explanation Documentation: missing ## Changed Areas",
		"Explanation Documentation: missing ## Design Decisions",
		"Explanation Documentation: missing ## Verification",
		"Explanation Documentation: missing ## Compatibility",
		"Explanation Documentation: missing ## Limitations",
	}
	for i := 0; i < 50; i++ {
		if got := validateDocument(body, 42, strings.Repeat("a", 40), nil, nil); !slices.Equal(got, want) {
			t.Fatalf("iteration %d failures=%v, want contract order %v", i, got, want)
		}
	}
}

func TestValidateDocument_ReportsInventedPathsInSortedOrder(t *testing.T) {
	base := strings.Repeat("b", 40)
	body := "## Build Binding\n- Cycle: 42\n- Base SHA: " + base + "\n\n" +
		"## Summary\nA sufficiently detailed summary of the implementation.\n\n" +
		"## Rationale\nA sufficiently detailed rationale explaining the selected implementation tradeoff.\n\n" +
		"## Changed Areas\n- `z/file.go` — explains the invented z path in detail.\n- `a/file.go` — explains the invented a path in detail.\n\n" +
		"## Design Decisions\nThe design retains one clear source of truth.\n\n" +
		"## Verification\nTargeted behavioral tests exercise the contract.\n\n" +
		"## Compatibility\nNone.\n\n" +
		"## Limitations\nNone.\n"
	want := []string{
		"Explanation Documentation: cited path a/file.go is not in the Build diff",
		"Explanation Documentation: cited path z/file.go is not in the Build diff",
	}
	for i := 0; i < 50; i++ {
		if got := validateDocument(body, 42, base, nil, nil); !slices.Equal(got, want) {
			t.Fatalf("iteration %d failures=%v, want sorted path order %v", i, got, want)
		}
	}
}

func TestVerify_RejectsSamePathContentMutationAfterSeal(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}

	// Preserve the exact changed path set while changing the implementation
	// bytes after Build approval. Audit/Ship/Retro must detect this drift.
	f.write(t, "config/app.yaml", "enabled: compromised\n")
	if _, active, err := Verify(context.Background(), f.binding()); err == nil || !active || !strings.Contains(err.Error(), "diff SHA256") {
		t.Fatalf("same-path post-seal mutation was accepted: active=%v err=%v", active, err)
	}
}

func TestDiffSHA256_UntrackedFilesUseUnambiguousFraming(t *testing.T) {
	f := newFixture(t)
	headerB := "\nuntracked:1:b:-rw-r--r--\n"
	f.write(t, "a", headerB+"X")
	f.write(t, "b", "Y")
	first, err := diffSHA256(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "a", "")
	f.write(t, "b", "X"+headerB+"Y")
	second, err := diffSHA256(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("distinct untracked file tuples produced the same serialized digest")
	}
}

func TestDiffSHA256_RejectsOversizeUntrackedContent(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.worktree, "oversize.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(65 << 20); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := diffSHA256(context.Background(), f.worktree, f.base); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize untracked content was accepted: %v", err)
	}
}

func TestDiffSHA256_IsInvariantAcrossStageAndCommit(t *testing.T) {
	f := newFixture(t)
	f.write(t, "config/app.yaml", "enabled: true\n")
	f.write(t, "new-file.txt", "new content\n")
	before, err := diffSHA256(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatal(err)
	}
	f.git(t, "add", "-A")
	staged, err := diffSHA256(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatal(err)
	}
	f.git(t, "commit", "-q", "-m", "material change")
	committed, err := diffSHA256(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatal(err)
	}
	if before != staged || staged != committed {
		t.Fatalf("digest changed for identical final content: untracked=%s staged=%s committed=%s", before, staged, committed)
	}
}

func TestDiffSHA256_HandlesDeletedLastFileInDirectory(t *testing.T) {
	f := newFixture(t)
	f.write(t, "nested/only.txt", "delete me\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "nested base")
	base := f.git(t, "rev-parse", "HEAD")
	if err := os.RemoveAll(filepath.Join(f.worktree, "nested")); err != nil {
		t.Fatal(err)
	}
	before, err := diffSHA256(context.Background(), f.worktree, base)
	if err != nil {
		t.Fatalf("digest deleted directory leaf: %v", err)
	}
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "delete nested leaf")
	after, err := diffSHA256(context.Background(), f.worktree, base)
	if err != nil || before != after {
		t.Fatalf("deleted leaf digest before=%s after=%s err=%v", before, after, err)
	}
}

func TestRebaseBuild_UpdatesBaseAndInvalidatesApprovedSnapshot(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	f.write(t, "peer.txt", "peer change\n")
	f.git(t, "add", "peer.txt")
	f.git(t, "commit", "-q", "-m", "new base")
	newBase := f.git(t, "rev-parse", "HEAD")
	if err := RebaseBuild(context.Background(), f.binding(), newBase); err != nil {
		t.Fatalf("RebaseBuild: %v", err)
	}
	rebased := f.binding()
	rebased.BaseSHA = newBase
	if _, err := LoadSnapshot(rebased); err == nil {
		t.Fatalf("approved pre-rebase snapshot remained usable: %v", err)
	}
	found, active, err := ActivationForCycle(f.root, f.cycle, f.workspace)
	if err != nil || !active || found.BaseSHA != newBase {
		t.Fatalf("rebased activation=%+v active=%v err=%v", found, active, err)
	}
	if err := RebaseBuild(context.Background(), rebased, "not-a-commit"); err == nil {
		t.Fatal("RebaseBuild accepted malformed base SHA")
	}
}

func TestRebaseBuildAndPersist_PreservesWriteAheadMarkerWhenCheckpointWritePartiallyFails(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	f.write(t, "peer.txt", "peer change\n")
	f.git(t, "add", "peer.txt")
	f.git(t, "commit", "-q", "-m", "new base")
	newBase := f.git(t, "rev-parse", "HEAD")
	wantErr := errors.New("run-state mirror failed after checkpoint commit")
	checkpointBase := f.base
	err := RebaseBuildAndPersist(context.Background(), f.binding(), newBase, func() error {
		checkpointBase = newBase
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RebaseBuildAndPersist err=%v", err)
	}
	active, ok, loadErr := ActivationForCycle(f.root, f.cycle, f.workspace)
	if loadErr != nil || !ok || active.BaseSHA != newBase {
		t.Fatalf("host write-ahead marker was rolled back: active=%+v ok=%v err=%v", active, ok, loadErr)
	}
	recovered := f.binding()
	recovered.BaseSHA = checkpointBase
	got, recoverable, recoverErr := RecoverRebaseSplit(context.Background(), recovered)
	if recoverErr != nil || !recoverable || got != newBase {
		t.Fatalf("partial-checkpoint recovery base=%q recoverable=%v err=%v, want %q/true/nil", got, recoverable, recoverErr, newBase)
	}
}

func TestSealResult_CanceledContextStopsBeforeSealing(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SealResult(ctx, f.binding()); !errors.Is(err, context.Canceled) {
		t.Fatalf("SealResult err=%v, want context.Canceled", err)
	}
}

func TestCheckBuild_NeedsNoTriageArtifact(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if _, err := os.Stat(filepath.Join(f.workspace, "triage-decision.json")); !os.IsNotExist(err) {
		t.Fatalf("test premise: triage-decision.json err=%v", err)
	}
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("cycle-owned explanation rejected: %v", failures)
	}
}

func TestCheckBuild_RejectsSymlinkedWorkspaceBeforeWritingManifest(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	outside := t.TempDir()
	if err := os.RemoveAll(f.workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, f.workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "build-report.md"), []byte("## Explanation Documentation\n- Status: REQUIRED\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if failures := f.check(t); !containsFailure(failures, "workspace") {
		t.Fatalf("symlinked workspace accepted: %v", failures)
	}
	if _, err := os.Lstat(filepath.Join(outside, manifestFilename)); !os.IsNotExist(err) {
		t.Fatalf("manifest escaped through workspace symlink: %v", err)
	}
}

func TestNotApplicable_IsDerivedOnlyForNonMaterialDiff(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.write(t, "config/app_test.go", "package config\n")
	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: the Build changed tests only\n")
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("test-only N/A rejected: %v", failures)
	}
	view, err := Load(f.workspace)
	if err != nil || view.Status != statusNA || view.Reason != "the Build changed tests only" || view.DocumentPath != "" {
		t.Fatalf("N/A view=%+v err=%v", view, err)
	}

	f.write(t, "config/app.yaml", "enabled: true\n")
	if failures := f.check(t); !containsFailure(failures, "Status must be REQUIRED") {
		t.Fatalf("material diff accepted N/A: %v", failures)
	}
}

func TestRequiredDocument_IsCycleOwnedAndImmutable(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	f.write(t, "docs/explain/builds/cycle-41-prior-run.md", "foreign history rewrite\n")
	if failures := f.check(t); !containsFailure(failures, "foreign cycle") {
		t.Fatalf("foreign cycle record accepted: %v", failures)
	}
}

func TestArchiveUnpublishedContinuationRecords_PreservesRecordsAbsentAtBase(t *testing.T) {
	f := newFixture(t)
	oldRecord := "docs/explain/builds/cycle-41-old-run.md"
	f.write(t, oldRecord, "unshipped explanation\n")
	f.git(t, "add", oldRecord)
	f.git(t, "commit", "-q", "-m", "failed build snapshot")
	archived, err := ArchiveUnpublishedContinuationRecords(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatalf("ArchiveUnpublishedContinuationRecords: %v", err)
	}
	if len(archived) != 1 || !strings.Contains(archived[0], "docs/private/research/archived-") || filepath.Base(archived[0]) != filepath.Base(oldRecord) {
		t.Fatalf("archived=%v", archived)
	}
	if _, err := os.Stat(filepath.Join(f.worktree, filepath.FromSlash(oldRecord))); !os.IsNotExist(err) {
		t.Fatalf("canonical unpublished record still exists: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(f.worktree, filepath.FromSlash(archived[0]))); err != nil || string(body) != "unshipped explanation\n" {
		t.Fatalf("archived record body=%q err=%v", body, err)
	}
	paths, err := changedSince(context.Background(), f.worktree, f.base)
	if err != nil {
		t.Fatal(err)
	}
	if contains(paths, oldRecord) {
		t.Fatalf("canonical continuation record remains in the base-bound diff: %v", paths)
	}
	if !contains(paths, archived[0]) {
		t.Fatalf("archived continuation record is absent from the base-bound diff: %v", paths)
	}
}

func TestNotApplicable_CannotRewritePublishedCycleDocument(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.write(t, "docs/explain/builds/cycle-41-prior-run.md", "rewritten prior explanation\n")
	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: documentation-only Build\n")
	if failures := f.check(t); !containsFailure(failures, "immutable cycle explanation") {
		t.Fatalf("N/A accepted a prior cycle-document rewrite: %v", failures)
	}
}

func TestNotApplicable_AllowsNonRecordCycleDocumentation(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.write(t, "docs/explain/builds/cycle-overview.md", "editable overview\n")
	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: documentation-only Build\n")
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("ordinary cycle documentation misclassified as immutable history: %v", failures)
	}
}

func TestVerify_NotApplicableRejectsPostSealHistoryMutation(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.write(t, "config/app_test.go", "package config\n")
	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: the Build changed tests only\n")
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	f.write(t, "docs/explain/builds/cycle-41-old-run.md", "post-seal history mutation\n")
	if _, active, err := Verify(context.Background(), f.binding()); !active || err == nil || !strings.Contains(err.Error(), "diff SHA256") {
		t.Fatalf("history mutation Verify active=%v err=%v", active, err)
	}
}

func TestRequiredDocument_CannotExistAtBase(t *testing.T) {
	f := newFixture(t)
	f.write(t, cycleDocumentPath(f.cycle, f.runID), "placeholder\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "prepublish cycle doc")
	f.base = f.git(t, "rev-parse", "HEAD")
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); !containsFailure(failures, "existed at the cycle base") {
		t.Fatalf("pre-existing cycle document accepted: %v", failures)
	}
}

func TestRequiredDocument_RejectsEmptyHiddenAndUnexplainedContent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"empty section", func(body string) string {
			return strings.Replace(body, "## Summary\nEnable the application through its existing configuration field.", "## Summary\n", 1)
		}, "Summary must contain substantive"},
		{"commented rationale", func(body string) string {
			return strings.Replace(body, "## Rationale\nUsing the existing field is the smallest compatible behavior change and avoids a second configuration surface.", "## Rationale\n<!-- Using the existing field is the smallest compatible behavior change and avoids a second configuration surface. -->", 1)
		}, "Rationale must contain substantive"},
		{"indented heading", func(body string) string {
			return strings.Replace(body, "## Verification", "    ## Verification", 1)
		}, "missing ## Verification"},
		{"bare changed path", func(body string) string {
			return strings.Replace(body, "- `config/app.yaml` — flips the existing runtime setting while preserving its schema.", "- `config/app.yaml`", 1)
		}, "Changed Areas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.activate(t)
			f.prepareRequired(t)
			f.write(t, cycleDocumentPath(f.cycle, f.runID), tc.mutate(f.validDocument()))
			if failures := f.check(t); !containsFailure(failures, tc.want) {
				t.Fatalf("hostile document accepted: %v", failures)
			}
		})
	}
}

func TestBuildDeclaration_IgnoresHiddenExamplesAndRejectsDuplicates(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	f.writeWorkspace(t, "build-report.md", "<!--\n## Explanation Documentation\n- Status: REQUIRED\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n-->\n")
	if failures := f.check(t); !containsFailure(failures, "missing the required") {
		t.Fatalf("hidden declaration accepted: %v", failures)
	}

	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: REQUIRED\n- Status: NOT_APPLICABLE\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n")
	if failures := f.check(t); !containsFailure(failures, "duplicate status") {
		t.Fatalf("duplicate declaration accepted: %v", failures)
	}
}

func TestVerify_RejectsPostBuildTampering(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	f.write(t, cycleDocumentPath(f.cycle, f.runID), f.validDocument()+"\npost-build rewrite\n")
	if _, active, err := Verify(context.Background(), f.binding()); !active || err == nil || !strings.Contains(err.Error(), "diff SHA256") {
		t.Fatalf("tamper Verify active=%v err=%v", active, err)
	}
}

func TestSealResult_CorrectionReplacesApprovedSnapshot(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	first, _ := LoadSnapshot(f.binding())
	f.write(t, cycleDocumentPath(f.cycle, f.runID), strings.Replace(f.validDocument(), "This does not add per-user configuration.", "This does not add tenant-specific or per-user configuration.", 1))
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	latest, err := LoadSnapshot(f.binding())
	if err != nil || latest.DocumentSHA256 == first.DocumentSHA256 {
		t.Fatalf("correction snapshot not replaced: first=%+v latest=%+v err=%v", first, latest, err)
	}
}

func TestLegacyCycleWithoutMarker_IsNotActivatedByWorkspace(t *testing.T) {
	f := newFixture(t)
	f.prepareRequired(t)
	if failures := CheckBuild(context.Background(), CycleBinding{ProjectRoot: f.root, Cycle: f.cycle}); len(failures) != 0 {
		t.Fatalf("legacy CheckBuild=%v", failures)
	}
	if view, active, err := Verify(context.Background(), CycleBinding{ProjectRoot: f.root, Cycle: f.cycle}); err != nil || active || view != nil {
		t.Fatalf("legacy Verify=(%+v,%v,%v)", view, active, err)
	}
}

func TestActiveContract_FailsClosedOnMissingMarkerAndInvalidBase(t *testing.T) {
	f := newFixture(t)
	if failures := CheckBuild(context.Background(), f.binding()); !containsFailure(failures, "activation marker is missing") {
		t.Fatalf("missing marker did not fail: %v", failures)
	}
	f.activate(t)
	binding := f.binding()
	binding.BaseSHA = "short"
	if failures := CheckBuild(context.Background(), binding); !containsFailure(failures, "binding does not match host state") {
		t.Fatalf("invalid host base did not fail: %v", failures)
	}
}

func TestActivate_RejectsSymlinkedHostDirectory(t *testing.T) {
	f := newFixture(t)
	target := t.TempDir()
	parent := filepath.Join(f.root, activationDirectory)
	if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, parent); err != nil {
		t.Fatal(err)
	}
	binding := f.binding()
	binding.Worktree, binding.BaseSHA = "", ""
	if err := Activate(binding); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("symlinked activation parent err=%v", err)
	}
}

func TestMaterialPaths_RepositoryAwareTestArtifactsAreNonMaterial(t *testing.T) {
	got := materialPaths([]string{
		"go/internal/app/app.go",
		"go/internal/app/app_test.go",
		"tests/integration/app.go",
		"tests/integration/test_app.py",
		"pkg/app_test.py",
		"pkg/tests/helper.py",
		"web/app.spec.ts",
		"web/app.test.js",
		"go/internal/app/runtime.test.go",
		"config/app.test.yaml",
		"docs/guide.md",
		"config/testdata/input.yaml",
	})
	if !samePaths(got, []string{"config/app.test.yaml", "go/internal/app/app.go", "go/internal/app/runtime.test.go", "pkg/tests/helper.py", "tests/integration/app.go"}) {
		t.Fatalf("materialPaths=%v", got)
	}
}

func TestSealResult_RevalidatesManifestMutatedAfterBuildCheck(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	view, err := Load(f.workspace)
	if err != nil {
		t.Fatal(err)
	}
	view.MaterialPaths = []string{"forged/path.go"}
	view.DiffSHA256 = "forged"
	body, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	f.writeWorkspace(t, manifestFilename, string(body))

	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult should re-derive the valid host view: %v", err)
	}
	snapshot, err := LoadSnapshot(f.binding())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.DiffSHA256 == "forged" || !samePaths(snapshot.MaterialPaths, []string{"config/app.yaml"}) {
		t.Fatalf("SealResult preserved a post-check forged manifest: %+v", snapshot)
	}
}

func TestRefreshResult_PostBuildTestWriterKeepsAuditVerificationGreen(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	before, err := LoadSnapshot(f.binding())
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/app_test.go", "package config_test\n\nfunc Example() {}\n")

	requiresBuild, err := RefreshResult(context.Background(), f.binding())
	if err != nil || requiresBuild {
		t.Fatalf("RefreshResult requiresBuild=%v err=%v", requiresBuild, err)
	}
	after, active, err := Verify(context.Background(), f.binding())
	if err != nil || !active {
		t.Fatalf("first Audit verification after test writer: active=%v err=%v", active, err)
	}
	if after.DiffSHA256 == before.DiffSHA256 || !samePaths(after.MaterialPaths, before.MaterialPaths) {
		t.Fatalf("refresh did not preserve material contract while updating whole diff: before=%+v after=%+v", before, after)
	}
}

func TestRefreshResult_MaterialPostBuildDriftRequiresBuild(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/new.yaml", "enabled: true\n")

	requiresBuild, err := RefreshResult(context.Background(), f.binding())
	if err != nil || !requiresBuild {
		t.Fatalf("RefreshResult requiresBuild=%v err=%v, want true/nil", requiresBuild, err)
	}
}

func TestRefreshResult_ModifiedExistingMaterialPathRequiresBuild(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/app.yaml", "enabled: false\n")

	requiresBuild, err := RefreshResult(context.Background(), f.binding())
	if err != nil || !requiresBuild {
		t.Fatalf("RefreshResult requiresBuild=%v err=%v, want true/nil for changed material content", requiresBuild, err)
	}
}

func TestSealResultViewExpected_MaterialChangeDuringFinalSealRequiresBuild(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	prior, err := readResultSnapshot(f.binding())
	if err != nil {
		t.Fatal(err)
	}
	current, err := revalidateResult(context.Background(), f.binding())
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/app.yaml", "enabled: false\nchanged during seal\n")

	requiresBuild, err := sealResultViewExpected(context.Background(), f.binding(), current, prior.MaterialSHA256)
	if err != nil || !requiresBuild {
		t.Fatalf("sealResultViewExpected requiresBuild=%v err=%v, want true/nil", requiresBuild, err)
	}
	after, err := readResultSnapshot(f.binding())
	if err != nil {
		t.Fatal(err)
	}
	if after.MaterialSHA256 != prior.MaterialSHA256 {
		t.Fatal("final seal blessed material that changed after validation")
	}
}

func TestRecoverRebaseSplit_RecognizesNewMarkerWithOldApprovedSnapshot(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatal(failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatal(err)
	}
	f.write(t, "peer.txt", "peer change\n")
	f.git(t, "add", "peer.txt")
	f.git(t, "commit", "-q", "-m", "new base")
	newBase := f.git(t, "rev-parse", "HEAD")
	if err := RebaseBuild(context.Background(), f.binding(), newBase); err != nil {
		t.Fatal(err)
	}

	got, recoverable, err := RecoverRebaseSplit(context.Background(), f.binding())
	if err != nil || !recoverable || got != newBase {
		t.Fatalf("RecoverRebaseSplit base=%q recoverable=%v err=%v, want %q/true/nil", got, recoverable, err, newBase)
	}

	// The first recovery persists the new base before Build runs. If the host
	// crashes again in that window, the old-base snapshot remains the durable
	// witness that Build still has to regenerate its explanation.
	rebased := f.binding()
	rebased.BaseSHA = newBase
	got, recoverable, err = RecoverRebaseSplit(context.Background(), rebased)
	if err != nil || !recoverable || got != newBase {
		t.Fatalf("second RecoverRebaseSplit base=%q recoverable=%v err=%v, want %q/true/nil", got, recoverable, err, newBase)
	}
}

func TestSameView_UsesCompleteValue(t *testing.T) {
	a := &phaseio.ExplanationView{SchemaVersion: 1, ContractVersion: 1, Status: statusRequired, Cycle: 4, Reason: materialReason, BaseSHA: "base", DocumentPath: "doc", DocumentSHA256: "sha", MaterialPaths: []string{"a"}}
	b := *a
	if !SameView(a, &b) {
		t.Fatal("equal views did not match")
	}
	b.DocumentSHA256 = "other"
	if SameView(a, &b) || SameView(nil, a) {
		t.Fatal("different or nil views matched")
	}
}

func containsFailure(failures []string, want string) bool {
	for _, failure := range failures {
		if strings.Contains(failure, want) {
			return true
		}
	}
	return false
}

func TestActivationMarker_SeparatesHostSchemaAndContractVersion(t *testing.T) {
	f := newFixture(t)
	binding := f.binding()
	binding.Worktree, binding.BaseSHA = "", ""
	if err := Activate(binding); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(activationPath(f.root, f.cycle))
	if err != nil {
		t.Fatal(err)
	}
	var marker map[string]any
	if err := json.Unmarshal(body, &marker); err != nil {
		t.Fatal(err)
	}
	if marker["schema_version"] != float64(currentHostSchema) || marker["contract_version"] != float64(CurrentContractVersion) {
		t.Fatalf("marker versions=%v", marker)
	}
}

func TestSupportedVersionsRetainV1(t *testing.T) {
	if !supportedContract(contractV1) || !supportedHostSchema(hostSchemaV1) || !supportedArtifactSchema(artifactSchemaV1) {
		t.Fatal("v1 host and contract artifacts must remain readable after a future writer-version bump")
	}
}

func TestPermanentPathIncludesRunIdentityAndHostPathsAreCycleOwned(t *testing.T) {
	got, err := DocumentPath(42, "01ABCDEF")
	if err != nil || got != "docs/explain/builds/cycle-42-01abcdef.md" {
		t.Fatalf("DocumentPath=%q err=%v", got, err)
	}
	if _, err := DocumentPath(0, "unsafe/run"); err == nil {
		t.Fatal("DocumentPath accepted invalid identity")
	}
	if activationRel(42) != ".evolve/build-explanation-contracts/cycle-42.json" || resultSnapshotRel(42) != ".evolve/build-explanation-contracts/cycle-42-result.json" {
		t.Fatal("host contract paths must stay discoverable from the cycle identity")
	}
}

func TestPublicActivationLookupAndResumeVerification(t *testing.T) {
	f := newFixture(t)
	binding := f.binding()
	binding.Worktree, binding.BaseSHA = "", ""
	if err := Activate(binding); err != nil {
		t.Fatal(err)
	}
	found, active, err := ActivationForCycle(f.root, f.cycle, f.workspace)
	if err != nil || !active || found.Cycle != f.cycle || found.RunID != f.runID {
		t.Fatalf("ActivationForCycle=%+v active=%v err=%v", found, active, err)
	}
	if err := RequireActivation(binding); err != nil {
		t.Fatalf("RequireActivation matching marker: %v", err)
	}
	binding.RunID = "wrong-run"
	if err := RequireActivation(binding); err == nil {
		t.Fatal("RequireActivation accepted mismatched run")
	}
	if _, active, err := ActivationForCycle(f.root, f.cycle, filepath.Join(f.root, "missing")); err != nil || active {
		t.Fatalf("missing workspace active=%v err=%v", active, err)
	}
}

func TestActivationForCycle_IgnoresUnrelatedCorruptMarker(t *testing.T) {
	f := newFixture(t)
	binding := f.binding()
	binding.Worktree, binding.BaseSHA = "", ""
	if err := Activate(binding); err != nil {
		t.Fatal(err)
	}
	corrupt := activationPath(f.root, f.cycle+1)
	if err := os.WriteFile(corrupt, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, active, err := ActivationForCycle(f.root, f.cycle, f.workspace)
	if err != nil || !active || found.RunID != f.runID {
		t.Fatalf("unrelated corrupt marker affected exact lookup: found=%+v active=%v err=%v", found, active, err)
	}
}

func TestResolveActivation_ExistingMarkerFailsClosedForMissingOrWrongRunIdentity(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	for _, binding := range []CycleBinding{
		{ProjectRoot: f.root, Cycle: f.cycle},
		{ProjectRoot: f.root, Cycle: f.cycle, RunID: "wrong-run"},
	} {
		if _, active, err := resolveActivation(binding); err == nil || !active {
			t.Fatalf("resolveActivation(%+v) = active %v, err %v; want active fail-closed", binding, active, err)
		}
	}
}

// Adversarial takeover finding (PR #517 e2e diagnosis): changedSince counts
// UNTRACKED files, and the orchestrator symlinks .evolve/cycle-state.json into
// every cycle worktree. On a host whose repo lacks the production .gitignore's
// `.evolve/*` rule (the e2e temp projects; any fresh operator checkout), that
// runtime-state symlink read as a MATERIAL change — forcing Status: REQUIRED
// with a full cycle document for builds that changed no code at all, and
// hanging every e2e pipeline cycle in the correction ladder until timeout.
// Runtime state is not a material code change; the classifier must say so
// itself rather than relying on every host's ignore hygiene.
func TestMaterialPaths_RuntimeStateIsNeverMaterial(t *testing.T) {
	got := materialPaths([]string{
		".evolve/cycle-state.json",
		".evolve/runs/cycle-1/build-report.md",
		"go/internal/core/thing.go",
	})
	if len(got) != 1 || got[0] != "go/internal/core/thing.go" {
		t.Fatalf("materialPaths = %v, want only the real source change — .evolve/ runtime state must never force a REQUIRED explanation", got)
	}
}

// Review HIGH: the first fix excluded .evolve/ WHOLESALE, which laundered
// tracked config as non-material — a build whose only change flips
// .evolve/policy.json (gates, fleet width) could then declare NOT_APPLICABLE
// and skip the explanation document for exactly the consequential class this
// contract exists to document. Config is material; bookkeeping is not.
func TestMaterialPaths_TrackedEvolveConfigIsMaterial(t *testing.T) {
	got := materialPaths([]string{
		".evolve/policy.json",
		".evolve/profiles/builder.json",
		".evolve/cycle-state.json",
		".evolve/runs/cycle-9/build-report.md",
	})
	want := []string{".evolve/policy.json", ".evolve/profiles/builder.json"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("materialPaths = %v, want %v — config changes are consequential and must stay material; runtime bookkeeping must not", got, want)
	}
}

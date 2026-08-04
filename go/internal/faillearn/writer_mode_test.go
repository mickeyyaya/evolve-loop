package faillearn

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// writer_mode_test.go — RED contract for cycle-1290 T1
// (`faillearn-publish-mode-parity`, cycle-1287 audit defects[0] / F1 MEDIUM).
//
// The defect: writeIfAbsent publishes through os.CreateTemp (mode 0600) + os.Link
// with no Chmod, while internal/atomicwrite.Bytes documents and enforces 0644 for
// every other published runtime artifact. So the failure floor's OWN artifacts —
// retrospective-report.md, lessons/*.yaml, .evolve/inbox/*.json — land 0600: read
// only by the uid that minted them, while other fleet lanes and the operator are
// the intended readers. Nothing in the tree pins the mode today, which is why the
// 1285 stat-then-write → link-publish rewrite could drop it silently.
//
// publishedMode is the contract constant: the same literal atomicwrite.Bytes
// applies. It is spelled out here rather than imported so faillearn stays a leaf
// package (stdlib + yaml.v3) in test builds too; the parity is asserted against
// atomicwrite's documented value, cited at atomicwrite.go:61-63.
const publishedMode fs.FileMode = 0o644

// TestWriteArtifacts_PublishedArtifactsHaveMode0644 is the primary criterion. It
// drives the real production entry point (WriteArtifacts — the same call
// core.writeDeterministicLearning makes) and stats every artifact the call
// publishes, so it fails on the shipped defect rather than on a re-implementation
// of it. An explicit Chmod is required for this to green: os.CreateTemp's 0600 is
// not umask-derived, so no runner configuration can make the current code pass.
func TestWriteArtifacts_PublishedArtifactsHaveMode0644(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	paths := []string{filepath.Join(runDir, "retrospective-report.md")}
	for _, it := range remediationItems() {
		paths = append(paths, filepath.Join(inboxDir, it.ID+".json"))
	}
	lessons, err := os.ReadDir(lessonsDir)
	if err != nil {
		t.Fatalf("read lessons dir: %v", err)
	}
	for _, e := range lessons {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			paths = append(paths, filepath.Join(lessonsDir, e.Name()))
		}
	}
	if len(paths) != 4 { // report + 2 inbox items + 1 lesson
		t.Fatalf("expected 4 published artifacts to stat, got %d (%v)", len(paths), paths)
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("stat published artifact %s: %v", p, err)
			continue
		}
		if got := info.Mode().Perm(); got != publishedMode {
			t.Errorf("%s published with mode %04o, want %04o — os.CreateTemp yields 0600 and the publish path must Chmod to the atomicwrite contract before linking; a 0600 floor artifact is unreadable to the other fleet lanes and the operator that read it",
				filepath.Base(p), got, publishedMode)
		}
	}
}

// TestWriteArtifacts_ModeParityAlsoHoldsWithoutTheInboxOption covers the three
// option-free production callers (core/failure_learning.go, core/reset.go,
// cmd/evolve/cmd_loop_outcome.go): they publish the report and the lesson through
// the same path, so the fix must not be scoped to the inbox arm.
func TestWriteArtifacts_ModeParityAlsoHoldsWithoutTheInboxOption(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts without options: %v", err)
	}
	info, err := os.Stat(filepath.Join(runDir, "retrospective-report.md"))
	if err != nil {
		t.Fatalf("stat retrospective: %v", err)
	}
	if got := info.Mode().Perm(); got != publishedMode {
		t.Errorf("option-free retrospective published with mode %04o, want %04o", got, publishedMode)
	}
}

// TestWriteArtifacts_ExistingArtifactModeIsNotRewritten is the edge/OOD case and
// the guard against the over-broad fix. writeIfAbsent's contract is "an existing
// richer artifact wins"; a fix that chmods the DESTINATION instead of the temp
// file would also rewrite a pre-existing operator- or LLM-authored file's mode,
// which is a different (and unasked-for) behaviour change. The mode contract is
// about files this call CREATES.
func TestWriteArtifacts_ExistingArtifactModeIsNotRewritten(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	report := filepath.Join(runDir, "retrospective-report.md")
	if err := os.WriteFile(report, []byte("# richer LLM-authored retrospective\n"), 0o600); err != nil {
		t.Fatalf("seed existing retrospective: %v", err)
	}
	if err := os.Chmod(report, 0o600); err != nil { // defeat any umask interference on the seed
		t.Fatalf("chmod seed: %v", err)
	}

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	info, err := os.Stat(report)
	if err != nil {
		t.Fatalf("stat preserved retrospective: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("pre-existing retrospective mode was rewritten to %04o — the skip path must leave a preserved artifact entirely untouched (content AND mode)", got)
	}
	if body, err := os.ReadFile(report); err != nil || string(body) != "# richer LLM-authored retrospective\n" {
		t.Errorf("pre-existing retrospective content was clobbered: %q (err=%v)", string(body), err)
	}
}

package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const degradedRetroName = "retrospective-unqueued.md"

func blockedInboxPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(p, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("prepare blocked inbox path: %v", err)
	}
	return p
}

func TestWriteArtifacts_InboxFailureWritesUnqueuedRetro(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blockedInboxPath(t), remediationItems()))

	if err == nil {
		t.Fatal("WriteArtifacts must still return the inbox-write error — preserving the diagnosis is an ADDITION to failing loudly, never a replacement for it")
	}

	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while the remediation items reached no queue — that is the exact 1255 state, and it is what the abort ordering exists to make unreachable")
	}

	degraded := filepath.Join(runDir, degradedRetroName)
	raw, readErr := os.ReadFile(degraded)
	if readErr != nil {
		t.Fatalf("a disk-level inbox failure must still leave the diagnosis on disk as %s: %v", degradedRetroName, readErr)
	}
	body := string(raw)

	if !strings.Contains(body, "UNQUEUED") {
		t.Errorf("%s must carry an explicit UNQUEUED marker — an unmarked degraded retrospective reads as a complete one:\n%s", degradedRetroName, body)
	}
	for _, it := range remediationItems() {
		if !strings.Contains(body, it.ID) {
			t.Errorf("%s does not name unqueued remediation item %q — the items are the work that was lost, so omitting them loses it again:\n%s", degradedRetroName, it.ID, body)
		}
	}
	if !strings.Contains(body, remediationEvent().Summary) {
		t.Errorf("%s must contain the failure diagnosis (summary %q), not only a marker:\n%s", degradedRetroName, remediationEvent().Summary, body)
	}
	if info, statErr := os.Stat(degraded); statErr == nil {
		if got := info.Mode().Perm(); got != publishedMode {
			t.Errorf("%s published with mode %04o, want %04o", degradedRetroName, got, publishedMode)
		}
	}
}

func TestWriteArtifacts_InboxFailureDegradedRetroIsIdempotent(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	blocked := blockedInboxPath(t)

	first := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blocked, remediationItems()))
	if first == nil {
		t.Fatal("first call must return the inbox error")
	}
	before, err := os.ReadFile(filepath.Join(runDir, degradedRetroName))
	if err != nil {
		t.Fatalf("first call must write %s: %v", degradedRetroName, err)
	}

	second := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blocked, remediationItems()))
	if second == nil {
		t.Fatal("second call must still return the inbox error — a retry does not become a success because the marker is already there")
	}
	after, err := os.ReadFile(filepath.Join(runDir, degradedRetroName))
	if err != nil {
		t.Fatalf("read %s after retry: %v", degradedRetroName, err)
	}
	if string(before) != string(after) {
		t.Errorf("retry rewrote %s — the preserve-existing contract must govern the degraded artifact too", degradedRetroName)
	}
	if n := countFilesWithSuffix(t, runDir, ".md"); n != 1 {
		t.Errorf("retry left %d .md artifacts in the run dir, want exactly 1 (no per-attempt marker sprawl)", n)
	}
}

func TestWriteArtifacts_SuccessMintsNoUnqueuedMarker(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("healthy WriteArtifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, degradedRetroName)); err == nil {
		t.Errorf("%s was minted on a successful call — the degraded marker must appear only when the queue write actually failed", degradedRetroName)
	}
	if _, err := os.Stat(filepath.Join(runDir, "retrospective-report.md")); err != nil {
		t.Errorf("the canonical retrospective must still be the artifact of a successful call: %v", err)
	}

	plainRun, plainLessons := t.TempDir(), t.TempDir()
	if err := WriteArtifacts(remediationEvent(), plainRun, plainLessons); err != nil {
		t.Fatalf("option-free WriteArtifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(plainRun, degradedRetroName)); err == nil {
		t.Errorf("%s was minted by an option-free call — there was no queue to miss", degradedRetroName)
	}
}

func TestWriteArtifacts_InboxFailureWithNoRunDirStillErrors(t *testing.T) {
	lessonsDir := t.TempDir()

	err := WriteArtifacts(remediationEvent(), "", lessonsDir, WithInbox(blockedInboxPath(t), remediationItems()))
	if err == nil {
		t.Fatal("inbox failure with no run dir must still return an error")
	}
	if _, statErr := os.Stat(degradedRetroName); statErr == nil {
		t.Errorf("%s was written relative to the process working directory — an empty runDir means there is no workspace to publish into, not that the cwd is one", degradedRetroName)
	}
}

func TestWriteArtifacts_ItemLevelRejectionAlsoPreservesDiagnosis(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	bad := []InboxItem{{ID: "", Title: "unaddressable remediation item", Weight: 0.9, Kind: "bug", Priority: "H", InjectedBy: "retrofile"}}
	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, bad))
	if err == nil {
		t.Fatal("an item with no id must still be rejected loudly")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md must not be written when an item was rejected")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, degradedRetroName)); statErr != nil {
		t.Errorf("an item-level inbox rejection must preserve the diagnosis too, not only a disk-level one: %v", statErr)
	}
}

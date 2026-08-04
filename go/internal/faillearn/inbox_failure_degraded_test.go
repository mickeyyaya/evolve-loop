package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inbox_failure_degraded_test.go — RED contract for cycle-1290 T2
// (`faillearn-inbox-failure-preserves-diagnosis`), the residual the cycle-1287
// landing note explicitly "named here rather than closed":
//
//	"a disk-level inbox failure still suppresses the retrospective".
//
// The 1255 invariant is load-bearing and is NOT reversed: an on-disk
// `retrospective-report.md` may never claim remediation that reached no queue, so
// WriteArtifacts must keep aborting before it writes that file and must keep
// returning the error. What it must stop doing is losing the DIAGNOSIS with the
// queue write: today an unwritable inbox dir yields zero artifacts on disk, so the
// failure analysis dies with the failure it describes.
//
// Design decision this contract freezes (surfaced rather than assumed — the scout
// report asks for "a retrospective marked UNQUEUED" while
// inbox_transactional_test.go asserts `retrospective-report.md` is ABSENT on this
// arm, and that file is required to stay unmodified and green): the degraded
// artifact is published under a DISTINCT name, `retrospective-unqueued.md`. A
// distinct name satisfies both halves at once — no reader or gate that keys on
// `retrospective-report.md` can mistake a degraded diagnosis for a complete,
// queued one, and the diagnosis survives. Reusing the canonical name would force
// an edit to the transactional test, which hypothesis H3 defines as the signal
// that the design is wrong.
const degradedRetroName = "retrospective-unqueued.md"

// blockedInboxPath returns a path where the inbox DIRECTORY must go but a regular
// file sits instead: MkdirAll and create both fail ENOTDIR. Same injection the
// transactional test uses — deterministic, needs no fault-injection seam, and is
// not defeated by a root CI runner the way a chmod-based injection would be.
func blockedInboxPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(p, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("prepare blocked inbox path: %v", err)
	}
	return p
}

// TestWriteArtifacts_InboxFailureWritesUnqueuedRetro is the primary criterion:
// the failure arm gains a behaviour, the ordering keeps its. All four properties
// are asserted together because any three of them are satisfiable by a wrong fix.
func TestWriteArtifacts_InboxFailureWritesUnqueuedRetro(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blockedInboxPath(t), remediationItems()))

	// (1) Fail loudly — swallowing the error to "succeed with a marker" is the
	// cheapest gaming fake for this criterion.
	if err == nil {
		t.Fatal("WriteArtifacts must still return the inbox-write error — preserving the diagnosis is an ADDITION to failing loudly, never a replacement for it")
	}

	// (2) The 1255 invariant is untouched: no canonical retrospective on disk.
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while the remediation items reached no queue — that is the exact 1255 state, and it is what the abort ordering exists to make unreachable")
	}

	// (3) The diagnosis survives, under the degraded name.
	degraded := filepath.Join(runDir, degradedRetroName)
	raw, readErr := os.ReadFile(degraded)
	if readErr != nil {
		t.Fatalf("a disk-level inbox failure must still leave the diagnosis on disk as %s: %v", degradedRetroName, readErr)
	}
	body := string(raw)

	// (4) It is self-describing: an explicit UNQUEUED marker, and every
	// remediation item that did NOT reach the queue is named by id, so the
	// operator (or the next continuation's ledger reconcile) can requeue them
	// without re-deriving them from the failure.
	if !strings.Contains(body, "UNQUEUED") {
		t.Errorf("%s must carry an explicit UNQUEUED marker — an unmarked degraded retrospective reads as a complete one:\n%s", degradedRetroName, body)
	}
	for _, it := range remediationItems() {
		if !strings.Contains(body, it.ID) {
			t.Errorf("%s does not name unqueued remediation item %q — the items are the work that was lost, so omitting them loses it again:\n%s", degradedRetroName, it.ID, body)
		}
	}
	// The failure analysis itself, not just a stub: the summary the event carries
	// must be present, which is what makes this artifact worth keeping.
	if !strings.Contains(body, remediationEvent().Summary) {
		t.Errorf("%s must contain the failure diagnosis (summary %q), not only a marker:\n%s", degradedRetroName, remediationEvent().Summary, body)
	}
	// Published like every other floor artifact (T1's contract applies here too).
	if info, statErr := os.Stat(degraded); statErr == nil {
		if got := info.Mode().Perm(); got != publishedMode {
			t.Errorf("%s published with mode %04o, want %04o", degradedRetroName, got, publishedMode)
		}
	}
}

// TestWriteArtifacts_InboxFailureDegradedRetroIsIdempotent is the retry case. The
// floor is re-entered on the next attempt of the same failure; a second call must
// not clobber a richer artifact nor error on "already exists" — writeIfAbsent's
// preserve-existing contract governs the degraded file exactly as it governs the
// canonical one.
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

// TestWriteArtifacts_SuccessMintsNoUnqueuedMarker is the negative case: the
// degraded artifact is failure-arm-only. A fix that always writes it would make
// every healthy cycle look like a suppressed one.
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

	// And with no inbox configured at all (the three option-free callers).
	plainRun, plainLessons := t.TempDir(), t.TempDir()
	if err := WriteArtifacts(remediationEvent(), plainRun, plainLessons); err != nil {
		t.Fatalf("option-free WriteArtifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(plainRun, degradedRetroName)); err == nil {
		t.Errorf("%s was minted by an option-free call — there was no queue to miss", degradedRetroName)
	}
}

// TestWriteArtifacts_InboxFailureWithNoRunDirStillErrors is the edge/OOD case:
// loop-scope fatals call WriteArtifacts with an empty runDir (no cycle workspace
// exists). The degraded path must not invent a location, must not panic, and must
// still surface the error.
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

// TestWriteArtifacts_ItemLevelRejectionAlsoPreservesDiagnosis widens the arm from
// "unwritable directory" to the other reachable inbox failure: a rejected item
// (unaddressable id). The residual is about losing the diagnosis when the queue
// write fails — the REASON it failed is not part of the contract, and a fix keyed
// narrowly on ENOTDIR would leave the id-collision and bad-id arms still silent.
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

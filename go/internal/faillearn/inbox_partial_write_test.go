package faillearn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inbox_partial_write_test.go — RED contract for cycle-1292 T1, closing
// cycle-1290 defect D2 (`.evolve/runs/cycle-1290/defect-dispositions.json`,
// evidence `go/internal/faillearn/writer.go:96`):
//
//	writeInboxItems is not atomic across items — it writes one file per item and
//	returns on the FIRST failure, so every item BEFORE the failing one is already
//	on disk. preserveDiagnosis then lists every configured item as "still
//	UNQUEUED", so on a partial write the degraded artifact OVERCLAIMS which
//	remediation reached no queue.
//
// The overclaim is the defect the continuation-defect-ledger lane exists to
// catch: an artifact asserting on disk something that is false on disk. The
// direction of the error is safe (re-filing an identical item is idempotent by
// writeIfAbsent's same-content rule), so what is at stake is operator and
// next-continuation confusion — an item listed as unqueued that IS queued sends
// the reader looking for work that is already filed.
//
// What this contract does NOT freeze: the mechanism. preserveDiagnosis today has
// only c.inboxItems to work from and therefore CANNOT distinguish queued from
// unqueued (scout Key Finding 1); making it able to is the builder's design
// choice — a returned count, a returned id set, a partial-write marker type. The
// assertions below are on the EMITTED ARTIFACT only, so any of those fixes pass
// and none is mandated.
//
// The 1255 invariant and the 1287 residual fix are both untouched here:
// retrospective-report.md must still be absent on every failure arm, and
// WriteArtifacts must still return the error. Those are asserted alongside the
// new property because a fix that regresses either while getting the item list
// right is the fix being wrong.

// partialWriteItems is a THREE-item fixture: the middle item is the one made to
// fail, so the fixture separates "items before the failure" (queued) from "the
// failing item and everything after it" (unqueued). A two-item fixture cannot
// tell a correct fix from one that merely drops the last item.
func partialWriteItems() []InboxItem {
	return []InboxItem{
		{
			ID:         "retro-1292-first-item-reaches-disk",
			Title:      "First remediation item — written before the failure",
			Weight:     0.95,
			Kind:       "bug",
			Priority:   "H",
			Files:      []string{"go/internal/faillearn/inbox.go"},
			InjectedBy: "retrofile",
		},
		{
			ID:         "retro-1292-second-item-fails",
			Title:      "Second remediation item — the write that fails",
			Weight:     0.9,
			Kind:       "bug",
			Priority:   "H",
			Files:      []string{"go/internal/faillearn/writer.go"},
			InjectedBy: "retrofile",
		},
		{
			ID:         "retro-1292-third-item-never-attempted",
			Title:      "Third remediation item — never attempted",
			Weight:     0.85,
			Kind:       "bug",
			Priority:   "M",
			Files:      []string{"go/internal/faillearn/inbox.go"},
			InjectedBy: "retrofile",
		},
	}
}

// unqueuedSection returns ONLY the body of the degraded artifact's "still
// UNQUEUED" list, up to the next markdown heading.
//
// Section-scoped, never whole-file: the degraded artifact embeds the full
// rendered retrospective, whose defect text and evidence paths can legitimately
// mention an item id. A whole-file `strings.Contains` would therefore green on a
// tree that still lists the queued item, which is the exact defect under test.
func unqueuedSection(t *testing.T, body string) string {
	t.Helper()
	const heading = "still UNQUEUED"
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "#") && strings.Contains(ln, heading) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("degraded artifact has no %q section — the list of items that reached no queue is the payload of this artifact:\n%s", heading, body)
	}
	var out []string
	for _, ln := range lines[start:] {
		if strings.HasPrefix(ln, "#") {
			break
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// collidingInboxDir prepares an inbox directory in which the item at index
// failIdx of partialWriteItems() cannot be written: a file already sits under
// that id carrying DIFFERENT content, which writeInboxItems refuses to drop
// (the cycle-1282 DEF-4 id-collision rule).
//
// This injection — rather than an unwritable directory — is what produces a
// genuine PARTIAL write: every item before failIdx is written normally, so the
// arm exercises the "some reached disk" state instead of the "none did" state.
func collidingInboxDir(t *testing.T, failIdx int) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare inbox dir: %v", err)
	}
	victim := partialWriteItems()[failIdx]
	body, err := json.MarshalIndent(InboxItem{
		ID:         victim.ID,
		Title:      "a DIFFERENT item already filed under this id by another lane",
		Weight:     0.5,
		Kind:       "chore",
		Priority:   "L",
		InjectedBy: "other-lane",
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode colliding item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, victim.ID+".json"), body, 0o644); err != nil {
		t.Fatalf("write colliding item: %v", err)
	}
	return dir
}

// TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems is the primary criterion
// for 1290-D2: on a partial write the degraded artifact must name the items that
// reached no queue and MUST NOT name the item that did.
func TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	items := partialWriteItems()
	inboxDir := collidingInboxDir(t, 1)

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, items))

	// (1) Still fails loudly — the overclaim fix is an accuracy fix, never a
	// downgrade of the failure.
	if err == nil {
		t.Fatal("an id collision must still abort WriteArtifacts — preserving an accurate item list is an ADDITION to failing loudly")
	}
	// (2) The 1255 invariant is untouched.
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while remediation items reached no queue — the 1255 abort ordering must stay unreversed")
	}
	// (3) Premise of the whole test: item 0 really is on disk. If this fails the
	// injection stopped producing a PARTIAL write and the rest proves nothing.
	queued := items[0]
	if _, statErr := os.Stat(filepath.Join(inboxDir, queued.ID+".json")); statErr != nil {
		t.Fatalf("fixture premise broken: item %q was expected to reach disk before the failing item: %v", queued.ID, statErr)
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("the diagnosis must still be preserved as %s: %v", unqueuedRetroName, readErr)
	}
	section := unqueuedSection(t, string(raw))

	// (4) The defect: the item that DID reach the queue must not be listed as
	// unqueued.
	if strings.Contains(section, queued.ID) {
		t.Errorf("%s lists %q as still UNQUEUED, but that item is on disk in the inbox — the degraded artifact overclaims which remediation was lost (cycle-1290 D2)\n--- UNQUEUED section ---\n%s", unqueuedRetroName, queued.ID, section)
	}
	// (5) …and the items that genuinely reached no queue must still be named,
	// so the fix cannot be "list nothing".
	for _, it := range items[1:] {
		if !strings.Contains(section, it.ID) {
			t.Errorf("%s omits %q from the UNQUEUED list — that item reached no queue and is the work that would otherwise be lost\n--- UNQUEUED section ---\n%s", unqueuedRetroName, it.ID, section)
		}
	}
	// (6) The artifact is still self-describing.
	if !strings.Contains(string(raw), "UNQUEUED") {
		t.Errorf("%s must keep its explicit UNQUEUED marker", unqueuedRetroName)
	}
}

// TestWriteArtifacts_PartialWriteItemRejectionNamesOnlyUnqueuedItems widens the
// arm to the OTHER reachable per-item failure: an unaddressable id, rejected by
// writeInboxItems before any disk contact for that item. A fix keyed narrowly on
// the id-collision branch would leave this one still overclaiming.
func TestWriteArtifacts_PartialWriteItemRejectionNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	inboxDir := filepath.Join(t.TempDir(), "inbox")

	items := partialWriteItems()
	items[1].ID = "" // unaddressable — rejected loudly, after item 0 is on disk

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, items))
	if err == nil {
		t.Fatal("an item with no id must still be rejected loudly")
	}
	queued := items[0]
	if _, statErr := os.Stat(filepath.Join(inboxDir, queued.ID+".json")); statErr != nil {
		t.Fatalf("fixture premise broken: item %q was expected to reach disk before the rejected item: %v", queued.ID, statErr)
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("an item-level rejection must still preserve the diagnosis: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))

	if strings.Contains(section, queued.ID) {
		t.Errorf("%s lists %q as still UNQUEUED although it reached the inbox before the rejected item\n--- UNQUEUED section ---\n%s", unqueuedRetroName, queued.ID, section)
	}
	if !strings.Contains(section, items[2].ID) {
		t.Errorf("%s omits %q — an item after the rejection point reached no queue and must still be named\n--- UNQUEUED section ---\n%s", unqueuedRetroName, items[2].ID, section)
	}
}

// TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem is the negative /
// boundary case, and the strongest guard against the cheap wrong fix. When the
// inbox DIRECTORY itself is unwritable, item 0 never reaches disk either, so the
// correct list is ALL THREE items. A fix that assumes "everything before the
// error index succeeded", or that simply drops the first entry, under-claims
// here — losing exactly the work the degraded artifact exists to preserve.
func TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	items := partialWriteItems()

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blockedInboxPath(t), items))
	if err == nil {
		t.Fatal("an unwritable inbox directory must still return an error")
	}
	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("the diagnosis must be preserved on a total inbox failure: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))
	for _, it := range items {
		if !strings.Contains(section, it.ID) {
			t.Errorf("%s omits %q, but NOTHING reached the queue on this arm — under-claiming loses the work the artifact exists to preserve\n--- UNQUEUED section ---\n%s", unqueuedRetroName, it.ID, section)
		}
	}
}

// TestWriteArtifacts_PartialWrite_FirstItemFailsNamesEveryItem is the boundary
// at index 0: the failing item is the first one, so nothing precedes it and the
// full list is again correct. Pins the off-by-one a "skip items[:failIdx]" fix
// invites.
func TestWriteArtifacts_PartialWrite_FirstItemFailsNamesEveryItem(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	items := partialWriteItems()

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(collidingInboxDir(t, 0), items))
	if err == nil {
		t.Fatal("a collision on the first item must still abort")
	}
	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("read %s: %v", unqueuedRetroName, readErr)
	}
	section := unqueuedSection(t, string(raw))
	for _, it := range items {
		if !strings.Contains(section, it.ID) {
			t.Errorf("%s omits %q although the very first write failed — no item reached the queue on this arm\n--- UNQUEUED section ---\n%s", unqueuedRetroName, it.ID, section)
		}
	}
}

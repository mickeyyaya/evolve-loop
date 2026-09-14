package verdict

// artifact_test.go — the on-disk helpers (ADR-0103 unit 11 §6 tests 28-31): the
// single-read decision, the forensic renderers, the (size, mtime) snapshot, the
// challenge-token reader.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// Test 28 — the four branches of classifiedArtifact: the verified bytes when
// they are this artifact's, else ONE read, else "" for an absent contracted
// file, else the pane. Kills `pane returned for an absent contracted file`,
// `re-read when the snapshot matches`.
func TestClassifiedArtifact_VerifiedBytesThenOneReadThenPaneOrEmpty(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "audit-report.md")
	if err := os.WriteFile(path, []byte("on disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := classifiedArtifact(deliverable.Result{OK: true, ArtifactPath: path, Content: "verified"}, path, "pane"); got != "verified" {
		t.Errorf("same file + bytes ⇒ the verified bytes, never a re-read: %q", got)
	}
	if got := classifiedArtifact(deliverable.Result{OK: true, ArtifactPath: filepath.Join(ws, "other.md"), Content: "other"}, path, "pane"); got != "on disk" {
		t.Errorf("a different file's bytes ⇒ one read of the dispatched artifact: %q", got)
	}
	if got := classifiedArtifact(deliverable.Result{OK: true, ArtifactPath: path, Content: ""}, path, "pane"); got != "on disk" {
		t.Errorf("empty verified bytes are not evidence of absence ⇒ one read: %q", got)
	}
	absent := filepath.Join(ws, "absent.md")
	if got := classifiedArtifact(deliverable.Result{OK: false}, absent, "pane"); got != "" {
		t.Errorf("absent + !OK ⇒ empty (a coherent FAIL, not a pane-scraped one): %q", got)
	}
	if got := classifiedArtifact(deliverable.Result{OK: true}, absent, "pane"); got != "pane" {
		t.Errorf("absent + OK (a NoArtifact contract) ⇒ the pane: %q", got)
	}
}

// Test 29 — forensicSnapshot renders absent / size + the LAST tailN bytes
// quoted / a directory as size=N tail="" (the discarded ReadFile error, Q2);
// forensicCodes joins with a comma. Kills `head instead of tail`, `absent on
// read error`.
func TestForensicSnapshot_AbsentSizeTailAndDirectory(t *testing.T) {
	ws := t.TempDir()
	if got := forensicSnapshot(filepath.Join(ws, "absent"), 160); got != "absent" {
		t.Errorf("absent: %q", got)
	}
	body := strings.Repeat("a", 340) + strings.Repeat("z", 160)
	path := filepath.Join(ws, "report.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := forensicSnapshot(path, 160), `size=500 tail="`+strings.Repeat("z", 160)+`"`; got != want {
		t.Errorf("tail of 500 bytes at 160: %q", got)
	}
	if got := forensicSnapshot(path, 1000); got != `size=500 tail="`+body+`"` {
		t.Errorf("a tail wider than the file is the whole file: %q", got)
	}
	dir := filepath.Join(ws, "acs-verdict.json")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := forensicSnapshot(dir, 200); !strings.HasPrefix(got, "size=") || !strings.HasSuffix(got, ` tail=""`) {
		t.Errorf("a directory: Stat succeeds, the read yields nothing: %q", got)
	}
}

func TestForensicCodes_JoinsWithComma(t *testing.T) {
	if got := forensicCodes([]deliverable.Violation{{Code: "a"}, {Code: "b"}, {Code: "c"}}); got != "a,b,c" {
		t.Errorf("%q", got)
	}
	if got := forensicCodes(nil); got != "" {
		t.Errorf("no violations ⇒ empty: %q", got)
	}
}

// Test 30 — StatSnapshot needs a non-empty regular file; unchangedSince keys
// on size AND mtime and reads any error or non-regular file as changed
// (fail-open toward the pre-existing reconcile). Kills `size-only key`, `error
// reads as unchanged`, `Stat instead of Lstat`.
func TestStatSnapshot_RequiresNonEmptyRegularFile(t *testing.T) {
	ws := t.TempDir()
	empty := filepath.Join(ws, "empty.md")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := StatSnapshot(empty); ok {
		t.Error("an empty file does not snapshot")
	}
	if _, ok := StatSnapshot(ws); ok {
		t.Error("a directory does not snapshot")
	}
	if _, ok := StatSnapshot(filepath.Join(ws, "absent")); ok {
		t.Error("an absent file does not snapshot")
	}
	path := filepath.Join(ws, "report.md")
	if err := os.WriteFile(path, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "link.md")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, ok := StatSnapshot(link); ok {
		t.Error("a symlink is not a regular file under Lstat")
	}
	snap, ok := StatSnapshot(path)
	if !ok || snap.size != 4 {
		t.Fatalf("a non-empty regular file snapshots its size: %+v %v", snap, ok)
	}
}

func TestUnchangedSince_FailsOpenOnErrorAndNonRegular(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "report.md")
	if err := os.WriteFile(path, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	snap, _ := StatSnapshot(path)
	if !unchangedSince(path, snap) {
		t.Error("same size and mtime ⇒ unchanged")
	}
	if err := os.WriteFile(path, []byte("BODY"), 0o644); err != nil { // same size, new mtime
		t.Fatal(err)
	}
	if unchangedSince(path, snap) {
		t.Error("a touched mtime at the same size ⇒ changed (size alone is not the key)")
	}
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if !unchangedSince(path, snap) {
		t.Error("restoring the mtime restores the identity")
	}
	if err := os.WriteFile(path, []byte("longer body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if unchangedSince(path, snap) {
		t.Error("a different size at the same mtime ⇒ changed")
	}
	if unchangedSince(filepath.Join(ws, "absent"), snap) {
		t.Error("a missing file reads as changed")
	}
	if unchangedSince(ws, snap) {
		t.Error("a directory reads as changed")
	}
}

// Test 31a moved verbatim to phasecontract/challenge_token_test.go (review
// fold F1: the token reader is the contract's, beside RequireChallengeToken).
// Test 31b — the leaf reads the token through phasecontract.ChallengeToken and
// never spells the token file itself (a source scan, like test 31's `.evolve`).
// Kills `token read inline`.
func TestChallengeToken_ReadThroughPhasecontract(t *testing.T) {
	for _, name := range nonTestSources(t) {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "challenge-token.txt") {
			t.Errorf("%s spells the challenge-token file — read it through phasecontract.ChallengeToken", name)
		}
	}
}

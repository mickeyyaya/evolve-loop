package cycleoutcome

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestCodeStaleDropRepeated_RegisteredUnderInbox(t *testing.T) {
	owner, ok := signalcenter.IsRegistered(codeStaleDropRepeated)
	if !ok || owner != signalcenter.ModuleInbox {
		t.Fatalf("%s owner = %q (registered=%v), want %q", codeStaleDropRepeated, owner, ok, signalcenter.ModuleInbox)
	}
	for _, c := range signalcenter.RegistryConflicts() {
		if c.Code == codeStaleDropRepeated {
			t.Fatalf("registry conflict on %s: %+v", codeStaleDropRepeated, c)
		}
	}
}

func TestVerifyStaleEvidence_Branches(t *testing.T) {
	if verifyStaleEvidence("", "abc1234") || verifyStaleEvidence(t.TempDir(), "") {
		t.Error("empty root or empty token must return false")
	}
	if verifyStaleEvidence(t.TempDir(), "../outside.go") || verifyStaleEvidence(t.TempDir(), "/abs/path.go") {
		t.Error("escaping paths must return false")
	}
	if isHexSHA("123456") || isHexSHA("zzzzzzz") {
		t.Error("short or non-hex strings must not match SHA shape")
	}
	repo := gittest.Fixture(t)
	repo.Write("go/a.go", "package a\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "a")
	sha := strings.TrimSpace(repo.Git("rev-parse", "--short", "HEAD"))
	if !verifyStaleEvidence(repo.Dir, sha) {
		t.Errorf("ancestor sha %s must verify", sha)
	}
	if !verifyStaleEvidence(repo.Dir, "go/a.go:1") {
		t.Error("file at HEAD must verify")
	}
	if verifyStaleEvidence(repo.Dir, "go/missing.go:1") {
		t.Error("absent file must not verify")
	}
}

func TestVerifyScoutCitation_Branches(t *testing.T) {
	if verifyScoutCitation("", "id", "absent: x") || verifyScoutCitation(t.TempDir(), "", "absent: x") {
		t.Error("empty workspace or id must return false")
	}
	ws := t.TempDir()
	if !verifyScoutCitation(ws, "id", "absent: no scout report") {
		t.Error("missing scout-report.md has zero occurrences of id")
	}
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("line 1\nitem id here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !verifyScoutCitation(ws, "id", "absent: scout-report.md:1-3") {
		t.Error("cited range covering line 2 must verify")
	}
	if verifyScoutCitation(ws, "id", "invalid: scout-report.md:1") {
		t.Error("uncited line 2 must fail")
	}
	if !verifyScoutCitation(ws, "other", "absent: none") {
		t.Error("absent id must verify")
	}
	if got := parseScoutCitations("absent: scout-report.md:9-3"); len(got) != 0 {
		t.Errorf("inverted range must be ignored, got %+v", got)
	}
	wsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(wsDir, "scout-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if verifyScoutCitation(wsDir, "id", "absent: scout-report.md:1") {
		t.Error("unreadable scout-report.md must fail closed")
	}
}

func TestStampDropVerification_AppliesAndEscalates(t *testing.T) {
	root, ws := seedLane(t, []string{"item"}, []string{"item"}, `{"top_n":[],"dropped":[{"id":"item","reason":"stale: closed"}]}`)
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("item mentioned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, events := recordingCenter()
	in := NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}.WithSignals(c)
	if _, err := ApplyNoWork(in); err != nil {
		t.Fatalf("first ApplyNoWork: %v", err)
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	if _, err := inboxmover.RouteLane(opts, "item", "re-queued"); err != nil {
		t.Fatalf("RouteLane: %v", err)
	}
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 8, Stderr: io.Discard}.WithSignals(c)); err != nil {
		t.Fatalf("second ApplyNoWork: %v", err)
	}
	rec := itemRecord(t, filepath.Join(root, ".evolve", "inbox"), "item")
	if rec["evidence_verified"] != false || rec["stale_drop_count"] != float64(2) {
		t.Fatalf("rec = %v, want evidence_verified=false and stale_drop_count=2", rec)
	}
	repeated := 0
	for _, e := range *events {
		if e.Code == codeStaleDropRepeated {
			repeated++
		}
	}
	if repeated != 1 {
		t.Errorf("repeated events = %d, want 1", repeated)
	}

	root2, ws2 := seedLane(t, []string{"item"}, []string{"item"}, `{"top_n":[],"dropped":[{"id":"item","reason":"absent: none"}]}`)
	if err := os.WriteFile(filepath.Join(ws2, "scout-report.md"), []byte("no mention\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root2, Workspace: ws2, Cycle: 7, Stderr: io.Discard}); err != nil {
		t.Fatalf("absent ApplyNoWork: %v", err)
	}
	if rec2 := itemRecord(t, filepath.Join(root2, ".evolve", "inbox"), "item"); rec2["evidence_verified"] != true {
		t.Errorf("absent drop evidence_verified = %v, want true", rec2["evidence_verified"])
	}
	if err := stampDropVerification(NoWorkInputs{ProjectRoot: t.TempDir(), Workspace: ws2}, committedset.Disposition{ID: "ghost", Bucket: "dropped", Reason: "stale: deadbeef"}); err != nil {
		t.Errorf("unbacked item must not return an error, got %v", err)
	}
	badInbox := t.TempDir()
	badPath := filepath.Join(badInbox, ".evolve", "inbox", "bad.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Dir(badPath)
	if err := os.WriteFile(badPath, []byte(`{"id":"bad"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(badDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })
	if err := stampDropVerification(NoWorkInputs{ProjectRoot: badInbox, Workspace: ws2}, committedset.Disposition{ID: "bad", Bucket: "dropped", Reason: "stale: deadbeef"}); err == nil {
		t.Fatal("read-only inbox dir must return write error from stampDropVerification")
	}
}

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const (
	treeSHA40   = "1111111111111111111111111111111111111111"
	commitSHA40 = "2222222222222222222222222222222222222222"
)

func consoleRecord() DeliverableRecord {
	return DeliverableRecord{
		Schema:  RecordSchema,
		Source:  SourceConsolePR,
		Wave:    ConsoleWave,
		PR:      41,
		Commits: []CommitIdentity{{SHA: commitSHA40, BaseSHA: treeSHA40, PatchID: "p1"}},
		Audit:   AuditBinding{TreeSHA: treeSHA40, ArtifactSHA: fixtureArtifact},
	}
}

func TestConsoleRecordValidatesWithoutACycleNumber(t *testing.T) {
	t.Parallel()
	rec := consoleRecord()
	if err := rec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if rec.Cycle != 0 || rec.Trailer != "" {
		t.Fatalf("a console record must hold no cycle and no trailer: %+v", rec)
	}
	if got := rec.Label(); got != "PR #41" {
		t.Errorf("Label = %q, want PR #41", got)
	}
	if rec.Ref() != 41 {
		t.Errorf("Ref = %d, want 41", rec.Ref())
	}
	if id := rec.Deliverable().ID; id != "pr-41" {
		t.Errorf("Deliverable ID = %q, want pr-41", id)
	}
}

func TestConsoleRecordRefusesBadShapes(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*DeliverableRecord){
		"no pr":            func(r *DeliverableRecord) { r.PR = 0 },
		"a cycle":          func(r *DeliverableRecord) { r.Cycle = 7 },
		"no wave":          func(r *DeliverableRecord) { r.Wave = "" },
		"no commits":       func(r *DeliverableRecord) { r.Commits = nil },
		"bad commit":       func(r *DeliverableRecord) { r.Commits[0].PatchID = "" },
		"unknown kind":     func(r *DeliverableRecord) { r.Source = "mystery" },
		"no review":        func(r *DeliverableRecord) { r.Audit = AuditBinding{ArtifactSHA: fixtureArtifact} },
		"no digest":        func(r *DeliverableRecord) { r.Audit.ArtifactSHA = "" },
		"digest too short": func(r *DeliverableRecord) { r.Audit.ArtifactSHA = "abc" },
	} {
		rec := consoleRecord()
		rec.Commits = append([]CommitIdentity(nil), rec.Commits...)
		mutate(&rec)
		if err := rec.Validate(); err == nil {
			t.Errorf("%s: Validate accepted %+v", name, rec)
		}
	}
}

func TestLaneRecordStillValidatesAsBefore(t *testing.T) {
	t.Parallel()
	lane := DeliverableRecord{Wave: "135", Cycle: 9, Trailer: CycleTrailer(9), Commits: consoleRecord().Commits}
	if err := lane.Validate(); err != nil {
		t.Fatalf("lane Validate: %v", err)
	}
	if lane.Ref() != 9 || lane.Label() != "cycle 9" || lane.Deliverable().ID != "cycle-9" {
		t.Errorf("lane identity changed: %d %q %q", lane.Ref(), lane.Label(), lane.Deliverable().ID)
	}
	lane.Cycle = 0
	if err := lane.Validate(); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("a lane record without a cycle must stay refused, got %v", err)
	}
}

func TestConsoleRecordRoundTripsThroughDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := WriteRecord(dir, consoleRecord())
	if err != nil {
		t.Fatalf("WriteRecord: %v", err)
	}
	if want := PRRecordPath(dir, 41); path != want {
		t.Errorf("path = %s, want %s", path, want)
	}
	got, err := ReadRecord(path)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	if got.Source != SourceConsolePR || got.PR != 41 || got.Cycle != 0 {
		t.Errorf("round trip lost identity: %+v", got)
	}
	raw, _ := json.Marshal(DeliverableRecord{Wave: "1", Cycle: 2})
	if strings.Contains(string(raw), `"source"`) || strings.Contains(string(raw), `"pr"`) || strings.Contains(string(raw), `"class"`) {
		t.Errorf("a lane record must not grow new keys: %s", raw)
	}
}

func TestFindPRRecordReadsTheConsoleWave(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, ok := FindPRRecord(root, 41); ok {
		t.Fatal("found a record that was never written")
	}
	if _, err := WriteRecord(WaveDir(root, ConsoleWave), consoleRecord()); err != nil {
		t.Fatal(err)
	}
	rec, ok := FindPRRecord(root, 41)
	if !ok || rec.PR != 41 {
		t.Fatalf("FindPRRecord = %+v, %v", rec, ok)
	}
	if _, ok := FindPRRecord(root, 42); ok {
		t.Error("PR 42 has no record")
	}
}

func allPaths([]string) bool { return true }

func evidenceOf(diff string, commits ...string) Evidence {
	return Evidence{Commits: commits, Paths: []string{"a.md"}, DiffSHA: Digest([]byte(diff))}
}

func TestAttestedBindingNeedsTheAttestationOfThisChange(t *testing.T) {
	t.Parallel()
	ev := evidenceOf("diff of PR 5", commitSHA40)
	b, err := AttestedBinding(ev, ev.DiffSHA, Digest([]byte("raw")))
	if err != nil {
		t.Fatalf("AttestedBinding: %v", err)
	}
	if b.TreeSHA != ev.DiffSHA || b.ArtifactSHA != Digest([]byte("raw")) || b.Class != "" {
		t.Errorf("binding = %+v", b)
	}
	other := evidenceOf("diff of PR 6", commitSHA40)
	if _, err := AttestedBinding(ev, other.DiffSHA, Digest([]byte("raw"))); err == nil || !strings.Contains(err.Error(), "not the change") {
		t.Errorf("an attestation of another change bound the PR: %v", err)
	}
}

func TestClassBindingBindsTheClaimToTheChangeAndItsRule(t *testing.T) {
	t.Parallel()
	a, err := ClassBinding(evidenceOf("one", commitSHA40), "docs-review", allPaths)
	if err != nil {
		t.Fatalf("ClassBinding: %v", err)
	}
	sameCommit, _ := ClassBinding(evidenceOf("two", commitSHA40), "docs-review", allPaths)
	otherClass, _ := ClassBinding(evidenceOf("one", commitSHA40), "other-review", allPaths)
	if a.Class != "docs-review" || a.TreeSHA != "" || !artifactDigest.MatchString(a.ArtifactSHA) {
		t.Errorf("binding = %+v", a)
	}
	if a.ArtifactSHA == sameCommit.ArtifactSHA || a.ArtifactSHA == otherClass.ArtifactSHA {
		t.Error("the digest must cover the change and the class")
	}
	if _, err := ClassBinding(evidenceOf("one", commitSHA40), "docs-review", func([]string) bool { return false }); err == nil || !strings.Contains(err.Error(), "does not cover") {
		t.Errorf("a class whose rule fails bound the PR: %v", err)
	}
	if _, err := ClassBinding(evidenceOf("one"), "docs-review", allPaths); err == nil {
		t.Error("a class with no commits accepted")
	}
}

func TestHasSourcePRNamesTheExactPRLine(t *testing.T) {
	t.Parallel()
	msg := "evolve-pr 4: integrate\n\nSource-PR: #4\n"
	rec := consoleRecord()
	rec.PR = 4
	if !HasSourcePR(msg, 4) || !rec.OwnsMessage(msg) {
		t.Error("the Source-PR line of PR 4 must be found")
	}
	if HasSourcePR(msg, 41) || HasSourcePR("Source-PR: #41\n", 4) || HasSourcePR(msg, 0) || rec.OwnsMessage("Evolve-Cycle: 4\n") {
		t.Error("another PR, a prefix of a number or no PR must not match")
	}
	lane := DeliverableRecord{Cycle: 9}
	if !lane.OwnsMessage("evolve-cycle 9: integrate\n") || lane.OwnsMessage(msg) {
		t.Error("a lane record owns the cycle trailer and not a PR line")
	}
}

func TestConsoleComposeMessageCarriesClosesInboxAndNoCycle(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.console(t, 41, "feat: x\n\nCloses-Inbox: item-a\nCloses-Inbox: item-b\n- closes-inbox: item-a, item-c", map[string]string{"a.txt": "a\n"})
	msg, err := Composer{Repo: f.repo}.message(context.Background(), rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"evolve-pr 41: integrate", "Source-PR: #41", "Closes-Inbox: item-a\n", "Closes-Inbox: item-b\n", "Closes-Inbox: item-c\n"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
	if strings.Count(msg, "Closes-Inbox: item-a") != 1 {
		t.Errorf("one id must give one line:\n%s", msg)
	}
	if strings.Contains(msg, "evolve-cycle") || strings.Contains(msg, "Evolve-Cycle") {
		t.Errorf("a console message must hold no cycle:\n%s", msg)
	}
	rec.Commits[0].SHA = treeSHA40
	if _, err := (Composer{Repo: f.repo}).message(context.Background(), rec); err == nil {
		t.Error("an unknown commit must fail the message")
	}
}

func TestConsoleComposeAcceptsAClassBoundRecordAndRefusesAMissingCommit(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.console(t, 5, "docs: x", map[string]string{"d.md": "d\n"})
	class, err := ClassBinding(evidenceOf("d", rec.Commits[0].SHA), "docs-review", allPaths)
	if err != nil {
		t.Fatal(err)
	}
	rec.Audit = class
	c := Composer{Repo: f.repo, Stage: "shadow"}
	b := f.batch(t)
	if _, err := c.Compose(context.Background(), b, rec); err != nil {
		t.Fatalf("a class-bound record must compose: %v", err)
	}
	gone := rec
	gone.Commits = []CommitIdentity{{SHA: commitSHA40, BaseSHA: f.base, PatchID: "p"}}
	if _, err := c.Compose(context.Background(), f.batchAt(t, 2), gone); err == nil {
		t.Error("a record whose commit is absent must fail loudly")
	}
}

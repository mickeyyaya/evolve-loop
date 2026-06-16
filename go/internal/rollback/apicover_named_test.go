package rollback

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestResult_PopulatedByRun names the Result type and pins that Run copies the
// journal's identity (Version/Tag/CommitSHA) and each step's status into the
// returned Result — i.e. Result is the structured outcome of a Run, not an
// orphan struct. Asserted via field equality against a full happy-path Run.
func TestResult_PopulatedByRun(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	var res Result = mustRunOK(t, jp, repo)

	if res.Version != "1.2.3" {
		t.Errorf("Result.Version = %q, want 1.2.3", res.Version)
	}
	if res.Tag != "v1.2.3" {
		t.Errorf("Result.Tag = %q, want v1.2.3", res.Tag)
	}
	if res.CommitSHA != "abcdef1234567890" {
		t.Errorf("Result.CommitSHA = %q, want abcdef1234567890", res.CommitSHA)
	}
	if res.ReleaseDelete != "deleted" || res.TagDelete != "deleted" || res.Revert != "reverted" {
		t.Errorf("Result step statuses = (%q,%q,%q), want (deleted,deleted,reverted)",
			res.ReleaseDelete, res.TagDelete, res.Revert)
	}
	if !res.OverallSucceeded {
		t.Error("Result.OverallSucceeded = false, want true on all-OK run")
	}
}

// mustRunOK runs the happy path and returns the Result, failing the test on err.
func mustRunOK(t *testing.T, journalPath, repoRoot string) Result {
	t.Helper()
	res, err := Run(Options{JournalPath: journalPath, RepoRoot: repoRoot, Steps: allOkSteps()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

// TestJournal_JSONUnmarshalContract names the Journal type and pins its JSON
// field tags: the on-disk publish record's snake_case keys (commit_sha) must
// unmarshal into the Go fields ReadJournal validates. Built as a full-struct
// want and compared after a round-trip through encoding/json.
func TestJournal_JSONUnmarshalContract(t *testing.T) {
	const raw = `{"version":"9.9.9","tag":"v9.9.9","commit_sha":"cafebabe","branch":"release",` +
		`"release_url":"https://example/r","started_at":"2026-01-01T00:00:00Z"}`

	var got Journal
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := Journal{
		Version:    "9.9.9",
		Tag:        "v9.9.9",
		CommitSHA:  "cafebabe",
		Branch:     "release",
		ReleaseURL: "https://example/r",
		StartedAt:  "2026-01-01T00:00:00Z",
	}
	if got != want {
		t.Errorf("Journal = %+v, want %+v (snake_case json tags must bind)", got, want)
	}
}

// TestLedgerEntry_JSONMarshalKeys names the LedgerEntry type and pins that it
// marshals to the snake_case NDJSON schema appended to release-rollbacks.jsonl
// (the audit-trail contract downstream tooling parses), and round-trips losslessly.
func TestLedgerEntry_JSONMarshalKeys(t *testing.T) {
	entry := LedgerEntry{
		Timestamp:     "2026-01-02T03:04:05Z",
		Version:       "1.0.0",
		Tag:           "v1.0.0",
		CommitSHA:     "deadbeef",
		Reason:        "probe",
		ReleaseDelete: "deleted",
		TagDelete:     "not-present",
		Revert:        "reverted",
		DryRun:        false,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(b)
	for _, key := range []string{
		`"timestamp"`, `"version"`, `"tag"`, `"commit_sha"`,
		`"reason"`, `"release_delete"`, `"tag_delete"`, `"revert"`, `"dry_run"`,
	} {
		if !strings.Contains(out, key) {
			t.Errorf("LedgerEntry JSON missing key %s: %s", key, out)
		}
	}
	var back LedgerEntry
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if back != entry {
		t.Errorf("LedgerEntry round-trip = %+v, want %+v", back, entry)
	}
}

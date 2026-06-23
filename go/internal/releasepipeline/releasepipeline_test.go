package releasepipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/semvercheck"
)

// allOkSteps returns Steps where every step succeeds. The defaults that
// would otherwise shell out to git/bash are explicitly replaced.
func allOkSteps() Steps {
	return Steps{
		FullDryRunPreflight: func(string, string) error { return nil },
		Preflight:           func(string, string, bool, bool) error { return nil },
		ChangelogGen:        func(string, string, string, string, bool) error { return nil },
		VersionBump:         func(string, string, bool) error { return nil },
		RebuildBinary:       func(string, string, bool) error { return nil },
		ReleaseSh:           func(string, string) error { return nil },
		Ship:                func(string, string, string) (string, error) { return "deadbeef1234567890", nil },
		MarketplacePoll:     func(string, string, time.Duration) error { return nil },
		Rollback:            func(string, string, string) error { return nil },
		ReleaseVerify:       func(string, string, string) error { return nil },
	}
}

// fixedNow returns a deterministic clock used in journal timestamps.
func fixedNow(t *testing.T) func() time.Time {
	t.Helper()
	ts := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return ts }
}

// === Happy path: all steps succeed, journal contains all step records ======
func TestRun_HappyPath(t *testing.T) {
	repo := t.TempDir()
	var buf bytes.Buffer

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: 1 * time.Second,
		Now:         fixedNow(t),
		Steps:       allOkSteps(),
		Stderr:      &buf,
	})
	if err != nil {
		t.Fatalf("Run err = %v\nlog=%s", err, buf.String())
	}
	if res.NewCommitSHA != "deadbeef1234567890" {
		t.Errorf("NewCommitSHA = %q, want deadbeef…", res.NewCommitSHA)
	}
	expected := []string{
		"preflight", "changelog-gen", "version-bump", "rebuild-binary",
		"release-sh-check", "ship", "marketplace-poll", "release-verify",
	}
	if !equalStrings(res.StepsCompleted, expected) {
		t.Errorf("StepsCompleted = %v, want %v", res.StepsCompleted, expected)
	}
	// Verify journal on disk has the expected step records.
	body, err := os.ReadFile(res.JournalPath)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	var j Journal
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatalf("unmarshal journal: %v", err)
	}
	if j.Version != "1.2.3" || j.Tag != "v1.2.3" {
		t.Errorf("journal version/tag = %s / %s", j.Version, j.Tag)
	}
	if j.CommitSHA != "deadbeef1234567890" {
		t.Errorf("journal commit_sha = %q", j.CommitSHA)
	}
	if len(j.Steps) != 8 {
		t.Errorf("journal steps = %d, want 8 (rebuild-binary v12.2.2 + release-verify): %+v", len(j.Steps), j.Steps)
	}
	if j.CompletedAt == "" {
		t.Error("journal completed_at empty")
	}
}

// === Invalid semver target → ErrPrePublishFailed ===========================
func TestRun_InvalidSemver(t *testing.T) {
	_, err := Run(Options{
		Target:      "garbage",
		RepoRoot:    t.TempDir(),
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
}

// === Step 1 preflight fails → ErrPrePublishFailed, no later steps run =====
func TestRun_PreflightFails(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	steps.Preflight = func(string, string, bool, bool) error { return errors.New("simulated") }
	versionBumpCalls := 0
	steps.VersionBump = func(string, string, bool) error {
		versionBumpCalls++
		return nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if versionBumpCalls != 0 {
		t.Errorf("VersionBump called %d times after preflight failure (want 0)", versionBumpCalls)
	}
	if !contains(res.StepsFailed, "preflight") {
		t.Errorf("StepsFailed = %v, want contains 'preflight'", res.StepsFailed)
	}
}

// === Step 5 ship fails → ErrShipFailed (no rollback triggered) =============
func TestRun_ShipFails_NoRollback(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	steps.Ship = func(string, string, string) (string, error) {
		return "", errors.New("ship simulated")
	}
	rollbackCalls := 0
	steps.Rollback = func(string, string, string) error { rollbackCalls++; return nil }

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrShipFailed) {
		t.Fatalf("err = %v, want ErrShipFailed", err)
	}
	if rollbackCalls != 0 {
		t.Errorf("Rollback called %d times after ship fail (want 0 — nothing to undo)", rollbackCalls)
	}
	if !contains(res.StepsFailed, "ship") {
		t.Errorf("StepsFailed = %v, want contains 'ship'", res.StepsFailed)
	}
	if res.RollbackTriggered {
		t.Error("RollbackTriggered must be false when ship itself fails")
	}
}

// === Step 6 marketplace-poll fails → auto-rollback runs, ErrPostPublishFailed =
func TestRun_MarketplacePollFails_AutoRollback(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	steps.MarketplacePoll = func(string, string, time.Duration) error {
		return errors.New("propagation timeout")
	}
	rollbackCalls := 0
	rollbackReason := ""
	steps.Rollback = func(_, _, reason string) error {
		rollbackCalls++
		rollbackReason = reason
		return nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if rollbackCalls != 1 {
		t.Errorf("Rollback called %d times, want 1", rollbackCalls)
	}
	if !strings.Contains(rollbackReason, "marketplace") {
		t.Errorf("rollback reason = %q, want contains 'marketplace'", rollbackReason)
	}
	if !res.RollbackTriggered {
		t.Error("RollbackTriggered must be true after marketplace-poll fail")
	}
}

// === --no-rollback skips auto-rollback even on poll failure ================
func TestRun_NoRollbackFlag(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	steps.MarketplacePoll = func(string, string, time.Duration) error { return errors.New("nope") }
	rollbackCalls := 0
	steps.Rollback = func(string, string, string) error { rollbackCalls++; return nil }

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		NoRollback:  true,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if rollbackCalls != 0 {
		t.Errorf("Rollback called %d times with NoRollback (want 0)", rollbackCalls)
	}
	if res.RollbackTriggered {
		t.Error("RollbackTriggered must be false with NoRollback flag")
	}
}

// === --dry-run never invokes ship, never persists journal to permanent path =
func TestRun_DryRun(t *testing.T) {
	repo := t.TempDir()
	shipCalls := 0
	steps := allOkSteps()
	steps.Ship = func(string, string, string) (string, error) {
		shipCalls++
		return "wrongly-shipped", nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		DryRun:      true,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("dry-run err = %v", err)
	}
	if shipCalls != 0 {
		t.Errorf("Ship called %d times in dry-run (want 0)", shipCalls)
	}
	if res.NewCommitSHA != "" {
		t.Errorf("NewCommitSHA = %q, want empty in dry-run", res.NewCommitSHA)
	}
	// Dry-run journal should be in TempDir, not under repoRoot/.evolve.
	if !strings.Contains(res.JournalPath, os.TempDir()) {
		t.Errorf("dry-run journal path %q should be under TempDir", res.JournalPath)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "release-journal")); err == nil {
		t.Error("dry-run should NOT create .evolve/release-journal in repo")
	}
}

// === --require-preflight runs step 0; step 0 failure aborts pipeline =======
func TestRun_RequirePreflight_Failure(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	preflightCalls := 0
	steps.FullDryRunPreflight = func(string, string) error {
		preflightCalls++
		return errors.New("simulated harness fail")
	}
	preflightStep1Calls := 0
	steps.Preflight = func(string, string, bool, bool) error { preflightStep1Calls++; return nil }

	_, err := Run(Options{
		Target:           "1.2.3",
		RepoRoot:         repo,
		FromTag:          "v1.2.2",
		MaxPollWait:      time.Second,
		RequirePreflight: true,
		Steps:            steps,
		Now:              fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if preflightCalls != 1 {
		t.Errorf("FullDryRunPreflight calls = %d, want 1", preflightCalls)
	}
	if preflightStep1Calls != 0 {
		t.Errorf("step 1 Preflight called %d times after step 0 fail (want 0)", preflightStep1Calls)
	}
}

// === --require-preflight passes through when step 0 succeeds ===============
func TestRun_RequirePreflight_Success(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	preflightCalls := 0
	steps.FullDryRunPreflight = func(string, string) error { preflightCalls++; return nil }

	res, err := Run(Options{
		Target:           "1.2.3",
		RepoRoot:         repo,
		FromTag:          "v1.2.2",
		MaxPollWait:      time.Second,
		RequirePreflight: true,
		Steps:            steps,
		Now:              fixedNow(t),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if preflightCalls != 1 {
		t.Errorf("FullDryRunPreflight calls = %d, want 1", preflightCalls)
	}
	if !contains(res.StepsCompleted, "full-dry-run-preflight") {
		t.Errorf("StepsCompleted = %v, want contains 'full-dry-run-preflight'", res.StepsCompleted)
	}
}

// === extractReleaseNotes pulls the right entry =============================
func TestExtractReleaseNotes(t *testing.T) {
	d := t.TempDir()
	body := `# Changelog

## [1.2.3] - 2026-05-24

### Added

- Feature A
- Feature B

---

## [1.2.2] - 2026-05-23

### Added

- Older feature
`
	if err := os.WriteFile(filepath.Join(d, "CHANGELOG.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	notes := extractReleaseNotes(d, "1.2.3")
	if !strings.Contains(notes, "Feature A") {
		t.Errorf("notes missing Feature A: %q", notes)
	}
	if strings.Contains(notes, "Older feature") {
		t.Errorf("notes leaked older entry: %q", notes)
	}
}

func TestExtractReleaseNotes_NotFound(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "CHANGELOG.md"), []byte("no entry here"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	notes := extractReleaseNotes(d, "9.9.9")
	if notes != "" {
		t.Errorf("notes = %q, want empty", notes)
	}
}

func TestExtractReleaseNotes_NoChangelog(t *testing.T) {
	notes := extractReleaseNotes(t.TempDir(), "1.0.0")
	if notes != "" {
		t.Errorf("notes = %q, want empty when no CHANGELOG", notes)
	}
}

// === IsSemver guard ========================================================
func TestIsSemver(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1.2.3", true},
		{"11.7.5", true},
		{"v1.2.3", false},
		{"1.2", false},
		{"garbage", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := semvercheck.IsSemver(tc.in); got != tc.want {
			t.Errorf("IsSemver(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// === DefaultSteps wires all 8 step functions ===============================
func TestDefaultSteps(t *testing.T) {
	d := DefaultSteps()
	if d.FullDryRunPreflight == nil || d.Preflight == nil || d.ChangelogGen == nil ||
		d.VersionBump == nil || d.ReleaseSh == nil || d.Ship == nil ||
		d.MarketplacePoll == nil || d.Rollback == nil {
		t.Errorf("DefaultSteps missing fields: %+v", d)
	}
}

// === Journal persistence: appendStep writes file each call =================
func TestAppendStep_PersistsAcrossCalls(t *testing.T) {
	repo := t.TempDir()
	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	body, err := os.ReadFile(res.JournalPath)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	var j Journal
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, s := range j.Steps {
		if s.Timestamp == "" {
			t.Errorf("step %s has empty timestamp", s.Step)
		}
		if s.Status == "" {
			t.Errorf("step %s has empty status", s.Step)
		}
	}
}

// === setJournalField updates known fields and persists =====================
func TestSetJournalField(t *testing.T) {
	d := t.TempDir()
	j := &Journal{Version: "1.2.3", Tag: "v1.2.3", Steps: []StepRecord{}}
	path := filepath.Join(d, "j.json")
	if err := writeJournal(j, path); err != nil {
		t.Fatalf("write: %v", err)
	}
	setJournalField(j, path, "commit_sha", "abc123")
	setJournalField(j, path, "release_url", "https://example.com/r")
	setJournalField(j, path, "completed_at", "2026-05-24T13:00:00Z")
	setJournalField(j, path, "tag", "v1.2.3-modified")
	setJournalField(j, path, "branch", "develop")
	body, _ := os.ReadFile(path)
	var got Journal
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CommitSHA != "abc123" || got.ReleaseURL != "https://example.com/r" ||
		got.CompletedAt != "2026-05-24T13:00:00Z" || got.Tag != "v1.2.3-modified" ||
		got.Branch != "develop" {
		t.Errorf("journal not updated: %+v", got)
	}
	// Unknown field is a no-op (no crash).
	setJournalField(j, path, "bogus_field", "ignored")
}

// === initJournal places dry-run journal in TempDir =========================
func TestInitJournal_DryRun(t *testing.T) {
	repo := t.TempDir()
	j, path, err := initJournal(Options{
		Target:   "1.2.3",
		RepoRoot: repo,
		DryRun:   true,
	}, "v1.2.2", time.Now())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(path, os.TempDir()) {
		t.Errorf("dry-run journal path %q must be under TempDir", path)
	}
	if j.Version != "1.2.3" {
		t.Errorf("journal version = %q", j.Version)
	}
}

// === initJournal places real journal under repo/.evolve/release-journal ===
func TestInitJournal_RealPath(t *testing.T) {
	repo := t.TempDir()
	_, path, err := initJournal(Options{
		Target:   "1.2.3",
		RepoRoot: repo,
	}, "v1.2.2", time.Now())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := filepath.Join(repo, ".evolve", "release-journal")
	if !strings.HasPrefix(path, want) {
		t.Errorf("path %q does not start with %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("journal file not written: %v", err)
	}
}

// === initJournal honors JournalDir override ================================
func TestInitJournal_JournalDirOverride(t *testing.T) {
	d := t.TempDir()
	custom := filepath.Join(d, "custom-journals")
	_, path, err := initJournal(Options{
		Target:     "1.2.3",
		RepoRoot:   d,
		JournalDir: custom,
	}, "v1.2.2", time.Now())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(path, custom) {
		t.Errorf("path %q does not honor JournalDir override %q", path, custom)
	}
}

// === Rollback error is captured but pipeline still returns ErrPostPublishFailed
func TestRun_RollbackFailureCaptured(t *testing.T) {
	repo := t.TempDir()
	steps := allOkSteps()
	steps.MarketplacePoll = func(string, string, time.Duration) error { return errors.New("poll fail") }
	steps.Rollback = func(string, string, string) error { return errors.New("rollback also failed") }

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if !res.RollbackTriggered {
		t.Error("RollbackTriggered must be true")
	}
	if res.RollbackErr == nil || !strings.Contains(res.RollbackErr.Error(), "rollback also failed") {
		t.Errorf("RollbackErr = %v, want capture", res.RollbackErr)
	}
}

// === Helpers ===============================================================
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

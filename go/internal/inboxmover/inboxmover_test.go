package inboxmover

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// makeRepo sets up an inbox/-style repo with optional inbox/ files,
// processing/cycle-N/ files, and cycle-state.json. Returns the repo root.
// The temp-dir/.evolve scaffold comes from fixtures.NewWorkspace; the inbox/
// subdir is the domain-specific seeding this package needs.
func makeRepo(t *testing.T) string {
	t.Helper()
	ws := fixtures.NewWorkspace(t).Build()
	if err := os.MkdirAll(filepath.Join(ws.EvolveDir, "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return ws.Root
}

// dropInboxFile creates inbox/<name>.json with {"id":<id>} content.
func dropInboxFile(t *testing.T, repo, name, id string) string {
	t.Helper()
	dir := filepath.Join(repo, ".evolve", "inbox")
	path := filepath.Join(dir, name)
	body := fmt.Sprintf(`{"id":"%s","payload":"x"}`, id)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// dropProcessingFile creates processing/cycle-N/<name>.json.
func dropProcessingFile(t *testing.T, repo, cycle, name, id string) string {
	t.Helper()
	dir := filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-"+cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
	body := fmt.Sprintf(`{"id":"%s","payload":"x"}`, id)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// setCycleState writes cycle-state.json with the given active cycle id.
func setCycleState(t *testing.T, repo, id string) {
	t.Helper()
	path := filepath.Join(repo, ".evolve", "cycle-state.json")
	body := fmt.Sprintf(`{"cycle_id":%s}`, id)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// === Claim happy path =====================================================
func TestClaim_HappyPath(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	res, err := Claim(Options{ProjectRoot: repo}, "task-1", "5")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.DestPath, "processing/cycle-5/task-1.json") {
		t.Errorf("DestPath = %q", res.DestPath)
	}
	if _, err := os.Stat(res.DestPath); err != nil {
		t.Errorf("dest file missing: %v", err)
	}
	if _, err := os.Stat(res.SrcPath); err == nil {
		t.Errorf("src file should be gone")
	}
	// Ledger entry present.
	body, err := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.Contains(string(body), `"action":"claim"`) {
		t.Errorf("ledger missing claim entry: %s", body)
	}
}

// === Claim missing task → ErrNotFound =====================================
func TestClaim_NotFound(t *testing.T) {
	repo := makeRepo(t)
	_, err := Claim(Options{ProjectRoot: repo}, "task-missing", "5")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// === Claim with missing args → ErrBadArgs =================================
func TestClaim_BadArgs(t *testing.T) {
	repo := makeRepo(t)
	_, err := Claim(Options{ProjectRoot: repo}, "", "5")
	if !errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want ErrBadArgs", err)
	}
}

// === Promote: processing → processed (no SHA) ==============================
func TestPromote_ProcessedNoSHA(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "processed", PromoteOpts{Cycle: "5"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.NoOp {
		t.Error("NoOp = true, want false (real promote)")
	}
	if !strings.Contains(res.DestPath, "processed/cycle-5/task-1.json") {
		t.Errorf("DestPath = %q", res.DestPath)
	}
}

// === Promote: processing → processed (with SHA prefix) ====================
func TestPromote_ProcessedWithSHA(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "processed", PromoteOpts{
		Cycle:     "5",
		CommitSHA: "abcdef1234567890",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.DestPath, "processed/cycle-5/abcdef12-task-1.json") {
		t.Errorf("DestPath = %q, want sha8 prefix", res.DestPath)
	}
}

// === Promote: rejected/retry destination layout ===========================
func TestPromote_Rejected(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "rejected", PromoteOpts{Cycle: "5"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.DestPath, "rejected/cycle-5/task-1.json") {
		t.Errorf("DestPath = %q", res.DestPath)
	}
}

func TestPromote_Retry(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "retry", PromoteOpts{Cycle: "5"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.DestPath, "retry/task-1.json") {
		t.Errorf("DestPath = %q", res.DestPath)
	}
	if strings.Contains(res.DestPath, "cycle") {
		t.Errorf("retry path should not have cycle subdir: %q", res.DestPath)
	}
}

// === Promote: missing source (NoOp for ship.sh compat) ====================
func TestPromote_NotFound_NoOp(t *testing.T) {
	repo := makeRepo(t)
	res, err := Promote(Options{ProjectRoot: repo}, "task-missing", "processed", PromoteOpts{Cycle: "5"})
	if err != nil {
		t.Fatalf("err = %v (want nil for ship.sh compat)", err)
	}
	if !res.NoOp {
		t.Error("NoOp = false, want true")
	}
}

// === Promote: invalid state → ErrBadState =================================
func TestPromote_BadState(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	_, err := Promote(Options{ProjectRoot: repo}, "task-1", "invalid-state", PromoteOpts{Cycle: "5"})
	if !errors.Is(err, ErrBadState) {
		t.Errorf("err = %v, want ErrBadState", err)
	}
}

// === Promote: inbox/ fallback when source not in processing ==============
func TestPromote_InboxFallback(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "rejected", PromoteOpts{Cycle: "5"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.NoOp {
		t.Error("NoOp = true, want false (inbox/ fallback)")
	}
	// Ledger should record from with srcRel="inbox" (bash format preserves
	// the leading `.evolve/inbox/<srcRel>/` even when that double-nests for
	// the inbox fallback path — preserved here for byte-parity with bash).
	body, _ := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if !strings.Contains(string(body), `"from":".evolve/inbox/inbox/task-1.json"`) {
		t.Errorf("ledger missing inbox-fallback entry: %s", body)
	}
}

// === RecoverOrphans: dead cycles get recovered, active cycle skipped =====
func TestRecoverOrphans_HappyPath(t *testing.T) {
	repo := makeRepo(t)
	setCycleState(t, repo, "5")
	dropProcessingFile(t, repo, "3", "task-a.json", "task-a") // dead → recover
	dropProcessingFile(t, repo, "3", "task-b.json", "task-b") // dead → recover
	dropProcessingFile(t, repo, "5", "task-c.json", "task-c") // active → skip

	res, err := RecoverOrphans(Options{ProjectRoot: repo})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Recovered != 2 {
		t.Errorf("Recovered = %d, want 2", res.Recovered)
	}
	// task-a + task-b should now be at inbox/.
	for _, name := range []string{"task-a.json", "task-b.json"} {
		path := filepath.Join(repo, ".evolve", "inbox", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s should be in inbox/: %v", name, err)
		}
	}
	// task-c should still be in processing/cycle-5/.
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-5", "task-c.json")); err != nil {
		t.Errorf("task-c.json should still be in active cycle dir")
	}
}

// === RecoverOrphans: no processing dir → no-op ============================
func TestRecoverOrphans_NoProcessingDir(t *testing.T) {
	repo := makeRepo(t)
	res, err := RecoverOrphans(Options{ProjectRoot: repo})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d, want 0", res.Recovered)
	}
}

// === RecoverOrphans: missing cycle-state → all cycles treated as dead ====
func TestRecoverOrphans_NoCycleState(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "3", "task-a.json", "task-a")
	res, err := RecoverOrphans(Options{ProjectRoot: repo})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Recovered != 1 {
		t.Errorf("Recovered = %d, want 1 (no cycle state → all cycles dead)", res.Recovered)
	}
}

// === Ledger entry shape ===================================================
func TestLedgerEntry_Schema(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	fixedNow := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	_, err := Claim(Options{
		ProjectRoot: repo,
		Now:         func() time.Time { return fixedNow },
	}, "task-1", "7")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	var entry map[string]any
	if err := json.Unmarshal(body[:len(body)-1], &entry); err != nil {
		t.Fatalf("ledger not valid JSON: %v\n%s", err, body)
	}
	for _, field := range []string{"ts", "class", "action", "task_id", "from", "to", "cycle", "git_sha", "reason"} {
		if _, ok := entry[field]; !ok {
			t.Errorf("ledger entry missing field %q: %v", field, entry)
		}
	}
	if entry["ts"] != "2026-05-24T12:00:00Z" {
		t.Errorf("ts = %v, want fixed time", entry["ts"])
	}
	if entry["class"] != "inbox-lifecycle" {
		t.Errorf("class = %v", entry["class"])
	}
	// cycle should be numeric 7, git_sha should be null.
	if entry["cycle"] != float64(7) {
		t.Errorf("cycle = %v (type %T), want 7", entry["cycle"], entry["cycle"])
	}
	if entry["git_sha"] != nil {
		t.Errorf("git_sha = %v, want nil", entry["git_sha"])
	}
}

// === findFileByTaskID ignores non-JSON / unparseable files ================
func TestFindFileByTaskID_IgnoresNonJSON(t *testing.T) {
	d := t.TempDir()
	// Drop an unrelated file + a malformed JSON + the real target.
	_ = os.WriteFile(filepath.Join(d, "readme.md"), []byte("# hi"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "bad.json"), []byte("{not json"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "good.json"), []byte(`{"id":"target"}`), 0o644)

	got, err := findFileByTaskID(d, "target")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if filepath.Base(got) != "good.json" {
		t.Errorf("got %q, want good.json", got)
	}
}

// === promoteDestPath table =================================================
func TestPromoteDestPath(t *testing.T) {
	base := "task-x.json"
	cases := []struct {
		name     string
		state    string
		opts     PromoteOpts
		wantPath string
	}{
		{"processed default cycle", "processed", PromoteOpts{}, "processed/cycle-0/task-x.json"},
		{"processed with cycle", "processed", PromoteOpts{Cycle: "12"}, "processed/cycle-12/task-x.json"},
		{"processed with sha", "processed", PromoteOpts{Cycle: "12", CommitSHA: "deadbeef1234567890"}, "processed/cycle-12/deadbeef-task-x.json"},
		{"processed short sha (no truncate)", "processed", PromoteOpts{Cycle: "12", CommitSHA: "abc"}, "processed/cycle-12/abc-task-x.json"},
		{"rejected default cycle", "rejected", PromoteOpts{}, "rejected/cycle-0/task-x.json"},
		{"retry no cycle", "retry", PromoteOpts{}, "retry/task-x.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, dest := promoteDestPath("/inbox", base, tc.state, tc.opts)
			if !strings.HasSuffix(dest, tc.wantPath) {
				t.Errorf("dest = %q, want suffix %q", dest, tc.wantPath)
			}
		})
	}
}

// === intPtr / strPtr edge cases ===========================================
func TestIntPtr(t *testing.T) {
	if intPtr("") != nil {
		t.Errorf("intPtr('') = non-nil")
	}
	if intPtr("garbage") != nil {
		t.Errorf("intPtr('garbage') = non-nil")
	}
	v := intPtr("42")
	if v == nil || *v != 42 {
		t.Errorf("intPtr('42') = %v", v)
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Errorf("strPtr('') = non-nil")
	}
	v := strPtr("hi")
	if v == nil || *v != "hi" {
		t.Errorf("strPtr('hi') = %v", v)
	}
}

// === readActiveCycle: missing file ========================================
func TestReadActiveCycle_MissingFile(t *testing.T) {
	_, err := readActiveCycle("/tmp/this-cycle-state-does-not-exist-xyz.json")
	if err == nil {
		t.Error("want err for missing file")
	}
}

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

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// makeRepo returns a temp project root holding an empty .evolve/inbox.
func makeRepo(t *testing.T) string {
	t.Helper()
	ws := fixtures.NewWorkspace(t).Build()
	if err := os.MkdirAll(filepath.Join(ws.EvolveDir, "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return ws.Root
}

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

func setCycleState(t *testing.T, repo, id string) {
	t.Helper()
	path := filepath.Join(repo, ".evolve", "cycle-state.json")
	body := fmt.Sprintf(`{"cycle_id":%s}`, id)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

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
	body, err := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.Contains(string(body), `"action":"claim"`) {
		t.Errorf("ledger missing claim entry: %s", body)
	}
}

func TestClaim_NotFound(t *testing.T) {
	repo := makeRepo(t)
	_, err := Claim(Options{ProjectRoot: repo}, "task-missing", "5")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestClaim_BadArgs(t *testing.T) {
	repo := makeRepo(t)
	_, err := Claim(Options{ProjectRoot: repo}, "", "5")
	if !errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want ErrBadArgs", err)
	}
}

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

func TestPromote_ProcessedRefusedWhenNotLanded(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "598", "task-1.json", "task-1")
	opts := Options{
		ProjectRoot: repo,
		IsLandedFn: func(sha string) (bool, error) {
			if sha != "deadbeef00" {
				t.Errorf("IsLandedFn sha = %q, want deadbeef00", sha)
			}
			return false, nil
		},
	}
	res, err := Promote(opts, "task-1", "processed", PromoteOpts{Cycle: "598", CommitSHA: "deadbeef00"})
	if err != nil {
		t.Fatalf("err = %v (want nil — ship.sh compat, refusal is not a hard failure)", err)
	}
	if strings.Contains(res.DestPath, "processed") {
		t.Errorf("DestPath = %q, must NOT land in processed/ for an unlanded SHA", res.DestPath)
	}
	if !strings.Contains(res.DestPath, "retry") {
		t.Errorf("DestPath = %q, want rerouted to retry/", res.DestPath)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processed", "cycle-598", "task-1.json")); err == nil {
		t.Error("task-1.json must not exist under processed/cycle-598/")
	}
	body, err := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.Contains(string(body), "unlanded") {
		t.Errorf("ledger should record the unlanded-SHA refusal reason: %s", body)
	}
}

func TestPromote_ProcessedPromotesWhenLanded(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "598", "task-2.json", "task-2")
	opts := Options{
		ProjectRoot: repo,
		IsLandedFn: func(sha string) (bool, error) {
			return true, nil
		},
	}
	res, err := Promote(opts, "task-2", "processed", PromoteOpts{Cycle: "598", CommitSHA: "cafef00dab"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.DestPath, "processed/cycle-598/") {
		t.Errorf("DestPath = %q, want processed/cycle-598/ for a landed SHA", res.DestPath)
	}
}

func TestPromote_ProcessedNoSHASkipsAncestryCheck(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-3.json", "task-3")
	called := false
	opts := Options{
		ProjectRoot: repo,
		IsLandedFn: func(sha string) (bool, error) {
			called = true
			return false, nil
		},
	}
	res, err := Promote(opts, "task-3", "processed", PromoteOpts{Cycle: "5"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Error("IsLandedFn must not be called when CommitSHA is empty")
	}
	if !strings.Contains(res.DestPath, "processed/cycle-5/task-3.json") {
		t.Errorf("DestPath = %q", res.DestPath)
	}
}

func TestPromote_BadState(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	_, err := Promote(Options{ProjectRoot: repo}, "task-1", "invalid-state", PromoteOpts{Cycle: "5"})
	if !errors.Is(err, ErrBadState) {
		t.Errorf("err = %v, want ErrBadState", err)
	}
}

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
	// A root item's ledger From double-nests inbox/ (a preserved quirk).
	body, _ := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if !strings.Contains(string(body), `.evolve/inbox/inbox/task-1.json`) {
		t.Errorf("ledger missing inbox-fallback entry: %s", body)
	}
}

func TestRecoverOrphans_HappyPath(t *testing.T) {
	repo := makeRepo(t)
	setCycleState(t, repo, "5")
	dropProcessingFile(t, repo, "3", "task-a.json", "task-a")
	dropProcessingFile(t, repo, "3", "task-b.json", "task-b")
	dropProcessingFile(t, repo, "5", "task-c.json", "task-c")

	res, err := RecoverOrphans(Options{ProjectRoot: repo})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Recovered != 2 {
		t.Errorf("Recovered = %d, want 2", res.Recovered)
	}
	for _, name := range []string{"task-a.json", "task-b.json"} {
		path := filepath.Join(repo, ".evolve", "inbox", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s should be in inbox/: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-5", "task-c.json")); err != nil {
		t.Errorf("task-c.json should still be in active cycle dir")
	}
}

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
	for _, field := range []string{"ts", "kind", "action", "task_id", "cycle", "message", "prev_hash", "entry_seq"} {
		if _, ok := entry[field]; !ok {
			t.Errorf("ledger entry missing field %q: %v", field, entry)
		}
	}
	if entry["ts"] != "2026-05-24T12:00:00Z" {
		t.Errorf("ts = %v, want fixed time", entry["ts"])
	}
	if entry["kind"] != "inbox-lifecycle" {
		t.Errorf("kind = %v", entry["kind"])
	}
	if entry["action"] != "claim" || entry["task_id"] != "task-1" {
		t.Errorf("action/task_id = %v/%v, want claim/task-1", entry["action"], entry["task_id"])
	}
	if entry["cycle"] != float64(7) {
		t.Errorf("cycle = %v (type %T), want 7", entry["cycle"], entry["cycle"])
	}
	msg, _ := entry["message"].(string)
	if !strings.Contains(msg, ".evolve/inbox/task-1.json") || !strings.Contains(msg, "triage-claim") {
		t.Errorf("message lost the from/to/reason detail: %q", msg)
	}
}

func TestFindFileByTaskID_IgnoresNonJSON(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "readme.md"), []byte("# hi"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "bad.json"), []byte("{not json"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "good.json"), []byte(`{"id":"target"}`), 0o644)

	got, err := FindFileByTaskID(d, "target")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if filepath.Base(got) != "good.json" {
		t.Errorf("got %q, want good.json", got)
	}
}

func TestReadActiveCycle_MissingFile(t *testing.T) {
	_, err := readActiveCycle("/tmp/this-cycle-state-does-not-exist-xyz.json")
	if err == nil {
		t.Error("want err for missing file")
	}
}

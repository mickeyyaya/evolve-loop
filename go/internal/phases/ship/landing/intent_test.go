package landing

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIntentPath_KeepsEveryCycleInTheHostOwnedLandingDir(t *testing.T) {
	if got := IntentPath("/plane/.evolve", 1830); got != filepath.Join("/plane/.evolve", "landing", "cycle-1830.json") {
		t.Errorf("IntentPath = %q: the intent lives where the guards deny every phase write", got)
	}
}

func TestWriteIntent_RoundTripsEveryFieldAtomically(t *testing.T) {
	path := IntentPath(filepath.Join(t.TempDir(), ".evolve"), 1830)
	dir := filepath.Dir(path)
	want := preparedIntent()

	if err := WriteIntent(path, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := ReadIntent(path)

	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadIntent = %+v, %v, %v; want %+v", got, found, err, want)
	}
	if tmps, _ := filepath.Glob(filepath.Join(dir, "landing-intent.*.tmp")); len(tmps) != 0 {
		t.Errorf("temp files left behind: %v", tmps)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cycle", "run_id", "audit_artifact_sha256", "audited_tree", "lane_tree", "worktree_base_sha", "commit_sha", "commit_tree",
		"consumed_paths", "explanation_view_sha256", "pre_main", "branch", "lane_branch", "status"} {
		if !strings.Contains(string(body), `"`+key+`":`) {
			t.Errorf("the intent on disk lacks %q: %s", key, body)
		}
	}
}

func TestWriteIntent_OverwritesTheStatus(t *testing.T) {
	path := IntentPath(t.TempDir(), 7)
	in := preparedIntent()
	for _, status := range []IntentStatus{IntentPrepared, IntentComplete, IntentUnwound, IntentStale} {
		in.Status = status
		if err := WriteIntent(path, in); err != nil {
			t.Fatal(err)
		}
		if got, _, _ := ReadIntent(path); got.Status != status {
			t.Errorf("status %q after writing %q", got.Status, status)
		}
	}
}

func TestReadIntent_AbsentIsNotFoundAndCorruptIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cycle-1830.json")
	if _, found, err := ReadIntent(path); found || err != nil {
		t.Errorf("an absent intent: found=%v err=%v, want neither", found, err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReadIntent(path); found || err == nil {
		t.Errorf("a corrupt intent: found=%v err=%v, want an error", found, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReadIntent(path); found || err == nil {
		t.Errorf("an unreadable intent: found=%v err=%v, want an error", found, err)
	}
}

func TestWriteIntent_FailsWhenTheLandingDirCannotHoldIt(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "landing"), []byte("a file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteIntent(IntentPath(evolveDir, 1830), preparedIntent()); err == nil {
		t.Error("a file where the landing dir should be must fail the write")
	}
}

func TestIntent_CannotFailToMarshal(t *testing.T) {
	assertMarshalCannotFail(t, reflect.TypeOf(Intent{}), "Intent")
}

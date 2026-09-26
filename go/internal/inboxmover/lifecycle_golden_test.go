package inboxmover

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var fixedLifecycleClock = time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

func goldenTemplate(got, root string) string {
	got = strings.ReplaceAll(got, root, "{{ROOT}}")
	got = strings.ReplaceAll(got, ".tmp."+strconv.Itoa(os.Getpid()), ".tmp.{{PID}}")
	return regexp.MustCompile(`"prev_hash":"[0-9a-f]{64}"`).ReplaceAllString(got, `"prev_hash":"{{HASH}}"`)
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("golden %s: %v", name, err)
	}
	return string(b)
}

func assertGolden(t *testing.T, name, got, root string) {
	t.Helper()
	if templated, want := goldenTemplate(got, root), readGolden(t, name); templated != want {
		t.Errorf("%s drifted from the golden captured on 8e8f080f:\n--- got ---\n%s\n--- want ---\n%s", name, templated, want)
	}
}

// treeOf lists every path under dir, sorted and relative, with directories marked by a trailing slash.
func treeOf(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			rel += "/"
		}
		lines = append(lines, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

func writeItemAt(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func goldenOptions(repo string, stderr *strings.Builder) Options {
	return Options{
		ProjectRoot:   repo,
		Stderr:        stderr,
		Now:           func() time.Time { return fixedLifecycleClock },
		IsLandedFn:    func(sha string) (bool, error) { return sha != "unlanded00", nil },
		ActiveCycleFn: func() (string, error) { return "13", nil },
	}
}

func TestGolden_ClaimPromoteRelease(t *testing.T) {
	repo := makeRepo(t)
	var stderr strings.Builder
	opts := goldenOptions(repo, &stderr)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	dropInboxFile(t, repo, "task-2.json", "task-2")
	if _, err := Claim(opts, "task-1", "7"); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(opts, "task-2", "7"); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(opts, "task-1", "processed", PromoteOpts{Cycle: "7", CommitSHA: "abcdef1234567890"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReleaseCycleProcessing(opts, 7); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "g1.ledger.golden", readLedger(t, repo), repo)
	assertGolden(t, "g1.stderr.golden", stderr.String(), repo)
	assertGolden(t, "g1.tree.golden", treeOf(t, filepath.Join(repo, ".evolve", "inbox")), repo)
}

func TestGolden_FailDrain_QuarantineAtCeiling(t *testing.T) {
	repo := makeRepo(t)
	var stderr strings.Builder
	opts := goldenOptions(repo, &stderr)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItemAt(t, filepath.Join(inbox, "processing", "cycle-5", "task-a.json"),
		`{"id":"task-a","title":"A","failure_count":1,"continuation":{"snapshot_sha":"abc123","cycle":4},"zeta":true}`)
	writeItemAt(t, filepath.Join(inbox, "processing", "cycle-5", "task-b.json"), `{"id":"task-b","title":"B"}`)
	writeItemAt(t, filepath.Join(repo, ".evolve", "runs", "cycle-5", "continuation-manifest.json"), `{"snapshot_sha":"abc123","cycle":5}`)
	res, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 5, CommittedIDs: []string{"task-a"}, Ceiling: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Quarantined) != 1 || len(res.Released) != 1 {
		t.Fatalf("res = %+v", res)
	}
	item, err := os.ReadFile(filepath.Join(inbox, "quarantine", "task-a.json"))
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "g2.item.golden", string(item), repo)
	assertGolden(t, "g2.stderr.golden", stderr.String(), repo)
	assertGolden(t, "g2.ledger.golden", readLedger(t, repo), repo)
}

func TestGolden_RecoverOrphans_ActiveSkipped(t *testing.T) {
	repo := makeRepo(t)
	var stderr strings.Builder
	opts := goldenOptions(repo, &stderr)
	opts.ActiveCycleFn = nil // the default reader over cycle-state.json
	setCycleState(t, repo, "2")
	dropProcessingFile(t, repo, "1", "task-a.json", "task-a")
	dropProcessingFile(t, repo, "2", "task-b.json", "task-b")
	res, err := RecoverOrphans(opts)
	if err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	assertGolden(t, "g3.stderr.golden", stderr.String(), repo)
	assertGolden(t, "g3.ledger.golden", readLedger(t, repo), repo)
}

func TestGolden_ReleaseFromQuarantine_ItemBytes(t *testing.T) {
	repo := makeRepo(t)
	var stderr strings.Builder
	opts := goldenOptions(repo, &stderr)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItemAt(t, filepath.Join(inbox, "quarantine", "task-q.json"),
		`{"zeta": 1, "id": "task-q", "failure_count": 3, "last_failure_reason": "boom", "alpha": {"b": 2, "a": [1, 2]}}`)
	if _, err := ReleaseFromQuarantine(opts, "task-q"); err != nil {
		t.Fatal(err)
	}
	item, err := os.ReadFile(filepath.Join(inbox, "task-q.json"))
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "g4.item.golden", string(item), repo)
	assertGolden(t, "g4.stderr.golden", stderr.String(), repo)
	assertGolden(t, "g4.ledger.golden", readLedger(t, repo), repo)
}

// lifecycleSequence provokes one fault per replaced stderr line class; mk lets both goldens run the identical script.
func lifecycleSequence(t *testing.T, repo string, mk func() Options) {
	t.Helper()
	inbox := filepath.Join(repo, ".evolve", "inbox")
	proc := func(cycle int, name string) string {
		return filepath.Join(inbox, "processing", fmt.Sprintf("cycle-%d", cycle), name)
	}
	_, _ = Claim(mk(), "t1", "7")
	writeItemAt(t, filepath.Join(inbox, "c1.json"), `{"id":"c1","route":"console-manual"}`)
	_, _ = Claim(mk(), "c1", "7")
	// A FILE at processing/ makes the cycle dir's mkdir fail.
	writeItemAt(t, filepath.Join(inbox, "t1.json"), `{"id":"t1","payload":"x"}`)
	writeItemAt(t, filepath.Join(inbox, "processing"), "x")
	_, _ = Claim(mk(), "t1", "7")
	mustRemove(t, filepath.Join(inbox, "processing"))
	// A directory at the destination file path makes the rename fail.
	mustMkdirAll(t, proc(7, "t1.json"))
	_, _ = Claim(mk(), "t1", "7")
	mustRemove(t, proc(7, "t1.json"))
	_, _ = Claim(mk(), "t1", "7")
	_, _ = Claim(mk(), "", "7")
	_, _ = Claim(mk(), "t1", "x") // t1 is already claimed, so not-found fires before the cycle check
	_, _ = Promote(mk(), "ghost", "processed", PromoteOpts{Cycle: "7"})
	_, _ = Promote(mk(), "t1", "bogus", PromoteOpts{})
	// An unlanded sha reroutes to retry/.
	_, _ = Promote(mk(), "t1", "processed", PromoteOpts{Cycle: "7", CommitSHA: "unlanded00"})
	// A FILE at processed/ makes the destination mkdir fail.
	writeItemAt(t, proc(7, "t2.json"), `{"id":"t2"}`)
	writeItemAt(t, filepath.Join(inbox, "processed"), "x")
	_, _ = Promote(mk(), "t2", "processed", PromoteOpts{Cycle: "7"})
	mustRemove(t, filepath.Join(inbox, "processed"))
	// A directory at the destination basename makes the rename fail.
	mustMkdirAll(t, filepath.Join(inbox, "processed", "cycle-7", "t2.json"))
	_, _ = Promote(mk(), "t2", "processed", PromoteOpts{Cycle: "7"})
	mustRemove(t, filepath.Join(inbox, "processed", "cycle-7", "t2.json"))
	_, _ = Promote(mk(), "t2", "processed", PromoteOpts{Cycle: "7", CommitSHA: "abcdef1234567890"})
	// A directory at the manifest path, and a root twin of t3.
	mustMkdirAll(t, filepath.Join(repo, ".evolve", "runs", "cycle-8", "continuation-manifest.json"))
	writeItemAt(t, filepath.Join(inbox, "t3.json"), `{"id":"t3-root"}`)
	writeItemAt(t, proc(8, "t3.json"), `{"id":"t3"}`)
	writeItemAt(t, proc(8, "t4.json"), `{"id":"t4"}`)
	_, _ = ReleaseCycleProcessing(mk(), 8)
	// A directory at <path>.tmp.<pid> makes the continuation stamp fail.
	writeItemAt(t, filepath.Join(repo, ".evolve", "runs", "cycle-9", "continuation-manifest.json"), `{"snapshot_sha":"abc123","cycle":9}`)
	writeItemAt(t, proc(9, "t5.json"), `{"id":"t5"}`)
	mustMkdirAll(t, proc(9, "t5.json.tmp."+strconv.Itoa(os.Getpid())))
	_, _ = ReleaseCycleProcessing(mk(), 9)
	// The cycle dir made non-writable makes the release rename fail.
	writeItemAt(t, proc(10, "t6.json"), `{"id":"t6"}`)
	lockDir(t, proc(10, ""))
	_, _ = ReleaseCycleProcessing(mk(), 10)
	unlockDir(t, proc(10, ""))
	if _, err := os.Stat(proc(10, "t6.json")); err != nil {
		t.Fatalf("the read-only cycle dir must have kept t6 (the fault did not happen): %v", err)
	}
	// A FILE at quarantine/: the park's mkdir fails, the item releases.
	writeItemAt(t, proc(11, "t7.json"), `{"id":"t7"}`)
	writeItemAt(t, filepath.Join(inbox, "quarantine"), "x")
	_, _ = ApplyCycleOutcome(mk(), CycleOutcome{Cycle: 11, CommittedIDs: []string{"t7"}, Ceiling: 1})
	mustRemove(t, filepath.Join(inbox, "quarantine"))
	// A DIRECTORY at quarantine/<base>: the park's rename fails (NoOp).
	writeItemAt(t, proc(12, "t8.json"), `{"id":"t8"}`)
	mustMkdirAll(t, filepath.Join(inbox, "quarantine", "t8.json"))
	_, _ = ApplyCycleOutcome(mk(), CycleOutcome{Cycle: 12, CommittedIDs: []string{"t8"}, Ceiling: 1})
	writeItemAt(t, filepath.Join(inbox, "quarantine", "q1.json"), `{"id":"q1","failure_count":3,"last_failure_reason":"x"}`)
	_, _ = ReleaseFromQuarantine(mk(), "q1")
	// The active dir is skipped, t10's root directory twin fails the move, and the t3 the
	// double-move guard left behind clobbers its root twin (a preserved quirk).
	writeItemAt(t, proc(13, "t9.json"), `{"id":"t9"}`)
	writeItemAt(t, proc(14, "t10.json"), `{"id":"t10"}`)
	mustMkdirAll(t, filepath.Join(inbox, "t10.json"))
	writeItemAt(t, proc(14, "t11.json"), `{"id":"t11"}`)
	_, _ = RecoverOrphans(mk())
}

// lockDir makes dir read-only until cleanup; callers assert the fault happened, since root ignores the mode.
func lockDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func unlockDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestGolden_LifecycleSequence_StderrAndLedger(t *testing.T) {
	repo := makeRepo(t)
	var stderr strings.Builder
	lifecycleSequence(t, repo, func() Options { return goldenOptions(repo, &stderr) })
	assertGolden(t, "sequence.stderr.golden", stderr.String(), repo)
	assertGolden(t, "sequence.ledger.golden", readLedger(t, repo), repo)
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func goalIDsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(filepath.Join(inbox, "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, inbox, "a.json", "alpha-ready", 0.5)
	writeInboxFile(t, filepath.Join(root, ".evolve"), "b.json", `{"id":"bravo-console","weight":0.9,"route":"console"}`)
	writeInboxFile(t, filepath.Join(root, ".evolve"), "c.json", `{"id":"delta-waits","weight":0.4,"deps":["alpha-ready"]}`)
	writeInboxFile(t, filepath.Join(root, ".evolve"), "f.json", `{"id":"foxtrot","weight":0.2,"route":"console"}`)
	writeInboxFile(t, filepath.Join(root, ".evolve"), "consumed/d.json", `{"id":"charlie-done","consumed":{"at":"2026-10-01T00:00:00Z","via":"pr-1"}}`)
	return root
}

func checkGoalIDs(root, goal string, resume bool) (int, string) {
	var stderr bytes.Buffer
	cfg := loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), GoalText: goal, Resume: resume}
	return refuseInertGoalItems(cfg, &stderr), stderr.String()
}

func TestRunLoop_RefusesAGoalThatNamesItemsNoLaneCanTake(t *testing.T) {
	root := goalIDsFixture(t)
	var stdout, errBuf bytes.Buffer

	code := runLoop([]string{"--project-root", root, "--goal-text", "work `bravo-console`, charlie-done and delta-waits; skip alpha-ready", "--dry-run"}, nil, &stdout, &errBuf)
	stderr := errBuf.String()

	if code != exitGoalNamesInertItems {
		t.Fatalf("code=%d, want %d — a goal naming items no lane can take must fail at launch\nstderr=%s", code, exitGoalNamesInertItems, stderr)
	}
	for _, want := range []string{"bravo-console: route:console", "charlie-done: consumed", "delta-waits: deps unmet: needs alpha-ready", "goal text does not select or exclude inbox items"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "alpha-ready:") {
		t.Errorf("a lane-selectable item must not be listed as inert:\n%s", stderr)
	}
}

func TestRefuseInertGoalItems_AGoalNamingOnlySelectableItemsLaunches(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)

	code, stderr := checkGoalIDs(root, "work alpha-ready first; mention bravo-consoles, xcharlie-done and the foxtrot word", false)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s — a selectable id, a longer token that only contains an id, or a plain word that equals a one-word id must not refuse the launch", code, stderr)
	}
}

func TestRefuseInertGoalItems_AResumeIgnoresTheItemsItsGoalNames(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)

	code, stderr := checkGoalIDs(root, "work bravo-console", true)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s — a resume continues a checkpointed cycle and must not re-judge its goal", code, stderr)
	}
}

func TestInertGoalItems_ReportsAnUnreadableInbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(filepath.Dir(inbox), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := inertGoalItems(root, filepath.Join(root, ".evolve"), "work alpha-ready", &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), inbox) {
		t.Fatalf("err=%v — an unreadable inbox must be an error that names the inbox, not an empty verdict", err)
	}
}

func TestRefuseInertGoalItems_AnUnreadableInboxWarnsAndLaunches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stderr := checkGoalIDs(root, "work alpha-ready", false)

	if code != 0 || !strings.Contains(stderr, "cannot check the items the goal names") {
		t.Fatalf("code=%d stderr=%s — an unreadable inbox must warn, not block the launch", code, stderr)
	}
}

func TestRefuseInertGoalItems_ARefiledItemIsJudgedByItsPendingCopy(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)
	evolveDir := filepath.Join(root, ".evolve")
	writeInboxFile(t, evolveDir, "e.json", `{"id":"echo-refiled","weight":0.3,"created_at":"2026-10-02T00:00:00Z"}`)
	writeInboxFile(t, evolveDir, "consumed/e.json", `{"id":"echo-refiled","created_at":"2026-09-01T00:00:00Z","consumed":{"at":"2026-09-02T00:00:00Z","via":"pr-2"}}`)

	code, stderr := checkGoalIDs(root, "work echo-refiled", false)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s — a refiled item is selectable; its older consumed record must not refuse the launch", code, stderr)
	}
}

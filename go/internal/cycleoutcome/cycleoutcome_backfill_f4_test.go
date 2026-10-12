package cycleoutcome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func blockInboxPathF4(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", name), []byte("a file where a directory belongs"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyFailure_DrainFailureIsWrappedWithTheCycle(t *testing.T) {
	root, _, ws := seedProject(t, "task-a", []string{"task-a"})
	blockInboxPathF4(t, root, "processing")

	res, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Ceiling: 3})

	if err == nil || !strings.Contains(err.Error(), "apply cycle 7 failure outcome:") {
		t.Fatalf("err = %v", err)
	}
	if len(res.Quarantined) != 0 {
		t.Errorf("nothing may be quarantined by a failed drain: %+v", res)
	}
}

func TestApplyNoWork_NilStderrIsDiscardedAndRoutesTheAnsweredItem(t *testing.T) {
	root, ws := seedLane(t, []string{"answered"}, []string{"answered"}, `{"top_n":[],"dropped":[{"id":"answered","reason":"duplicate"}]}`)

	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7})

	if err != nil || len(res.Routed) != 1 || res.Routed[0] != "answered" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestApplyNoWork_AClaimHeldByAnotherCycleIsSkippedWithoutAnError(t *testing.T) {
	root, ws := seedLane(t, nil, []string{"held"}, `{"top_n":[],"dropped":[{"id":"held","reason":"duplicate"}]}`)
	heldDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-9")
	if err := os.MkdirAll(heldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	heldPath := filepath.Join(heldDir, "2026-09-26T00-00-00Z-held.json")
	if err := os.WriteFile(heldPath, []byte(`{"id":"held","title":"held by a live lane"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7})

	if err != nil {
		t.Fatalf("a peer lane's claim is not this closeout's failure: %v", err)
	}
	if len(res.Routed) != 0 {
		t.Errorf("a claim held by cycle 9 must not be routed by cycle 7: %v", res.Routed)
	}
	if body, _ := os.ReadFile(heldPath); strings.Contains(string(body), "console") {
		t.Errorf("the peer lane's claim was rewritten: %s", body)
	}
}

func TestApplyNoWork_EachLifecycleFaultIsReturnedAndTheRestStillRuns(t *testing.T) {
	t.Run("an item whose record cannot be rewritten", func(t *testing.T) {
		root, ws := seedLane(t, []string{"stuck"}, []string{"stuck"}, `{"top_n":[],"dropped":[{"id":"stuck","reason":"duplicate"}]}`)
		inbox := filepath.Join(root, ".evolve", "inbox")
		if err := os.Chmod(inbox, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
		if f, err := os.CreateTemp(inbox, "probe"); err == nil {
			f.Close()
			t.Skip("directory permissions are not enforced for this user")
		}

		res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7})

		if err == nil || !strings.Contains(err.Error(), "route-console: rewrite") {
			t.Fatalf("err = %v", err)
		}
		if len(res.Routed) != 0 {
			t.Errorf("an unrouted item must not be reported routed: %v", res.Routed)
		}
	})
	t.Run("a scoped id with no inbox item cannot be retired", func(t *testing.T) {
		root, ws := seedLane(t, nil, []string{"ghost"}, `{"top_n":[]}`)
		blockInboxPathF4(t, root, "rejected")

		res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7})

		if err == nil || !strings.Contains(err.Error(), `retire-unbacked "ghost"`) || len(res.Retired) != 0 {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("a release that cannot open the claim directory", func(t *testing.T) {
		root, ws := seedLane(t, []string{"answered"}, []string{"answered"}, `{"top_n":[],"dropped":[{"id":"answered","reason":"duplicate"}]}`)
		blockInboxPathF4(t, root, "processing")

		res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7})

		if err == nil || !strings.Contains(err.Error(), "release-cycle") {
			t.Fatalf("err = %v", err)
		}
		if len(res.Routed) != 1 {
			t.Errorf("routing happens before the release and must be reported: %+v", res)
		}
	})
}

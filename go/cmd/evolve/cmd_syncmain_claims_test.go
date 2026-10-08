package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func smClaim(t *testing.T, repo, name string, cycle string) string {
	t.Helper()
	claim := filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-"+cycle, name)
	if err := os.MkdirAll(filepath.Dir(claim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repo, ".evolve", "inbox", name), claim); err != nil {
		t.Fatal(err)
	}
	return claim
}

func TestSyncMain_AClaimedTrackedItemDoesNotBlockTheSyncOrComeBackAsADuplicate(t *testing.T) {
	repo, bare, item := smStampedPlane(t)
	claim := smClaim(t, repo, "x.json", "1836")
	smOriginWrites(t, bare, ".evolve/inbox/x.json", `{"id":"x","weight":0.9}`)
	smRemoteCommit(t, bare, "other.txt", "origin moves on")

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("sync-main = %d, want a claim treated as loop state, not dirt\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	if _, err := os.Stat(item); !os.IsNotExist(err) {
		t.Errorf("the root copy came back (%v): that is the duplicate of the cycle-1838 incident", err)
	}
	if got := smRead(t, claim); got != `{"id":"x","weight":0.5}` {
		t.Errorf("the claim = %s, want the claim copy untouched", got)
	}
	parked := filepath.Join(repo, ".evolve", "inbox", "origin-conflicts", "cycle-1836", "x.json")
	if got := smRead(t, parked); got != `{"id":"x","weight":0.9}` {
		t.Errorf("origin's copy = %s, want it parked beside the claim", got)
	}
	if !strings.Contains(out.String(), "sync-main: x differs from the claim of cycle-1836: the claim stays, and origin's copy is parked at "+parked) || strings.Contains(out.String(), "inbox stamp") {
		t.Errorf("stdout = %q, want the absorb line with zero stamps", out.String())
	}
}

func TestSyncMain_ADeletedTrackedItemThatNoClaimHoldsStillRefuses(t *testing.T) {
	repo, _, item := smStampedPlane(t)
	if err := os.Remove(item); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code == 0 || !strings.Contains(errb.String(), ".evolve/inbox/x.json") {
		t.Errorf("sync-main = %d (stderr %q), want a refusal: a deletion that no claim holds is dirt", code, errb.String())
	}
}

func TestSyncMain_AnAbsorbThatCannotParkFailsLoudly(t *testing.T) {
	repo, bare, item := smStampedPlane(t)
	smClaim(t, repo, "x.json", "1836")
	smOriginWrites(t, bare, ".evolve/inbox/x.json", `{"id":"x","weight":0.9}`)
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "inbox", "origin-conflicts"), []byte("a file, not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 1 || !strings.Contains(errb.String(), "the merge wrote a claimed inbox item back to the root, and the claim cannot absorb it: absorb x: park origin's copy") {
		t.Errorf("sync-main = %d (stderr %q), want exit 1 naming the failed move", code, errb.String())
	}
	if _, err := os.Stat(item); err != nil {
		t.Errorf("the root copy stays for the planner guard: %v", err)
	}
}

func TestSyncMain_AFailedReplayCommitExitsOneAndNamesTheReplay(t *testing.T) {
	repo, bare, item := smStampedPlane(t)
	smOriginWrites(t, bare, ".evolve/inbox/x.json", `{"id":"x","weight":0.9}`)
	if err := os.WriteFile(item, []byte(`{"id":"x","weight":0.5,"route":"console-manual"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeclitest.Install(t, filepath.Join(repo, ".git", "hooks", "commit-msg"), "#!/bin/sh\ngrep -q replayed \"$1\" && exit 1\nexit 0\n")

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 1 || !strings.Contains(errb.String(), "evolve sync-main: replaying the loop's inbox stamps onto origin's edits failed:") {
		t.Errorf("sync-main = %d (stderr %q), want exit 1 naming the failed replay", code, errb.String())
	}
	if got := smRead(t, item); !strings.Contains(got, "console-manual") || !strings.Contains(got, "0.9") {
		t.Errorf("x.json = %s, want origin's edit with the stamp replayed in the tree, uncommitted", got)
	}
}

func TestSyncMain_AHandRestoredCopyEqualToTheClaimIsRemovedWithNoStamps(t *testing.T) {
	repo, _, item := smStampedPlane(t)
	claim := smClaim(t, repo, "x.json", "1836")
	smGit(t, repo, "restore", ".evolve/inbox/x.json")

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 0 || !strings.Contains(out.String(), "sync-main: removed the root copy of x, which equals the claim of cycle-1836") {
		t.Fatalf("sync-main = %d (stdout %q, stderr %q), want the equal copy absorbed", code, out.String(), errb.String())
	}
	if _, err := os.Stat(item); !os.IsNotExist(err) {
		t.Errorf("the root copy must be gone: %v", err)
	}
	if _, err := os.Stat(claim); err != nil {
		t.Errorf("the claim stays: %v", err)
	}
}

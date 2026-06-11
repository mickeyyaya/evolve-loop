// release_verify_test.go — RED contract for the terminal release-verify step
// (inbox release-rebuild-binary-not-committed acceptance, v18.3.0→v18.5.0
// recurrence): after `evolve release X.Y.Z`, the release must be PROVEN
// self-consistent — tracked go/evolve on disk == the blob in the release
// commit == state.json:expected_ship_sha, `go/evolve --version` reports
// X.Y.Z, and the local tag vX.Y.Z exists at the release commit. A failing
// verify is a post-publish failure: auto-rollback unless --no-rollback.
package releasepipeline

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// withReleaseVerify clones allOkSteps and records ReleaseVerify invocations.
func withReleaseVerify(rec *[][3]string, fail error) Steps {
	s := allOkSteps()
	s.ReleaseVerify = func(repoRoot, target, commitSHA string) error {
		if rec != nil {
			*rec = append(*rec, [3]string{repoRoot, target, commitSHA})
		}
		return fail
	}
	return s
}

func TestRun_ReleaseVerify_RunsAfterPollWithShipSHA(t *testing.T) {
	repo := t.TempDir()
	var rec [][3]string

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       withReleaseVerify(&rec, nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec) != 1 {
		t.Fatalf("ReleaseVerify called %d times, want 1", len(rec))
	}
	if rec[0][0] != repo || rec[0][1] != "1.2.3" || rec[0][2] != "deadbeef1234567890" {
		t.Errorf("ReleaseVerify args = %v, want (repo, 1.2.3, ship SHA)", rec[0])
	}
	if got := res.StepsCompleted[len(res.StepsCompleted)-1]; got != "release-verify" {
		t.Errorf("last completed step = %s, want release-verify (terminal proof)", got)
	}
}

func TestRun_ReleaseVerifyFails_RollsBack(t *testing.T) {
	repo := t.TempDir()
	rollbackCalled := 0
	steps := withReleaseVerify(nil, errors.New("binary not at HEAD"))
	steps.Rollback = func(string, string, string) error { rollbackCalled++; return nil }

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       steps,
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if !strings.Contains(err.Error(), "release-verify") {
		t.Errorf("error must name the failing step: %v", err)
	}
	if rollbackCalled != 1 || !res.RollbackTriggered {
		t.Errorf("rollback: called=%d triggered=%v, want 1/true — an unverifiable release must not stand", rollbackCalled, res.RollbackTriggered)
	}
}

func TestRun_ReleaseVerifyFails_NoRollbackFlagHonored(t *testing.T) {
	repo := t.TempDir()
	rollbackCalled := 0
	steps := withReleaseVerify(nil, errors.New("version mismatch"))
	steps.Rollback = func(string, string, string) error { rollbackCalled++; return nil }

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    repo,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		NoRollback:  true,
		Now:         fixedNow(t),
		Steps:       steps,
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if rollbackCalled != 0 || res.RollbackTriggered {
		t.Errorf("--no-rollback must suppress auto-rollback (called=%d triggered=%v)", rollbackCalled, res.RollbackTriggered)
	}
}

func TestRun_RebuildBinary_ReceivesTargetVersion(t *testing.T) {
	repo := t.TempDir()
	gotTarget := ""
	steps := withReleaseVerify(nil, nil)
	steps.RebuildBinary = func(_ string, target string, _ bool) error {
		gotTarget = target
		return nil
	}

	if _, err := Run(Options{
		Target:      "4.5.6",
		RepoRoot:    repo,
		FromTag:     "v4.5.5",
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       steps,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotTarget != "4.5.6" {
		t.Errorf("RebuildBinary target = %q, want 4.5.6 — without it the ldflags version stamp cannot match the release", gotTarget)
	}
}

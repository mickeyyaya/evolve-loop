package releasepipeline

import (
	"errors"
	"testing"
	"time"
)

// TestRun_RebuildBinaryStepInvokedBeforeShip is the regression for
// v12.2.1 bug #2: `evolve release X.Y.Z` previously shipped source
// only, leaving the marketplace binary frozen at the previous build.
// The pipeline now runs RebuildBinary between version-bump and
// release-sh-check, BEFORE ship's `git add -A` picks up the new bytes.
func TestRun_RebuildBinaryStepInvokedBeforeShip(t *testing.T) {
	var order []string
	rec := func(name string) func() {
		return func() { order = append(order, name) }
	}
	steps := Steps{
		Preflight:    func(_, _ string, _, _ bool) error { rec("preflight")(); return nil },
		ChangelogGen: func(_, _, _, _ string, _ bool) error { rec("changelog-gen")(); return nil },
		VersionBump:  func(_, _ string, _ bool) error { rec("version-bump")(); return nil },
		RebuildBinary: func(_, _ string, _ bool) error {
			rec("rebuild-binary")()
			return nil
		},
		ReleaseSh:     func(_, _ string) error { rec("release-sh-check")(); return nil },
		ReleaseVerify: func(_, _, _ string) error { rec("release-verify")(); return nil },
		Ship: func(_, _, _ string) (string, error) {
			rec("ship")()
			return "deadbeef", nil
		},
		MarketplacePoll: func(_, _ string, _ time.Duration) error { rec("marketplace-poll")(); return nil },
		Rollback:        func(_, _, _ string) error { return nil },
	}
	res, err := Run(Options{
		Target:      "12.2.2",
		RepoRoot:    t.TempDir(),
		MaxPollWait: time.Second,
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("Run: %v\nresult=%+v", err, res)
	}
	want := []string{"preflight", "changelog-gen", "version-bump", "rebuild-binary", "release-sh-check", "ship", "marketplace-poll", "release-verify"}
	if !equalSlices(order, want) {
		t.Errorf("step order = %v\nwant      %v", order, want)
	}
}

func TestRun_RebuildBinaryFailureBlocksShip(t *testing.T) {
	var shipCalled bool
	steps := Steps{
		Preflight:    func(_, _ string, _, _ bool) error { return nil },
		ChangelogGen: func(_, _, _, _ string, _ bool) error { return nil },
		VersionBump:  func(_, _ string, _ bool) error { return nil },
		RebuildBinary: func(_, _ string, _ bool) error {
			return errors.New("go build failed: undefined: foo")
		},
		ReleaseSh:       func(_, _ string) error { return nil },
		Ship:            func(_, _, _ string) (string, error) { shipCalled = true; return "", nil },
		MarketplacePoll: func(_, _ string, _ time.Duration) error { return nil },
		Rollback:        func(_, _, _ string) error { return nil },
	}
	res, err := Run(Options{
		Target:      "12.2.2",
		RepoRoot:    t.TempDir(),
		MaxPollWait: time.Second,
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
		Steps:       steps,
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("want ErrPrePublishFailed, got %v", err)
	}
	if shipCalled {
		t.Errorf("ship MUST NOT be called when rebuild-binary fails")
	}
	found := false
	for _, s := range res.StepsFailed {
		if s == "rebuild-binary" {
			found = true
		}
	}
	if !found {
		t.Errorf("StepsFailed = %v, want contains 'rebuild-binary'", res.StepsFailed)
	}
}

func TestRun_RebuildBinarySkippedInDryRun(t *testing.T) {
	var rebuildInvoked bool
	steps := Steps{
		Preflight:    func(_, _ string, _, _ bool) error { return nil },
		ChangelogGen: func(_, _, _, _ string, _ bool) error { return nil },
		VersionBump:  func(_, _ string, _ bool) error { return nil },
		RebuildBinary: func(_, _ string, _ bool) error {
			rebuildInvoked = true
			return nil
		},
		ReleaseSh:       func(_, _ string) error { return nil },
		Ship:            func(_, _, _ string) (string, error) { return "", nil },
		MarketplacePoll: func(_, _ string, _ time.Duration) error { return nil },
		ReleaseVerify:   func(_, _, _ string) error { return nil },
		Rollback:        func(_, _, _ string) error { return nil },
	}
	res, err := Run(Options{
		Target:      "12.2.2",
		RepoRoot:    t.TempDir(),
		DryRun:      true,
		MaxPollWait: time.Second,
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("dry-run should not error: %v\nresult=%+v", err, res)
	}
	if rebuildInvoked {
		t.Errorf("RebuildBinary MUST NOT be called in dry-run; it has side effects")
	}
}

// TestDefaultSteps_WiresRebuildBinary asserts the production default
// includes rebuild-binary so operators using `evolve release X.Y.Z`
// without injected Steps get the fix automatically.
func TestDefaultSteps_WiresRebuildBinary(t *testing.T) {
	d := DefaultSteps()
	if d.RebuildBinary == nil {
		t.Errorf("DefaultSteps must wire RebuildBinary; nil would silently restore the v12.2.1 bug")
	}
}

func equalSlices(a, b []string) bool {
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

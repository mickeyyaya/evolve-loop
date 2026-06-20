package releasepipeline

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestRun_EmitsCINotVerifiedAdvisory asserts the success path prints an explicit
// advisory that this gh-free pipeline does NOT verify GitHub CI. Without it a
// direct `evolve release` caller can believe a green pipeline means green CI —
// the v20.1.0 false-success class (the pipeline exited 0 while the released
// commit's `go` workflow was red). CI-gating itself lives in /publish (Option A:
// the binary stays self-contained/headless-safe and only discloses the gap).
func TestRun_EmitsCINotVerifiedAdvisory(t *testing.T) {
	var out bytes.Buffer
	dir := makeHermeticGitRepo(t)
	res, err := Run(Options{
		Target:      "99.2.0",
		RepoRoot:    dir,
		FromTag:     "v0.0.1",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
		Stderr:      &out,
	})
	if err != nil {
		t.Fatalf("Run success path: %v (result=%+v)", err, res)
	}
	if !strings.Contains(out.String(), "GitHub CI is NOT verified by this pipeline") {
		t.Errorf("success output missing CI-not-verified advisory; got:\n%s", out.String())
	}
}

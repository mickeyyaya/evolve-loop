package releasepipeline

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

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

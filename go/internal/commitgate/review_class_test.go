package commitgate

import (
	"context"
	"testing"
)

func TestRun_TheAttestationNamesTheReviewClassOfTheChange(t *testing.T) {
	t.Parallel()
	code, _ := goGate(t, "code-simplifier,go-reviewer")
	cases := map[string]struct {
		res  *Result
		want string
	}{
		"a code change":      {code.Run(context.Background()), "code"},
		"a docs-only change": {docsGate(t, "docs-review").Run(context.Background()), "docs"},
		"a proven comment removal": {runWaiverCase(t, waiverCase{
			files:   "x.go\n",
			onDisk:  map[string]string{"x.go": plainTrimmed},
			atHead:  map[string]string{"x.go": commentedAtHead},
			wantRun: ExitPass,
		}), "comment-only"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if c.res.ExitCode != ExitPass || c.res.Attestation == nil {
				t.Fatalf("ExitCode=%d logs=%v", c.res.ExitCode, c.res.Logs)
			}
			if got := c.res.Attestation.ReviewClass; got != c.want {
				t.Errorf("ReviewClass = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAttestation_TheReviewClassIsWrittenAfterTheWaiverAndBeforeTheRuns(t *testing.T) {
	t.Parallel()
	att := &Attestation{
		TreeStateSHA: "z",
		TS:           "t",
		ReviewersRun: []string{"go-reviewer"},
		ReviewWaiver: "comment-only",
		ReviewClass:  "code",
		ReviewRuns:   []ReviewRun{{RunID: "r", Profile: "go-reviewer"}},
		Tool:         "shasum",
	}
	want := `{
  "tree_state_sha": "z",
  "ts": "t",
  "checks_passed": [],
  "reviewers_run": ["go-reviewer"],
  "review_waiver": "comment-only",
  "review_class": "code",
  "review_runs": [{"run_id":"r","profile":"go-reviewer","cli":"","family":"","model":""}],
  "tool": "shasum"
}
`
	if got, err := att.Marshal(); err != nil || string(got) != want {
		t.Fatalf("Marshal mismatch (%v):\n%s", err, got)
	}
}

func TestReviewRunToken_IsTheTokenThatTheGateResolves(t *testing.T) {
	t.Parallel()
	token := ReviewRunToken("agent-go-reviewer-1")
	if token != "run:agent-go-reviewer-1" || !isReviewRun(token) {
		t.Fatalf("ReviewRunToken = %q, want a run: token that isReviewRun accepts", token)
	}
}

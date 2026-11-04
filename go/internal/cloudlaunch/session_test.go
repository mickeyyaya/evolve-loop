package cloudlaunch

import (
	"strings"
	"testing"
)

func sampleSession() Session {
	return Session{
		Schema: Schema, Outcome: OutcomeLaunched, ID: "session_01ABC", URL: "https://claude.ai/code/session_01ABC?from=cli&m=0",
		Title: "Fix the flaky test", Repo: "acme/widgets", Branch: "feature/x", Profile: "builder", CLI: "claude-cloud", TS: "2026-11-03T10:00:00Z",
	}
}

func TestRender_ThenParse_ReturnsTheSession(t *testing.T) {
	want := sampleSession()
	got, ok := Parse(Render(want, ""))
	if !ok {
		t.Fatalf("Parse(Render(s)) found no session in %q", Render(want, ""))
	}
	if got != want {
		t.Errorf("Parse(Render(s)) = %+v, want %+v", got, want)
	}
}

func TestRender_NamesTheOutcomeTheURLAndTheLaterPullRequest(t *testing.T) {
	report := Render(sampleSession(), "")
	for _, want := range []string{"Outcome: launched", "https://claude.ai/code/session_01ABC", "pull request", "acme/widgets", "feature/x"} {
		if !strings.Contains(report, want) {
			t.Errorf("Render() lacks %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "challenge-token") {
		t.Errorf("Render() with no challenge carries a challenge line:\n%s", report)
	}
}

func TestRender_CarriesTheChallengeTokenFirst(t *testing.T) {
	report := Render(sampleSession(), "tok-123")
	if !strings.HasPrefix(report, "<!-- challenge-token: tok-123 -->\n") {
		t.Errorf("Render() = %q, want the challenge-token line first", report)
	}
	if _, ok := Parse(report); !ok {
		t.Error("Parse() of a report with a challenge line found no session")
	}
}

func TestParse_RefusesAReportThatHoldsNoLaunchedSession(t *testing.T) {
	launched := Render(sampleSession(), "")
	other := sampleSession()
	other.Outcome = "failed"
	noID := sampleSession()
	noID.ID = ""
	cases := map[string]string{
		"a review report":          "# Review\n\nVerdict: PASS\n",
		"an unclosed block":        strings.TrimSuffix(launched, "```\n"),
		"a block that is not JSON": "```json " + Schema + "\n{\n```\n",
		"another schema":           strings.Replace(launched, `"schema": "`+Schema+`"`, `"schema": "evolve.cloud.launch/0"`, 1),
		"another outcome":          Render(other, ""),
		"no session id":            Render(noID, ""),
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			if s, ok := Parse(report); ok {
				t.Errorf("Parse() = %+v, true; want no session", s)
			}
		})
	}
}

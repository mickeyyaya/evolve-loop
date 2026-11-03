package cloudlaunch

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Schema          = "evolve.cloud.launch/1"
	OutcomeLaunched = "launched"
)

const (
	blockOpen  = "```json " + Schema + "\n"
	blockClose = "```\n"
)

type Session struct {
	Schema  string `json:"schema"`
	Outcome string `json:"outcome"`
	ID      string `json:"session_id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch"`
	Profile string `json:"profile,omitempty"`
	CLI     string `json:"cli"`
	TS      string `json:"ts"`
}

func Render(s Session, challenge string) string {
	raw, _ := json.MarshalIndent(s, "", "  ")
	var b strings.Builder
	if challenge != "" {
		fmt.Fprintf(&b, "<!-- challenge-token: %s -->\n", challenge)
	}
	fmt.Fprintf(&b, "# Cloud launch\n\nOutcome: %s. The cloud session %s (%s) does the work on %s, branch %s.\n", s.Outcome, s.ID, s.Title, s.Repo, s.Branch)
	b.WriteString("Its deliverable is a later pull request, not this report.\n\n")
	fmt.Fprintf(&b, "View: %s\n\n%s%s\n%s", s.URL, blockOpen, raw, blockClose)
	return b.String()
}

func Parse(report string) (Session, bool) {
	_, rest, found := strings.Cut(report, blockOpen)
	if !found {
		return Session{}, false
	}
	body, _, closed := strings.Cut(rest, "\n"+blockClose)
	if !closed {
		return Session{}, false
	}
	var s Session
	if json.Unmarshal([]byte(body), &s) != nil || s.Schema != Schema || s.Outcome != OutcomeLaunched || s.ID == "" {
		return Session{}, false
	}
	return s, true
}

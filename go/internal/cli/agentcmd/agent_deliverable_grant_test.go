package agentcmd

import (
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
)

func writeRuleCovers(rule, rel string) bool {
	if rule == "Write" {
		return true
	}
	pattern, ok := strings.CutPrefix(rule, "Write(")
	if !ok || !strings.HasSuffix(pattern, ")") {
		return false
	}
	return globSegments(strings.Split(strings.TrimSuffix(pattern, ")"), "/"), strings.Split(rel, "/"))
}

func globSegments(pattern, segs []string) bool {
	if len(pattern) == 0 {
		return len(segs) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if globSegments(pattern[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], segs[0])
	return err == nil && ok && globSegments(pattern[1:], segs[1:])
}

func anyRuleCovers(rules []string, rel string) bool {
	for _, rule := range rules {
		if writeRuleCovers(rule, rel) {
			return true
		}
	}
	return false
}

func TestWriteRuleCovers(t *testing.T) {
	rel := ".evolve/ephemeral/agent-runs/agent-x-20261027T000000Z-0001/x-report.md"
	cases := []struct {
		rule string
		want bool
	}{
		{"Write", true},
		{"Write(.evolve/ephemeral/agent-runs/*/x-report.md)", true},
		{"Write(.evolve/ephemeral/**)", true},
		{"Write(.evolve/ephemeral/agent-runs/*/other.md)", false},
		{"Write(.evolve/runs/cycle-*/x-report.md)", false},
		{"Write(.evolve/ephemeral/agent-runs/*)", false},
		{"Write(.evolve/ephemeral/agent-runs/*/x-report.md", false},
		{"Write([)", false},
		{"Edit", false},
		{"Edit(.evolve/ephemeral/agent-runs/*/x-report.md)", false},
	}
	for _, c := range cases {
		if got := writeRuleCovers(c.rule, rel); got != c.want {
			t.Errorf("writeRuleCovers(%q) = %t, want %t", c.rule, got, c.want)
		}
	}
}

func deliverableWriteProblem(allowed, disallowed []string, rel string) string {
	switch {
	case anyRuleCovers(disallowed, rel):
		return "disallowed_tools deny Write on " + rel
	case !anyRuleCovers(allowed, rel):
		return "allowed_tools grant no Write on " + rel
	}
	return ""
}

func TestDeliverableWriteProblem(t *testing.T) {
	rel := ".evolve/ephemeral/agent-runs/agent-x-20261027T000000Z-0001/x-report.md"
	scoped := "Write(.evolve/ephemeral/agent-runs/*/x-report.md)"
	cases := []struct {
		name                string
		allowed, disallowed []string
		want                string
	}{
		{"scoped grant, Edit denied", []string{"Read", scoped}, []string{"Edit"}, ""},
		{"blanket Write deny wins over the grant", []string{"Read", scoped}, []string{"Edit", "Write"}, "disallowed_tools deny Write on " + rel},
		{"scoped deny wins over the grant", []string{scoped}, []string{scoped}, "disallowed_tools deny Write on " + rel},
		{"no grant", []string{"Read", "Bash"}, []string{"Edit"}, "allowed_tools grant no Write on " + rel},
		{"a grant on another file", []string{"Write(.evolve/ephemeral/agent-runs/*/y.md)"}, nil, "allowed_tools grant no Write on " + rel},
	}
	for _, c := range cases {
		if got := deliverableWriteProblem(c.allowed, c.disallowed, rel); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestEveryAgentRunProfileGrantsWriteOnItsDeliverable(t *testing.T) {
	repo := filepath.Join("..", "..", "..", "..")
	cat, err := phasecmd.LoadCatalog(repo)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := listProfiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) == 0 {
		t.Fatal("evolve agent list names no profile")
	}
	at := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, p := range listed {
		spec, ok := cat.Get(p.Name)
		if !ok {
			t.Errorf("%s: evolve agent list names it, but the catalog has no spec", p.Name)
			continue
		}
		profile, err := profileOf(spec, repo)
		if err != nil {
			t.Errorf("%s: no profile: %v", p.Name, err)
			continue
		}
		rel := path.Join(".evolve", runsSubpath, runLabel(p.Name, at), specrunner.ArtifactName(spec))
		if problem := deliverableWriteProblem(profile.AllowedTools, profile.DisallowedTools, rel); problem != "" {
			t.Errorf("%s (profile %s): %s", p.Name, profile.Name, problem)
		}
	}
}

package profiles

import (
	"slices"
	"strings"
	"testing"
)

const inboxCuratorSeat = "inbox-curator"

var curatorReadCommands = []string{"Bash(git log:*)", "Bash(git show:*)"}

func TestInboxCurator_IsAReadOnlySeatThatWritesOnlyItsAuditReport(t *testing.T) {
	loader, _ := RealTreeProfiles(t)
	p, err := loader.Get(inboxCuratorSeat)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range append(slices.Clone(agentStartingTools), "Bash(evolve inbox:*)", "Bash(git commit:*)", "Bash(git push:*)", "Bash(git worktree:*)", "Bash(git grep:*)", "Bash(gh:*)") {
		if slices.Contains(p.AllowedTools, tool) || !slices.Contains(p.DisallowedTools, tool) {
			t.Errorf("%s: %s must be denied and never allowed", inboxCuratorSeat, tool)
		}
	}
	for _, tool := range p.AllowedTools {
		if tool == "Bash" || tool == "Edit" || (strings.HasPrefix(tool, "Bash(") && !slices.Contains(curatorReadCommands, tool)) {
			t.Errorf("%s allows %q: the seat runs only the git read commands", inboxCuratorSeat, tool)
		}
		if strings.HasPrefix(tool, "Write(") && tool != "Write(.evolve/ephemeral/agent-runs/*/inbox-audit.json)" {
			t.Errorf("%s allows %q: the seat writes only inbox-audit.json", inboxCuratorSeat, tool)
		}
	}
	sb := p.Sandbox
	if sb == nil || !sb.Enabled || !sb.ReadOnlyRepo || sb.AllowNetwork || !slices.Equal(sb.WriteSubpaths, []string{".evolve/runs/cycle-*"}) {
		t.Fatalf("%s: the sandbox must be on, read-only on the repository, with no network and no write outside the run directory: %+v", inboxCuratorSeat, sb)
	}
	for _, target := range []string{".evolve/inbox/x.json", ".evolve/inbox/consumed/x.json", ".evolve/policy.json", ".evolve/profiles/builder.json", "go/internal/core/x.go", "agents/x.md"} {
		if sandboxWrites(*sb, target) {
			t.Errorf("%s can write %s", inboxCuratorSeat, target)
		}
	}
}

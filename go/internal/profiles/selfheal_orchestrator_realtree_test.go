package profiles

import (
	"slices"
	"strings"
	"testing"
)

const orchestratorSeat = "selfheal-orchestrator"

var agentStartingTools = []string{"Task", "Agent", "Bash(evolve agent:*)", "Bash(evolve phase:*)", "Bash(evolve subagent:*)", "Bash(evolve loop:*)",
	"Bash(evolve selfheal:*)", "Bash(claude:*)", "Bash(codex:*)", "Bash(agy:*)", "Bash(gemini:*)", "Bash(ollama:*)"}

func TestSelfhealOrchestrator_HasNoToolThatStartsAnAgentOrADispatch(t *testing.T) {
	loader, _ := RealTreeProfiles(t)
	p, err := loader.Get(orchestratorSeat)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range agentStartingTools {
		if slices.Contains(p.AllowedTools, tool) || !slices.Contains(p.DisallowedTools, tool) {
			t.Errorf("%s: %s must be denied and never allowed (I-1, I-3)", orchestratorSeat, tool)
		}
	}
	for _, tool := range p.AllowedTools {
		if tool == "Bash" || tool == "Edit" || (strings.HasPrefix(tool, "Bash(") && !strings.HasPrefix(tool, "Bash(evolve context ")) {
			t.Errorf("%s allows %q: the seat reads its evidence through evolve context only", orchestratorSeat, tool)
		}
		if strings.HasPrefix(tool, "Write(") && !strings.HasSuffix(tool, "/diagnosis.json)") {
			t.Errorf("%s allows %q: the seat writes only diagnosis.json", orchestratorSeat, tool)
		}
	}
	sb := p.Sandbox
	if sb == nil || !sb.Enabled || !sb.ReadOnlyRepo || sb.AllowNetwork || !slices.Equal(sb.WriteSubpaths, []string{".evolve/runs/cycle-*"}) {
		t.Fatalf("%s: the sandbox must be on, read-only on the repository, with no network and no write outside the run directory: %+v", orchestratorSeat, sb)
	}
	for _, target := range []string{".evolve/inbox/x.json", ".evolve/policy.json", ".evolve/profiles/builder.json", ".evolve/incidents/proposals/x.json", "go/internal/core/x.go", "agents/x.md"} {
		if sandboxWrites(*sb, target) {
			t.Errorf("%s can write %s", orchestratorSeat, target)
		}
	}
}

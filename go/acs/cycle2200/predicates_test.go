//go:build acs

package cycle2200

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

var authorityClaims = []string{"verdict authority", "verdict-driven decisions", "makes verdict"}

func loadPersona(t *testing.T, name string) prompts.Prompt {
	t.Helper()
	p, err := prompts.NewForProject(acsassert.RepoRoot(t)).Agent(name)
	if err != nil {
		t.Fatalf("the production persona loader refused %s: %v", name, err)
	}
	return p
}

func frontmatterText(t *testing.T, p prompts.Prompt, key string) string {
	t.Helper()
	v, ok := p.Frontmatter[key].(string)
	if !ok || strings.TrimSpace(v) == "" {
		t.Fatalf("persona %s has no %q frontmatter string", p.Name, key)
	}
	return v
}

func proseLines(body string) []string {
	var out []string
	for _, l := range strings.Split(body, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, ">") {
			out = append(out, t)
		}
	}
	return out
}

func TestC2200_001_OrchestratorFrontmatterClaimsNoVerdictAuthority(t *testing.T) {
	p := loadPersona(t, "evolve-orchestrator")
	for _, key := range []string{"description", "perspective"} {
		text := strings.ToLower(frontmatterText(t, p, key))
		for _, claim := range authorityClaims {
			if strings.Contains(text, claim) {
				t.Errorf("RED: frontmatter %q still claims %q: %s", key, claim, text)
			}
		}
	}
}

func TestC2200_002_OrchestratorFrontmatterSaysTheModelProposes(t *testing.T) {
	p := loadPersona(t, "evolve-orchestrator")
	for _, key := range []string{"description", "perspective"} {
		text := strings.ToLower(frontmatterText(t, p, key))
		if !strings.Contains(text, "propos") {
			t.Errorf("RED: frontmatter %q never says the orchestrator proposes: %s", key, text)
		}
	}
	desc := strings.ToLower(frontmatterText(t, p, "description"))
	if !strings.Contains(desc, "kernel") && !strings.Contains(desc, "go ") {
		t.Errorf("RED: description does not name Go or the kernel as the decider: %s", desc)
	}
}

func TestC2200_003_OrchestratorBodyProseSaysTheKernelDecidesAndClaimsNoAuthority(t *testing.T) {
	p := loadPersona(t, "evolve-orchestrator")
	prose := proseLines(p.Body)
	if len(prose) == 0 {
		t.Fatal("persona body has no prose outside the blockquote")
	}
	proposes := false
	for _, line := range prose {
		low := strings.ToLower(line)
		for _, claim := range authorityClaims {
			if strings.Contains(low, claim) {
				t.Errorf("RED: body line still claims %q: %s", claim, line)
			}
		}
		if strings.Contains(low, "propos") && (strings.Contains(low, "kernel") || strings.Contains(line, "Go ")) {
			proposes = true
		}
	}
	if !proposes {
		t.Error("RED: no prose line outside the blockquote says the orchestrator proposes and the kernel or Go decides")
	}
}

func TestC2200_004_KernelStillDecidesEveryAdjudicatedAuditFail(t *testing.T) {
	names := []string{
		"TestDecideAfterAuditFail_AdjudicatorCannotOverturnTheFloor",
		"TestDecideAfterAuditFail_TheAdjudicatorCannotSendAnExplanationOnlyFailToTDD",
		"TestClampAdjudication_HaltingEnvelopeIsNotNegotiable",
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+strings.Join(names, "|")+")$", "-count=1", "-v", corePkg)
	out := stdout + stderr
	if code != 0 {
		t.Fatalf("kernel decision tests fail (exit %d, err %v)\n%s", code, err, acsassert.OutputTail(out, 30))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC2200_005_OrchestratorPersonaKeepsItsNonLLMStatementAndRendersStayInSync(t *testing.T) {
	p := loadPersona(t, "evolve-orchestrator")
	for _, want := range []string{"NON-LLM", "core.Orchestrator", "Model proposes, kernel disposes"} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("persona lost %q", want)
		}
	}
	drift, err := skillcheck.Check(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("skills check failed to run: %v", err)
	}
	if len(drift) != 0 {
		t.Errorf("skills check reports drift: %v", drift)
	}
}

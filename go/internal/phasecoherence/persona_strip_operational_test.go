package phasecoherence

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// operationalSentinelRE is deliberately coarse: a false positive costs a relocation above the
// strip marker, while a false negative strips a directive from every dispatched prompt.
var operationalSentinelRE = regexp.MustCompile(`MANDATORY|STOP CRITERION|Completion Gates|force-FAIL|REQUIRED|POSTHOC|Verdict Rules|Constitutional audit`)

// strippedPersonaBody returns (body, stripped body) as the runner's dispatch path derives them.
func strippedPersonaBody(t *testing.T, root, name string) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "agents", name))
	if err != nil {
		t.Fatalf("read persona %s: %v", name, err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("frontmatter %s: %v", name, err)
	}
	return body, prompts.StripOnDemandSections(body)
}

func TestAuditorStripKeepsOperationalContract(t *testing.T) {
	root := repoRootForPairing(t)
	_, stripped := strippedPersonaBody(t, root, "evolve-auditor.md")

	anchors := []string{
		"defect-dispositions.json",
		"Continuation dispositions",
		`"status": "FIXED"`,
		"## Verdict Rules",
		"## STOP CRITERION",
		"acs-verdict.json",
		"## POSTHOC verification",
		"## Constitutional audit checklist",
	}
	for _, a := range anchors {
		if !strings.Contains(stripped, a) {
			t.Errorf("compacted auditor prompt lost operational anchor %q — it sits below the ## Reference Index marker and is stripped from every dispatched audit (the cycle-1390-1429 disposition-preflight storm); relocate it above the marker in agents/evolve-auditor.md", a)
		}
	}
}

func TestPersonaStripKeepsOperationalSentinels(t *testing.T) {
	// pendingCuration may only shrink; adding an entry needs a written justification.
	pendingCuration := map[string]bool{}

	root := repoRootForPairing(t)
	tracked, err := repostate.TrackedFiles(root, "agents")
	if err != nil {
		t.Fatalf("tracked agents/: %v", err)
	}
	personas := 0
	for _, rel := range tracked {
		name := filepath.Base(rel)
		if !strings.HasPrefix(name, "evolve-") || !strings.HasSuffix(name, ".md") ||
			strings.HasSuffix(name, "-reference.md") { // reference docs are read on demand, never dispatched through the strip
			continue
		}
		personas++
		t.Run(name, func(t *testing.T) {
			body, stripped := strippedPersonaBody(t, root, name)
			var strippedSentinels []string
			for _, line := range strings.Split(body, "\n") {
				if operationalSentinelRE.MatchString(line) && !strings.Contains(stripped, line) {
					strippedSentinels = append(strippedSentinels, strings.TrimSpace(line))
				}
			}
			if pendingCuration[name] {
				if len(strippedSentinels) == 0 {
					t.Errorf("%s is now clean — remove it from pendingCuration so the sentinel guard arms for it", name)
				}
				return
			}
			for _, line := range strippedSentinels {
				t.Errorf("operational line stripped from dispatched %s prompt: %q — relocate above the ## Reference Index marker (or move the directive out of the on-demand tail)", name, line)
			}
		})
	}
	if personas < 8 {
		t.Fatalf("bound only %d tracked dispatched personas — expected the full fleet (10 at pin time); TrackedFiles scope broken?", personas)
	}
}

package phasecoherence

// persona_strip_operational_test.go — pins that CompactPrompts stripping
// (prompts.StripOnDemandSections, default ON since policy.go CompactPrompts=true)
// never removes OPERATIONAL directives from a dispatched persona.
//
// Incident (2026-08-10, cycles 1390-1429): agents/evolve-auditor.md carried its
// "## Reference Index" marker at line 75 of 272 — every section appended after
// it over months (Verdict Rules, STOP CRITERION, completion gates, POSTHOC,
// the MANDATORY continuation-disposition contract) was silently stripped from
// every dispatched audit prompt. Result: 15/30 FAILs on disposition-preflight,
// 0/11 continuation passes, auditors approving work the gate then force-FAILed.
// The compaction test suite guarded only that stripping REMOVED bytes
// (compaction_coverage_test.go), never that it KEPT the load-bearing ones.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// operationalSentinelRE marks a line as an operational directive that must
// survive prompt compaction. Deliberately coarse: a false positive costs a
// persona author a relocation above the marker; a false negative re-arms the
// cycle-1390-1429 lobotomy class.
var operationalSentinelRE = regexp.MustCompile(`MANDATORY|STOP CRITERION|Completion Gates|force-FAIL|REQUIRED|POSTHOC|Verdict Rules|Constitutional audit`)

// strippedPersonaBody loads a persona from the repo and returns (full body,
// stripped body) exactly as the production dispatch path derives them
// (runner.go: ParseFrontmatter then StripOnDemandSections under CompactPrompts).
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

// TestAuditorStripKeepsOperationalContract is the incident's direct regression
// pin: the exact anchors whose loss caused the 2026-08 zero-ship batches must
// survive compaction of the REAL auditor persona.
func TestAuditorStripKeepsOperationalContract(t *testing.T) {
	root := repoRootForPairing(t)
	_, stripped := strippedPersonaBody(t, root, "evolve-auditor.md")

	anchors := []string{
		// The #1 killer: continuation disposition duty + its literal example.
		"defect-dispositions.json",
		"Continuation dispositions",
		`"status": "FIXED"`,
		// Verdict semantics and termination contract.
		"## Verdict Rules",
		"## STOP CRITERION",
		"acs-verdict.json",
		// Post-verdict integrity layers.
		"## POSTHOC verification",
		"## Constitutional audit checklist",
	}
	for _, a := range anchors {
		if !strings.Contains(stripped, a) {
			t.Errorf("compacted auditor prompt lost operational anchor %q — it sits below the ## Reference Index marker and is stripped from every dispatched audit (the cycle-1390-1429 disposition-preflight storm); relocate it above the marker in agents/evolve-auditor.md", a)
		}
	}
}

// TestPersonaStripKeepsOperationalSentinels generalizes the pin: for every
// tracked persona, no line carrying an operational sentinel may sit below the
// ## Reference Index strip marker.
//
// Exceptions are personas known-broken at pin time, queued for curation in the
// follow-up landing (their below-marker tails need floor-respecting
// reorganization, see compact_marker_gate_test.go savings floors). This list
// may only SHRINK.
func TestPersonaStripKeepsOperationalSentinels(t *testing.T) {
	pendingCuration := map[string]bool{
		"evolve-builder.md":      true, // STOP CRITERION, Completion Gates, POSTHOC below marker
		"evolve-scout.md":        true, // STOP CRITERION + six gates below marker
		"evolve-tdd-engineer.md": true, // REQUIRED predicate-quality reading below marker
		// evolve-triage.md's tail (inbox ingestion, idempotency skip-list) is
		// also operational but carries NO sentinel keyword — this guard cannot
		// see it. Its curation is tracked in the incident doc follow-ups, not
		// here; listing it would falsely imply the sentinel guard covers it.
	}

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
				// Self-pruning: the moment a persona's tail is curated clean,
				// its exception entry must be deleted so the guard arms.
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

package phasecontract

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// FooterMarker prefixes the volatile path line at the end of the prompt.
const FooterMarker = "DELIVERABLE PATH:"

// RenderContractBlock returns the invariant, cache-safe instruction block: deterministic and free of absolute paths.
func RenderContractBlock(c Contract) string {
	return RenderContractBlockStage(c, false)
}

// RenderContractBlockStage renders the block, adding the PhaseIO self-report-failure instruction when includePhaseIO is set.
func RenderContractBlockStage(c Contract, includePhaseIO bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Deliverable Contract (%s)\n\n", c.Phase)
	b.WriteString("Your phase is judged ONLY on the deliverable below. Produce it exactly:\n")
	fmt.Fprintf(&b, "- Write it to the EXACT absolute path shown under \"%s\" at the END of this prompt. Write it nowhere else (not the repo root, not the worktree root, not the current directory).\n", FooterMarker)

	switch c.Kind {
	case KindJSON:
		if len(c.RequiredKeys) > 0 {
			fmt.Fprintf(&b, "- It MUST be a valid JSON object containing these top-level keys: %s.\n", quoteJoin(c.RequiredKeys))
		} else {
			fmt.Fprintf(&b, "- It MUST be a valid JSON %s.\n", c.TopLevelJSONShape())
		}
	default:
		if names := sectionNames(c.Sections); names != "" {
			fmt.Fprintf(&b, "- It MUST contain these sections: %s.\n", names)
		}
		if len(c.Verdicts) > 0 {
			fmt.Fprintf(&b, "- End it with this machine-readable line (use your real verdict, one of %s):\n  %s\n",
				bracketJoin(c.Verdicts), RenderVerdictSentinel(c.Phase, c.Verdicts[0]))
		}
		if c.RequireFailureContext {
			fmt.Fprintf(&b, "- On FAIL or WARN, the sentinel MUST carry your structured failure context (one defect per list entry; evidence_paths are workspace-relative artifacts that prove it). \"class\" MUST be one of [%s] — it drives the retry envelope, so an invented class forfeits the repair round; put your judgment in defects/prescription:\n  %s\n",
				failurelog.VocabularyList(), RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase)))
		} else if c.RequireFailureContextPhaseIO && includePhaseIO {
			// A successful run emits no verdict: the gate bites only on a FAIL/WARN sentinel without the block.
			fmt.Fprintf(&b, "- If this phase fails or you must flag a blocking problem, emit a machine-readable verdict line declaring FAIL (or WARN) that ALSO carries your structured failure context (one defect per list entry; evidence_paths are workspace-relative artifacts that prove it):\n  %s\n  A successful run needs no verdict line.\n",
				RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase)))
		}
	}

	if len(c.AgentOwedFiles) > 0 {
		fmt.Fprintf(&b, "- Also write these agent-owed files at the EXACT paths listed under <owed-files> at the END of this prompt: %s. The orchestrator's gate verifies each exists, is non-empty and (for .json/.ndjson) parses — a missing one is rejected like a missing deliverable.\n", quoteJoin(c.AgentOwedFiles))
	}
	if len(c.Effects) > 0 {
		fmt.Fprintf(&b, "- Declared effects the gate verifies at the phase boundary (your instructions say how to perform each): %s.\n", quoteJoin(c.Effects))
	}
	fmt.Fprintf(&b, "- Before you finish, run:  %s\n", selfCheckCommand(c.Phase))
	b.WriteString("  Fix every violation it reports. Do not declare done until it exits 0.\n\n---\n\n")
	return b.String()
}

// RenderContractFooter returns the volatile path line appended as the last line of the prompt.
func RenderContractFooter(c Contract, artifactPath string) string {
	return fmt.Sprintf("\n\n%s %s\n", FooterMarker, artifactPath)
}

// RenderContractTail returns the footer plus an XML <deliverable-contract> block; owed files resolve against workspace.
func RenderContractTail(c Contract, artifactPath, workspace string) string {
	footer := RenderContractFooter(c, artifactPath)
	if c.NoArtifact {
		return footer
	}
	var b strings.Builder
	b.WriteString(footer)
	fmt.Fprintf(&b, "\n<deliverable-contract phase=%q>\n", c.Phase)
	fmt.Fprintf(&b, "  <artifact-path>%s</artifact-path>\n", artifactPath)
	switch c.Kind {
	case KindJSON:
		fmt.Fprintf(&b, "  <format>a single valid JSON %s — write nothing else to this file</format>\n", c.TopLevelJSONShape())
		if len(c.RequiredKeys) > 0 {
			b.WriteString("  <required-keys>\n")
			for _, k := range c.RequiredKeys {
				fmt.Fprintf(&b, "    <key>%s</key>\n", k)
			}
			b.WriteString("  </required-keys>\n")
		}
	default:
		if len(c.Sections) > 0 {
			b.WriteString("  <required-sections>\n")
			for _, s := range c.Sections {
				fmt.Fprintf(&b, "    <section>%s</section>\n", s.Canonical)
			}
			b.WriteString("  </required-sections>\n")
		}
		if len(c.Verdicts) > 0 {
			// The tail is the copy the agent follows, so where a FAIL/WARN without a
			// failure block is a violation, the exemplar must carry the block.
			if c.RequireFailureContext || c.RequireFailureContextPhaseIO {
				fmt.Fprintf(&b, "  <verdict-sentinel verdicts=%q note=\"a FAIL or WARN verdict MUST carry the failure block shown here\">%s</verdict-sentinel>\n",
					bracketJoin(c.Verdicts), RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase)))
			} else {
				fmt.Fprintf(&b, "  <verdict-sentinel verdicts=%q>%s</verdict-sentinel>\n",
					bracketJoin(c.Verdicts), RenderVerdictSentinel(c.Phase, c.Verdicts[0]))
			}
		}
	}
	if len(c.AgentOwedFiles) > 0 {
		b.WriteString("  <owed-files note=\"write each at exactly this path; the gate verifies it there\">\n")
		for _, f := range c.AgentOwedFiles {
			fmt.Fprintf(&b, "    <owed-file>%s</owed-file>\n", OwedPath(workspace, f))
		}
		b.WriteString("  </owed-files>\n")
	}
	if len(c.Effects) > 0 {
		b.WriteString("  <effects note=\"verified at the phase boundary\">\n")
		for _, e := range c.Effects {
			fmt.Fprintf(&b, "    <effect>%s</effect>\n", e)
		}
		b.WriteString("  </effects>\n")
	}
	// Placeholders stay literal, not XML-escaped: an agent pastes this into a shell verbatim.
	fmt.Fprintf(&b, "  <self-check>%s</self-check>\n", selfCheckCommand(c.Phase))
	b.WriteString("</deliverable-contract>\n")
	return b.String()
}

// selfCheckCommand is the one rendering of the self-check invocation, shared by the block and the tail.
func selfCheckCommand(phase string) string {
	return fmt.Sprintf("evolve phase verify %s --workspace <your workspace dir>", phase)
}

// failureExemplar is the placeholder failure block every failure instruction shows.
func failureExemplar(phase string) *FailureBlock {
	return &FailureBlock{
		Class:         exemplarClass(phase),
		Defects:       []string{"<one line per defect>"},
		EvidencePaths: []string{"<artifact path>"},
	}
}

func sectionNames(sections []Section) string {
	names := make([]string, 0, len(sections))
	for _, s := range sections {
		names = append(names, "\""+s.Canonical+"\"")
	}
	return strings.Join(names, ", ")
}

func quoteJoin(keys []string) string {
	q := make([]string, len(keys))
	for i, k := range keys {
		q[i] = "\"" + k + "\""
	}
	return strings.Join(q, ", ")
}

func bracketJoin(vs []string) string {
	return strings.Join(vs, "|")
}

// OwedPath joins an agent-owed file's basename to the workspace; the gate and the prompt tail both use it.
func OwedPath(workspace, name string) string {
	return filepath.Join(workspace, filepath.Base(name))
}

// exemplarClass draws the exemplar's class from the failurelog vocabulary, which the gate validates against.
func exemplarClass(phase string) string {
	if phase == "audit" {
		return string(failurelog.CodeAuditFail)
	}
	return string(failurelog.CodeBuildFail)
}

package phasecontract

import (
	"fmt"
	"strings"
)

// Rendering of the Deliverable Contract into the prompt (ADR-0034, Layer 2).
// Split into two pieces for prompt-cache safety AND instruction recency:
//
//   - RenderContractBlock: the INVARIANT instruction block. Identical across
//     cycles for a given phase, so it stays in the cacheable prompt prefix
//     (injected alongside the rules/policy blocks). Carries NO absolute path.
//   - RenderContractFooter: the VOLATILE one-line path declaration, appended as
//     the LAST line of the prompt. The per-cycle path therefore never pollutes
//     the cache prefix, and lands where recency bias makes the model most likely
//     to obey it.
//
// Why this fixes the bug: today the agent must infer its output path (read the
// workspace from cycle context, recall the filename from prose, join them). The
// footer states the exact absolute path; the block tells it to use exactly that
// path, emit the verdict sentinel, and self-check with `evolve phase verify`
// before finishing.

// FooterMarker prefixes the volatile path line so the agent (and any tooling)
// can locate it unambiguously at the end of the prompt.
const FooterMarker = "DELIVERABLE PATH:"

// RenderContractBlock returns the invariant instruction block for a contract.
// Deterministic; contains no absolute path (cache-safety). It is
// RenderContractBlockStage with the PhaseIO instruction OFF — the byte-identical
// default that keeps non-loop callers (and EVOLVE_PHASE_IO=off) unchanged.
func RenderContractBlock(c Contract) string {
	return RenderContractBlockStage(c, false)
}

// RenderContractBlockStage renders the contract block, optionally adding the
// PhaseIO self-report-failure instruction (ADR-0050 §3.8b). includePhaseIO is
// set by the dispatch path when EVOLVE_PHASE_IO>=advisory; it instructs
// build/scout/triage — phases that emit no verdict by default — to self-report a
// FAIL/WARN via a sentinel carrying a structured failure block. A false value is
// byte-identical to the pre-3.8b block, so production (off) prompts never change.
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
			b.WriteString("- It MUST be a valid JSON object.\n")
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
			fmt.Fprintf(&b, "- On FAIL or WARN, the sentinel MUST carry your structured failure context (one defect per list entry; evidence_paths are workspace-relative artifacts that prove it):\n  %s\n",
				RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase)))
		} else if c.RequireFailureContextPhaseIO && includePhaseIO {
			// build/scout/triage emit no verdict by default. When the PhaseIO
			// rollout activates (stage>=advisory), give them a self-report-failure
			// channel: a FAIL/WARN sentinel carrying a structured block. A
			// successful run emits nothing — no forced verdict (matching the gate,
			// which only bites on a FAIL/WARN sentinel that lacks the block).
			fmt.Fprintf(&b, "- If this phase fails or you must flag a blocking problem, emit a machine-readable verdict line declaring FAIL (or WARN) that ALSO carries your structured failure context (one defect per list entry; evidence_paths are workspace-relative artifacts that prove it):\n  %s\n  A successful run needs no verdict line.\n",
				RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase)))
		}
	}

	fmt.Fprintf(&b, "- Before you finish, run:  evolve phase verify %s --workspace <your workspace dir>\n", c.Phase)
	b.WriteString("  Fix every violation it reports. Do not declare done until it exits 0.\n\n---\n\n")
	return b.String()
}

// RenderContractFooter returns the volatile one-line path declaration to append
// as the last line of the prompt.
func RenderContractFooter(c Contract, artifactPath string) string {
	return fmt.Sprintf("\n\n%s %s\n", FooterMarker, artifactPath)
}

// failureExemplar is the placeholder failure block shown in the prompt so the
// agent sees the exact shape to emit. Shared by the audit (unconditional) and
// PhaseIO (build/scout/triage) instruction branches so they can never drift.
func failureExemplar(phase string) *FailureBlock {
	return &FailureBlock{
		Class:         "code-" + phase + "-fail",
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

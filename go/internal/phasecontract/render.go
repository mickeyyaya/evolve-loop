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
// Deterministic; contains no absolute path (cache-safety).
func RenderContractBlock(c Contract) string {
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
				RenderVerdictSentinelWithFailure(c.Phase, "FAIL", &FailureBlock{
					Class:         "code-" + c.Phase + "-fail",
					Defects:       []string{"<one line per defect>"},
					EvidencePaths: []string{"<artifact path>"},
				}))
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

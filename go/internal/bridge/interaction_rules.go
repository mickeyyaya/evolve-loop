package bridge

// interaction_rules.go — ADR-0045 I4 CONSUMPTION side: merge promoted,
// enforce-stage auto-respond rules into a launch's active rule set. The
// PROMOTION side (the quarantined advisor that mints a rule from a novel
// escalation) reuses interaction.PromoteRule and is wired with the in-bridge
// advisor tail (a named follow-up — the same deferred-LLM plumbing the I3
// advisor needs). This file makes the registry LIVE: once a rule exists under
// .evolve/instincts/interaction-rules/, it fires here, re-validated against
// the immutable healthy-pane corpus at every load (corpus-rot demotes a rule
// whose pattern a new CLI version's banner now matches — threat S3).

import (
	_ "embed"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

//go:embed interaction/healthy-corpus.txt
var healthyCorpusRaw string

// healthyCorpus is the parsed immutable fixture (comment/blank lines dropped),
// passed to interaction.ValidateRule so a promoted rule that matches normal
// output is refused at promotion AND demoted at boot.
var healthyCorpus = parseHealthyCorpus(healthyCorpusRaw)

func parseHealthyCorpus(raw string) []string {
	var lines []string
	for _, ln := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		lines = append(lines, t)
	}
	return lines
}

// interactionRulesDir is the durable promoted-rule registry, a sibling of the
// fatal-signature registry (ADR-0044 Slice 5) under the same instincts root.
func interactionRulesDir(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	return filepath.Join(projectRoot, ".evolve", "instincts", "interaction-rules")
}

// loadPromotedPrompts returns the ENFORCE-stage promoted rules as
// ManifestPrompts ready to append to a launch's auto-respond set. Shadow-stage
// rules are deliberately excluded from the active set — they ride in the
// registry until measured-clean (their would-fire is the I1 soak signal, a
// follow-up). An empty root or registry yields nothing. Re-validation against
// the current corpus happens inside interaction.LoadRules.
func loadPromotedPrompts(projectRoot string) []ManifestPrompt {
	dir := interactionRulesDir(projectRoot)
	if dir == "" {
		return nil
	}
	var out []ManifestPrompt
	for _, r := range interaction.LoadRules(dir, healthyCorpus) {
		if r.Stage != interaction.RuleStageEnforce {
			continue
		}
		out = append(out, ManifestPrompt{
			Name:         "promoted:" + r.ID,
			Regex:        r.Regex,
			ResponseKeys: r.ResponseKeys,
			Policy:       "auto_respond",
			Note:         r.Note,
		})
	}
	return out
}

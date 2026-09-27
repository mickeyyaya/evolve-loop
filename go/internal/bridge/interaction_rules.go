package bridge

import (
	_ "embed"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

//go:embed interaction/healthy-corpus.txt
var healthyCorpusRaw string

// healthyCorpus is the parsed immutable fixture (comment/blank lines dropped), passed to
// interaction.ValidateRule so a promoted rule that matches normal output is refused at promotion and
// demoted at boot.
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

// interactionRulesDir is the durable promoted-rule registry, a sibling of the fatal-signature registry
// under the same instincts root.
// See ADR-0044.
func interactionRulesDir(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	return filepath.Join(projectRoot, ".evolve", "instincts", "interaction-rules")
}

// EnforceMeasuredRule flips one shadow rule to enforce after the batch-end sweep finds it measured-clean;
// the healthy corpus stays single-sourced here, so callers never supply their own.
func EnforceMeasuredRule(projectRoot, id string) error {
	return interaction.EnforceRule(interactionRulesDir(projectRoot), id, healthyCorpus)
}

// shadowObserver pairs a shadow-stage promoted rule with its compiled pattern for observe-only matching
// in the auto-respond tick; it never sends keys or alters control flow.
type shadowObserver struct {
	id string
	re *regexp.Regexp
}

// loadShadowObservers returns the shadow-stage promoted rules, compiled; loadPromotedPrompts returns the
// enforce set, and together they cover the registry — a rule is exactly one of the two.
func loadShadowObservers(projectRoot string) []shadowObserver {
	dir := interactionRulesDir(projectRoot)
	if dir == "" {
		return nil
	}
	var out []shadowObserver
	for _, r := range interaction.LoadRules(dir, healthyCorpus) {
		if r.Stage != interaction.RuleStageShadow {
			continue
		}
		re, err := regexp.Compile(r.Regex)
		if err != nil {
			continue // LoadRules already validated; belt-and-suspenders
		}
		out = append(out, shadowObserver{id: r.ID, re: re})
	}
	return out
}

// loadPromotedPrompts returns the enforce-stage promoted rules as ManifestPrompts ready to append to a
// launch's auto-respond set; shadow-stage rules stay observe-only until the sweep measures them clean.
// An empty root or registry yields nothing.
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

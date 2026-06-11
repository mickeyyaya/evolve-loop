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
	"regexp"
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

// EnforceMeasuredRule flips one shadow rule to enforce after the batch-end
// sweep (R8.2) finds it measured-clean. The healthy corpus stays single-
// sourced here (the same embedded corpus load-time demotion validates
// against) — callers never supply their own.
func EnforceMeasuredRule(projectRoot, id string) error {
	return interaction.EnforceRule(interactionRulesDir(projectRoot), id, healthyCorpus)
}

// shadowObserver pairs a shadow-stage promoted rule with its compiled
// pattern for OBSERVE-ONLY matching in the auto-respond tick — the I1
// would-fire soak signal that feeds the I4 measured auto-enforce sweep
// (R8.2). Never sends keys, never alters control flow.
type shadowObserver struct {
	id string
	re *regexp.Regexp
}

// loadShadowObservers returns the SHADOW-stage promoted rules, compiled.
// (loadPromotedPrompts below returns the enforce set; together they cover
// the registry — a rule is exactly one of the two.)
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

// loadPromotedPrompts returns the ENFORCE-stage promoted rules as
// ManifestPrompts ready to append to a launch's auto-respond set. Shadow-
// stage rules are deliberately excluded from the active set — they ride in
// the registry observe-only (loadShadowObservers) until the R8.2 sweep
// measures them clean. An empty root or registry yields nothing.
// Re-validation against the current corpus happens inside
// interaction.LoadRules.
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

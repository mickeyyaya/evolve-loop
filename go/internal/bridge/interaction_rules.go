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

func interactionRulesDir(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	return filepath.Join(projectRoot, ".evolve", "instincts", "interaction-rules")
}

func EnforceMeasuredRule(projectRoot, id string) error {
	return interaction.EnforceRule(interactionRulesDir(projectRoot), id, healthyCorpus)
}

var compileShadowPattern = regexp.Compile

type shadowObserver struct {
	id string
	re *regexp.Regexp
}

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
		re, err := compileShadowPattern(r.Regex)
		if err != nil {
			continue
		}
		out = append(out, shadowObserver{id: r.ID, re: re})
	}
	return out
}

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

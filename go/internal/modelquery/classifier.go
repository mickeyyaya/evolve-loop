package modelquery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

type PromptDispatcher interface {
	DispatchPrompt(ctx context.Context, cli, prompt string) (string, error)
}

type CLIClassifier struct {
	CLI        string
	Dispatcher PromptDispatcher
}

func (c CLIClassifier) Classify(ctx context.Context, targetCLI string, modelIDs []string) (map[string]string, error) {
	if c.CLI == "" {
		return nil, fmt.Errorf("modelquery: classifier CLI not set")
	}
	if len(modelIDs) == 0 {
		return nil, fmt.Errorf("modelquery: no model ids to classify")
	}
	if c.Dispatcher == nil {
		return nil, fmt.Errorf("modelquery: classifier %s has no PromptDispatcher", c.CLI)
	}
	out, err := c.Dispatcher.DispatchPrompt(ctx, c.CLI, buildClassifyPrompt(targetCLI, modelIDs))
	if err != nil {
		return nil, fmt.Errorf("classifier %s: %w (output: %s)", c.CLI, err, truncate(out, 200))
	}
	objs := jsonObjects(out)
	if len(objs) == 0 {
		return nil, fmt.Errorf("classifier %s: no JSON object in reply: %s", c.CLI, truncate(out, 200))
	}
	for _, raw := range objs {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			continue
		}
		if tiers := sanitizeTierMap(parsed, modelIDs); len(tiers) > 0 {
			return tiers, nil
		}
	}
	return nil, fmt.Errorf("classifier %s: no JSON object mapped a tier to an offered model; reply: %s", c.CLI, truncate(out, 200))
}

func sanitizeTierMap(parsed map[string]string, offered []string) map[string]string {
	valid := make(map[string]struct{}, len(offered))
	for _, id := range offered {
		valid[id] = struct{}{}
	}
	out := make(map[string]string)
	for _, tier := range modelcatalog.CanonicalTiers {
		model := parsed[tier]
		if model == "" {
			continue
		}
		if _, ok := valid[model]; !ok {
			continue
		}
		out[tier] = model
	}
	return out
}

var tierBriefs = map[string]string{
	"fast":     "cheapest / lowest-latency",
	"balanced": "mid capability/cost",
	"deep":     "most capable for hard work",
	"top":      "the frontier tier — the single most capable model offered",
}

func buildClassifyPrompt(targetCLI string, ids []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You map LLM model ids to capability tiers for the %q CLI.\n", targetCLI)
	b.WriteString("Models offered (use these ids EXACTLY, do not invent):\n")
	for _, id := range ids {
		fmt.Fprintf(&b, "  - %s\n", id)
	}
	b.WriteString("\nPick the single best model id for each tier:\n")
	for _, tier := range modelcatalog.CanonicalTiers {
		fmt.Fprintf(&b, "  %-8s = %s\n", tier, tierBriefs[tier])
	}
	b.WriteString("If fewer distinct models exist than tiers, reuse the closest id.\n")
	b.WriteString("Reply with ONLY this JSON, no prose:\n")
	parts := make([]string, 0, len(modelcatalog.CanonicalTiers))
	for _, tier := range modelcatalog.CanonicalTiers {
		parts = append(parts, fmt.Sprintf(`"%s":"<id>"`, tier))
	}
	b.WriteString("{" + strings.Join(parts, ",") + "}")
	return b.String()
}

func jsonObjects(s string) []string {
	var out []string
	depth, start := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case esc:
			esc = false
		case ch == '\\' && inStr:
			esc = true
		case ch == '"':
			inStr = !inStr
		case inStr:
		case ch == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case ch == '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					out = append(out, s[start:i+1])
					start = -1
				}
			}
		}
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

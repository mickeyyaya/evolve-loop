package modelquery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// PromptDispatcher sends a one-shot prompt to cli and returns its reply. The
// production implementation lives in cmd/evolve, routing through
// bridge.Engine.Launch (sandboxed, liveness-probed, cli_fallback-aware) — this
// seam is what keeps that fragile, live-only dispatch out of the unit-tested
// classifier package (mirrors ModelCapturer/recipe.go). Named generically
// (DispatchPrompt, not Classify-specific) so any future one-shot
// model-reaching helper in this package can reuse it.
type PromptDispatcher interface {
	DispatchPrompt(ctx context.Context, cli, prompt string) (string, error)
}

// CLIClassifier maps raw model ids to canonical tiers by asking a working LLM
// CLI (codex/agy/claude) to do the judgment — never a hardcoded model table.
type CLIClassifier struct {
	// CLI is the ready CLI used to run the one-shot classification prompt.
	CLI string
	// Dispatcher sends the classification prompt to CLI. Required — a nil
	// Dispatcher errors rather than falling back to a raw exec (every
	// LLM-CLI control that reaches a model must go through the bridge; see
	// docs/architecture — C1).
	Dispatcher PromptDispatcher
}

// Classify asks CLIClassifier.CLI to map the offered model ids of targetCLI into
// fast/balanced/deep, then validates the reply: only canonical tiers whose model
// is actually one of the offered ids survive (guards against LLM hallucination).
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
	// CLIs like `codex exec` echo the prompt (which contains our literal JSON
	// template) BEFORE the real answer, so scan EVERY balanced JSON object and
	// take the first that yields a non-empty valid tier map. The prompt-echo
	// template sanitizes to empty (its model ids are the literal "<id>"), so it
	// is skipped naturally.
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

// sanitizeTierMap keeps only canonical tiers whose mapped model is one of the
// offered ids. A reply that maps a tier to a hallucinated/empty model is dropped
// for that tier (the caller then falls back to detect for the missing tiers).
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

// buildClassifyPrompt is deterministic so identical inputs cache the same way at
// the LLM layer. It instructs the model to pick one offered id per tier.
func buildClassifyPrompt(targetCLI string, ids []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You map LLM model ids to capability tiers for the %q CLI.\n", targetCLI)
	b.WriteString("Models offered (use these ids EXACTLY, do not invent):\n")
	for _, id := range ids {
		fmt.Fprintf(&b, "  - %s\n", id)
	}
	b.WriteString("\nPick the single best model id for each tier:\n")
	b.WriteString("  fast     = cheapest / lowest-latency\n")
	b.WriteString("  balanced = mid capability/cost\n")
	b.WriteString("  deep     = most capable\n")
	b.WriteString("If fewer than three distinct models exist, reuse the closest id.\n")
	b.WriteString("Reply with ONLY this JSON, no prose:\n")
	b.WriteString(`{"fast":"<id>","balanced":"<id>","deep":"<id>"}`)
	return b.String()
}

// jsonObjects returns every balanced top-level {...} object in s, in order.
// Tolerates header/footer framing and prompt-echo so the caller can pick the
// object that actually answers (brace-aware, string/escape-aware).
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
			// braces inside strings don't count
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

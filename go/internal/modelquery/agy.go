package modelquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// agyModelID is the shape of the id column in `agy models` output: lowercase
// alphanumerics plus separators, never a space. Used to tell a data row from a
// header or banner.
var agyModelID = regexp.MustCompile(`^[a-z0-9][a-z0-9.:_-]*$`)

// AgyLister enumerates agy's models via `agy models` — a clean,
// non-interactive listing, the same shape as OllamaLister.
//
// Why agy does NOT use the /model picker like codex and claude do: agy's
// picker renders the model list and the reasoning effort as two SEPARATE
// controls (a name list plus a low/medium/high slider), so a pane capture
// yields bare family names — "Gemini 3.7 Flash", "Gemini 3.1 Pro". Those are
// not valid `--model` arguments. agy requires the two COMBINED, exactly as
// `agy models` already prints them: "Gemini 3.7 Flash (Low)".
//
// Feeding agy a bare family name is not a loud failure. It warns once and
// serves a different model for the entire session:
//
//	⎿ model Gemini 3.1 Pro is not recognized as a known model or custom model
//	  in settings. Using "Gemini 3.5 Flash (Medium)" instead.
//
// so every tier silently collapsed onto one fallback model. The picker is
// therefore not merely awkward for agy, it is INSUFFICIENT — no amount of
// parser care recovers a field the pane does not contain.
type AgyLister struct {
	// Run defaults to the package's exec runner when nil.
	Run Runner
}

// List returns the display names from `agy models`. The cli argument is
// ignored (this lister is agy-specific) but kept for interface uniformity.
//
// Deliberate divergence from OllamaLister: a zero-result listing is an ERROR
// here, not an empty slice. Downstream (query.go liveTiers) treats both
// identically, so this buys no behaviour — only a WARN that says which CLI
// could not be enumerated instead of one that reads as "agy offers nothing".
//
// C1 exception, same as OllamaLister: `agy models` is metadata-only — it
// enumerates the account's offering and reaches no model, so it is exempt from
// the PromptDispatcher/bridge requirement governing model-reaching dispatch.
// No prompt or stdin is ever passed.
func (l AgyLister) List(ctx context.Context, _ string) ([]string, error) {
	run := l.Run
	if run == nil {
		run = defaultRunner
	}
	out, err := run(ctx, "agy", []string{"models"}, "")
	if err != nil {
		return nil, fmt.Errorf("agy models: %w", err)
	}
	names := parseAgyModels(out)
	if len(names) == 0 {
		// Never degrade to "agy has no models" — that reads as a true empty
		// offering downstream and would commit a catalog with no agy entry.
		return nil, fmt.Errorf("agy models: parsed no models from output")
	}
	return names, nil
}

// parseAgyModels extracts the DISPLAY NAME column from `agy models` output.
//
// Each model line is "<id>\t<display name>" (e.g.
// "gemini-3.7-flash-low\tGemini 3.7 Flash (Low)"). The display name is taken
// rather than the id because it is what the rest of the catalog speaks —
// tier_fallbacks and the manifest model_tier_map are display names — and agy
// accepts either. Mixing the two vocabularies in one catalog is how the
// original drift became hard to see.
//
// Lines without a tab (the "Fetching available models..." progress line,
// blanks, any future banner) are not models and are skipped.
//
// A tab alone is NOT sufficient proof of a model row: a future `agy models`
// that gains a header ("ID\tDISPLAY NAME") would otherwise contribute
// "DISPLAY NAME" as a model. agy ids are lowercase kebab/dot tokens with no
// spaces, so requiring that shape rejects any header or banner while
// accepting every id the live CLI emits (gemini-3.7-flash-high,
// claude-opus-4-6-thinking, gpt-oss-120b-medium). OllamaLister skips its
// header by name; this is the same intent expressed as a shape.
func parseAgyModels(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		id, display, ok := strings.Cut(line, "\t")
		if !ok || !agyModelID.MatchString(strings.TrimSpace(id)) {
			continue
		}
		if name := strings.TrimSpace(display); name != "" {
			names = append(names, name)
		}
	}
	return names
}

package modelquery

import (
	"context"
	"fmt"
)

// ModelCapturer drives a CLI's interactive /model picker and returns the raw
// captured pane text. The production implementation lives in the bridge (it
// owns the tmux primitives); tests inject a fake. This is the seam that keeps
// the fragile, live-only REPL driving out of the unit-tested parsing path.
type ModelCapturer interface {
	CaptureModelPicker(ctx context.Context, cli string) (string, error)
}

// RecipeLister enumerates a CLI's models by capturing its /model picker and
// parsing it with the per-CLI Strategy. It implements Lister for the
// interactive CLIs (codex/agy/claude); ollama uses OllamaLister instead.
type RecipeLister struct {
	// Capturer drives the picker. Required.
	Capturer ModelCapturer
	// Parsers optionally overrides the built-in per-CLI parser registry (open
	// for extension — a new CLI's parser can be supplied without editing this
	// package). nil falls back to pickerParsers.
	Parsers map[string]PickerParser
}

// List captures cli's /model picker and parses the offered model ids.
func (l RecipeLister) List(ctx context.Context, cli string) ([]string, error) {
	parse := l.parserFor(cli)
	if parse == nil {
		return nil, fmt.Errorf("modelquery: no /model parser for cli %q", cli)
	}
	if l.Capturer == nil {
		return nil, fmt.Errorf("modelquery: RecipeLister has no Capturer")
	}
	pane, err := l.Capturer.CaptureModelPicker(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("modelquery: capture /model for %s: %w", cli, err)
	}
	ids := parse(pane)
	if len(ids) == 0 {
		return nil, fmt.Errorf("modelquery: parsed no models from %s /model picker", cli)
	}
	return ids, nil
}

func (l RecipeLister) parserFor(cli string) PickerParser {
	if l.Parsers != nil {
		if p, ok := l.Parsers[cli]; ok {
			return p
		}
	}
	return pickerParsers[cli]
}

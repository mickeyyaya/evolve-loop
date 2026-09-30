package modelquery

import (
	"context"
	"fmt"
)

type ModelCapturer interface {
	CaptureModelPicker(ctx context.Context, cli string) (string, error)
}

type RecipeLister struct {
	Capturer ModelCapturer
	Parsers  map[string]PickerParser
}

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

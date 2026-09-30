package modelquery

import (
	"context"
	"fmt"
	"strings"
)

type OllamaLister struct {
	Run Runner
}

func (l OllamaLister) List(ctx context.Context, _ string) ([]string, error) {
	run := l.Run
	if run == nil {
		run = defaultRunner
	}
	// `ollama list` is metadata-only and reaches no model, so it needs no PromptDispatcher.
	out, err := run(ctx, "ollama", []string{"list"}, "")
	if err != nil {
		return nil, fmt.Errorf("ollama list: %w", err)
	}
	return parseOllamaList(out), nil
}

func parseOllamaList(out string) []string {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "NAME" {
			continue
		}
		ids = append(ids, name)
	}
	return ids
}

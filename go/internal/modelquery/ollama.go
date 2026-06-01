package modelquery

import (
	"context"
	"fmt"
	"strings"
)

// OllamaLister enumerates locally available models via `ollama list` — a clean,
// non-interactive listing (no REPL driving needed).
type OllamaLister struct {
	// Run defaults to the package's exec runner when nil.
	Run Runner
}

// List returns the model ids from `ollama list`. The cli argument is ignored
// (this lister is ollama-specific) but kept for interface uniformity.
func (l OllamaLister) List(ctx context.Context, _ string) ([]string, error) {
	run := l.Run
	if run == nil {
		run = defaultRunner
	}
	out, err := run(ctx, "ollama", []string{"list"}, "")
	if err != nil {
		return nil, fmt.Errorf("ollama list: %w", err)
	}
	return parseOllamaList(out), nil
}

// parseOllamaList extracts model names from `ollama list` table output. The
// first whitespace-delimited token per line is the model id; the NAME header
// row and blank lines are skipped.
func parseOllamaList(out string) []string {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "NAME" { // header row
			continue
		}
		ids = append(ids, name)
	}
	return ids
}

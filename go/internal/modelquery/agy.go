package modelquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var agyModelID = regexp.MustCompile(`^[a-z0-9][a-z0-9.:_-]*$`)

type AgyLister struct {
	Run Runner
}

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
		return nil, fmt.Errorf("agy models: parsed no models from output")
	}
	return names, nil
}

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

package trail

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

var (
	trackingOff = []string{"DO_NOT_TRACK=1", "GRAFT_NO_REFRESH=1"}

	absentLeads   = []string{"✗ no graph found", "env: node: No such file"}
	notFoundLeads = []string{"✗ no symbol"}
)

func (b Backend) env() []string {
	kept := slices.DeleteFunc(slices.Clone(b.cfg.Env), func(entry string) bool {
		return slices.ContainsFunc(trackingOff, func(off string) bool { return envKey(entry) == envKey(off) })
	})
	return append(kept, trackingOff...)
}

func envKey(entry string) string {
	key, _, _ := strings.Cut(entry, "=")
	return key
}

func (b Backend) execute(ctx context.Context, args []string) (string, error) {
	var stdout, stderr strings.Builder
	code, err := b.cfg.Run(ctx, b.cfg.Bin, "", args, b.env(), nil, &stdout, &stderr)
	if err != nil {
		return "", b.runError(err)
	}
	if code != 0 {
		return "", b.exitError(code, stdout.String()+stderr.String())
	}
	return stdout.String(), nil
}

func (b Backend) runError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: the trail binary %s does not exist (index %s)", codeindex.ErrAbsent, b.cfg.Bin, b.cfg.IndexDir)
	}
	return fmt.Errorf("trail: run %s: %w", b.cfg.Bin, err)
}

func (b Backend) exitError(code int, text string) error {
	message := firstLine(text)
	switch {
	case startsWithAny(message, absentLeads):
		return fmt.Errorf("%w: no trail graph or no node for %s (%s)", codeindex.ErrAbsent, b.cfg.IndexDir, message)
	case startsWithAny(message, notFoundLeads):
		return fmt.Errorf("%w: %s", codeindex.ErrNotFound, message)
	}
	return fmt.Errorf("trail: exit %d: %s", code, message)
}

func noHits(verb string) error {
	return fmt.Errorf("%w: trail %s answered with no hit", codeindex.ErrNotFound, verb)
}

func startsWithAny(line string, leads []string) bool {
	return slices.ContainsFunc(leads, func(lead string) bool { return strings.HasPrefix(line, lead) })
}

func firstLine(text string) string {
	for _, line := range cleanLines(text) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

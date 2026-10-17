// Package trail is the only package that runs the trail-code binary; it adapts the binary to codeindex.Backend.
// See docs/architecture/packages/internal-adapters-codeindex-trail.md.
package trail

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	backendName = "trail"
	mapAllDirs  = "1000"
)

type Config struct {
	Bin      string
	IndexDir string
	Env      []string
	Run      sysexec.RunFunc
}

type Backend struct {
	cfg Config
}

func New(cfg Config) Backend {
	return Backend{cfg: cfg}
}

func (b Backend) Name() string { return backendName }

func (b Backend) Capabilities() codeindex.Capabilities { return codeindex.AllCapabilities() }

func (b Backend) Callers(ctx context.Context, symbol string, dir codeindex.Direction) ([]codeindex.Hit, error) {
	return b.query(ctx, []string{"callers", "--direction", string(dir), "--", symbol, b.cfg.IndexDir}, parseCallers)
}

func (b Backend) API(ctx context.Context, file string) ([]codeindex.Hit, error) {
	return b.query(ctx, []string{"skeleton", "--", file, b.cfg.IndexDir}, func(lines []string) []codeindex.Hit {
		return parseSkeleton(file, lines)
	})
}

func (b Backend) Find(ctx context.Context, query string) ([]codeindex.Hit, error) {
	return b.query(ctx, []string{"ask", "--", query, b.cfg.IndexDir}, parseAsk)
}

func (b Backend) Map(ctx context.Context, dir string) ([]codeindex.Hit, error) {
	prefix := dirPrefix(dir)
	args := []string{"map", "--", b.cfg.IndexDir}
	if prefix != "" {
		args = []string{"map", "--max-dirs", mapAllDirs, "--", b.cfg.IndexDir}
	}
	return b.query(ctx, args, func(lines []string) []codeindex.Hit {
		return underPrefix(parseMap(lines), prefix)
	})
}

func (b Backend) query(ctx context.Context, args []string, parse func([]string) []codeindex.Hit) ([]codeindex.Hit, error) {
	out, err := b.execute(ctx, args)
	if err != nil {
		return nil, err
	}
	hits := parse(cleanLines(out))
	if len(hits) == 0 {
		return nil, noHits(args[0])
	}
	return hits, nil
}

func dirPrefix(dir string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(dir), "./")
	if trimmed == "" || trimmed == "." {
		return ""
	}
	return strings.TrimSuffix(trimmed, "/") + "/"
}

func underPrefix(hits []codeindex.Hit, prefix string) []codeindex.Hit {
	var kept []codeindex.Hit
	for _, hit := range hits {
		if strings.HasPrefix(hit.File, prefix) {
			kept = append(kept, hit)
		}
	}
	return kept
}

func (b Backend) Refresh(ctx context.Context) error {
	_, err := b.execute(ctx, []string{"build", "--", b.cfg.IndexDir})
	return err
}

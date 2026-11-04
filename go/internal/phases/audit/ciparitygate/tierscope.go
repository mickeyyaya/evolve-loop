package ciparitygate

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func (g *Gates) wholeSuite(ctx context.Context, dir string) ([]string, error) {
	listOut, err := sysexec.Output(ctx, g.run, dir, "go", "list", "./...")
	if err != nil {
		return nil, fmt.Errorf("integration-tier gate: go list: %w", err)
	}
	pkgs, _, _ := ciparity.TierScope(strings.Fields(listOut))
	return pkgs, nil
}

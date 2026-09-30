package ciparitygate

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func tierScope(changed []string) (scoped, envExclusive []string, wholeModule bool) {
	scoped = make([]string, 0, len(changed))
	for _, p := range changed {
		if p == "./..." {
			return nil, nil, true
		}
		if strings.Contains(p, "/acs/") {
			continue
		}
		if envExclusivePkg(p) {
			envExclusive = append(envExclusive, p)
			continue
		}
		scoped = append(scoped, p)
	}
	return scoped, envExclusive, false
}

func (g *Gates) wholeSuite(ctx context.Context, dir string) ([]string, error) {
	listOut, err := sysexec.Output(ctx, g.run, dir, "go", "list", "./...")
	if err != nil {
		return nil, fmt.Errorf("integration-tier gate: go list: %w", err)
	}
	var pkgs []string
	for _, p := range strings.Fields(listOut) {
		if strings.Contains(p, "/acs/") || envExclusivePkg(p) {
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

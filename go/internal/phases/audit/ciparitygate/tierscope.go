package ciparitygate

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// tierScope is the PURE split of a cycle's already-derived changed-package
// set into the `go test` patterns the integration tier runs: the TOUCHED
// packages themselves (the same O(change) scoping the apicover-enforce gate
// uses), minus /acs/ (which has its own -tags acs gate), minus the
// env-exclusive set (returned separately — the record table is the
// authority). The one whole-module fallback is a module-root change (a
// `./...` pattern from go.mod/go.sum/root main.go) — rare, and no narrower
// scope exists; it wins the moment it is seen.
//
// Scoping is load-bearing for RELIABILITY, not just speed: the tier stays
// O(change) so the -race run's contention exposure is bounded, and the
// serialized retake-on-red absorbs what contention remains. Whole-repo
// integration coverage remains CI's job — the identical backstop
// apicover-enforce relies on.
func tierScope(changed []string) (scoped, envExclusive []string, wholeModule bool) {
	scoped = make([]string, 0, len(changed))
	for _, p := range changed {
		if p == "./..." {
			return nil, nil, true // module-root change → whole module
		}
		if strings.Contains(p, "/acs/") {
			continue // acs has its own -tags acs gate (ACSDurable)
		}
		if envExclusivePkg(p) {
			envExclusive = append(envExclusive, p)
			continue
		}
		scoped = append(scoped, p)
	}
	return scoped, envExclusive, false
}

// wholeSuite lists every module package minus /acs/ (go.yml's `go list ./...
// | grep -v /acs/` filter) minus the env-exclusive set — each record in
// tierEnvExclusive names where those tests actually run instead — for the
// module-root fallback. The listing runs under the caller's env (not the
// scrubbed tier env), as before.
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

package ship

import (
	"context"
	"strings"
	"testing"
)

func TestReconcileManifest_OnlyExactEnforceLiteralBlocks(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"Enforce", "ENFORCE", " enforce", "enforce ", "off", "strict", "block"} {
		opts := manifestLeakOpts(t)
		opts.ManifestGate = mode
		res := &RunResult{}
		err := reconcileManifest(context.Background(), opts, res, "wt", "main", "cycle")
		if err != nil {
			t.Errorf("ManifestGate=%q must NOT block (only the exact %q literal blocks), got %v", mode, ManifestGateEnforce, err)
		}
		if !strings.Contains(strings.Join(res.Logs, "\n"), "leak_test.go") {
			t.Errorf("ManifestGate=%q must still LOG the leak even when misconfigured; logs=%v", mode, res.Logs)
		}
	}
}

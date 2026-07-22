package ship

import (
	"context"
	"strings"
	"testing"
)

// TestReconcileManifest_OnlyExactEnforceLiteralBlocks amplifies the existing
// shadow-regression test (which only exercises "" and "shadow"): it pins
// that reconcileManifest's block/no-block decision is a STRICT equality
// against the exact ManifestGateEnforce literal, not a case-insensitive or
// whitespace-tolerant match. A hand-edited policy.json typo ("Enforce",
// padded, wrong-case) must silently behave as shadow (log-only) rather than
// either blocking unexpectedly or panicking — and every leak must still be
// logged regardless of the gate value, so the operator has evidence even
// when the gate itself is misconfigured.
func TestReconcileManifest_OnlyExactEnforceLiteralBlocks(t *testing.T) {
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

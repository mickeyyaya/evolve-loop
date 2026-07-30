package policy_test

// artifact_budget_docs_pin_test.go — docs-drift pin for the per-phase artifact
// budget's OPERATOR SURFACE.
//
// Why this exists: an operator who follows the docs and gets no effect has no
// signal at all — the phase simply keeps dying at the builtin deadline. The only
// authoritative spelling of this knob is the JSON tag on
// BridgePolicy.PhaseArtifactTimeoutS under the `bridge` block, so the doc's key
// is derived from that tag by REFLECTION here rather than transcribed: renaming
// the field's tag, or publishing any other prefix for it, turns this test RED
// instead of shipping a doc that silently does nothing.
//
// It also pins the compiled DEFAULTS table in the same row, so a future cycle
// cannot widen defaultPhaseArtifactTimeoutS without publishing the new budget.
//
// Idiom: the repo-root walk + t.Skip-on-bare-env shape used by
// cmd/evolve/docs_contract_test.go's findRepoRoot.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// policyJSONKey returns the json tag name of a BridgePolicy field, so the doc
// assertion is bound to the real wire key rather than a copied string.
func policyJSONKey(t *testing.T, field string) string {
	t.Helper()
	f, ok := reflect.TypeOf(policy.BridgePolicy{}).FieldByName(field)
	if !ok {
		t.Fatalf("BridgePolicy has no field %q — the operator surface moved; this pin must move with it", field)
	}
	tag := f.Tag.Get("json")
	if tag == "" {
		t.Fatalf("BridgePolicy.%s has no json tag — it is not an operator-writable key", field)
	}
	return strings.Split(tag, ",")[0]
}

// runtimeReference locates docs/operations/runtime-reference.md by walking up
// from the test's cwd, and skips (never fails) in a bare checkout-free env.
func runtimeReference(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		p := filepath.Join(dir, "docs", "operations", "runtime-reference.md")
		if _, err := os.Stat(p); err == nil {
			body, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read %s: %v", p, err)
			}
			return string(body)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("docs/operations/runtime-reference.md not found (bare test env)")
	return ""
}

// TestRuntimeReference_PublishesTheRealArtifactBudgetKey is the wrong-key guard:
// EVERY mention of the wire key in the operator reference must be qualified with
// the `bridge.` block it actually lives under. A row that publishes
// `workflow.phase_artifact_timeout_s`, a bare `phase_artifact_timeout_s`, or any
// other prefix sends the operator to a key `policy.Load` ignores.
func TestRuntimeReference_PublishesTheRealArtifactBudgetKey(t *testing.T) {
	doc := runtimeReference(t)
	key := policyJSONKey(t, "PhaseArtifactTimeoutS")
	qualified := "bridge." + key

	if !strings.Contains(doc, qualified) {
		t.Fatalf("runtime-reference.md never mentions %q — the per-phase artifact budget has no documented "+
			"operator surface", qualified)
	}
	// Walk every occurrence of the bare key; each must be the tail of the
	// qualified form.
	for i := 0; ; {
		j := strings.Index(doc[i:], key)
		if j < 0 {
			break
		}
		at := i + j
		if !strings.HasPrefix(doc[max0(at-len("bridge.")):], qualified) {
			line := lineAt(doc, at)
			t.Errorf("runtime-reference.md publishes %q unqualified or under the wrong block:\n  %s\n"+
				"the ONLY key policy.Load reads is %q — a doc-following operator otherwise gets silent no-op",
				key, line, qualified)
		}
		i = at + len(key)
	}
}

// TestRuntimeReference_PublishesCompiledArtifactBudgets pins the DEFAULTS: the
// documented row must carry every compiled (label, budget) pair verbatim in the
// `"label": budget` JSON shape. Widening defaultPhaseArtifactTimeoutS without
// documenting the new budget fails here — the drift that let the 300s cliff sit
// undocumented for six lost cycles.
func TestRuntimeReference_PublishesCompiledArtifactBudgets(t *testing.T) {
	doc := runtimeReference(t)
	key := policyJSONKey(t, "PhaseArtifactTimeoutS")
	row := ""
	for _, line := range strings.Split(doc, "\n") {
		if strings.Contains(line, key) {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatalf("no runtime-reference.md row documents %q", key)
	}
	for label, budget := range (policy.BridgePolicy{}).PhaseArtifactTimeouts() {
		pair := fmt.Sprintf("%q: %d", label, budget)
		if !strings.Contains(row, pair) {
			t.Errorf("compiled default %s is not published in the runtime-reference row — an operator cannot "+
				"see the budget their phase actually gets", pair)
		}
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// lineAt returns the whole line containing byte offset at, bounded for error text.
func lineAt(doc string, at int) string {
	start := strings.LastIndexByte(doc[:at], '\n') + 1
	end := strings.IndexByte(doc[at:], '\n')
	if end < 0 {
		end = len(doc)
	} else {
		end += at
	}
	line := doc[start:end]
	if len(line) > 200 {
		return line[:200] + "…"
	}
	return line
}

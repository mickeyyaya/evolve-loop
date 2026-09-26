package policy_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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

func TestRuntimeReference_PublishesTheRealArtifactBudgetKey(t *testing.T) {
	doc := runtimeReference(t)
	key := policyJSONKey(t, "PhaseArtifactTimeoutS")
	qualified := "bridge." + key

	if !strings.Contains(doc, qualified) {
		t.Fatalf("runtime-reference.md never mentions %q — the per-phase artifact budget has no documented "+
			"operator surface", qualified)
	}
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

package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeReference_DoesNotClaimCompactItemRewrite(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"compact JSON with sorted keys, like the route verbs", "compare fields, not lines"} {
		if strings.Contains(string(body), stale) {
			t.Errorf("runtime-reference.md still says %q, but the item writer keeps the filed format", stale)
		}
	}
}

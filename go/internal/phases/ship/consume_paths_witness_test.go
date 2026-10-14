package ship

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var consumedPathsAssignRe = regexp.MustCompile(`\binternalConsumedPaths\s*=[^=]`)

func TestInternalConsumedPaths_OnlyWrittenInConsumeGo(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			name == "consume.go" || name == "native.go" {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if loc := consumedPathsAssignRe.Find(body); loc != nil {
			t.Errorf("%s writes internalConsumedPaths — the ONLY sanctioned writer is consumeCommittedItems (consume.go); a second writer widens the tree-drift tolerance into a smuggling channel (see native.go field doc)", name)
		}
	}
}

package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestAgyClaudeTmux_TheLiveIndividualQuotaWallIsClassifiedByTheManifestsExhaustedRegex(t *testing.T) {
	pane, err := os.ReadFile(filepath.Join("testdata", "agy-claude-individual-quota-wall.txt"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest("agy-claude-tmux")
	if err != nil {
		t.Fatal(err)
	}

	if !ClassifyExhausted("agy-claude", string(pane)) {
		t.Fatalf("agy-claude-tmux's exhausted_regex %q missed the live wall", manifestExhaustedPattern(m))
	}
	for _, rule := range m.InteractivePrompts {
		if (rule.Name == "rate_limit" || rule.Name == "quota_exhausted") && regexp.MustCompile(rule.Regex).Match(pane) {
			t.Errorf("the %s rule also matches; the wall's classification has one home, the exhausted_regex", rule.Name)
		}
	}
}

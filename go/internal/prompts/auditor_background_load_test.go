package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestAuditorPersona_BackgroundLoadIsBoundedAndStoppedBeforeTheReport(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-auditor.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rule string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "**Background load (cycle 1762).**") {
			rule = line
		}
	}
	for _, want := range []string{"date +%s", "&", "$!", "kill", "before you write `audit-report.md`"} {
		if !strings.Contains(rule, want) {
			t.Errorf("the auditor's background-load rule %q does not name %q: a load loop needs its own deadline, a recorded pid and a kill before the report (cycle 1762 left 14 loops running)", rule, want)
		}
	}
}

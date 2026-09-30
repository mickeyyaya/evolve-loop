//go:build acs

package cycle1014

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokensPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runTokensTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, tokensPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, tokensPkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

func TestC1014_001_tripwire_fires_and_names_cli_agent_cycle(t *testing.T) {
	runTokensTest(t,
		"^TestTokensReport_TripwireFiresOnNonClaudeSuccess$",
		"TestTokensReport_TripwireFiresOnNonClaudeSuccess")
}

func TestC1014_002_tripwire_silent_on_claude_short_and_abort(t *testing.T) {
	runTokensTest(t,
		"^TestTokensReport_TripwireSilentOnClaudeShortAndAbort$",
		"TestTokensReport_TripwireSilentOnClaudeShortAndAbort")
}

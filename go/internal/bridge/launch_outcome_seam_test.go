package bridge

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
)

func TestExitCodes_HostAliasesAreTheLeafValues(t *testing.T) {
	for name, pair := range map[string][2]int{
		"ExitOK":               {ExitOK, launchoutcome.ExitOK},
		"ExitSafetyGate":       {ExitSafetyGate, launchoutcome.ExitSafetyGate},
		"ExitCostLeak":         {ExitCostLeak, launchoutcome.ExitCostLeak},
		"ExitBadFlags":         {ExitBadFlags, launchoutcome.ExitBadFlags},
		"ExitREPLBootTimeout":  {ExitREPLBootTimeout, launchoutcome.ExitREPLBootTimeout},
		"ExitArtifactTimeout":  {ExitArtifactTimeout, launchoutcome.ExitArtifactTimeout},
		"ExitUnknownPrompt":    {ExitUnknownPrompt, launchoutcome.ExitUnknownPrompt},
		"ExitRespondLoopGuard": {ExitRespondLoopGuard, launchoutcome.ExitRespondLoopGuard},
		"ExitRequireFullUnmet": {ExitRequireFullUnmet, launchoutcome.ExitRequireFullUnmet},
		"ExitCmdTimeout":       {ExitCmdTimeout, launchoutcome.ExitCmdTimeout},
		"ExitMissingBinary":    {ExitMissingBinary, launchoutcome.ExitMissingBinary},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s: host %d, leaf %d — the numeric contract is spelled twice on purpose and must agree", name, pair[0], pair[1])
		}
	}
	hostMarker := artifactTimeoutMarker
	if hostMarker != launchoutcome.ArtifactTimeoutMarker {
		t.Errorf("the marker is the leaf's: %q", hostMarker)
	}
	if hostMarker != "artifact-timeout: " {
		t.Errorf("the marker spelling the emitters print: %q", hostMarker)
	}
	for i, pair := range [][2]launchoutcome.TimeoutCause{
		{artifactTimeoutContextCancelled, launchoutcome.TimeoutContextCancelled},
		{artifactTimeoutDetectorError, launchoutcome.TimeoutDetectorError},
		{artifactTimeoutSubmitWedged, launchoutcome.TimeoutSubmitWedged},
		{artifactTimeoutTransientUpstream, launchoutcome.TimeoutTransientUpstream},
		{artifactTimeoutReviewStop, launchoutcome.TimeoutReviewStop},
		{artifactTimeoutReviewPause, launchoutcome.TimeoutReviewPause},
		{artifactTimeoutIncomplete, launchoutcome.TimeoutIncomplete},
	} {
		if pair[0] != pair[1] {
			t.Errorf("cause alias %d: host %q, leaf %q", i, pair[0], pair[1])
		}
	}
	for name, stderr := range launchStderrFixtures {
		if artifactTimeoutSummary(stderr) != launchoutcome.ArtifactTimeoutSummary(stderr) {
			t.Errorf("fixture %s: the facade is the leaf's summary", name)
		}
	}
	if s := artifactTimeoutSummary(launchStderrFixtures["marker-submit-wedged"]); !strings.HasPrefix(s, artifactTimeoutMarker+"cause=submit_wedged") {
		t.Errorf("the facade mines the marker: %q", s)
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// launchoutcome leaf whose source contains needle (sorted, module-relative).
func nonTestSourcesMentioning(t *testing.T, needle string) []string {
	t.Helper()
	return nonTestSourcesWhere(t, func(body string) bool { return strings.Contains(body, needle) })
}

func nonTestSourcesWhere(t *testing.T, matches func(body string) bool) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/bridge/launchoutcome/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if matches(string(body)) {
			hits = append(hits, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	sort.Strings(hits)
	return hits
}

func TestLaunchOutcome_OneClassificationSite(t *testing.T) {
	for needle, want := range map[string][]string{
		"launchoutcome.Classify(":  {"internal/bridge/engine.go"},
		"launchoutcome.CauseCode(": {"internal/bridge/attempt_telemetry.go"},
		"ValidateRequest(req)":     {"internal/adapters/bridge/bridge.go", "internal/bridge/engine.go"},
	} {
		got := nonTestSourcesMentioning(t, needle)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q is spelled by %v, want exactly %v", needle, got, want)
		}
	}
}

func TestTimeoutCauseVocabulary_DriverAndClassifierAgree(t *testing.T) {
	evidence := []artifactTimeoutEvidence{
		{cancellationErr: errCancelledForTest{}},
		{terminalDetectorErrored: true},
		{submitWedged: true},
		{transient: true},
		{reviewAction: ReviewStop},
		{reviewAction: ReviewPause},
		{},
	}
	seen := map[artifactTimeoutCause]bool{}
	for _, ev := range evidence {
		cause := selectArtifactTimeoutCause(ev)
		seen[cause] = true
		line := "[bridge] " + artifactTimeoutDiagnostic{cause: cause, phase: "build", driver: "claude-tmux", artifact: "a.md", waitedS: 1, intervalS: 1, lastReview: "none", liveness: "unknown"}.String() + "\n"
		if got := launchoutcome.CauseCode(ExitArtifactTimeout, line); got != string(cause) {
			t.Errorf("driver cause %q → classifier cause %q; the vocabulary must round-trip", cause, got)
		}
		if !launchoutcome.TimeoutCause(cause).Known() {
			t.Errorf("driver cause %q is not in the classifier's closed set", cause)
		}
	}
	if len(seen) != 7 {
		t.Errorf("the driver selects %d distinct causes, want the seven-token vocabulary", len(seen))
	}
}

type errCancelledForTest struct{}

func (errCancelledForTest) Error() string { return "context canceled" }

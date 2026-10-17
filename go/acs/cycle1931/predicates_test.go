//go:build acs

package cycle1931

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePackage      = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	replayTest       = "TestCycle1637Replay_ShipsOnFirstRecovery"
	divergentTest    = "TestCycle1637Replay_ByteDivergentRebaseDeclinesCarry"
	replayFile       = "carry_chase_replay_test.go"
	carrySource      = "internal/core/identity_carry_forward.go"
	carrySignature   = "func (o *Orchestrator) identityCarryForward(ctx context.Context, cycle int, cs CycleState, base0, projectRoot string) bool {"
	regressionIndex  = "docs/incidents/REGRESSION-COVERAGE-INDEX.md"
	incidentsDirName = "docs/incidents"
)

var (
	replayNamePattern = regexp.MustCompile(`TestCycle1637Replay_[A-Za-z0-9_]+`)
	markdownLink      = regexp.MustCompile(`\]\(([^)\s]+)\)`)
)

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		t.Cleanup(cancel)
	}
	return ctx
}

func runReplayTests(t *testing.T, root string, names []string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: corePackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func preCarryOverlay(t *testing.T, root string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, "go", filepath.FromSlash(carrySource)))
	if err != nil {
		t.Fatalf("RED: read the carry source: %v", err)
	}
	if !strings.Contains(string(src), carrySignature) {
		t.Fatalf("RED: %s no longer declares identityCarryForward with the audited signature", carrySource)
	}
	preCarry := strings.Replace(string(src), carrySignature, carrySignature+"\n\treturn false", 1)
	dir := t.TempDir()
	preCarryFile := filepath.Join(dir, "identity_carry_forward.go")
	if err := os.WriteFile(preCarryFile, []byte(preCarry), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(root, "go", filepath.FromSlash(carrySource)): preCarryFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	overlayFile := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlayFile
}

func testOutcomes(t *testing.T, jsonEvents string) map[string]string {
	t.Helper()
	outcomes := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(jsonEvents))
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var event struct{ Action, Test string }
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Test == "" {
			continue
		}
		if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
			outcomes[event.Test] = event.Action
		}
	}
	return outcomes
}

func incidentRecord(t *testing.T, root string) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, incidentsDirName, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, p := range paths {
		if filepath.Base(p) == filepath.Base(regressionIndex) {
			continue
		}
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "1637") && strings.Contains(text, "1691") && strings.Contains(text, replayTest) {
			found = append(found, p)
		}
	}
	if len(found) != 1 {
		t.Fatalf("RED: want exactly one record under %s naming cycles 1637 and 1691 and %s, found %v", incidentsDirName, replayTest, found)
	}
	return found[0]
}

func replayIndexRow(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(regressionIndex)))
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "|") && strings.Contains(line, replayFile) && strings.Contains(line, replayTest) {
			rows = append(rows, line)
		}
	}
	if len(rows) == 0 {
		t.Fatalf("RED: %s has no row naming %s::%s", regressionIndex, replayFile, replayTest)
	}
	return rows[0]
}

func TestC1931_001_TheCycle1637ReplayShipsOnTheFirstRecoveryAndADivergentRebaseDoesNot(t *testing.T) {
	root := acsassert.RepoRoot(t)
	runReplayTests(t, root, []string{replayTest, divergentTest})
	if !acsassert.FileExists(t, filepath.Join(root, "go", "internal", "core", replayFile)) {
		t.Errorf("RED: the replay lives beside the carry's recovery tests in go/internal/core/%s", replayFile)
	}
}

func TestC1931_002_TheReplayIsRedOnAPreCarryBase(t *testing.T) {
	root := acsassert.RepoRoot(t)
	overlay := preCarryOverlay(t, root)
	out, stderr, _, err := sysexec.Capture(testContext(t), sysexec.DefaultRunner, filepath.Join(root, "go"), "go",
		"test", "-json", "-count=1", "-vet=off", "-overlay", overlay,
		"-run", "^("+replayTest+"|"+divergentTest+")$", "./internal/core")
	if err != nil {
		t.Fatalf("go test under the pre-carry overlay did not run: %v\nstderr:\n%s", err, stderr)
	}
	outcomes := testOutcomes(t, out)
	if got := outcomes[replayTest]; got != "fail" {
		t.Errorf("RED: with identityCarryForward declining every carry (the pre-carry base), %s = %q, want fail: the replay must catch the chase\nstderr:\n%s", replayTest, got, stderr)
	}
	if got := outcomes[divergentTest]; got != "pass" {
		t.Errorf("RED: on the pre-carry base %s = %q, want pass: a divergent rebase never carried, so the overlay must not break it", divergentTest, got)
	}
}

// acs-predicate: config-check
func TestC1931_003_DocsIncidentsHoldsARecordForCycles1637And1691(t *testing.T) {
	root := acsassert.RepoRoot(t)
	record := incidentRecord(t, root)
	rel, err := filepath.Rel(root, record)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", rel)
	}
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"GIT_FLEET_REBASE_NEEDED", replayFile} {
		if !strings.Contains(string(body), marker) {
			t.Errorf("RED: %s does not name %s", rel, marker)
		}
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	if findings := stelint.Check(body, stelint.Options{Words: words}); len(findings) != 0 {
		t.Errorf("RED: %s has %d ste-lint finding(s): %+v", rel, len(findings), findings)
	}
	for _, m := range markdownLink.FindAllStringSubmatch(string(body), -1) {
		target := strings.SplitN(m[1], "#", 2)[0]
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(record), filepath.FromSlash(target))); err != nil {
			t.Errorf("RED: %s links to %s, which does not resolve: %v", rel, m[1], err)
		}
	}
}

func TestC1931_004_TheRegressionIndexRowNamesThePassingReplayAndLinksTheRecord(t *testing.T) {
	root := acsassert.RepoRoot(t)
	row := replayIndexRow(t, root)
	record := incidentRecord(t, root)
	if !strings.Contains(row, "("+filepath.Base(record)+")") {
		t.Errorf("RED: the replay row does not link the incident record %s: %s", filepath.Base(record), row)
	}
	if !strings.Contains(row, "✅") {
		t.Errorf("RED: the replay row is not marked covered: %s", row)
	}
	names := replayNamePattern.FindAllString(row, -1)
	runReplayTests(t, root, dedupe(names))
}

func dedupe(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

//go:build acs

package cycle1849

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	writeDeadlineTest = "TestServer_SSEStreamOutlivesAnyWriteDeadline"
	firstSnapshotTest = "TestServer_SSEFirstSnapshotIDIsNeverRepeated"
	boardStatusTest   = "TestHandleCycle_BoardLaneStatusWins"
)

func dashboardFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "dashboard", name)
}

func mutantOverlay(t *testing.T, name, anchor, replacement string) string {
	t.Helper()
	original := dashboardFile(t, name)
	src, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(anchor)
	if !re.Match(src) {
		t.Fatalf("mutation anchor %q not found in %s: the rule it removes is gone or reshaped", anchor, name)
	}
	dir := t.TempDir()
	mutated := filepath.Join(dir, name)
	if err := os.WriteFile(mutated, re.ReplaceAll(src, []byte(replacement)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {original: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runDashboardTests(t *testing.T, overlay, pattern string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	args := []string{"test", "-count=1", "-v"}
	if overlay != "" {
		args = append(args, "-overlay", overlay)
	}
	if pattern != "" {
		args = append(args, "-run", pattern)
	}
	cmd := exec.CommandContext(ctx, "go", append(args, "./internal/dashboard")...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func requireMutantKilledBy(t *testing.T, overlay, testName string) {
	t.Helper()
	out, err := runDashboardTests(t, overlay, "^"+testName+"$")
	if err == nil {
		t.Fatalf("%s survives the removal of its rule\n%s", testName, out)
	}
	if !strings.Contains(out, "--- FAIL: "+testName+" ") {
		t.Fatalf("the rule's removal did not fail %s itself (absent, or a build error)\n%s", testName, out)
	}
}

func TestC1849_001_ThreePinningTestsRunAndPass(t *testing.T) {
	names := []string{writeDeadlineTest, firstSnapshotTest, boardStatusTest}
	out, err := runDashboardTests(t, "", "^("+strings.Join(names, "|")+")$")
	if err != nil {
		t.Fatalf("the pinning tests fail at HEAD: %v\n%s", err, out)
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass\n%s", name, out)
		}
	}
}

func TestC1849_002_WriteDeadlineOnTheServerFailsTheStreamTest(t *testing.T) {
	overlay := mutantOverlay(t, "server.go", `&http\.Server\{`, "&http.Server{\n\t\tWriteTimeout: 100 * time.Millisecond,")
	requireMutantKilledBy(t, overlay, writeDeadlineTest)
}

func TestC1849_003_DroppingTheEchoFilterRepeatsTheFirstSnapshotID(t *testing.T) {
	overlay := mutantOverlay(t, "sse.go", `if seq <= last \{\s*continue\s*\}`, "")
	requireMutantKilledBy(t, overlay, firstSnapshotTest)
}

func TestC1849_004_DroppingTheBoardOverlayFailsHandleCycleTest(t *testing.T) {
	overlay := mutantOverlay(t, "server.go", `\n\s*cs = withBoardLaneStatus\(cs, snap\.Cycles\)\n`, "\n")
	requireMutantKilledBy(t, overlay, boardStatusTest)
}

func TestC1849_005_ServerAndSSECarryNoProseComments(t *testing.T) {
	// acs-predicate: config-check
	for _, name := range []string{"server.go", "sse.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), dashboardFile(t, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, group := range f.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") {
					t.Errorf("%s still carries the comment %q", name, c.Text)
				}
			}
		}
	}
}

func TestC1849_006_RemovingTheWorkspaceCapFailsTheDashboardSuite(t *testing.T) {
	overlay := mutantOverlay(t, "collect.go", `if len\(ids\) > c\.maxCycles \{\s*ids = ids\[:c\.maxCycles\]\s*\}`, "")
	out, err := runDashboardTests(t, overlay, "")
	if err == nil {
		t.Fatalf("no dashboard test pins the cap on run-workspace cycles\n%s", out)
	}
	if !regexp.MustCompile(`--- FAIL: Test\w*Cap\w* `).MatchString(out) {
		t.Fatalf("the cap's removal failed no cap-named test (a build error or an unrelated failure)\n%s", out)
	}
}

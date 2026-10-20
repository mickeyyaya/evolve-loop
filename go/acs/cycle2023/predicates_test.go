//go:build acs

package cycle2023

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPkg    = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildOut  string
	buildCode int
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-c2023-")
		if err != nil {
			buildCode = -1
			buildOut = err.Error()
			return
		}
		builtBin = filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "build", "-o", builtBin, cmdPkg)
		buildOut, buildCode = stdout+stderr, code
	})
	if buildCode != 0 {
		t.Fatalf("go build %s failed (exit %d):\n%s", cmdPkg, buildCode, buildOut)
	}
	return builtBin
}

const (
	scoutLine = `{"ts":"2026-10-20T00:00:00Z","agent":"scout","phase":"scout","cli":"agy-tmux","model":"balanced","requested_model":"balanced","dispatched_model":"gemini-3.8-flash-high","dispatch_source":"argv","attempt":1,"served_model":"Gemini 3.8 Flash (High)"}`
	buildLine = `{"ts":"2026-10-20T00:01:00Z","agent":"build","phase":"build","cli":"agy-tmux","model":"deep","requested_model":"deep","dispatched_model":"gemini-4-argon-medium","dispatch_source":"argv","attempt":1}`
	auditLine = `{"ts":"2026-10-20T00:02:00Z","agent":"audit","phase":"audit","cli":"claude-tmux","model":"deep","requested_model":"deep","dispatched_model":"claude-opus-5-5","dispatch_source":"argv","attempt":1,"served_model":"claude-opus-5-5"}`
)

func fixtureRoot(t *testing.T, cycle string, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(lines) > 0 {
		body := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(ws, "llm-calls.ndjson"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runModels(t *testing.T, root, cycle string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"cycle", "models", "--project-root", root, "--cycle", cycle}, extra...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), args...)
	return stdout, stderr, code
}

func rowOf(t *testing.T, out, phase string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == phase {
			return line
		}
	}
	t.Errorf("RED: no row for phase %q in output:\n%s", phase, out)
	return ""
}

func TestC2023_001_CycleModelsPrintsTierSelectorAndServedModelPerPhase(t *testing.T) {
	root := fixtureRoot(t, "7", scoutLine, buildLine, auditLine)

	stdout, stderr, code := runModels(t, root, "7")
	if code != 0 {
		t.Fatalf("RED: evolve cycle models exit %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	wants := map[string][]string{
		"scout": {"balanced", "gemini-3.8-flash-high", "Gemini 3.8 Flash (High)"},
		"audit": {"deep", "claude-opus-5-5"},
	}
	for phase, tokens := range wants {
		row := rowOf(t, stdout, phase)
		for _, tok := range tokens {
			if !strings.Contains(row, tok) {
				t.Errorf("RED: %s row %q lacks %q", phase, row, tok)
			}
		}
	}
	if scout := rowOf(t, stdout, "scout"); strings.Contains(scout, "claude-opus-5-5") || strings.Contains(scout, "argon") {
		t.Errorf("RED: scout row carries another phase's model: %q", scout)
	}
}

func TestC2023_002_ALaunchThroughAFakeDriverRecordsTheServedModelOnTheLlmCallsLine(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run",
		"^(TestTryRecord_ServedModelIsTheModelTheTranscriptReportsNotTheTier|TestTryRecord_ServedModelIsTheVerifiedFooterLabel|TestTryRecord_ServedModelStaysEmptyWhenTheCLIReportsNone)$", bridgePkg)
	out := stdout + stderr
	if code != 0 {
		t.Fatalf("RED: bridge recording tests failed (exit %d, err %v)\n%s", code, err, out)
	}
	for _, name := range []string{
		"TestTryRecord_ServedModelIsTheModelTheTranscriptReportsNotTheTier",
		"TestTryRecord_ServedModelIsTheVerifiedFooterLabel",
		"TestTryRecord_ServedModelStaysEmptyWhenTheCLIReportsNone",
	} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func TestC2023_003_RecordsWithoutAServingModelPrintUnreported(t *testing.T) {
	root := fixtureRoot(t, "8", buildLine, auditLine)

	stdout, stderr, code := runModels(t, root, "8")
	if code != 0 {
		t.Fatalf("RED: evolve cycle models exit %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if build := rowOf(t, stdout, "build"); !strings.Contains(build, "unreported") {
		t.Errorf("RED: build row has no served_model and must print unreported: %q", build)
	}
	if audit := rowOf(t, stdout, "audit"); strings.Contains(audit, "unreported") {
		t.Errorf("RED: audit row reports claude-opus-5-5 and must not print unreported: %q", audit)
	}
}

func TestC2023_004_ACycleWithNoCallRecordsExitsOneAndSaysWhy(t *testing.T) {
	root := fixtureRoot(t, "9")

	stdout, stderr, code := runModels(t, root, "9")
	if code != 1 {
		t.Errorf("RED: empty cycle exit %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.Contains(stderr, "unknown subcommand") {
		t.Errorf("RED: models is not a known subcommand: %s", stderr)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Errorf("RED: an empty result must explain itself on stderr")
	}
}

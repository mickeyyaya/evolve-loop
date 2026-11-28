//go:build acs

package cycle2666

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const rationaleSignal = "debugger.rationale"

var jsonFence = regexp.MustCompile("(?s)```json\n(.*?)\n[ \t]*```")

func fencedJSONAfter(t *testing.T, path, anchor string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	at := regexp.MustCompile(regexp.QuoteMeta(anchor)).FindStringIndex(text)
	if at == nil {
		t.Fatalf("RED: anchor %q not found in %s", anchor, path)
	}
	m := jsonFence.FindStringSubmatch(text[at[1]:])
	if m == nil {
		t.Fatalf("RED: no json fence after %q in %s", anchor, path)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(m[1]), &doc); err != nil {
		t.Fatalf("RED: json fence after %q is not valid JSON: %v", anchor, err)
	}
	return doc
}

func TestC2666_001_ParserReadsRationaleAndLegacyReasoning(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(
		filepath.Join(root, "go"), "go", "test", "-count=1", "-run", "TestDecisionRationale", "./internal/phases/debugger")
	if err != nil || code != 0 {
		t.Fatalf("RED: TestDecisionRationale did not pass (exit=%d, err=%v):\n%s%s",
			code, err, acsassert.OutputTail(stdout, 25), acsassert.OutputTail(stderr, 10))
	}
}

func TestC2666_002_PersonaOutputContractIsParsedAsRationale(t *testing.T) {
	root := acsassert.RepoRoot(t)
	contract := fencedJSONAfter(t, filepath.Join(root, "agents", "evolve-debugger.md"), "## Output contract")
	if _, legacy := contract["reasoning"]; legacy {
		t.Errorf("RED: persona output contract still names the legacy field reasoning")
	}
	want, ok := contract["rationale"].(string)
	if !ok || want == "" {
		t.Fatalf("RED: persona output contract names no rationale field: %v", contract)
	}
	contract["action"] = "RESHIP"
	body, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "debug-decision.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	_, signals, _ := debugger.Classify(dir)
	if got := signals[rationaleSignal]; got != want {
		t.Errorf("RED: parser read %q from the persona example, want %q: persona and parser name different fields", got, want)
	}
}

// acs-predicate: config-check
func TestC2666_003_BuildPhaseFailedApproachNamesRootCause(t *testing.T) {
	root := acsassert.RepoRoot(t)
	entry := fencedJSONAfter(t, filepath.Join(root, "skills", "loop", "phase3-build.md"), "under `failedApproaches`")
	if _, legacy := entry["reasoning"]; legacy {
		t.Errorf("RED: failed-approach entry still names the field reasoning")
	}
	if _, ok := entry["root_cause"].(string); !ok {
		t.Errorf("RED: failed-approach entry names no root_cause field: %v", entry)
	}
}

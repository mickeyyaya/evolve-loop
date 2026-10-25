package cycleoutcome

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func failureInputsWithPolicy(t *testing.T, body string) (FailureInputs, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	in := FailureInputsFor(root, evolveDir, t.TempDir(), 7, &stderr)
	return in, stderr.String()
}

func TestFailureInputsFor_ReadsTheRetryCeilingOfAValidPolicy(t *testing.T) {
	in, warn := failureInputsWithPolicy(t, `{"failure_policy": {"thresholds": {"task_retry_ceiling": 5}}}`)

	if in.Ceiling != 5 || warn != "" {
		t.Fatalf("a valid policy sets the ceiling with no warning: ceiling %d warn %q", in.Ceiling, warn)
	}
}

func TestFailureInputsFor_WarnsWhenThePolicyIsRefused(t *testing.T) {
	in, warn := failureInputsWithPolicy(t, `{"failure_policy": {"thresholds": {"task_retry_ceiling": 5}}, "fleet": {"cuont": 3}}`)

	if in.Ceiling != 2 || !strings.Contains(warn, `"fleet.cuont"`) {
		t.Fatalf("a refused policy is loud and the compiled ceiling applies: ceiling %d warn %q", in.Ceiling, warn)
	}
}

func TestFailureInputsFor_WarnsWhenTheFailurePolicyIsInvalid(t *testing.T) {
	in, warn := failureInputsWithPolicy(t, `{"failure_policy": {"thresholds": {"task_retry_ceiling": 5}, "categories": {"x": {"level": "galaxy", "action": "retry-with-fix"}}}}`)

	if in.Ceiling != 2 || !strings.Contains(warn, "failure policy unreadable") || !strings.Contains(warn, "galaxy") {
		t.Fatalf("an invalid failure policy is loud and the compiled ceiling applies: ceiling %d warn %q", in.Ceiling, warn)
	}
}

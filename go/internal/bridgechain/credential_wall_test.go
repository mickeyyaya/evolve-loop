package bridgechain_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

// A fresh escalation report naming a credential wall benches the family and the log names the operator's
// fix, so the next dispatch in the wave routes around the family instead of into the login prompt.
func TestBenchOnEscalation_ACredentialWallBenchesAndNamesTheOperator(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	now := func() time.Time { return time.Date(2026, 9, 26, 22, 40, 0, 0, time.UTC) }
	b, _ := json.Marshal(map[string]any{"captured_at": now(), "cli": "claude-tmux", "pattern_name": clihealth.CredentialPattern, "pane_tail": "Please log in to continue"})
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var logs []string
	bridgechain.BenchOnEscalation(root, ws, "claude-tmux", now().Add(-time.Minute), map[string]string{}, now, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	entry, ok := clihealth.NewStore(root, now).Active()["claude"]
	if !ok || entry.Reason != clihealth.CredentialPattern {
		t.Fatalf("claude must be benched for the credential wall; active=%v", clihealth.NewStore(root, now).Active())
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "benched family claude") || !strings.Contains(logs[0], clihealth.OperatorAction("claude", clihealth.CredentialPattern)) {
		t.Fatalf("the bench line must name the operator's fix; logs=%q", logs)
	}
}

package orchestrator

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestSupply_TheNarrowFenceAdmitsNonGateFilesOfBridgeCoreAndTheInboxLifecycle(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	lane := map[string]string{
		"bridge-driver":     "go/internal/bridge/driver_codex.go",
		"bridge-engine":     "go/internal/bridge/engine.go",
		"bridge-channel":    "go/internal/bridge/channel/supervisor.go",
		"bridge-manifest":   "go/internal/bridge/manifests/claude-tmux.json",
		"core-advisor":      "go/internal/core/advisor/advisor.go",
		"core-continuation": "go/internal/core/continuation_stamp.go",
		"core-resume":       "go/internal/core/resume.go",
		"inbox-lifecycle":   "go/internal/inboxmover/lifecycle/absorb.go",
	}
	gate := map[string]string{
		"bridge-sandbox":     "go/internal/bridge/sandbox_wrap.go",
		"bridge-credential":  "go/internal/bridge/dispatch_credential.go",
		"core-state-machine": "go/internal/core/statemachine.go",
		"core-ship-recovery": "go/internal/core/ship_recovery.go",
		"ledger-chain":       "go/internal/adapters/ledger/ledger.go",
		"policy-parser":      "go/internal/policy/strict_keys.go",
		"claim-refusal":      "go/internal/inboxmover/lifecycle/claim.go",
	}
	for id, file := range lane {
		writeItem(t, inbox, id, map[string]any{"files": []string{file}})
	}
	for id, file := range gate {
		writeItem(t, inbox, id, map[string]any{"files": []string{file}})
	}
	census := Census{
		Inbox:     inboxmover.Options{InboxDir: inbox, Stderr: io.Discard},
		Rank:      inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: epoch},
		Protected: guards.IsProtectedScope,
		Now:       func() time.Time { return epoch },
	}

	got, err := census.Count()

	if err != nil {
		t.Fatal(err)
	}
	if got.Counts[ClassAdmissible] != len(lane) || got.Counts[ClassConsoleProtected] != len(gate) {
		t.Errorf("counts = %v, want %d admissible (the non-gate files) and %d console_protected (the gate files)", got.Counts, len(lane), len(gate))
	}
	refused := map[string]bool{}
	for _, r := range got.Refusals {
		refused[r.ID] = true
	}
	for id := range lane {
		if refused[id] {
			t.Errorf("%s (%s) was refused; ADR-0157 makes a non-gate file lane work", id, lane[id])
		}
	}
	for id := range gate {
		if !refused[id] {
			t.Errorf("%s (%s) was admitted; a gate file stays on the console", id, gate[id])
		}
	}
}

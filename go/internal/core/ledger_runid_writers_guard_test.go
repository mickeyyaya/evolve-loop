package core

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// agentSubprocessWriters is the closed set of files that construct an
// agent_subprocess ledger entry.
var agentSubprocessWriters = map[string]string{
	"internal/core/phase_bindings.go":           "orchestrator bindings — stamped centrally, see centrallyStamped",
	"internal/subagent/subagentrun/ledger.go":   "out-of-process `evolve subagent run` — the cycle-1571 H1 writer, since ADR-0103 unit 16 the dispatcher's ledger step; stamped through a port, see centrallyStamped",
	"internal/subagent/subagent.go":             "in-process Runner.Run",
	"internal/cyclesimulator/cyclesimulator.go": "simulator",
}

// centrallyStamped names writers that append through the Orchestrator's
// stampingLedger (core/runid.go), which stamps every entry for them.
var centrallyStamped = map[string]string{
	"internal/core/phase_bindings.go":         "appends via o.ledger == stampingLedger (core/runid.go stamps run_id)",
	"internal/subagent/subagentrun/ledger.go": "a leaf that cannot import core: its Dispatcher takes the resolver as the Deps.RunID port, which the ONE wired construction in internal/subagent/run.go (wiredDispatcher) binds to core.RunIDFromWorkspace, resolved once after admission; subagentrun/ledger_test.go and resolve_test.go are the behavioural pins (the run id reaches the emitted bytes)",
}

func TestAgentSubprocessWriters_AllStampRunID(t *testing.T) {
	t.Parallel()
	found := scanAgentSubprocessWriters(t)

	for _, rel := range found {
		body := mustReadRepoFile(t, rel)
		// Matching a CALL (the open paren), not the bare identifier, is
		// deliberate: a doc comment merely naming the resolver must not satisfy
		// the requirement or false-red an exemption.
		mentionsRunID := strings.Contains(body, "RunIDFromWorkspace(")
		reason, exempt := centrallyStamped[rel]

		switch {
		case exempt && mentionsRunID:
			t.Errorf("%s is listed in centrallyStamped (%q) but now references run_id directly — "+
				"delist it so the exemption cannot rot into a blanket excuse", rel, reason)
		case !exempt && !mentionsRunID:
			t.Errorf("%s writes agent_subprocess ledger entries but never calls the run-id resolver.\n"+
				"Since PR #503 a run-scoped binding lookup REFUSES an entry with no run_id, so ship "+
				"hard-stops AUDIT_BINDING_NO_AUDITOR on anything this file writes. Populate it via "+
				"core.RunIDFromWorkspace(<run workspace>), or add the file to centrallyStamped "+
				"naming the mechanism that stamps it for you.", rel)
		}
	}
}

func TestAgentSubprocessWriters_SetIsClosed(t *testing.T) {
	t.Parallel()
	found := scanAgentSubprocessWriters(t)
	for _, rel := range found {
		if _, known := agentSubprocessWriters[rel]; !known {
			t.Errorf("%s constructs agent_subprocess ledger entries but is not in agentSubprocessWriters.\n"+
				"Add it, and decide there how it obtains its run id — an unstamped entry is invisible "+
				"to ship's run-scoped binding.", rel)
		}
	}
	for rel := range agentSubprocessWriters {
		if !slices.Contains(found, rel) {
			t.Errorf("agentSubprocessWriters lists %s but it no longer writes agent_subprocess entries — remove it", rel)
		}
	}
}

// scanAgentSubprocessWriters walks the module for non-test .go files whose
// source ASSIGNS kind=agent_subprocess.
func scanAgentSubprocessWriters(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..") // go/internal/core -> go/
	var out []string
	for _, sub := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, sub), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if !writesAgentSubprocessKind(string(b)) {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("scan found NO agent_subprocess writers — the detector is broken, not the tree (a vacuous guard is worse than none)")
	}
	sort.Strings(out)
	return out
}

// writesAgentSubprocessKind reports whether any single line both names the kind
// and assigns it, as opposed to comparing it (readers) or naming it in prose.
func writesAgentSubprocessKind(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, "agent_subprocess") {
			continue
		}
		if strings.Contains(line, "!=") || strings.Contains(line, "==") {
			continue // a comparison — this is a reader
		}
		if strings.Contains(line, "Kind:") || strings.Contains(line, `"kind":`) {
			return true
		}
	}
	return false
}

func mustReadRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

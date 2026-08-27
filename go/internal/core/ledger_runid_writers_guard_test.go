package core

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// ledger_runid_writers_guard_test.go — the DURABLE half of the cycle-1571 H1
// fix. Fixing the three unstamped writers closes today's hole; this guard is
// what stops the fourth from being added silently.
//
// PR #503 made run_id load-bearing: ship's binding lookup and the composition
// snapshot both refuse an auditor entry that does not carry THIS run's id. That
// turned every agent_subprocess writer into a participant in a ship-gate
// contract, but nothing enforced participation — the premise "every current
// recorder stamps run_id" was simply asserted, and was false for three of four
// writers.
//
// The scan is deliberately line-level: a line that ASSIGNS the kind (`Kind:` or
// `"kind":`) writes entries; a line that COMPARES it (`!=`, `==`) merely reads
// them, and readers owe nothing here.
//
// KNOWN LIMIT, stated because it already bit once: this guard proves the
// resolver is CALLED, never that its value reaches the emitted bytes. The first
// attempt at stamping cyclesimulator called it and still emitted nothing —
// jsonCompact drops any key outside its own allowlist — and this test was green
// throughout. Source scanning closes "a writer was added and nobody noticed";
// only a behavioural test over the real writer closes "the value is dropped on
// the way out". Every writer therefore also owes one: subagent/runid_stamp_test.go,
// cyclesimulator/runid_stamp_test.go, core/phase_bindings_fail_verdict_test.go.

// agentSubprocessWriters is the closed set of files that CONSTRUCT an
// agent_subprocess ledger entry. A new writer must be added here deliberately,
// which is the point: the addition is where you decide how it gets its run id.
var agentSubprocessWriters = map[string]string{
	"internal/core/phase_bindings.go":           "orchestrator bindings — stamped centrally, see centrallyStamped",
	"internal/subagent/run.go":                  "out-of-process `evolve subagent run` — the cycle-1571 H1 writer",
	"internal/subagent/subagent.go":             "in-process Runner.Run",
	"internal/cyclesimulator/cyclesimulator.go": "simulator",
}

// centrallyStamped names writers that legitimately do not mention run_id
// themselves because they append through the Orchestrator's stampingLedger,
// which stamps every entry (core/runid.go). SELF-PRUNING: if such a file starts
// referencing the field directly it no longer needs the exemption and this test
// fails until it is delisted, so the list cannot rot into a blanket excuse.
var centrallyStamped = map[string]string{
	"internal/core/phase_bindings.go": "appends via o.ledger == stampingLedger (core/runid.go stamps run_id)",
}

func TestAgentSubprocessWriters_AllStampRunID(t *testing.T) {
	t.Parallel()
	found := scanAgentSubprocessWriters(t)

	for _, rel := range found {
		body := mustReadRepoFile(t, rel)
		// Requiring the RESOLVER, not merely the words "run_id", is deliberate:
		// a writer can declare the field and still never populate it, which is
		// unit-green and live-dark — the same failure class as the composition
		// seam this PR also pins. Naming RunIDFromWorkspace proves the identity
		// is actually obtained at the call site.
		// Match a CALL, not the bare identifier: a doc comment naming the
		// resolver must neither satisfy the requirement nor — for an excepted
		// file — red the exemption. In a codebase this comment-dense that is a
		// live hazard, not a theoretical one.
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

// TestAgentSubprocessWriters_SetIsClosed fails when a NEW file starts writing
// agent_subprocess entries. Without this, the guard above only ever inspects
// files someone remembered to list — the same "asserted, not verified" shape
// that produced H1.
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

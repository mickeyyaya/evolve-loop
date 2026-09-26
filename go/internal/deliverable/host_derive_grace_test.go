package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The agent's final write is not atomic with the host's read after it: the derivation waits out a write in
// flight, as the gate does, so a decision the agent is landing is never replaced by a derived one.
func TestHostEffects_WaitsOutTheAgentsWriteInFlight(t *testing.T) {
	const written = `{"top_n":[{"id":"chosen-by-the-agent"}]}`
	in := triageInput(t, "/project", "", "")
	writeFile(t, in.Workspace, "triage-report.md", triageReport)
	decision := filepath.Join(in.Workspace, "triage-decision.json")
	saved := graceSleep
	graceSleep = func(time.Duration) { writeFile(t, in.Workspace, "triage-decision.json", written) }
	t.Cleanup(func() { graceSleep = saved })
	var calls []claimCall

	err := NewHostEffects(derivingCatalog(t), recordingClaimer(&calls)).Perform(context.Background(), in)

	if err != nil {
		t.Fatalf("Perform: %v", err)
	}
	if got, _ := os.ReadFile(decision); string(got) != written {
		t.Fatalf("the agent's landing decision was replaced: %s", got)
	}
}

// A decision that is present but cannot be read is not absent: the host declines rather than overwrite it.
func TestHostEffects_DeclinesWhenTheDecisionIsPresentButUnreadable(t *testing.T) {
	in := triageInput(t, "/project", "", "")
	writeFile(t, in.Workspace, "triage-report.md", triageReport)
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	decision := filepath.Join(in.Workspace, "triage-decision.json")
	writeFile(t, in.Workspace, "triage-decision.json", `{"top_n":[{"id":"sealed"}]}`)
	if err := os.Chmod(decision, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(decision, 0o644) })
	var calls []claimCall

	err := NewHostEffects(derivingCatalog(t), recordingClaimer(&calls)).Perform(context.Background(), in)

	if err == nil || !strings.Contains(err.Error(), "derive triage-decision.json") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("want a loud decline naming the read fault, got %v", err)
	}
	if cerr := os.Chmod(decision, 0o644); cerr != nil {
		t.Fatal(cerr)
	}
	if got, _ := os.ReadFile(decision); string(got) != `{"top_n":[{"id":"sealed"}]}` {
		t.Fatalf("the unreadable decision was replaced: %s", got)
	}
}

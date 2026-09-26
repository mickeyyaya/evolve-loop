package phasecmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// projectWithRegistryTriage copies the checked-in triage entry, so the fixture cannot drift from the real declaration.
func projectWithRegistryTriage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Phases []json.RawMessage `json:"phases"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatal(err)
	}
	var triage json.RawMessage
	for _, p := range reg.Phases {
		var head struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(p, &head) == nil && head.Name == "triage" {
			triage = p
		}
	}
	if triage == nil {
		t.Fatal("checked-in registry has no triage entry")
	}
	project := t.TempDir()
	regDir := filepath.Join(project, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":4,"phases":[` + string(triage) + `]}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestPhaseVerify_DeclaredEffectFollowsCycleState(t *testing.T) {
	project := projectWithRegistryTriage(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	ws := filepath.Join(project, ".evolve", "runs", "cycle-7")
	inbox := filepath.Join(project, ".evolve", "inbox")
	for _, d := range []string{ws, inbox} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(ws, "triage-report.md", "# Triage\n\n## top_n\n- x: fix it — priority=H, files=a.go, source=inbox\n")
	write(ws, "triage-decision.json", `{"cycle":7,"top_n":[{"id":"x"}],"deferred":[]}`)
	write(ws, core.RunStateFile, `{"cycle_id":7,"phase":"triage"}`)
	write(inbox, "2026-09-12T00-00-00Z-x.json", `{"id":"x","title":"fixture"}`)

	if code, _, errb := runVerify(t, "triage", "--workspace="+ws); code != 1 || !strings.Contains(errb, "missing_effect") {
		t.Fatalf("unclaimed commitment: exit=%d stderr=%q — the self-check must demand the claim the gate demands", code, errb)
	}
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: project, Stderr: io.Discard}, "x", "7"); err != nil {
		t.Fatal(err)
	}
	if code, _, errb := runVerify(t, "triage", "--workspace="+ws); code != 0 {
		t.Fatalf("claimed commitment must verify OK: exit=%d stderr=%q", code, errb)
	}
	if err := os.Remove(filepath.Join(ws, core.RunStateFile)); err != nil {
		t.Fatal(err)
	}
	if code, out, errb := runVerify(t, "triage", "--workspace="+ws); code != 2 || strings.Contains(out, "OK:") || !strings.Contains(errb, core.RunStateFile) {
		t.Fatalf("unreadable state with a declared effect: exit=%d stdout=%q stderr=%q — want exit 2 naming the state file", code, out, errb)
	}
}

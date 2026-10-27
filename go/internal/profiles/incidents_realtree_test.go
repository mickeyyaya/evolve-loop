package profiles

import "testing"

const incidentsDir = ".evolve/incidents"

func TestNoAgentProfileCanWriteTheIncidentRecords(t *testing.T) {
	if writers := agentWritable(realSandboxes(t), incidentsDir); len(writers) > 0 {
		t.Fatalf("profiles %v can write %s: only the incident detector writes the records (ADR-0155, llm-isolation.md §18.2)", writers, incidentsDir)
	}
	if writers := agentWritable(realSandboxes(t), incidentsDir+"/inc-2026-10-27-0a1b2c3d.json"); len(writers) > 0 {
		t.Fatalf("profiles %v can write a record under %s", writers, incidentsDir)
	}
}

func TestNoAgentProfileCanWriteTheIncidentRecords_TheCheckSeesAWideGrant(t *testing.T) {
	t.Parallel()
	for _, grant := range []string{".evolve", ".evolve/*", ".evolve/incidents/inc-x.json"} {
		if !sandboxWrites(SandboxConfig{WriteSubpaths: []string{grant}}, incidentsDir) {
			t.Errorf("a grant of %s reaches the incident records; the check must see it", grant)
		}
	}
}

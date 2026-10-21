package pipelinerates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureCycle struct {
	id                int
	runShipped        bool
	shipLanded        bool
	seal              string
	codes             []string
	item, commit      string
	finalVerdictInRun string
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func signalLine(kind, code, reason string) string {
	return fmt.Sprintf(`{"schema_version":"signal/1.0","seq":1,"ts":"2026-10-21T10:00:00Z","module":"orchestrator","kind":%q,"code":%q,"severity":"INFO","origin":"Fixture.emit","reason":%q,"fields":{"termination_reason":%q}}`, kind, code, reason, reason)
}

func writeCycle(t *testing.T, root string, c fixtureCycle) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", c.id))
	verdict := c.finalVerdictInRun
	if verdict == "" {
		verdict = "PASS"
	}
	writeJSON(t, filepath.Join(dir, "run.json"), map[string]any{"final_verdict": verdict, "shipped": c.runShipped})
	if c.item != "" {
		writeJSON(t, filepath.Join(dir, "triage-decision.json"), map[string]any{"top_n": []map[string]string{{"id": c.item}}})
	}
	if c.commit != "" {
		writeJSON(t, filepath.Join(dir, "ship-binding.json"), map[string]string{"commit_sha": c.commit})
	}
	var lines []string
	if c.shipLanded {
		lines = append(lines, signalLine("ship.landed", "", "landed"))
	}
	for _, code := range c.codes {
		lines = append(lines, signalLine("system.failure", code, "host"))
	}
	if c.seal != "" {
		lines = append(lines, signalLine("cycle.sealed", "", c.seal))
	}
	if err := os.WriteFile(filepath.Join(dir, "signals.ndjson"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeWave(t *testing.T, root string, number, floor int) {
	t.Helper()
	writeJSON(t, filepath.Join(root, ".evolve", "waves", fmt.Sprintf("wave-%d.json", number)), map[string]int{"number": number, "cycle_floor": floor})
}

func wavesFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for n, floor := range map[int]int{137: 2008, 138: 2013, 139: 2019, 140: 2024} {
		writeWave(t, root, n, floor)
	}
	shipped := map[int]bool{2009: true, 2011: true, 2014: true, 2021: true, 2023: true, 2025: true, 2027: true, 2028: true}
	for id := 2008; id <= 2028; id++ {
		c := fixtureCycle{id: id, seal: "final verdict PASS"}
		if shipped[id] {
			c.shipLanded, c.commit = true, fmt.Sprintf("sha%d", id)
		}
		writeCycle(t, root, c)
	}
	writeCycle(t, root, fixtureCycle{id: 2024, finalVerdictInRun: "PASS", seal: "cycle aborted in phase retro (abnormal-exit epilogue)"})
	writeCycle(t, root, fixtureCycle{id: 2010})
	writeCycle(t, root, fixtureCycle{id: 2012, seal: "stopped: lane deadline"})
	writeCycle(t, root, fixtureCycle{id: 2015, seal: "final verdict FAIL", codes: []string{"BRIDGE_EXIT_REPL_BOOT_TIMEOUT"}})
	return root
}

package phaseobserver_test

// defaults_pin_test.go — ADR-0103 unit 12 step 0: the two owners of the
// observer's default thresholds are the host's zero-value defaults (Run) and
// policy's compiled ObserverConfig() (the manual subcommand dereferences the
// latter into Config). They agree on PollS 5 / StallS 600; NudgeS is the NAMED
// divergence (policy 300 — the subcommand nudges because phasecmd feeds the
// policy value; host 0 — a bare Run never nudges); EOFGraceS 10 is host-only
// (policy leaves it 0). An external package so the pin can import policy
// without a cycle. Kills M19 (a host default drifting from policy's).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseobserver"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestWithDefaults_MatchPolicyCompiledDefaults(t *testing.T) {
	t.Parallel()
	compiled := policy.Policy{}.ObserverConfig()
	if *compiled.PollS != 5 || *compiled.StallS != 600 || *compiled.NudgeS != 300 || compiled.EOFGraceS != 0 {
		t.Fatalf("policy's compiled observer defaults moved: %+v", compiled)
	}
	ws := t.TempDir()
	t0 := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	calls := 0
	rc := phaseobserver.Run(phaseobserver.Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		// Every tunable zero → the host's defaults apply; the clock jumps 400 s
		// (past policy's NudgeS 300) so a defaulted-on nudge would show.
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls <= 2 {
				return t0
			}
			return t0.Add(400 * time.Second)
		},
		StopAfterMS: 200,
	}, "", os.Stderr)
	if rc != phaseobserver.ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var started map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &started); err != nil {
		t.Fatal(err)
	}
	data, _ := started["data"].(map[string]any)
	if started["type"] != "observer_started" || int(data["poll_s"].(float64)) != *compiled.PollS || int(data["stall_s"].(float64)) != *compiled.StallS {
		t.Errorf("the host's zero-value defaults must equal policy's compiled PollS/StallS (5/600): %v", started)
	}
	if strings.Contains(string(raw), "soft_stall_nudge") {
		t.Errorf("a bare Run defaults NudgeS to 0 (nudge OFF) — the named divergence from policy's 300:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(ws, ".bridge-inbox")); !os.IsNotExist(err) {
		t.Errorf("no inbox is written when the nudge is off, stat err=%v", err)
	}
}

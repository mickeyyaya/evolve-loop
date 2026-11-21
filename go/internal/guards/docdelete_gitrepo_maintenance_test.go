package guards

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type traceEvent struct {
	Event string   `json:"event"`
	Argv  []string `json:"argv"`
}

func spawnedMaintenance(t *testing.T, tracePath string) [][]string {
	t.Helper()
	f, err := os.Open(tracePath)
	if err != nil {
		t.Fatalf("trace file: %v", err)
	}
	defer f.Close()
	var found [][]string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		var ev traceEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Event != "child_start" {
			continue
		}
		if slices.Contains(ev.Argv, "maintenance") || slices.Contains(ev.Argv, "gc") {
			found = append(found, ev.Argv)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return found
}

func TestGitRepo_CommitStartsNoDetachedMaintenanceWriter(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_TRACE2_EVENT", tracePath)

	gitRepo(t, map[string]string{"docs/a.md": "a committed doc\n"})

	if got := spawnedMaintenance(t, tracePath); len(got) != 0 {
		t.Fatalf("gitRepo's commit started background git writers that outlive the test: %v", got)
	}
}

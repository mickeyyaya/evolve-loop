package main

// cmd_inbox_quarantine_test.go — the coverage-gate CRITICAL remediation for
// cycle-1019 (its report prescribed exactly these three groups): the
// `evolve inbox quarantine` operator surface, the isTaskLevelFailure
// classification the S5 quarantine decision hinges on, and the runInbox
// dispatch wiring. Salvaged console-first from the preserved worktree.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
)

func seedQuarantineItem(t *testing.T, root, id string) string {
	t.Helper()
	qDir := filepath.Join(root, ".evolve", "inbox", "quarantine")
	if err := os.MkdirAll(qDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"id": id, "title": "poison todo " + id, "weight": 0.5,
		"failure_count": 3, "last_failure_reason": "code-audit-fail x3",
	})
	p := filepath.Join(qDir, id+".json")
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runQuarantineCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	code := runInboxQuarantine(args, nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunInboxQuarantine_ListEmptyPopulatedAndJSON(t *testing.T) {
	root := t.TempDir()
	// Empty (dir absent): zero items, exit 0.
	if code, out, _ := runQuarantineCLI(t, root, "list"); code != 0 || !strings.Contains(out, "0 quarantined") {
		t.Fatalf("empty list: code=%d out=%q", code, out)
	}
	seedQuarantineItem(t, root, "poison-alpha")
	code, out, _ := runQuarantineCLI(t, root, "list")
	if code != 0 || !strings.Contains(out, "poison-alpha") {
		t.Fatalf("populated list: code=%d out=%q", code, out)
	}
	// --json emits a decodable array carrying the item.
	code, out, _ = runQuarantineCLI(t, root, "list", "--json")
	if code != 0 {
		t.Fatalf("json list: code=%d", code)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(out), &items); err != nil || len(items) != 1 {
		t.Fatalf("json list not decodable single item: %v %q", err, out)
	}
	// Unknown flag is a usage error.
	if code, _, eb := runQuarantineCLI(t, root, "list", "--nope"); code != 10 || !strings.Contains(eb, "unknown arg") {
		t.Fatalf("unknown list arg: code=%d stderr=%q", code, eb)
	}
}

func TestRunInboxQuarantine_ReleaseSuccessAndMissing(t *testing.T) {
	root := t.TempDir()
	seedQuarantineItem(t, root, "poison-beta")
	code, out, eb := runQuarantineCLI(t, root, "release", "poison-beta")
	if code != 0 {
		t.Fatalf("release: code=%d stderr=%q", code, eb)
	}
	if !strings.Contains(out, "released poison-beta") {
		t.Fatalf("release output: %q", out)
	}
	// The item is back at the inbox root with its failure count reset.
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "poison-beta.json"))
	if err != nil {
		t.Fatalf("released item not at inbox root: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if fc, _ := m["failure_count"].(float64); fc != 0 {
		t.Fatalf("failure_count not reset on release: %v", m["failure_count"])
	}
	// Missing id fails loudly, exit 1.
	if code, _, eb := runQuarantineCLI(t, root, "release", "no-such-id"); code != 1 || eb == "" {
		t.Fatalf("missing id: code=%d stderr=%q", code, eb)
	}
	// Empty id is a usage error.
	if code, _, _ := runQuarantineCLI(t, root, "release"); code != 10 {
		t.Fatalf("bare release: code=%d", code)
	}
}

func TestRunInboxQuarantine_UsageAndUnknown(t *testing.T) {
	root := t.TempDir()
	if code, _, eb := runQuarantineCLI(t, root); code != 10 || !strings.Contains(eb, "usage") {
		t.Fatalf("no args: code=%d stderr=%q", code, eb)
	}
	if code, _, eb := runQuarantineCLI(t, root, "obliterate"); code != 10 || !strings.Contains(eb, "unknown subcommand") {
		t.Fatalf("unknown sub: code=%d stderr=%q", code, eb)
	}
}

// TestIsTaskLevelFailure_AllClassifications pins AC4 (S3 precedence): ONLY
// genuine per-task defect classes quarantine; infrastructure and
// system/kernel classes never do — they take the S3 halt path instead.
func TestIsTaskLevelFailure_AllClassifications(t *testing.T) {
	cases := []struct {
		c    cycleclassify.Classification
		want bool
	}{
		{cycleclassify.ClassBuildFail, true},
		{cycleclassify.ClassAuditFail, true},
		{cycleclassify.ClassShipGateConfig, true},
		{cycleclassify.ClassInfrastructure, false},
		{cycleclassify.ClassIntegrityBreach, false},
	}
	for _, tc := range cases {
		if got := isTaskLevelFailure(tc.c); got != tc.want {
			t.Errorf("isTaskLevelFailure(%v)=%v, want %v", tc.c, got, tc.want)
		}
	}
}

// TestRunInbox_DispatchesQuarantine pins the ONLY wiring between the CLI and
// runInboxQuarantine.
func TestRunInbox_DispatchesQuarantine(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, eb bytes.Buffer
	if code := runInbox([]string{"quarantine", "list"}, nil, &out, &eb); code != 0 || !strings.Contains(out.String(), "quarantined") {
		t.Fatalf("dispatch: code=%d out=%q stderr=%q", code, out.String(), eb.String())
	}
}

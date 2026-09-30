package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writePendingItem(t *testing.T, evolveDir, name, body string) string {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func withProjectRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
}

// The fixture's kind is "pipeline-repair": kind:"pipeline-defect" matches ZERO
// live items, so gating on it would pass a synthetic fixture and never fire
// in production.
func TestRunInbox_Consume_MovesItemAndAcksFingerprint(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	name := "2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json"
	itemPath := writePendingItem(t, evolveDir, name,
		`{"id":"pipeline-blocker","kind":"pipeline-repair","consumed_by":"`+realConsumedByNarrative+`"}`)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume", itemPath}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	if _, err := os.Stat(itemPath); err == nil {
		t.Error("the item must LEAVE the pending inbox — a consume that copies leaves the item drawable by a lane")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "consumed", name)); err != nil {
		t.Fatalf("the item must land in .evolve/inbox/consumed/: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("consumption must ack the fingerprint in the SAME transaction, with no manual `evolve inbox ack-fingerprint` step: %v", err)
	}
	if !strings.Contains(string(raw), incidentFingerprint) {
		t.Fatalf("ledger must carry the consumed item's fingerprint, got %s", raw)
	}
}

func TestRunInbox_Consume_ItemWithoutFingerprintStillMoves(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	itemPath := writePendingItem(t, evolveDir, "plain-feature.json",
		`{"id":"plain-feature","kind":"feature","consumed_by":"console: shipped in #415"}`)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume", itemPath}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("an item with no fingerprint is a normal consumption, not an error: rc=%d stderr=%q", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "consumed", "plain-feature.json")); err != nil {
		t.Fatalf("the item must still land in consumed/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "resolved-fingerprints.json")); err == nil {
		t.Error("no fingerprint parsed ⇒ no ledger record; the ledger must never accumulate empty/garbage entries")
	}
}

func TestRunInbox_Consume_MissingItemReturnsNonZero(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")

	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"consume", filepath.Join(evolveDir, "inbox", "ghost.json")}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatal("a missing item path must exit non-zero, never silently succeed")
	}
	// The failure must be about the ITEM, not about an unrecognised
	// subcommand — otherwise this predicate passes on a tree where `consume`
	// was never registered at all.
	if !strings.Contains(stderr.String(), "ghost.json") {
		t.Errorf("the error must name the item that could not be read, not fall through to subcommand usage; stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "resolved-fingerprints.json")); err == nil {
		t.Error("a failed consume must write no ledger record")
	}
}

func TestRunInbox_Consume_NoArgReturnsUsage(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume"}, nil, &stdout, &stderr); rc == 0 {
		t.Fatal("`evolve inbox consume` with no item path must exit non-zero")
	}
	if !strings.Contains(stderr.String(), "consume") {
		t.Errorf("usage must name the subcommand; stderr=%q", stderr.String())
	}
}

// consumedStamp mirrors the shape continuation_release.go already reads
// (`{"consumed": {"cycle": ...}}`) plus the sibling fields this task adds.
type consumedStamp struct {
	At         string `json:"at"`
	Via        string `json:"via"`
	Cycle      string `json:"cycle"`
	Resolution string `json:"resolution"`
}

func readConsumedStamp(t *testing.T, path string) consumedStamp {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading moved item %s: %v", path, err)
	}
	var doc struct {
		Consumed consumedStamp `json:"consumed"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("moved item %s is not valid JSON after consume: %v\nbody=%s", path, err, raw)
	}
	return doc.Consumed
}

// TestRunInbox_Consume_WithFlagsStampsConsumedRecord is AC1: passing
// --resolution and --cycle must leave the moved item carrying a
// consumed{at, via, cycle, resolution} stamp with those exact values, and
// the caller-passed via must override the "console-manual" default.
func TestRunInbox_Consume_WithFlagsStampsConsumedRecord(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	name := "2026-09-29T16-00-00Z-flagged-item.json"
	itemPath := writePendingItem(t, evolveDir, name, `{"id":"flagged-item","kind":"feature"}`)

	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{
		"consume", itemPath,
		"--resolution", "shipped in #999",
		"--cycle", "1776",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	dest := filepath.Join(evolveDir, "inbox", "consumed", name)
	stamp := readConsumedStamp(t, dest)
	if stamp.At == "" {
		t.Error("consumed.at must be non-empty")
	}
	if _, err := time.Parse(time.RFC3339, stamp.At); err != nil {
		t.Errorf("consumed.at must be RFC3339 UTC, got %q: %v", stamp.At, err)
	}
	if stamp.Via != "console-manual" {
		t.Errorf("consumed.via default = %q, want %q", stamp.Via, "console-manual")
	}
	if stamp.Cycle != "1776" {
		t.Errorf("consumed.cycle = %q, want %q", stamp.Cycle, "1776")
	}
	if stamp.Resolution != "shipped in #999" {
		t.Errorf("consumed.resolution = %q, want %q", stamp.Resolution, "shipped in #999")
	}
}

// TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp is the no-flag path of
// AC1: a plain `evolve inbox consume <item>` (no --resolution/--cycle) must
// still stamp the moved item, defaulting via to "console-manual", cycle to
// "console", and resolution to "".
func TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	name := "2026-09-29T16-00-01Z-noflag-item.json"
	itemPath := writePendingItem(t, evolveDir, name, `{"id":"noflag-item","kind":"feature"}`)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume", itemPath}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	dest := filepath.Join(evolveDir, "inbox", "consumed", name)
	stamp := readConsumedStamp(t, dest)
	if stamp.At == "" {
		t.Error("consumed.at must be non-empty even with no flags")
	}
	if stamp.Via != "console-manual" {
		t.Errorf("consumed.via default = %q, want %q", stamp.Via, "console-manual")
	}
	if stamp.Cycle != "console" {
		t.Errorf("consumed.cycle default = %q, want %q", stamp.Cycle, "console")
	}
	if stamp.Resolution != "" {
		t.Errorf("consumed.resolution default = %q, want empty", stamp.Resolution)
	}
}

// TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing is the
// negative/rejection test (adversarial-testing SKILL §6): flags must not
// bypass the existing missing-item error path, and a failed consume must
// never leave a stamped file behind anywhere.
func TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")

	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{
		"consume", filepath.Join(evolveDir, "inbox", "ghost.json"),
		"--resolution", "shipped in #999",
		"--cycle", "1776",
	}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatal("a missing item path must exit non-zero even when --resolution/--cycle are passed")
	}
	if !strings.Contains(stderr.String(), "ghost.json") {
		t.Errorf("the error must name the missing item; stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "consumed")); err == nil {
		t.Error("a failed consume must not create the consumed/ directory")
	}
}

func TestRunInbox_Consume_StampEdgeCases(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		flags       func(item string) []string
		wantRC      int
		wantMoved   bool
		wantCycle   string
		wantKeptKey string
	}{
		{
			name:   "flags before the item path",
			body:   `{"id":"a","kind":"feature"}`,
			flags:  func(item string) []string { return []string{"--cycle", "42", "--resolution", "done", item} },
			wantRC: 0, wantMoved: true, wantCycle: "42",
		},
		{
			name:   "unrelated fields survive the stamp verbatim",
			body:   `{"id":"b","weight":0.35,"acceptance":["x"]}`,
			flags:  func(item string) []string { return []string{item} },
			wantRC: 0, wantMoved: true, wantCycle: "console", wantKeptKey: "acceptance",
		},
		{
			name:   "non-numeric cycle is refused before the move",
			body:   `{"id":"c"}`,
			flags:  func(item string) []string { return []string{item, "--cycle", "17x"} },
			wantRC: 10,
		},
		{
			name:   "negative cycle is refused before the move",
			body:   `{"id":"d"}`,
			flags:  func(item string) []string { return []string{item, "--cycle", "-3"} },
			wantRC: 10,
		},
		{
			name:   "stray positional argument is refused",
			body:   `{"id":"e"}`,
			flags:  func(item string) []string { return []string{item, "extra"} },
			wantRC: 10,
		},
		{
			name:   "malformed item stays pending",
			body:   `not json`,
			flags:  func(item string) []string { return []string{item, "--resolution", "done"} },
			wantRC: 1,
		},
		{
			name:   "JSON array item stays pending",
			body:   `["not","an","object"]`,
			flags:  func(item string) []string { return []string{item} },
			wantRC: 1,
		},
		{
			name:   "JSON null item stays pending",
			body:   `null`,
			flags:  func(item string) []string { return []string{item} },
			wantRC: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			withProjectRoot(t, root)
			evolveDir := filepath.Join(root, ".evolve")
			name := "edge-item.json"
			itemPath := writePendingItem(t, evolveDir, name, tc.body)

			var stdout, stderr bytes.Buffer
			rc := runInbox(append([]string{"consume"}, tc.flags(itemPath)...), nil, &stdout, &stderr)
			if rc != tc.wantRC {
				t.Fatalf("rc=%d want %d; stderr=%q", rc, tc.wantRC, stderr.String())
			}
			dest := filepath.Join(evolveDir, "inbox", "consumed", name)
			_, destErr := os.Stat(dest)
			_, srcErr := os.Stat(itemPath)
			if !tc.wantMoved {
				if destErr == nil || srcErr != nil {
					t.Fatalf("a refused consume must leave the item pending (src err=%v, dest err=%v)", srcErr, destErr)
				}
				raw, _ := os.ReadFile(itemPath)
				if string(raw) != tc.body {
					t.Errorf("a refused consume must not rewrite the pending item; got %q", raw)
				}
				return
			}
			if destErr != nil || srcErr == nil {
				t.Fatalf("item must be moved (src err=%v, dest err=%v)", srcErr, destErr)
			}
			if got := readConsumedStamp(t, dest).Cycle; got != tc.wantCycle {
				t.Errorf("consumed.cycle = %q, want %q", got, tc.wantCycle)
			}
			if tc.wantKeptKey != "" {
				var got, want map[string]any
				raw, _ := os.ReadFile(dest)
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
					t.Fatal(err)
				}
				for k, v := range want {
					gb, _ := json.Marshal(got[k])
					wb, _ := json.Marshal(v)
					if string(gb) != string(wb) {
						t.Errorf("field %q = %s, want %s", k, gb, wb)
					}
				}
			}
		})
	}
}

func TestRunInbox_Consume_ReconsumeReplacesEarlierStamp(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	name := "reconsumed.json"
	itemPath := writePendingItem(t, evolveDir, name,
		`{"id":"r","consumed":{"at":"2020-01-01T00:00:00Z","via":"old","cycle":"1","resolution":"stale"}}`)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume", itemPath, "--cycle", "7", "--resolution", "fresh"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	stamp := readConsumedStamp(t, filepath.Join(evolveDir, "inbox", "consumed", name))
	if stamp.Via != "console-manual" || stamp.Cycle != "7" || stamp.Resolution != "fresh" || stamp.At == "2020-01-01T00:00:00Z" {
		t.Errorf("re-consume must replace the earlier stamp; got %+v", stamp)
	}
}

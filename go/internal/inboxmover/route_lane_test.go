package inboxmover

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func routedFields(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func routeLaneRoot(t *testing.T, body string) (Options, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, "fix.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{ProjectRoot: root, Stderr: os.Stderr}, path
}

func TestRouteLane_LetsALaneClaimAPipelineRepairItem(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "fix.json"), []byte(`{"id":"fix","kind":"pipeline-repair"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr}
	if _, err := Claim(opts, "fix", "1779"); !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("Claim before the route: err = %v, want ErrConsoleRouted", err)
	}

	if _, err := RouteLane(opts, "fix", "operator"); err != nil {
		t.Fatalf("RouteLane: %v", err)
	}

	if _, err := Claim(opts, "fix", "1780"); err != nil {
		t.Errorf("Claim after the route: %v, want the lane to take the item", err)
	}
}

func TestRouteLane_JudgesWithTheOptionsLanePredicate(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "fix.json"), []byte(`{"id":"fix","kind":"pipeline-repair","files":["go/internal/guards/phase.go"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr, IsProtectedPath: func(p string) bool { return p == "go/internal/guards/phase.go" }}

	_, err := RouteLane(opts, "fix", "operator")

	if !errors.Is(err, ErrConsoleRouted) {
		t.Errorf("err = %v, want ErrConsoleRouted: the claim floor's own predicate refuses a declared protected file", err)
	}
}

func TestRouteLane_KeepsTheReasonOfAnEarlierRoute(t *testing.T) {
	opts, path := routeLaneRoot(t, `{"id":"fix","kind":"pipeline-repair","route":"console-manual",`+
		`"routed_reason":"needs the console's judgement","routed_cycle":12,"routed_at":"2026-10-01T00:00:00Z"}`)

	if _, err := RouteLane(opts, "fix", "lane work after all"); err != nil {
		t.Fatal(err)
	}

	fields := routedFields(t, path)
	var reason string
	if err := json.Unmarshal(fields["routed_reason"], &reason); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reason, "lane work after all") || !strings.Contains(reason, "needs the console's judgement") {
		t.Errorf("routed_reason = %q, want the new reason and the earlier one", reason)
	}
	if _, has := fields["routed_cycle"]; has {
		t.Error("routed_cycle stayed on a lane-routed item")
	}
}

func TestRouteLane_AFirstRouteStoresTheReasonAsGiven(t *testing.T) {
	for name, body := range map[string]string{
		"no earlier reason":   `{"id":"fix","kind":"pipeline-repair"}`,
		"a blank earlier one": `{"id":"fix","kind":"pipeline-repair","routed_reason":"  "}`,
		"a non-text earlier":  `{"id":"fix","kind":"pipeline-repair","routed_reason":7}`,
	} {
		opts, path := routeLaneRoot(t, body)

		if _, err := RouteLane(opts, "fix", "lane work"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if got := string(routedFields(t, path)["routed_reason"]); got != `"lane work"` {
			t.Errorf("%s: routed_reason = %s, want the reason as given", name, got)
		}
	}
}

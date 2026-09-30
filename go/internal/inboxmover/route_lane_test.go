package inboxmover

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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

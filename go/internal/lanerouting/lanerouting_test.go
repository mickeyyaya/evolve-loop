package lanerouting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func writeProfile(t *testing.T, root, name string, deny ...string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"name": name, "sandbox": map[string]any{"enabled": true, "deny_subpaths": deny}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestForbidden_ForbidsSandboxDeniedPathsAndProtectedSurface(t *testing.T) {
	root := t.TempDir()
	writeProfile(t, root, "writer", ".evolve/profiles")
	forbidden, err := Forbidden(root, "writer")
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		".evolve/profiles/historian.json": true,
		"go/internal/guards/role.go":      true,
		"go/internal/fleet/fleet.go":      false,
		"docs/architecture/x.md":          false,
	} {
		if got := forbidden(path); got != want {
			t.Errorf("forbidden(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestForbidden_FollowsTheProfileNotACopy(t *testing.T) {
	root := t.TempDir()
	writeProfile(t, root, "writer")
	forbidden, err := Forbidden(root, "writer")
	if err != nil {
		t.Fatal(err)
	}
	if forbidden(".evolve/profiles/historian.json") {
		t.Fatal("with no deny list in the profile, routing must not forbid the path")
	}
}

func TestForbidden_AProfileWithoutASandboxAddsNothingToProtectedSurface(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "writer.json"), []byte(`{"name":"writer"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	forbidden, err := Forbidden(root, "writer")
	if err != nil {
		t.Fatal(err)
	}
	if forbidden(".evolve/profiles/historian.json") || !forbidden("go/internal/guards/role.go") {
		t.Fatal("with no sandbox block nothing is sandbox-denied; protected surface still is")
	}
}

func TestForbidden_WithoutTheProfileIsAnError(t *testing.T) {
	if _, err := Forbidden(t.TempDir(), "writer"); err == nil {
		t.Fatal("a missing source-writing profile must be an error, never a silent pass")
	}
}

func TestForbidden_RoutesAnItemThatNeedsASandboxDeniedFileToTheConsole(t *testing.T) {
	root := t.TempDir()
	writeProfile(t, root, "writer", ".evolve/profiles")
	forbidden, err := Forbidden(root, "writer")
	if err != nil {
		t.Fatal(err)
	}
	item := inboxbatch.Item{ID: "historian", Files: []string{".evolve/profiles/historian.json (new)", "go/cmd/evolve/cmd_loop_historian.go (new)"}}
	dispatchable, console, _ := inboxbatch.PartitionConsole([]inboxbatch.Item{item}, forbidden)
	if len(dispatchable) != 0 || len(console) != 1 {
		t.Fatalf("dispatchable=%d console=%d, want the item console-routed", len(dispatchable), len(console))
	}
}

func TestForbidden_ADisabledSandboxDeniesNothingBecauseNothingEnforcesIt(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"writer","sandbox":{"enabled":false,"deny_subpaths":[".evolve/profiles"]}}`
	if err := os.WriteFile(filepath.Join(dir, "writer.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	forbidden, err := Forbidden(root, "writer")
	if err != nil {
		t.Fatal(err)
	}
	if forbidden(".evolve/profiles/historian.json") {
		t.Fatal("the bridge enforces deny_subpaths only when the sandbox is enabled; routing must follow what is enforced")
	}
}

func TestForbidden_AnEmptyProjectRootIsAnError(t *testing.T) {
	cwd := t.TempDir()
	writeProfile(t, cwd, "writer", ".evolve/profiles")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if _, err := Forbidden("", "writer"); err == nil {
		t.Fatal("an empty root must never resolve profiles against the working directory")
	}
}

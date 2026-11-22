// Package inboxtest holds the inbox fixtures that the tests of cmd/evolve and internal/cli/inboxcmd share, so that each fixture has one home.
package inboxtest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type Routed struct {
	Route        string `json:"route"`
	RoutedReason string `json:"routed_reason"`
	RoutedCycle  int    `json:"routed_cycle"`
}

func EmptyRoot(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func WriteFile(t testing.TB, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func WritePolicy(t testing.TB, root, text string) {
	t.Helper()
	WriteFile(t, filepath.Join(root, ".evolve", "policy.json"), text)
}

func SeedItems(t testing.TB, root string, items map[string]string) {
	t.Helper()
	for rel, body := range items {
		WriteFile(t, filepath.Join(root, ".evolve", "inbox", rel), body)
	}
}

func CurationRoot(t testing.TB, items map[string]string) string {
	t.Helper()
	root := EmptyRoot(t)
	SeedItems(t, root, items)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

func ItemFields(t testing.TB, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func ReadRouted(t testing.TB, path string) Routed {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f Routed
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TreeBytes(t testing.TB, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		tree[path] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func WriteLiveRunLease(t testing.TB, root string) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func WriteItem(t testing.TB, dir, id string) string {
	t.Helper()
	path := filepath.Join(dir, "2026-08-16T15-10-00Z-"+id+".json")
	WriteFile(t, path, fmt.Sprintf("{\n  \"id\": %q,\n  \"kind\": \"bug\",\n  \"priority\": \"high\",\n  \"title\": \"fixture item %s\"\n}\n", id, id))
	return path
}

func Bind(t testing.TB, root, id string, c continuation.Continuation) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, c); err != nil {
		t.Fatal(err)
	}
}

func Bound(t testing.TB, root, id string) bool {
	t.Helper()
	_, ok, err := continuation.ReadRegistryEntry(root, id)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func MkdirAll(t testing.TB, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func RealDir(t testing.TB, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

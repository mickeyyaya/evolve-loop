//go:build integration

package sandbox

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSBPL_EmptyRootPreservesDeclaredPolicy(t *testing.T) {
	probe := Probe()
	if probe.OS != "darwin" || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED native macOS sandbox: %+v", probe)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(root, "protected")
	if err := os.WriteFile(protected, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	sb := New(Config{WritePaths: []string{root}, DenyPaths: []string{protected}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var output bytes.Buffer
	err = sb.Exec(ctx, []string{"/bin/sh", "-c", `printf allowed > "$1" || exit 30; if printf denied > "$2"; then exit 31; fi; cat "$2"`, "fixture", filepath.Join(root, "allowed"), protected}, nil, &output, &output)
	if err != nil {
		t.Fatalf("empty root rejected valid profile or broke declared policy: %v: %s", err, &output)
	}
	for name, want := range map[string]string{"allowed": "allowed", "protected": "retained"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: got %q, err %v; want %q", name, got, err, want)
		}
	}
}

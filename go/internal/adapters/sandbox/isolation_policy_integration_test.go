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

func TestSandboxFixtureChildEnforcesReadAndWritePolicy(t *testing.T) {
	probe := Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED: OS sandbox unavailable or could not apply: %+v", probe)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	private, evals := filepath.Join(root, "private-fixture"), filepath.Join(root, "eval-fixture")
	for _, dir := range []string{private, evals} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	secret, eval := filepath.Join(private, "sample"), filepath.Join(evals, "sample")
	for _, p := range []string{secret, eval} {
		if err := os.WriteFile(p, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(root, "private-alias")
	if err := os.Symlink(private, alias); err != nil {
		t.Fatal(err)
	}
	sb := New(Config{RepoRoot: root, WritePaths: []string{root}, DenyPaths: []string{evals}, DenyReadPaths: []string{private}, AllowNetwork: true})
	run := func(script string, args ...string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var output bytes.Buffer
		err := sb.Exec(ctx, append([]string{"/bin/sh", "-c", script, "fixture"}, args...), nil, &output, &output)
		return output.String(), err
	}
	// Prove THIS policy starts and permits useful work before interpreting a
	// denied operation. A profile/setup failure must never count as containment.
	if out, err := run(`cat "$1" && printf ok > "$2"`, eval, filepath.Join(root, "allowed")); err != nil {
		t.Fatalf("policy failed to start or rejected legitimate work (not verified): %v: %s", err, out)
	}
	for _, p := range []string{secret, filepath.Join(alias, "sample")} {
		if out, err := run(`cat "$1"`, p); err == nil {
			t.Errorf("private fixture readable via %s: %s", p, out)
		}
	}
	if out, err := run(`printf changed > "$1"`, eval); err == nil {
		t.Errorf("protected eval write succeeded: %s", out)
	}
	if b, err := os.ReadFile(eval); err != nil || string(b) != "fixture" {
		t.Fatalf("eval source changed: %q %v", b, err)
	}
}

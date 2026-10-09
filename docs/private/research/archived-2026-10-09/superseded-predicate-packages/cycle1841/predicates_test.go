//go:build acs

package cycle1841

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const filedItem = `{"id":"midwave-probe","kind":"feature","priority_class":"debuggability","weight":0.5,` +
	`"title":"probe","files":["go/cmd/evolve/cmd_inbox.go"],"summary":"s","fix":"f","acceptance":["a"]}`

func evolveBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}
	return bin
}

func projectWithItemAndLease(t *testing.T, leaseAge time.Duration) (root, item string) {
	t.Helper()
	root = t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item = filepath.Join(inbox, "2026-10-09T00-00-00Z-midwave-probe.json")
	if err := os.WriteFile(item, []byte(filedItem), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now().Add(-leaseAge)); err != nil {
		t.Fatal(err)
	}
	return root, item
}

func runInboxVerb(t *testing.T, bin, root string, args ...string) (rc int, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append([]string{"inbox"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+root)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), errBuf.String()
	}
	if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return 0, errBuf.String()
}

func TestC1841_001_RouteVerbsRefuseThroughTheProductionCLIWhileALaneIsLive(t *testing.T) {
	bin := evolveBinary(t)
	for _, args := range [][]string{
		{"route-console", "midwave-probe", "operator owns this", "1790"},
		{"route-lane", "midwave-probe", "lane may take it"},
	} {
		root, item := projectWithItemAndLease(t, 0)
		before, err := os.ReadFile(item)
		if err != nil {
			t.Fatal(err)
		}
		rc, stderr := runInboxVerb(t, bin, root, args...)
		if rc != 1 || !strings.Contains(stderr, "a loop lane is live") {
			t.Errorf("%s: rc = %d stderr = %q, want exit 1 refusing while a lane is live", args[0], rc, stderr)
		}
		after, err := os.ReadFile(item)
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("%s: refused verb changed the item (err %v)", args[0], err)
		}
	}
}

func TestC1841_002_RouteVerbsStillRouteWhenTheLeaseIsStale(t *testing.T) {
	bin := evolveBinary(t)
	for _, args := range [][]string{
		{"route-console", "midwave-probe", "operator owns this", "1790"},
		{"route-lane", "midwave-probe", "lane may take it"},
	} {
		root, _ := projectWithItemAndLease(t, time.Hour)
		if rc, stderr := runInboxVerb(t, bin, root, args...); rc != 0 {
			t.Errorf("%s with a stale lease: rc = %d stderr = %q, want the route applied", args[0], rc, stderr)
		}
	}
}

func TestC1841_003_UsageErrorsOutrankTheMidWaveRefusal(t *testing.T) {
	bin := evolveBinary(t)
	root, _ := projectWithItemAndLease(t, 0)
	if rc, stderr := runInboxVerb(t, bin, root, "route-console", "midwave-probe", "reason", "not-a-cycle"); rc != 10 {
		t.Errorf("bad cycle arg: rc = %d stderr = %q, want 10", rc, stderr)
	}
}

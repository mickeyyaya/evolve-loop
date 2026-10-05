package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func TestScopedCoverFunc_BehindAHeldLockWaitsOnlyItsBoundThenRunsUnserialized(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "go")
	writeCoverFixture(t, moduleDir)
	hold, err := verifylock.Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	saved := coverLockWait
	coverLockWait = 200 * time.Millisecond
	defer func() { coverLockWait = saved }()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var status int
	start := time.Now()
	stderr := captureStderr(t, func() { _, _, status = scopedCoverFunc(ctx, moduleDir, []string{"./p"}) })
	waited := time.Since(start)

	if status != coverStatusOK {
		t.Fatalf("status = %d, want coverStatusOK: a waiter past its bound must still run the coverage pass, unserialized", status)
	}
	if waited > 45*time.Second {
		t.Fatalf("the coverage pass took %s behind a held lock with a 200ms wait bound; the wait must end at the bound, not at the caller's deadline", waited)
	}
	if !strings.Contains(stderr, "running unserialized") || !strings.Contains(stderr, "wait bound") {
		t.Errorf("a waiter that gives up must say so and name the bound; stderr:\n%s", stderr)
	}
}

func TestCoverLockWait_IsTheSharedVerificationWaitBound(t *testing.T) {
	if coverLockWait != verifylock.MaxWait {
		t.Errorf("coverLockWait = %s, want verifylock.MaxWait (%s): the floor's coverage pass waits for the host lock exactly as long as the ACS suite does", coverLockWait, verifylock.MaxWait)
	}
}

func writeCoverFixture(t *testing.T, moduleDir string) {
	t.Helper()
	files := map[string]string{
		"go.mod":      "module example.com/coverfixture\n\ngo 1.23\n",
		"p/p.go":      "package p\n\nfunc One() int { return 1 }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\tif One() != 1 {\n\t\tt.Fatal(\"one\")\n\t}\n}\n",
	}
	for name, body := range files {
		path := filepath.Join(moduleDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

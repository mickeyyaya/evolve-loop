package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	reRFC3339DateTime = regexp.MustCompile(`20\d\d-\d\d-\d\dT?\d\d[:.]\d\d`)
	reEpochSeconds    = regexp.MustCompile(`\b\d{10,}\b`)
)

func stashLabelHasTimestamp(msg string) bool {
	return reRFC3339DateTime.MatchString(msg) || reEpochSeconds.MatchString(msg)
}

func labelOnly(stashMsg string) string {
	if i := strings.LastIndex(stashMsg, ": "); i >= 0 {
		return strings.TrimSpace(stashMsg[i+2:])
	}
	return strings.TrimSpace(stashMsg)
}

func brTopStashMessage(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "stash", "list")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git stash list: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		t.Fatalf("expected at least one stash entry after quarantine; got none")
	}
	return lines[0]
}

func TestDefaultBootRecovery_QuarantineStashLabelIsTimestamped(t *testing.T) {
	t.Parallel()
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repo, "go", "internal", "leak.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("package leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bootRecoveryGit(t, repo, "add", "-A")
	bootRecoveryGit(t, repo, "commit", "-m", "add leak.go")
	if err := os.WriteFile(src, []byte("package leak\n// leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)
	if !res.Quarantined {
		t.Fatalf("precondition: dirty tracked source must be quarantined; res=%+v stderr=%q", res, stderr.String())
	}

	msg := brTopStashMessage(t, repo)
	if !strings.Contains(msg, "boot-quarantine") {
		t.Fatalf("quarantine stash label must retain the boot-quarantine prefix; got %q", msg)
	}
	if labelOnly(msg) == "boot-quarantine" {
		t.Fatalf("quarantine stash label must be TIMESTAMPED, got the fixed constant %q — successive boot quarantines are then indistinguishable/unrecoverable", "boot-quarantine")
	}
	if !stashLabelHasTimestamp(msg) {
		t.Fatalf("quarantine stash label must contain a timestamp (RFC3339 datetime or >=10-digit epoch); got %q", msg)
	}
}

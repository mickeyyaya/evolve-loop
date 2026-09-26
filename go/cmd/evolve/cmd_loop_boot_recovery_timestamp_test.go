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

// stashLabelHasTimestamp accepts either an RFC3339-style datetime or a >=10-digit
// epoch, so the Builder is free to pick the timestamp encoding — only the PRESENCE
// of a real timestamp is contractual, not its exact format.
func stashLabelHasTimestamp(msg string) bool {
	return reRFC3339DateTime.MatchString(msg) || reEpochSeconds.MatchString(msg)
}

// labelOnly strips the "stash@{0}: On <branch>: " prefix git prepends to a
// `stash push -m` message so the bare-constant assertion sees only the label the
// loop chose. RFC3339 colons carry no following space, so LastIndex(": ") lands
// on the separator after the branch name.
func labelOnly(stashMsg string) string {
	if i := strings.LastIndex(stashMsg, ": "); i >= 0 {
		return strings.TrimSpace(stashMsg[i+2:])
	}
	return strings.TrimSpace(stashMsg)
}

// brTopStashMessage returns the most-recent stash entry's message line (empty
// output → fatal, since the caller has just asserted a quarantine occurred).
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

// TestDefaultBootRecovery_QuarantineStashLabelIsTimestamped drives the real
// boot-recovery seam (bootRecoverFn) against a dirty git repo and inspects the
// actual stash git created, not a source grep.
func TestDefaultBootRecovery_QuarantineStashLabelIsTimestamped(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed source file, then a leaked uncommitted edit (the leak vector).
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
	// The label must remain recognisable as a boot quarantine (operator finds it).
	if !strings.Contains(msg, "boot-quarantine") {
		t.Fatalf("quarantine stash label must retain the boot-quarantine prefix; got %q", msg)
	}
	// ...and must NOT be the bare fixed constant — that is the whole defect.
	if labelOnly(msg) == "boot-quarantine" {
		t.Fatalf("quarantine stash label must be TIMESTAMPED, got the fixed constant %q — successive boot quarantines are then indistinguishable/unrecoverable", "boot-quarantine")
	}
	// ...and must carry a real timestamp component (RFC3339 datetime or epoch).
	if !stashLabelHasTimestamp(msg) {
		t.Fatalf("quarantine stash label must contain a timestamp (RFC3339 datetime or >=10-digit epoch); got %q", msg)
	}
}

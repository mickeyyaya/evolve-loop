package releasepipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultReleaseVerify_RelativeRepoRoot(t *testing.T) {
	err := defaultReleaseVerify("relative/repo", "1.2.3", "deadbeef")
	if err == nil {
		t.Fatal("defaultReleaseVerify with relative repoRoot returned nil error")
	}
	if !strings.Contains(err.Error(), "repoRoot must be absolute") {
		t.Fatalf("defaultReleaseVerify error = %q, want absolute-path guard", err)
	}
}

func TestDefaultReleaseVerify_MissingBinaryOnDisk(t *testing.T) {
	err := defaultReleaseVerify(t.TempDir(), "1.2.3", "deadbeef")
	if err == nil {
		t.Fatal("defaultReleaseVerify with missing go/evolve returned nil error")
	}
	if !strings.Contains(err.Error(), "tracked binary missing on disk") {
		t.Fatalf("defaultReleaseVerify error = %q, want missing tracked binary", err)
	}
}

func TestDefaultShip_BinaryNotFound(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", t.TempDir())

	_, err := defaultShip(t.TempDir(), "release: test", "notes")
	if err == nil {
		t.Fatal("defaultShip with no evolve binary returned nil error")
	}
	if !strings.Contains(err.Error(), "evolve binary not found") {
		t.Fatalf("defaultShip error = %q, want binary-not-found guard", err)
	}
}

func TestDefaultReleaseVerify_Success(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repoRoot := t.TempDir()

	// 1. git init
	runGit := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")

	// 2. Setup mock executable under go/evolve
	goDir := filepath.Join(repoRoot, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}
	binAbs := filepath.Join(goDir, "evolve")
	scriptContent := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then\n  echo \"evolve version 1.2.3\"\nelse\n  echo \"mock\"\nfi\n"
	if err := os.WriteFile(binAbs, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(binAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// 3. Commit go/evolve to git
	runGit("add", "go/evolve")
	runGit("commit", "-m", "add mock binary")
	commitSHA := runGit("rev-parse", "HEAD")

	// Calculate the expected blob SHA
	diskBytes, err := os.ReadFile(binAbs)
	if err != nil {
		t.Fatal(err)
	}
	blobSHA := fmt.Sprintf("%x", sha256.Sum256(diskBytes))

	// 4. Setup state.json to test the expected_ship_sha repinning
	evolveDir := filepath.Join(repoRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(evolveDir, "state.json")
	initialState := map[string]any{
		"expected_ship_sha":     "stale_sha",
		"expected_ship_version": "0.0.0",
		"other_field":           "preserved",
	}
	stateBytes, err := json.Marshal(initialState)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, stateBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// 5. Run defaultReleaseVerify
	err = defaultReleaseVerify(repoRoot, "1.2.3", commitSHA)
	if err != nil {
		t.Fatalf("defaultReleaseVerify failed: %v", err)
	}

	// 6. Verify state.json was updated
	updatedBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var updatedState map[string]any
	if err := json.Unmarshal(updatedBytes, &updatedState); err != nil {
		t.Fatal(err)
	}
	if updatedState["expected_ship_sha"] != blobSHA {
		t.Errorf("expected_ship_sha = %q, want %q", updatedState["expected_ship_sha"], blobSHA)
	}
	if updatedState["expected_ship_version"] != "1.2.3" {
		t.Errorf("expected_ship_version = %q, want %q", updatedState["expected_ship_version"], "1.2.3")
	}
	if updatedState["other_field"] != "preserved" {
		t.Errorf("other_field was lost or modified: %v", updatedState["other_field"])
	}

	// 7. Verify tag v1.2.3 was created
	tags := runGit("tag", "-l", "v1.2.3")
	if tags != "v1.2.3" {
		t.Errorf("expected tag v1.2.3 to be created, got list: %q", tags)
	}
}

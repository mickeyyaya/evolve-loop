//go:build integration

package treefence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFenceVerificationRejectsIgnoreRuleEvasion(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()
	f := Begin(ctx, dir, true)
	write(t, dir, ".gitignore", "bin/\nsrc/hidden_probe_test.go\n")
	write(t, dir, "src/hidden_probe_test.go", "package src // auditor probe\n")
	out := f.End(ctx)
	if _, err := os.Stat(filepath.Join(dir, "src/hidden_probe_test.go")); err == nil && out.Verified {
		t.Fatalf("verified=true while auditor-added probe survives restored ignore rules: %+v", out)
	}
}

func TestFenceFailedRestoreCannotVerify(t *testing.T) {
	dir := initRepo(t)
	fence := Begin(context.Background(), dir, true)
	if fence.TakeErr() != nil {
		t.Fatal(fence.TakeErr())
	}
	if err := os.Rename(filepath.Join(dir, ".git"), filepath.Join(t.TempDir(), "git")); err != nil {
		t.Fatal(err)
	}
	got := fence.End(context.Background())
	if got.Verified || got.RestoreErr == nil {
		t.Fatalf("failed restoration authenticated: %+v", got)
	}
}

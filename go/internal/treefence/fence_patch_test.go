package treefence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const revertedPatchName = "fence-reverted.patch"

func gitApply(t *testing.T, dir, patch string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "apply", patch)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git apply %s: %v: %s", patch, err, out)
	}
}

func diagnosticText(o Outcome, phase string) string {
	var parts []string
	for _, d := range o.Diagnostics(phase) {
		parts = append(parts, d.Message)
	}
	return strings.Join(parts, "\n")
}

func TestEnd_FencedWritesLeaveAPatchThatGivesThemBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)
	workspace := t.TempDir()
	f := Begin(ctx, dir, true).WithWorkspace(workspace)
	binaryBody := []byte{0, 1, 2, 0xff, 0xfe, 0, 7, 0x80}
	write(t, dir, "src/mat.go", "package src // MUTATED by a probe\n")
	write(t, dir, "src/zz_probe_test.go", "package src // auditor probe\n")
	if err := os.WriteFile(filepath.Join(dir, "src", "blob.bin"), binaryBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "src", "keep.go")); err != nil {
		t.Fatal(err)
	}

	out := f.End(ctx)
	if !out.Verified || out.RestoreErr != nil {
		t.Fatalf("End must restore and verify: %+v", out)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Fatalf("restore did not run: %q", got)
	}

	patch := filepath.Join(workspace, revertedPatchName)
	if _, err := os.Stat(patch); err != nil {
		t.Fatalf("a fenced write must leave %s: %v", patch, err)
	}
	gitApply(t, dir, patch)
	if got := read(t, dir, "src/mat.go"); got != "package src // MUTATED by a probe\n" {
		t.Errorf("patch did not give back the modified file: %q", got)
	}
	if got := read(t, dir, "src/zz_probe_test.go"); got != "package src // auditor probe\n" {
		t.Errorf("patch did not give back the added file: %q", got)
	}
	gotBinary, err := os.ReadFile(filepath.Join(dir, "src", "blob.bin"))
	if err != nil || string(gotBinary) != string(binaryBody) {
		t.Errorf("patch did not give back the binary file byte for byte: %v %v", gotBinary, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "keep.go")); !os.IsNotExist(err) {
		t.Errorf("patch did not give back the deletion: %v", err)
	}
}

func TestEnd_PatchHoldsOnlyTheRevertedPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)
	workspace := t.TempDir()
	f := Begin(ctx, dir, true, "src/mat.go").WithWorkspace(workspace)
	write(t, dir, "src/mat.go", "package src // a write this dispatch may keep\n")
	write(t, dir, "src/zz_probe_test.go", "package src // auditor probe\n")

	out := f.End(ctx)
	if len(out.Kept) != 1 || len(out.Restored) != 1 {
		t.Fatalf("setup: Kept=%v Restored=%v", out.Kept, out.Restored)
	}
	body, err := os.ReadFile(filepath.Join(workspace, revertedPatchName))
	if err != nil {
		t.Fatalf("the reverted path must leave a patch: %v", err)
	}
	if !strings.Contains(string(body), "src/zz_probe_test.go") {
		t.Errorf("patch is missing the reverted path:\n%s", body)
	}
	if strings.Contains(string(body), "src/mat.go") {
		t.Errorf("patch must not carry a write the dispatch may keep:\n%s", body)
	}
}

func TestOutcomeDiagnostics_NamesThePatchPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)
	workspace := t.TempDir()
	f := Begin(ctx, dir, true).WithWorkspace(workspace)
	write(t, dir, "src/mat.go", "package src // MUTATED by a probe\n")

	text := diagnosticText(f.End(ctx), "audit")
	want := filepath.Join(workspace, revertedPatchName)
	if !strings.Contains(text, want) {
		t.Errorf("the fence diagnostic must name %s, got:\n%s", want, text)
	}
}

func TestEnd_NoFencedWriteWritesNoPatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cleanDir := initRepo(t)
	cleanWorkspace := t.TempDir()
	clean := Begin(ctx, cleanDir, true).WithWorkspace(cleanWorkspace)
	cleanOut := clean.End(ctx)
	if _, err := os.Stat(filepath.Join(cleanWorkspace, revertedPatchName)); !os.IsNotExist(err) {
		t.Errorf("a clean phase must write no patch: %v", err)
	}
	if text := diagnosticText(cleanOut, "audit"); strings.Contains(text, revertedPatchName) {
		t.Errorf("a clean phase must not name a patch: %s", text)
	}

	keptDir := initRepo(t)
	keptWorkspace := t.TempDir()
	keptOnly := Begin(ctx, keptDir, true, "src/mat.go").WithWorkspace(keptWorkspace)
	write(t, keptDir, "src/mat.go", "package src // a write this dispatch may keep\n")
	keptOut := keptOnly.End(ctx)
	if len(keptOut.Kept) != 1 || len(keptOut.Restored) != 0 {
		t.Fatalf("setup: Kept=%v Restored=%v", keptOut.Kept, keptOut.Restored)
	}
	if _, err := os.Stat(filepath.Join(keptWorkspace, revertedPatchName)); !os.IsNotExist(err) {
		t.Errorf("a phase that reverted nothing must write no patch: %v", err)
	}
	if text := diagnosticText(keptOut, "build"); strings.Contains(text, revertedPatchName) {
		t.Errorf("a phase that reverted nothing must not name a patch: %s", text)
	}
}

func TestEnd_UnwritablePatchLocationStillRestoresAndSaysSo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(blocker, "workspace")
	f := Begin(ctx, dir, true).WithWorkspace(workspace)
	write(t, dir, "src/mat.go", "package src // MUTATED by a probe\n")

	out := f.End(ctx)
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Errorf("a patch that cannot be saved must not stop the restore: %q", got)
	}
	if len(out.Restored) != 1 {
		t.Errorf("Restored = %v, want the one reverted path", out.Restored)
	}
	if text := diagnosticText(out, "audit"); !strings.Contains(text, revertedPatchName) {
		t.Errorf("a patch that could not be saved must be reported by name:\n%s", text)
	}
}

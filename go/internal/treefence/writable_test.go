package treefence

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFence_KeepsTheWritesToItsWritablePathsAndRestoresEveryOther(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	f := Begin(ctx, dir, true, "src/mat.go", "src/untouched.go")
	write(t, dir, "src/mat.go", "package src // the debugger's resolution\n")
	write(t, dir, "src/keep.go", "package src // not this dispatch's to write\n")
	write(t, dir, "src/stray.go", "package src\n")

	out := f.End(ctx)

	if !out.Verified || out.RestoreErr != nil {
		t.Fatalf("a fence that kept only its writable paths must verify: %+v", out)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // the debugger's resolution\n" {
		t.Errorf("the writable path was restored: %q", got)
	}
	if got := read(t, dir, "src/keep.go"); got != "package src\n" {
		t.Errorf("a path the dispatch may not write kept its write: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "stray.go")); !os.IsNotExist(err) {
		t.Errorf("a file the dispatch may not add survived: %v", err)
	}
	if !reflect.DeepEqual(out.Kept, []string{"src/mat.go"}) || !reflect.DeepEqual(out.Restored, []string{"src/keep.go", "src/stray.go"}) {
		t.Errorf("kept %v restored %v, want kept [src/mat.go] restored [src/keep.go src/stray.go]", out.Kept, out.Restored)
	}
}

func TestFence_WithoutWritablePathsRestoresTheSameWriteAsBefore(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	f := Begin(ctx, dir, true)
	write(t, dir, "src/mat.go", "package src // the debugger's resolution\n")

	out := f.End(ctx)

	if !out.Verified || len(out.Kept) != 0 || !reflect.DeepEqual(out.Restored, []string{"src/mat.go"}) {
		t.Fatalf("a fence with no writable path restores every write: %+v", out)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Errorf("the write survived: %q", got)
	}
}

func TestOutcome_DiagnosticsNameTheWritesItKept(t *testing.T) {
	diags := Outcome{Verified: true, Kept: []string{"go/.apicover-enforce"}}.Diagnostics("debugger")

	if len(diags) != 1 || diags[0].Severity != "warning" || !strings.Contains(diags[0].Message, "kept") || !strings.Contains(diags[0].Message, "go/.apicover-enforce") {
		t.Fatalf("the kept write must be reported by name: %+v", diags)
	}
}

func TestFence_KeepsAWritablePathTheDispatchCreated(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	f := Begin(ctx, dir, true, "src/recreated.go")
	write(t, dir, "src/recreated.go", "package src // re-created by the resolution\n")

	out := f.End(ctx)

	if !out.Verified || !reflect.DeepEqual(out.Kept, []string{"src/recreated.go"}) {
		t.Fatalf("a writable path absent at the snapshot must be kept: %+v", out)
	}
	if got := read(t, dir, "src/recreated.go"); got != "package src // re-created by the resolution\n" {
		t.Errorf("the created writable file was removed: %q", got)
	}
}

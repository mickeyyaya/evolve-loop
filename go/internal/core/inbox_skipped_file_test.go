package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeBrokenClaim(t *testing.T, evolveDir string, cycle int) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox", "processing", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLaneDeclaredFiles_AnUnreadableClaimFileRefusesThePartialScope(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeScopeDeltaLane(t, evolveDir, 4, "", "readable", "a.go")
	writeBrokenClaim(t, evolveDir, 4)

	files, err := LaneDeclaredFiles(evolveDir, 4)

	if err == nil || !strings.Contains(err.Error(), "broken.json") || files != nil {
		t.Fatalf("LaneDeclaredFiles = %v, %v; want a refusal that names broken.json", files, err)
	}
}

func TestProcessingClaimIDs_NamesAnUnreadableClaimFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeScopeDeltaLane(t, evolveDir, 6, "", "readable", "a.go")
	writeBrokenClaim(t, evolveDir, 6)
	var warn bytes.Buffer

	ids := processingClaimIDs(root, 6, &warn)

	if len(ids) != 1 || ids[0] != "readable" || !strings.Contains(warn.String(), "WARN skipped broken.json") {
		t.Fatalf("ids = %v, warn = %q; want the readable claim and a WARN that names broken.json", ids, warn.String())
	}
}

func TestProcessingClaimIDs_AnUnreadableClaimDirIsAWarnLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	processing := filepath.Join(root, ".evolve", "inbox", "processing")
	if err := os.MkdirAll(processing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(processing, "cycle-7"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	if ids := processingClaimIDs(root, 7, &warn); ids != nil || !strings.Contains(warn.String(), "cycle 7: WARN claims unreadable") {
		t.Fatalf("ids = %v, warn = %q; want no ids and a WARN line", ids, warn.String())
	}
}

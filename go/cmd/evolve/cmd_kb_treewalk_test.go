package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunKBRejectsBadInvocations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cases := [][]string{
		nil,
		{"graph"},
		{"graph", "nonsense", "x"},
		{"graph", "neighbors"},
		{"graph", "neighbors", "--project-root", root},
		{"graph", "neighbors", "cycle:1", "--project-root", root},
		{"graph", "neighbors", "cycle:1", "extra", "--project-root", root},
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		if code := runKB(args, nil, &out, &errb); code == 0 {
			t.Errorf("runKB(%v) = 0, want failure", args)
		}
	}
}

func TestRunKBRecurrencesPrintsZeroForUnknownFingerprint(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := runKB([]string{"graph", "recurrences", "fp-x", "--project-root", t.TempDir(), "--json"}, nil, &out, &errb)
	if code != 0 || !strings.Contains(out.String(), `"count": 0`) {
		t.Errorf("code=%d out=%s err=%s", code, out.String(), errb.String())
	}
}

func TestRunTreewalkSnapshotCheckLeakAndUsage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(t.TempDir(), "b.json")
	var out, errb bytes.Buffer
	if code := runTreewalk([]string{"snapshot", "--root", root, "--out", base}, nil, &out, &errb); code != 0 {
		t.Fatalf("snapshot = %d: %s", code, errb.String())
	}
	if code := runTreewalk([]string{"check", "--root", root, "--baseline", base}, nil, &out, &errb); code != 0 {
		t.Fatalf("clean check = %d: %s", code, errb.String())
	}
	if err := os.MkdirAll(filepath.Join(root, "leak"), 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := runTreewalk([]string{"check", "--root", root, "--baseline", base}, nil, &out, &errb); code != 1 || !strings.Contains(out.String(), "LEAK leak") {
		t.Errorf("leak check code=%d out=%s", code, out.String())
	}
	for _, args := range [][]string{nil, {"bogus"}, {"snapshot", "--root", root}, {"check", "--root", root}} {
		if code := runTreewalk(args, nil, &out, &errb); code == 0 {
			t.Errorf("runTreewalk(%v) = 0, want failure", args)
		}
	}
}

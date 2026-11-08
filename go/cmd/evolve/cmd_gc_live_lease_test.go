package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunGC_DryRunLiveRunGuard(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	root, _, _ := gcWorktreeEnv(t, "")
	liveLaneLease(t, root, 0)

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--dry-run", "--project-root", root}, nil, &stdout, &stderr)
	if rc != cliExitOK {
		t.Fatalf("rc=%d, want %d\nstdout=%s\nstderr=%s", rc, cliExitOK, stdout.String(), stderr.String())
	}

	const wantLiveNotice = "evolve gc: 1 run(s) hold a live lease (cycle-1790), so the sweeps that stop processes (sessions, sockets, cycle and dispatch processes) wait for a wave boundary\n"
	if !strings.Contains(stderr.String(), wantLiveNotice) {
		t.Errorf("stderr = %q, want live lease notice %q", stderr.String(), wantLiveNotice)
	}

	out := stdout.String()
	for _, banned := range []string{
		"orphan session(s) would be reaped",
		"dead per-run socket(s) would be reaped",
		"finished-cycle orphan process(es) would be terminated",
		"dispatch process(es) would be stopped",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("stdout contains process sweep line %q while a run holds a live lease:\n%s", banned, out)
		}
	}

	if _, err := os.Stat(filepath.Join(root, ".evolve", gcLeaseFile)); !os.IsNotExist(err) {
		t.Errorf("dry-run created %s: err=%v", gcLeaseFile, err)
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGC([]string{"--dry-run", "--json", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("json rc=%d, want %d\nstderr=%s", rc, cliExitOK, stderr.String())
	}
	var got gcResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal json: %v\nraw=%s", err, stdout.String())
	}
	if !got.DryRun || !slices.Equal(got.LiveRuns, []string{"cycle-1790"}) {
		t.Errorf("json result = %+v, want DryRun=true and LiveRuns=[cycle-1790]", got)
	}
}

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const sessionsTestSocket = "evolve-bridge-p4242"

var sessionsNow = time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)

func liveRunWithSnapshot(t *testing.T, root string, cycle int, snap panewatch.Snapshot) string {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(ws, runlease.Lease{RunID: "run-" + fmt.Sprint(cycle), OwnerPID: os.Getpid()}, sessionsNow); err != nil {
		t.Fatal(err)
	}
	if err := panewatch.Write(ws, snap); err != nil {
		t.Fatal(err)
	}
	return ws
}

func routerSnapshot() panewatch.Snapshot {
	return panewatch.Snapshot{
		Session: "evolve-bridge-agy-c1806-router-pid4242-n3-1790781838", Socket: sessionsTestSocket,
		CLI: "agy-tmux", Agent: "router", Cycle: 1806, Model: "Gemini 3.1 Pro (High)", ModelLabel: "Gemini 3.1 Pro · high",
		Busy: true, ProgressHash: "h", ProgressAt: sessionsNow.Add(-5 * time.Minute), UpdatedAt: sessionsNow.Add(-time.Minute),
		TokenLine: "▸ Thought for 20s, 2.3k tokens", WriterPID: os.Getpid(),
	}
}

func fakeSessionsTmux(lines map[string]string) func(context.Context, string) (string, error) {
	return func(_ context.Context, socket string) (string, error) {
		out, ok := lines[socket]
		if !ok {
			return "", errors.New("no server running on " + socket)
		}
		return out, nil
	}
}

func withSessionsSeams(t *testing.T, tmux func(context.Context, string) (string, error), sockets []string) {
	t.Helper()
	prevTmux, prevSockets, prevNow := bridgeSessionsTmux, bridgeLiveSockets, bridgeSessionsNow
	bridgeSessionsTmux = tmux
	bridgeLiveSockets = func() []string { return sockets }
	bridgeSessionsNow = func() time.Time { return sessionsNow }
	t.Cleanup(func() { bridgeSessionsTmux, bridgeLiveSockets, bridgeSessionsNow = prevTmux, prevSockets, prevNow })
}

func runSessionsJSON(t *testing.T, root string) ([]bridgeSessionRow, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := runBridge([]string{"sessions", "--json", "--project-root=" + root}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("bridge sessions exit %d; stderr:\n%s", code, stderr.String())
	}
	var rows []bridgeSessionRow
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	return rows, stderr.String()
}

func TestBridgeSessions_ListsEachLivePaneWithItsLiveness(t *testing.T) {
	root := t.TempDir()
	snap := routerSnapshot()
	liveRunWithSnapshot(t, root, 1806, snap)
	probe := "evolve-recipe-c0-usage-probe-pid4242-n4-1791262829"
	withSessionsSeams(t, fakeSessionsTmux(map[string]string{sessionsTestSocket: fmt.Sprintf("%s\t%d\n%s\t%d\n",
		snap.Session, sessionsNow.Add(-3*time.Second).Unix(), probe, sessionsNow.Add(-50*time.Minute).Unix())}), nil)

	rows, _ := runSessionsJSON(t, root)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the router pane and the unwatched probe", rows)
	}
	r := rows[0]
	if r.Cycle != 1806 || r.Phase != "router" || r.CLI != "agy-tmux" || r.Model != "Gemini 3.1 Pro · high" || r.State != "busy" {
		t.Errorf("router row = %+v", r)
	}
	if r.ProgressAgeS != 300 || r.DrawnAgeS != 3 || r.TokenLine != snap.TokenLine || r.Session != snap.Session {
		t.Errorf("router row ages/tokens = %+v", r)
	}
	if rows[1].State != "unwatched" || rows[1].Session != probe || rows[1].DrawnAgeS != 3000 {
		t.Errorf("probe row = %+v, want an unwatched session with its drawn age", rows[1])
	}
}

func TestBridgeSessions_ASnapshotWhoseSessionIsGoneReadsGone(t *testing.T) {
	root := t.TempDir()
	liveRunWithSnapshot(t, root, 1806, routerSnapshot())
	withSessionsSeams(t, fakeSessionsTmux(map[string]string{sessionsTestSocket: ""}), nil)

	rows, _ := runSessionsJSON(t, root)
	if len(rows) != 1 || rows[0].State != "gone" || rows[0].DrawnAgeS != -1 {
		t.Errorf("rows = %+v, want the router row marked gone with no drawn age", rows)
	}
}

func TestBridgeSessions_AnUnreachableSocketWarnsAndKeepsTheSnapshotState(t *testing.T) {
	root := t.TempDir()
	liveRunWithSnapshot(t, root, 1806, routerSnapshot())
	withSessionsSeams(t, fakeSessionsTmux(nil), nil)

	rows, stderr := runSessionsJSON(t, root)
	if len(rows) != 1 || rows[0].State != "busy" || rows[0].DrawnAgeS != -1 {
		t.Errorf("rows = %+v, want the snapshot's busy state with an unknown drawn age", rows)
	}
	if !strings.Contains(stderr, sessionsTestSocket) {
		t.Errorf("an unreachable socket must be named on stderr; got %q", stderr)
	}
}

func TestBridgeSessions_LiveLoopSocketsShowUnwatchedSessions(t *testing.T) {
	withSessionsSeams(t, fakeSessionsTmux(map[string]string{"evolve-bridge-p77": fmt.Sprintf("evolve-recipe-c0-usage-probe-pid77-n5-1\t%d\n", sessionsNow.Unix())}),
		[]string{"evolve-bridge-p77"})
	rows, _ := runSessionsJSON(t, t.TempDir())
	if len(rows) != 1 || rows[0].State != "unwatched" || rows[0].Phase != "-" {
		t.Errorf("rows = %+v, want the live loop socket's probe session listed as unwatched", rows)
	}
}

func TestBridgeSessions_TableNamesEveryColumn(t *testing.T) {
	root := t.TempDir()
	liveRunWithSnapshot(t, root, 1806, routerSnapshot())
	withSessionsSeams(t, fakeSessionsTmux(map[string]string{sessionsTestSocket: routerSnapshot().Session + "\t" + fmt.Sprint(sessionsNow.Unix())}), nil)
	var stdout, stderr bytes.Buffer
	if code := runBridge([]string{"sessions", "--project-root=" + root}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"CYCLE", "PHASE", "CLI", "MODEL", "STATE", "PROGRESS", "DRAWN", "TOKENS", "1806", "router", "agy-tmux", "busy", "5m0s", "0s", "2.3k tokens"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
}

func TestBridgeSessions_EmptyProjectSaysSo(t *testing.T) {
	withSessionsSeams(t, fakeSessionsTmux(nil), nil)
	var stdout, stderr bytes.Buffer
	if code := runBridge([]string{"sessions", "--project-root=" + t.TempDir()}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "no live bridge sessions") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestBridgeSessions_BadFlagIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runBridge([]string{"sessions", "--bogus"}, nil, &stdout, &stderr); code != 10 {
		t.Errorf("exit %d, want 10", code)
	}
}

func TestBridgeSessions_WritesNothing(t *testing.T) {
	root := t.TempDir()
	liveRunWithSnapshot(t, root, 1806, routerSnapshot())
	withSessionsSeams(t, fakeSessionsTmux(map[string]string{sessionsTestSocket: routerSnapshot().Session + "\t1"}), nil)
	before := treeDigest(t, root)
	runSessionsJSON(t, root)
	var stdout, stderr bytes.Buffer
	runBridge([]string{"sessions", "--project-root=" + root}, nil, &stdout, &stderr)
	if after := treeDigest(t, root); after != before {
		t.Error("evolve bridge sessions changed the project tree; it must be read-only while a wave runs")
	}
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s|%v|%d|%d\n", path, d.IsDir(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestBridgeSessions_ADeadWritersSnapshotIsNotListedAsLive(t *testing.T) {
	root := t.TempDir()
	snap := routerSnapshot()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	snap.WriterPID = cmd.Process.Pid
	liveRunWithSnapshot(t, root, 1806, snap)
	withSessionsSeams(t, fakeSessionsTmux(map[string]string{sessionsTestSocket: ""}), nil)

	rows, _ := runSessionsJSON(t, root)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want none: a snapshot whose writer died is not a live pane", rows)
	}
}

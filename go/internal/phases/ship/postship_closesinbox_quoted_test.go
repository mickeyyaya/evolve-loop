package ship

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeBuildReportBody(t *testing.T, root string, cycleID int, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPromoteInbox_QuotedClosesInboxMarkerConsumesNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"fenced code block", "# Build Report\n\nUpstream said:\n\n```\nCloses-Inbox: quoted-item\n```\n"},
		{"indented quote", "# Build Report\n\n    Closes-Inbox: quoted-item\n"},
		{"nested bullet", "# Build Report\n\n- upstream finding\n  - Closes-Inbox: quoted-item\n"},
	}
	for i, tc := range cases {
		tc := tc
		cid := 2118 + i
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeDrainCycleState(t, root, cid)
			writeBuildReportBody(t, root, cid, tc.body)
			inbox := filepath.Join(root, ".evolve", "inbox")
			writeDrainInboxItem(t, inbox, "quoted-item")

			res := &RunResult{}
			if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
				t.Fatalf("promoteInbox: %v", err)
			}
			if _, err := os.Stat(promotedPath(root, cid, "quoted-item")); !os.IsNotExist(err) {
				t.Fatalf("quoted upstream text forged a closure claim and consumed an item that never shipped (logs:\n%s)", strings.Join(res.Logs, "\n"))
			}
		})
	}
}

func TestPromoteInbox_ColumnZeroMarkerAfterFenceStillConsumes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 2121
	writeDrainCycleState(t, root, cid)
	writeBuildReportBody(t, root, cid, "# Build Report\n\n```\nCloses-Inbox: quoted-item\n```\n\nCloses-Inbox: real-item\n")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "quoted-item")
	writeDrainInboxItem(t, inbox, "real-item")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(promotedPath(root, cid, "real-item")); err != nil {
		t.Fatalf("a genuine column-zero marker after a fence was not honoured (logs:\n%s)", strings.Join(res.Logs, "\n"))
	}
	if _, err := os.Stat(promotedPath(root, cid, "quoted-item")); !os.IsNotExist(err) {
		t.Fatalf("the fenced marker was consumed alongside the real one")
	}
}

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	c2035AcFive = "retro-minting-files-through-inbox-add-lane-ac-5-console"
	c2035Unify  = "retro-minting-files-through-inbox-add-console"
)

var c2035Round1 = []string{
	"M1: go/internal/core/failurelearning floor tests fail (3) with this diff, because faillearn now mints timestamped files under .bump.lock; protected surface, fix is console item " + c2035AcFive,
	"M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules of go/internal/inboxmover/lifecycle/file.go (protected); unification queued as " + c2035Unify,
}

func c2035Inbox(t *testing.T) string {
	t.Helper()
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, route := range map[string]string{c2035AcFive: "console-manual", c2035Unify: "console-manual", "free-lane-item": "lane"} {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(`{"id":"`+id+`","weight":0.5,"route":"`+route+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return inbox
}

func TestRepairFixability_Cycle2035RoundOneStopsOnM1WithItsOpenConsoleItem(t *testing.T) {
	t.Parallel()
	var warn strings.Builder
	check := repairFixability(func(string) bool { return false }, &warn)

	got := check(core.RepairCheck{Defects: c2035Round1, InboxDir: c2035Inbox(t), Salvage: "/dev/cl-c2035-lane"})

	want := core.RepairFindings{Stop: true, Reason: "handoff_blocker", Handoff: []string{"M1", "M2"}, Blockers: []string{"M1"}, ConsoleItems: []string{c2035AcFive}, Salvage: "/dev/cl-c2035-lane"}
	if !reflect.DeepEqual(got, want) || warn.Len() != 0 {
		t.Errorf("findings = %+v, warn %q; want %+v", got, warn.String(), want)
	}
}

func TestRepairFixability_AnUnreadableInboxWarnsAndNamesNoConsoleItem(t *testing.T) {
	t.Parallel()
	notADir := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	got := repairFixability(func(string) bool { return false }, &warn)(core.RepairCheck{Defects: c2035Round1[:1], InboxDir: notADir})

	if got.Stop || len(got.Handoff) != 0 || !reflect.DeepEqual(got.Lane, []string{"M1"}) || !strings.Contains(warn.String(), "WARN fixability: the inbox") {
		t.Errorf("findings = %+v, warn %q; want M1 as a lane finding and a WARN", got, warn.String())
	}
}

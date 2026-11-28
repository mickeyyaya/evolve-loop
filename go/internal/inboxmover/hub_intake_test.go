package inboxmover

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestCodeHubFilesUnnamed_IsRegisteredUnderInbox(t *testing.T) {
	t.Parallel()
	owner, ok := signalcenter.IsRegistered(CodeHubFilesUnnamed)
	if !ok || owner != signalcenter.ModuleInbox {
		t.Fatalf("%s owner = (%q, %v), want %q", CodeHubFilesUnnamed, owner, ok, signalcenter.ModuleInbox)
	}
	if !CodeHubFilesUnnamed.Valid() || !CodeHubFilesUnnamed.BelongsTo(signalcenter.ModuleInbox) {
		t.Errorf("%s failed code validation or inbox prefix check", CodeHubFilesUnnamed)
	}
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("registry conflicts: %+v", conflicts)
	}
}

func TestWarnUnnamedHubFiles_EmitsTypedWarningForWholeHubDirectories(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		id         string
		files      []string
		wantHubs   []string
		wantFamily string
	}{
		"cmd hub bare": {
			id: "hub-cmd-bare", files: []string{"go/cmd/evolve"},
			wantHubs: []string{"go/cmd/evolve"}, wantFamily: "cmd_",
		},
		"cmd hub trailing slash": {
			id: "hub-cmd-slash", files: []string{"go/cmd/evolve/"},
			wantHubs: []string{"go/cmd/evolve"}, wantFamily: "cmd_",
		},
		"cmd hub duplicated": {
			id: "hub-cmd-dup", files: []string{"go/cmd/evolve", "./go/cmd/evolve/"},
			wantHubs: []string{"go/cmd/evolve"}, wantFamily: "cmd_",
		},
		"core hub bare": {
			id: "hub-core-bare", files: []string{"go/internal/core"},
			wantHubs: []string{"go/internal/core"}, wantFamily: ".go",
		},
		"mixed named and hub": {
			id: "hub-mixed", files: []string{"go/internal/inboxfile/inboxfile.go", "go/internal/core"},
			wantHubs: []string{"go/internal/core"}, wantFamily: ".go",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rc := newRecordingCenter()
			got := WarnUnnamedHubFiles(inboxbatch.Item{ID: tc.id, Files: tc.files}, rc.c)
			rc.c.Flush()
			if !slices.Equal(got, tc.wantHubs) {
				t.Fatalf("WarnUnnamedHubFiles = %v, want %v", got, tc.wantHubs)
			}
			if len(rc.events) != 1 {
				t.Fatalf("events = %+v, want exactly 1", rc.events)
			}
			ev := rc.events[0]
			if ev.Module != signalcenter.ModuleInbox || ev.Kind != signalcenter.KindInboxWarning ||
				ev.Severity != signalcenter.SeverityWarn || ev.Code != CodeHubFilesUnnamed {
				t.Errorf("header = (%s, %s, %s, %s), want (inbox, inbox.warning, WARN, %s)",
					ev.Module, ev.Kind, ev.Severity, ev.Code, CodeHubFilesUnnamed)
			}
			if ev.Fields["hub"] != tc.wantHubs[0] || ev.Fields["task_id"] != tc.id || ev.Fields["stage"] != "shadow" || ev.Fields["step"] != "intake" {
				t.Errorf("fields = %+v, want hub=%s task_id=%s stage=shadow step=intake", ev.Fields, tc.wantHubs[0], tc.id)
			}
			if !strings.Contains(ev.Reason, tc.wantHubs[0]) || !strings.Contains(ev.Reason, tc.wantFamily) || !strings.Contains(ev.Fields["family"], tc.wantFamily) {
				t.Errorf("reason=%q family=%q must name hub %q and family %q", ev.Reason, ev.Fields["family"], tc.wantHubs[0], tc.wantFamily)
			}
		})
	}
}

func TestWarnUnnamedHubFiles_SkipsNamedFilesNonHubsAndNilSignals(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"named cmd hub file":  {"go/cmd/evolve/cmd_inbox.go"},
		"named core hub file": {"go/internal/core/statemachine.go"},
		"named hub file pair": {"go/cmd/evolve/cmd_inbox.go", "go/cmd/evolve/cmd_inbox_test.go"},
		"whole non-hub dir":   {"go/internal/inboxfile"},
		"empty files":         nil,
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rc := newRecordingCenter()
			got := WarnUnnamedHubFiles(inboxbatch.Item{ID: "clean-item", Files: files}, rc.c)
			rc.c.Flush()
			if len(got) != 0 || len(rc.events) != 0 {
				t.Fatalf("got hubs=%v events=%+v, want none", got, rc.events)
			}
		})
	}
	if got := WarnUnnamedHubFiles(inboxbatch.Item{ID: "nil-signals", Files: []string{"go/cmd/evolve"}}, nil); !slices.Equal(got, []string{"go/cmd/evolve"}) {
		t.Fatalf("nil signals returned %v, want [go/cmd/evolve]", got)
	}
}

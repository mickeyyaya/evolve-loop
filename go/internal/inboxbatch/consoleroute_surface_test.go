package inboxbatch_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// decode goes through json.Unmarshal, as every production reader does, so mentions are derived.
func decode(t *testing.T, record string) inboxbatch.Item {
	t.Helper()
	var it inboxbatch.Item
	if err := json.Unmarshal([]byte(record), &it); err != nil {
		t.Fatalf("decode %s: %v", record, err)
	}
	return it
}

func routed(t *testing.T, record string) (bool, string) {
	t.Helper()
	return inboxbatch.ConsoleRouted(decode(t, record), guards.IsProtectedScope)
}

func TestConsoleRouted_ReplaysTheEighteenRefusals(t *testing.T) {
	cases := []struct {
		name, record, wantReason string
	}{
		{"verdict-sentinel (9 refusals): kind",
			`{"id":"verdict-sentinel-as-tool-call","kind":"pipeline-integrity","summary":"verdict rides go/internal/bridge/streamjson_verdict.go"}`,
			"kind:pipeline-integrity"},
		{"tokenopt (4 refusals): a declared directory holding protected files",
			`{"id":"tokenopt-handoff-digests-per-edge-remainder","kind":"feature","files":["go/internal/core/","go/internal/phases/runner/","go/internal/phases/"]}`,
			"protected fix surface: go/internal/core/"},
		{"settle-wait (3 refusals): no files[], the text names runner.go",
			`{"id":"nonconforming-deliverable-settle-wait-latency","kind":"perf","files":[],"fix":"bound the settle wait in go/internal/phases/runner/runner.go"}`,
			"mentions protected path: go/internal/phases/runner/runner.go"},
		{"integration-tier (1 refusal): kind",
			`{"id":"integration-tier-tmux-live-dispatch-exclusion","kind":"pipeline-repair","notes":"go/internal/bridge/driver_tmux_repl.go"}`,
			"kind:pipeline-repair"},
		{"explanation-identity (1 refusal): kind",
			`{"id":"explanation-identity-belief-remaining-copies","kind":"pipeline-repair","root_cause":"go/internal/explanationdocs/explanationdocs.go"}`,
			"kind:pipeline-repair"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ok, reason := routed(t, tc.record); !ok || !strings.HasPrefix(reason, tc.wantReason) {
				t.Errorf("ConsoleRouted = %v %q, want console with reason prefix %q", ok, reason, tc.wantReason)
			}
		})
	}
}

func TestConsoleRouted_UndeclaredSurfaceIsTheFilesTheTextNames(t *testing.T) {
	for _, field := range []string{"title", "summary", "fix", "notes", "root_cause", "problem", "details"} {
		rec := `{"id":"x","kind":"bug","` + field + `":"see go/internal/bridge/autorespond.go for the stall"}`
		if ok, reason := routed(t, rec); !ok || !strings.Contains(reason, "go/internal/bridge/autorespond.go") {
			t.Errorf("%s: ConsoleRouted = %v %q, want console naming the mentioned file", field, ok, reason)
		}
	}
	if ok, reason := routed(t, `{"id":"y","kind":"bug","fix":"tighten go/internal/inboxbatch/item.go and docs/architecture/x.md"}`); ok {
		t.Errorf("unprotected file mentions routed console: %q", reason)
	}
	// JSON may escape the slash as "\/".
	if ok, _ := routed(t, `{"id":"z","kind":"bug","fix":"see go\/internal\/bridge\/autorespond.go"}`); !ok {
		t.Error(`an escaped "\/" path must still be read as a mention`)
	}
}

func TestConsoleRouted_DirectoryMentionsAreContextNotSurface(t *testing.T) {
	rec := `{"id":"d","kind":"bug","summary":"reproduced from /Users/x/console/go with go/cmd/evolve; the stall shows in go/internal/core and github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"}`
	if ok, reason := routed(t, rec); ok {
		t.Errorf("directory mentions routed console: %q", reason)
	}
}

func TestConsoleRouted_MachineFieldsAreNotTheItemsText(t *testing.T) {
	rec := `{"id":"m","kind":"bug","routed_reason":"names go/internal/bridge/x.go","continuation":{"note":"go/internal/core/cyclerun.go"}}`
	if ok, reason := routed(t, rec); ok {
		t.Errorf("machine-written fields routed console: %q", reason)
	}
}

func TestConsoleRouted_DeclaredSurfaceWinsOverMentions(t *testing.T) {
	rec := `{"id":"w","kind":"bug","files":["go/internal/inboxbatch/item.go"],"summary":"the stall shows in go/internal/bridge/autorespond.go"}`
	if ok, reason := routed(t, rec); ok {
		t.Errorf("declared unprotected surface routed console by a prose mention: %q", reason)
	}
}

func TestConsoleRouted_PlaceholdersDeclareNothing(t *testing.T) {
	for _, files := range []string{`["TBD"]`, `["N/A"]`, `["()"]`, `["role.go"]`, `["see notes"]`} {
		rec := `{"id":"p","kind":"bug","files":` + files + `,"fix":"tighten go/internal/guards/role.go"}`
		if ok, reason := routed(t, rec); !ok || !strings.Contains(reason, "go/internal/guards/role.go") {
			t.Errorf("files=%s: ConsoleRouted = %v %q, want console via the text's protected file", files, ok, reason)
		}
	}
}

func TestItem_DeclaredSurface(t *testing.T) {
	cases := map[string]bool{
		`[]`: false, `[""]`: false, `["TBD"]`: false, `["N/A"]`: false, `["w/o"]`: false,
		`["()"]`: false, `["role.go"]`: false,
		`["go/internal/core/"]`: true, `["docs/x.md"]`: true, `["go/internal/inboxbatch"]`: true,
		`["(see go/internal/inboxbatch/item.go)"]`: true,
		`["skills/audit"]`:                         true, `["go/"]`: true, `["a/b"]`: false,
	}
	for files, want := range cases {
		if got := decode(t, `{"id":"s","files":`+files+`}`).DeclaredSurface(); got != want {
			t.Errorf("files=%s: DeclaredSurface() = %v, want %v", files, got, want)
		}
	}
}

func TestConsoleRouted_MentionDerivationKeepsTheClamp(t *testing.T) {
	const text = `"fix":"edit go/internal/bridge/autorespond.go"`
	if ok, _ := routed(t, `{"id":"a","kind":"bug","route":"lane",`+text+`}`); ok {
		t.Error("operator-authored route:lane override was not honored")
	}
	if ok, reason := routed(t, `{"id":"b","kind":"bug","route":"lane","injected_by":"loop-escalation",`+text+`}`); !ok || !strings.Contains(reason, "route:lane ignored") {
		t.Errorf("agent-autofiled route:lane widened authority: %v %q", ok, reason)
	}
	if ok, _ := inboxbatch.ConsoleRouted(decode(t, `{"id":"c","kind":"bug",`+text+`}`), nil); ok {
		t.Error("nil predicate must disable the mention derivation")
	}
}

func TestConsoleRouted_AnnotationWordsAreNotSurface(t *testing.T) {
	if ok, reason := routed(t, `{"id":"n","kind":"bug","files":["go/internal/triagecap/topn_width.go (go test)"]}`); ok {
		t.Errorf("an annotation word routed console: %q", reason)
	}
	for _, files := range []string{`["skills/audit"]`, `["go/"]`} {
		if ok, reason := routed(t, `{"id":"s","kind":"bug","files":`+files+`}`); !ok || !strings.HasPrefix(reason, "protected fix surface: ") {
			t.Errorf("files=%s: ConsoleRouted = %v %q, want console (the directory holds protected surface)", files, ok, reason)
		}
	}
}

func TestItem_UnmarshalJSON_DerivesTheMentionedSurface(t *testing.T) {
	var it inboxbatch.Item
	if err := it.UnmarshalJSON([]byte(`{"id":"u","kind":"bug","fix":"see go/internal/bridge/autorespond.go"}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if it.ID != "u" || it.Kind != "bug" {
		t.Errorf("decoded fields = %q/%q, want u/bug", it.ID, it.Kind)
	}
	if ok, _ := inboxbatch.ConsoleRouted(it, guards.IsProtectedScope); !ok {
		t.Error("the decoded item lost the protected file its text names")
	}
	var bad inboxbatch.Item
	if err := bad.UnmarshalJSON([]byte(`{"id":`)); err == nil {
		t.Error("a malformed record must be an error")
	}
}

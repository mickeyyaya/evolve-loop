//go:build acs

package cycle2118

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func runNamedTests(t *testing.T, pkg, pattern string, names ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", pattern, "-v", pkg)
	if err != nil || code != 0 {
		t.Errorf("`-run %s %s` is not green (exit=%d err=%v)\n%s\n%s",
			pattern, pkg, code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "=== RUN   "+name) {
			t.Errorf("`-run %s %s` never selected %s — the fixture that proves this criterion did not run", pattern, pkg, name)
		}
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not pass under `-run %s %s`", name, pattern, pkg)
		}
	}
}

func TestC2118_001_ConsumedRecordParsesTheTrackedCorpus(t *testing.T) {
	corpus := filepath.Join(acsassert.RepoRoot(t), ".evolve", "inbox", "consumed")
	entries, err := os.ReadDir(corpus)
	if err != nil || len(entries) == 0 {
		t.Fatalf("consumed corpus %s unreadable or empty: %v", corpus, err)
	}
	fields := map[string]bool{}
	rt := reflect.TypeOf(inboxmover.ConsumedRecord{})
	for i := 0; i < rt.NumField(); i++ {
		fields[strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	objects := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(corpus, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		rec, ok, perr := inboxmover.ParseConsumedRecord(body)
		if perr != nil {
			t.Errorf("%s: ParseConsumedRecord: %v", e.Name(), perr)
			continue
		}
		var doc struct {
			Consumed map[string]any `json:"consumed"`
		}
		if json.Unmarshal(body, &doc) != nil || doc.Consumed == nil {
			continue
		}
		objects++
		if !ok {
			t.Errorf("%s: a consumed object parsed as absent", e.Name())
		}
		for k := range doc.Consumed {
			if !fields[k] {
				t.Errorf("%s: corpus key %q has no ConsumedRecord field — consume cycles would re-derive it by precedent", e.Name(), k)
			}
		}
		if want, _ := doc.Consumed["at"].(string); rec.At != want {
			t.Errorf("%s: At = %q, want %q", e.Name(), rec.At, want)
		}
		if want, _ := doc.Consumed["via"].(string); rec.Via != want {
			t.Errorf("%s: Via = %q, want %q", e.Name(), rec.Via, want)
		}
		if want, _ := doc.Consumed["cycle"].(string); rec.Cycle != want {
			t.Errorf("%s: Cycle = %q, want %q", e.Name(), rec.Cycle, want)
		}
	}
	if objects < 100 {
		t.Fatalf("only %d consumed objects found; the tracked corpus holds hundreds", objects)
	}
}

func TestC2118_002_ConsumedRecordNormalisesCycleAndLegacyShapes(t *testing.T) {
	cases := []struct {
		name    string
		item    string
		want    inboxmover.ConsumedRecord
		wantOK  bool
		wantErr bool
	}{
		{"numeric cycle becomes decimal text", `{"consumed":{"cycle":5}}`, inboxmover.ConsumedRecord{Cycle: "5"}, true, false},
		{"console cycle label", `{"consumed":{"cycle":"console"}}`, inboxmover.ConsumedRecord{Cycle: "console"}, true, false},
		{"bare timestamp is the consume time", `{"consumed":"2026-09-01T12:30:00Z"}`, inboxmover.ConsumedRecord{At: "2026-09-01T12:30:00Z"}, true, false},
		{"null is not consumed", `{"consumed":null}`, inboxmover.ConsumedRecord{}, false, false},
		{"absent is not consumed", `{"id":"x"}`, inboxmover.ConsumedRecord{}, false, false},
		{"array is a schema error", `{"consumed":["x"]}`, inboxmover.ConsumedRecord{}, false, true},
		{"malformed item is an error", `{"consumed":`, inboxmover.ConsumedRecord{}, false, true},
	}
	for _, c := range cases {
		got, ok, err := inboxmover.ParseConsumedRecord([]byte(c.item))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if ok != c.wantOK || got != c.want {
			t.Errorf("%s: ParseConsumedRecord(%s) = %#v, %v; want %#v, %v", c.name, c.item, got, ok, c.want, c.wantOK)
		}
	}
}

func TestC2118_003_ConsumedRecordIsCalledFromAProductionPath(t *testing.T) {
	pkgDir := filepath.Join(goDir(t), "internal", "inboxmover")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	var callers []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "consumed_record.go" {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "ParseConsumedRecord" {
					callers = append(callers, name)
				}
			}
			return true
		})
	}
	if len(callers) == 0 {
		t.Errorf("ParseConsumedRecord has no production caller in internal/inboxmover: a type only its own test reads is dead code, and retiredAtCycle keeps re-deriving the consumed fields")
	}
}

func TestC2118_004_ConsumedBindingReconcileBehaviourIsPreserved(t *testing.T) {
	runNamedTests(t, "./internal/inboxmover", "TestReconcileConsumedBindings")
}

func TestC2118_005_ClosesInboxMarkerRejectsQuotedText(t *testing.T) {
	cases := []struct{ name, body string }{
		{"backtick fence", "Upstream said:\n```\nCloses-Inbox: quoted-item\n```\n"},
		{"tilde fence", "~~~\nCloses-Inbox: quoted-item\n~~~\n"},
		{"fence with info string and bullet", "```markdown\n- Closes-Inbox: quoted-item\n```\n"},
		{"unclosed fence swallows the rest", "```\nCloses-Inbox: quoted-item\n"},
		{"space indented", "  Closes-Inbox: quoted-item\n"},
		{"tab indented", "\tCloses-Inbox: quoted-item\n"},
		{"nested bullet", "- upstream finding\n  - Closes-Inbox: quoted-item\n"},
		{"blockquote", "> Closes-Inbox: quoted-item\n"},
	}
	for _, c := range cases {
		if got := inboxmover.ClosesInboxIDs([]byte(c.body)); len(got) != 0 {
			t.Errorf("%s: ClosesInboxIDs(%q) = %#v, want none — quoted upstream text must not forge a closure claim", c.name, c.body, got)
		}
	}
}

func TestC2118_006_ClosesInboxMarkerStillHonoursColumnZeroClaims(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"plain", "Closes-Inbox: real-item\n", []string{"real-item"}},
		{"column-zero bullet", "- Closes-Inbox: real-item\n", []string{"real-item"}},
		{"CRLF and case", "closes-inbox: real-item\r\n", []string{"real-item"}},
		{"after a closed fence", "```\nCloses-Inbox: quoted-item\n```\nCloses-Inbox: real-item\n", []string{"real-item"}},
		{"before a fence", "Closes-Inbox: real-item\n```\nCloses-Inbox: quoted-item\n```\n", []string{"real-item"}},
	}
	for _, c := range cases {
		if got := inboxmover.ClosesInboxIDs([]byte(c.body)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ClosesInboxIDs(%q) = %#v, want %#v — hardening must not drop genuine claims", c.name, c.body, got, c.want)
		}
	}
}

func TestC2118_007_QuotedMarkerConsumesNothingAtShip(t *testing.T) {
	runNamedTests(t, "./internal/phases/ship",
		"TestPromoteInbox_(QuotedClosesInboxMarkerConsumesNothing|ColumnZeroMarkerAfterFenceStillConsumes)",
		"TestPromoteInbox_QuotedClosesInboxMarkerConsumesNothing",
		"TestPromoteInbox_ColumnZeroMarkerAfterFenceStillConsumes",
	)
}

func TestC2118_008_ReleaseStampRedactsHostPaths(t *testing.T) {
	runNamedTests(t, "./internal/inboxmover/lifecycle",
		"TestMover_Release_ContinuationStampRedactsHostPaths",
		"TestMover_Release_ContinuationStampRedactsHostPaths",
	)
}

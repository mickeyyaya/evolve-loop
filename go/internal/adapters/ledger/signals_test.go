package ledger

// signals_test.go — ADR-0101 S4a: the Signal Center observes the file ledger
// at its ONE append chokepoint. Every core.LedgerEntry written through Append
// — the orchestrator's records, the bridge's stop_review, the inbox mover's
// lifecycle lines (AppendLifecycle), the seal's segment anchor — is also a
// ledger.appended INFO signal naming its ledger line; a failed Append is a
// WARN LEDGER_APPEND_FAILED and the error still returns. The observer is
// installed by the WithSignals construction option, not by a wrapper type:
// Go embedding promotes methods without virtual dispatch, so a Decorator over
// *FileLedger let AppendLifecycle and Seal append unobserved (S4a
// architecture review HIGH-1). The go/ast guard at the end keeps every line
// writer either on the chokepoint or on the documented exempt inventory.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingLedgerSignals() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

// entriesBySeq reads the whole chain back (sealed segments + live tail) so a
// signal can be matched to the line it names.
func entriesBySeq(t *testing.T, l *FileLedger) map[int]core.LedgerEntry {
	t.Helper()
	lines, err := l.gatherAllLines()
	if err != nil {
		t.Fatalf("gatherAllLines: %v", err)
	}
	out := map[int]core.LedgerEntry{}
	for _, line := range lines {
		if _, e, err := decodeLedgerLine(line); err == nil {
			out[e.EntrySeq] = e
		}
	}
	return out
}

func TestWithSignals_EveryEntryThroughAppendIsALedgerAppendedSignal(t *testing.T) {
	t.Parallel()
	c, got := recordingLedgerSignals()
	l := New(t.TempDir(), WithSignals(c))
	if !l.SignalsWired() {
		t.Fatal("the observer reports its Center wired")
	}
	ctx := context.Background()
	if err := l.Append(ctx, core.LedgerEntry{TS: "2026-09-13T00:00:00Z", Cycle: 1632, Role: "ship", Kind: "ship_error", ExitCode: 1, ArtifactPath: "/ws/ship-error.json"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := l.AppendLifecycle(ctx, LifecycleRecord{TS: "2026-09-13T00:00:01Z", Action: "claim", TaskID: "poison", Cycle: 1632}); err != nil {
		t.Fatalf("AppendLifecycle: %v", err)
	}
	// Sealing the first line into a segment appends the segment anchor through
	// the same chokepoint.
	if err := l.Seal(ctx, 1); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("the observed ledger still chains: %v", err)
	}
	if len(*got) != 3 {
		t.Fatalf("one ledger.appended per entry (append, lifecycle, seal anchor), got %d: %+v", len(*got), *got)
	}
	e := (*got)[0]
	if e.Module != signalcenter.ModuleLedger || e.Kind != signalcenter.KindLedgerAppended || e.Severity != signalcenter.SeverityInfo || e.Code != "" ||
		e.Cycle != 1632 || e.Origin != "FileLedger.Append" || e.Fields["role"] != "ship" || e.Fields["kind"] != "ship_error" || e.Fields["exit_code"] != "1" ||
		e.Fields["path"] != "/ws/ship-error.json" || e.Reason != "ledger: ship ship_error" {
		t.Errorf("the signal names the entry (role, kind, exit code, artifact): %+v", e)
	}
	if life := (*got)[1]; life.Fields["kind"] != "inbox-lifecycle" || life.Fields["role"] != "orchestrator" || life.Cycle != 1632 {
		t.Errorf("the promoted lifecycle append is observed too (HIGH-1): %+v", life)
	}
	if anchor := (*got)[2]; anchor.Fields["kind"] != SealKind || anchor.Fields["role"] != "operator" {
		t.Errorf("the seal's segment anchor is observed too: %+v", anchor)
	}
	// Every signal names the ledger line it reports: fields.entry_seq is the
	// line's chained sequence number.
	lines := entriesBySeq(t, l)
	for _, e := range *got {
		seq, err := strconv.Atoi(e.Fields["entry_seq"])
		if err != nil {
			t.Fatalf("fields.entry_seq names the chained line: %+v", e)
		}
		if line, ok := lines[seq]; !ok || line.Kind != e.Fields["kind"] {
			t.Errorf("signal seq %d does not match a ledger line of kind %q (live+sealed: %v)", seq, e.Fields["kind"], lines)
		}
	}
}

func TestWithSignals_AppendFailureIsAWarnAndTheErrorStillReturns(t *testing.T) {
	t.Parallel()
	c, got := recordingLedgerSignals()
	notADir := filepath.Join(t.TempDir(), "evolve-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(notADir, WithSignals(c))
	err := l.Append(context.Background(), core.LedgerEntry{Cycle: 5, Role: "audit", Kind: "audit_verdict"})
	if err == nil {
		t.Fatal("an unwritable ledger must fail the Append")
	}
	if len(*got) != 1 {
		t.Fatalf("one LEDGER_APPEND_FAILED, got %d", len(*got))
	}
	e := (*got)[0]
	if e.Kind != signalcenter.KindLedgerAppended || e.Severity != signalcenter.SeverityWarn || e.Code != CodeLedgerAppendFailed || e.Cycle != 5 ||
		!strings.Contains(e.Reason, err.Error()) || e.Fields["role"] != "audit" {
		t.Errorf("a failed append is a WARN carrying the error and the entry: %+v", e)
	}
	if _, named := e.Fields["entry_seq"]; named {
		t.Errorf("a failed append names no ledger line: %+v", e)
	}
}

// Options apply in order at construction: the later observer wins, so a root
// that composes options cannot end up with two Centers observing one ledger.
func TestOption_AppliesInOrderTheLaterObserverWins(t *testing.T) {
	t.Parallel()
	first, gotFirst := recordingLedgerSignals()
	second, gotSecond := recordingLedgerSignals()
	opts := []Option{WithSignals(first), WithSignals(second)}
	l := New(t.TempDir(), opts...)
	if err := l.Append(context.Background(), core.LedgerEntry{Cycle: 2, Role: "scout", Kind: "scout_done"}); err != nil {
		t.Fatal(err)
	}
	if len(*gotFirst) != 0 || len(*gotSecond) != 1 {
		t.Errorf("the last WithSignals option installs the observer: first=%d second=%d", len(*gotFirst), len(*gotSecond))
	}
}

func TestWithSignals_NilCenterIsTheNullObject(t *testing.T) {
	t.Parallel()
	plain := New(t.TempDir())
	if plain.SignalsWired() {
		t.Error("a ledger built without the option is unobserved")
	}
	l := New(t.TempDir(), WithSignals(nil))
	if l.SignalsWired() {
		t.Error("nil is reported unwired")
	}
	if err := l.Append(context.Background(), core.LedgerEntry{Cycle: 1, Role: "scout", Kind: "scout_done"}); err != nil {
		t.Fatalf("Append through the nil Null Object still appends: %v", err)
	}
}

// Every function in this package that writes a ledger line either reaches
// the Append chokepoint (where the observer sits) or is on this inventory
// with its reason — so the next promoted or self-constructed writer cannot
// be forgotten silently. The inventory is the honest list of lines the
// Signal Center does NOT see; shrinking it is S4b work.
func TestFileLedger_EveryLineWriterReachesTheAppendChokepointOrIsInventoried(t *testing.T) {
	t.Parallel()
	exempt := map[string]string{
		"Append":                  "the chokepoint itself — the observer runs here",
		"Rebaseline":              "evolve ledger rebaseline: an operator repair root outside the orchestrator process; the marker chains from the physical tail",
		"WriteCompositionVerdict": "a composition record (not a core.LedgerEntry) written by a self-constructed ledger; S4b threads the root's ledger",
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	type writer struct {
		raw     bool            // calls a raw chained-line writer directly
		callees map[string]bool // same-receiver method calls
	}
	writers := map[string]*writer{}
	rawWriters := map[string]bool{"appendChained": true, "appendChainedFromTail": true, "appendLineAndReplaceTip": true}
	for _, file := range pkgs["ledger"].Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			w := &writer{callees: map[string]bool{}}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr: // l.method(…) / pkg.Func(…)
					name = fn.Sel.Name
				case *ast.Ident: // a plain helper(…) of this package (re-review MEDIUM-A)
					name = fn.Name
				}
				switch {
				case name == "":
				case rawWriters[name]:
					w.raw = true
				case name != "Append": // a call INTO the chokepoint is observed; stop there
					w.callees[name] = true
				}
				return true
			})
			writers[fd.Name.Name] = w
		}
	}
	// reachesRaw walks same-package calls (methods and functions share the
	// name space here) without passing through Append.
	var reachesRaw func(name string, seen map[string]bool) bool
	reachesRaw = func(name string, seen map[string]bool) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		w, ok := writers[name]
		if !ok {
			return false
		}
		if w.raw {
			return true
		}
		for callee := range w.callees {
			if reachesRaw(callee, seen) {
				return true
			}
		}
		return false
	}
	for name := range writers {
		if !ast.IsExported(name) {
			continue
		}
		if _, listed := exempt[name]; listed {
			continue
		}
		if reachesRaw(name, map[string]bool{}) {
			t.Errorf("%s writes a ledger line without passing through Append — decorate it through the chokepoint or add it to the inventory with a reason", name)
		}
	}
	for name := range exempt {
		if _, present := writers[name]; !present {
			t.Errorf("inventory entry %q no longer exists; drop it", name)
		}
	}
}

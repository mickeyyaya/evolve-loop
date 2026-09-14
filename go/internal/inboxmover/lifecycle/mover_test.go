package lifecycle

// mover_test.go — the Mover's construction, its Null-Object defaults and the
// ONE producer's two links (ADR-0103 unit 06 §6 tests 19-21).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// recordingCenter captures every event a Mover emits.
type recordingCenter struct {
	c      *signalcenter.Center
	events []signalcenter.Event
}

func newRecordingCenter() *recordingCenter {
	r := &recordingCenter{c: signalcenter.New()}
	r.c.Subscribe(func(e signalcenter.Event) { r.events = append(r.events, e) })
	return r
}

func (r *recordingCenter) accessor() func() *signalcenter.Center {
	return func() *signalcenter.Center { return r.c }
}

func (r *recordingCenter) codes() []signalcenter.Code {
	var out []signalcenter.Code
	for _, e := range r.events {
		out = append(out, e.Code)
	}
	return out
}

// recordingAppender captures lifecycle records; fail makes every append err.
type recordingAppender struct {
	records []ledger.LifecycleRecord
	fail    error
	onEach  func()
}

func (r *recordingAppender) AppendLifecycle(_ context.Context, rec ledger.LifecycleRecord) error {
	if r.onEach != nil {
		r.onEach()
	}
	if r.fail != nil {
		return r.fail
	}
	r.records = append(r.records, rec)
	return nil
}

var fixedClock = time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

func newInbox(t *testing.T) string {
	t.Helper()
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	return inbox
}

func writeItem(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func procPath(inbox string, cycle int, name string) string {
	return filepath.Join(inbox, "processing", "cycle-"+strconv.Itoa(cycle), name)
}

func tmpPathOf(path string) string { return path + ".tmp." + strconv.Itoa(os.Getpid()) }

// faultLines returns the fallback-link lines (the legacy WARN/ERROR tokens)
// in stderr, in order — the INFO lines are kept lines and print on both links.
func faultLines(stderr string) []string {
	var out []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, LegacyPrefix+"WARN: ") || strings.HasPrefix(line, LegacyPrefix+"ERROR: ") {
			out = append(out, line)
		}
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Test 19 — New(dir, nil) is a working Null-Object Mover: no Center, no ledger,
// no retire hook, no active-cycle reader, no landing probe, no run workspace;
// every With*(nil) keeps the default.
func TestNew_NullObjectDefaults(t *testing.T) {
	inbox := newInbox(t)
	m := New(inbox, nil, WithStderr(nil), WithNow(nil), WithActiveCycle(nil), WithLanded(nil), WithRetire(nil),
		WithProtectedPath(nil), WithRunWorkspace(nil), WithSignals(nil))
	if m.SignalsWired() {
		t.Error("no accessor ⇒ not wired")
	}
	if _, err := m.Claim("ghost", "1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Claim on an empty inbox: %v", err)
	}
	writeItem(t, procPath(inbox, 1, "a.json"), `{"id":"a"}`)
	writeItem(t, procPath(inbox, 2, "b.json"), `{"id":"b"}`)
	if res, err := m.RecoverOrphans(); err != nil || res.Recovered != 2 {
		t.Errorf("nil active-cycle reader ⇒ every dir recovers: %+v %v", res, err)
	}
	if res, err := m.Promote("a", "processed", PromoteOpts{Cycle: "1", CommitSHA: "deadbeef00"}); err != nil || res.NoOp || !strings.Contains(res.DestPath, "processed/cycle-1/deadbeef-a.json") {
		t.Errorf("nil landing probe ⇒ the gate is not consulted (landed): %+v %v", res, err)
	}
	writeItem(t, procPath(inbox, 3, "c.json"), `{"id":"c"}`)
	if res, err := m.Release(3, "", nil); err != nil || res.Recovered != 1 {
		t.Errorf("nil run workspace ⇒ no manifest read, the item releases: %+v %v", res, err)
	}
	nilCenter := New(inbox, nil, WithSignals(func() *signalcenter.Center { return nil }))
	if nilCenter.SignalsWired() {
		t.Error("an accessor returning nil ⇒ not wired")
	}
	wired := New(inbox, nil, WithSignals(newRecordingCenter().accessor()))
	if !wired.SignalsWired() {
		t.Error("an accessor returning a Center ⇒ wired")
	}
}

// Test 19b — every With*(nil) is a no-op AFTER a real seam too: the eight
// options share one idiom (nil keeps what is installed), so a stray nil in an
// option list never clobbers the predicate or the run-workspace spelling.
func TestNew_NilOptionNeverClobbersAnInstalledSeam(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "p.json"), `{"id":"p","files":["go/internal/guards/role.go (fix)"]}`)
	ws := filepath.Join(inbox, "..", "runs", "cycle-3")
	mkdirAll(t, filepath.Join(ws, "continuation-manifest.json"))
	mkdirAll(t, procPath(inbox, 3, ""))
	var stderr strings.Builder
	m := New(inbox, nil, WithStderr(&stderr),
		WithProtectedPath(func(string) bool { return true }), WithProtectedPath(nil),
		WithRunWorkspace(func(int) string { return ws }), WithRunWorkspace(nil))
	if _, err := m.Claim("p", "3"); !errors.Is(err, ErrConsoleRouted) {
		t.Errorf("WithProtectedPath(nil) after a real predicate keeps the predicate: %v", err)
	}
	if _, err := m.Release(3, "", nil); err != nil || !strings.Contains(stderr.String(), "continuation manifest unreadable for cycle 3") {
		t.Errorf("WithRunWorkspace(nil) after a real spelling keeps the spelling: %v %q", err, stderr.String())
	}
}

// warnRow provokes one fault class and names what the two links must produce.
type warnRow struct {
	name       string
	arrange    func(t *testing.T, inbox string) []Option
	act        func(m *Mover)
	origin     string
	cycle      int
	wantCodes  []signalcenter.Code
	wantSteps  []string
	wantStderr func(inbox string) []string
}

func warnRows() []warnRow {
	return []warnRow{
		{name: "claim not found", act: func(m *Mover) { _, _ = m.Claim("t1", "7") }, origin: "Mover.Claim", cycle: 7,
			wantCodes: []signalcenter.Code{CodeClaimNotFound}, wantSteps: []string{"locate"},
			wantStderr: func(inbox string) []string {
				return []string{"[inbox-mover] WARN: claim: task 't1' not found in " + inbox}
			}},
		{name: "claim refused", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, filepath.Join(inbox, "c1.json"), `{"id":"c1","route":"console-manual"}`)
			return nil
		}, act: func(m *Mover) { _, _ = m.Claim("c1", "7") }, origin: "Mover.Claim", cycle: 7,
			wantCodes: []signalcenter.Code{CodeClaimRefused}, wantSteps: []string{"route"},
			wantStderr: func(string) []string {
				return []string{"[inbox-mover] WARN: claim: task 'c1' REFUSED — route:console-manual (operator-owned; lanes must not draw it)"}
			}},
		{name: "claim mkdir failed", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
			writeItem(t, filepath.Join(inbox, "processing"), "x")
			return nil
		}, act: func(m *Mover) { _, _ = m.Claim("t1", "7") }, origin: "Mover.Claim", cycle: 7,
			wantCodes: []signalcenter.Code{CodeClaimMoveFailed}, wantSteps: []string{"mkdir"},
			wantStderr: func(inbox string) []string {
				return []string{"[inbox-mover] ERROR: claim: mkdir -p '" + filepath.Join(inbox, "processing", "cycle-7") + "' failed: mkdir " + filepath.Join(inbox, "processing") + ": not a directory"}
			}},
		{name: "promote not found", act: func(m *Mover) { _, _ = m.Promote("ghost", "processed", PromoteOpts{Cycle: "3"}) },
			origin: "Mover.Promote", cycle: 3, wantCodes: []signalcenter.Code{CodePromoteNotFound}, wantSteps: []string{"locate"},
			wantStderr: func(string) []string {
				return []string{"[inbox-mover] WARN: promote: task 'ghost' not found in processing/ or inbox/ — already moved?"}
			}},
		{name: "promote unlanded sha", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
			return []Option{WithLanded(func(string) (bool, error) { return false, nil })}
		}, act: func(m *Mover) { _, _ = m.Promote("t1", "processed", PromoteOpts{Cycle: "3", CommitSHA: "deadbeef00"}) },
			origin: "Mover.Promote", cycle: 3, wantCodes: []signalcenter.Code{CodePromoteUnlandedSHA}, wantSteps: []string{"landing"},
			wantStderr: func(string) []string {
				return []string{"[inbox-mover] WARN: promote: ship SHA deadbeef00 for 't1' not landed on main — rerouting to retry/ instead of processed/"}
			}},
		{name: "promote mkdir failed", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
			writeItem(t, filepath.Join(inbox, "processed"), "x")
			return nil
		}, act: func(m *Mover) { _, _ = m.Promote("t1", "processed", PromoteOpts{Cycle: "3"}) },
			origin: "Mover.Promote", cycle: 3, wantCodes: []signalcenter.Code{CodePromoteMoveFailed}, wantSteps: []string{"mkdir"},
			wantStderr: func(inbox string) []string {
				return []string{"[inbox-mover] ERROR: promote: mkdir -p '" + filepath.Join(inbox, "processed", "cycle-3") + "' failed — leaving file in inbox/: mkdir " + filepath.Join(inbox, "processed") + ": not a directory"}
			}},
		{name: "landed check failed", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
			return []Option{WithLanded(func(string) (bool, error) { return false, errors.New("git exploded") })}
		}, act: func(m *Mover) { _, _ = m.Promote("t1", "processed", PromoteOpts{Cycle: "3", CommitSHA: "deadbeef00"}) },
			origin: "Mover.Promote", cycle: 3, wantCodes: []signalcenter.Code{CodeLandedCheckFailed}, wantSteps: []string{"landing"},
			wantStderr: func(string) []string {
				return []string{"[inbox-mover] WARN: promote: landed check for 't1' failed (git exploded) — treating deadbeef00 as landed (fail-open)"}
			}},
		{name: "release double move", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1-root"}`)
			writeItem(t, procPath(inbox, 3, "t1.json"), `{"id":"t1"}`)
			return nil
		}, act: func(m *Mover) { _, _ = m.Release(3, "", nil) }, origin: "Mover.Release", cycle: 3,
			wantCodes: []signalcenter.Code{CodeReleaseDoubleMove}, wantSteps: []string{"release_cycle"},
			wantStderr: func(string) []string {
				return []string{"[inbox-mover] WARN: release-cycle: t1.json already at inbox root (double-move for t1) — skipping"}
			}},
		{name: "recover move failed", arrange: func(t *testing.T, inbox string) []Option {
			mkdirAll(t, filepath.Join(inbox, "t1.json"))
			writeItem(t, procPath(inbox, 3, "t1.json"), `{"id":"t1"}`)
			return nil
		}, act: func(m *Mover) { _, _ = m.RecoverOrphans() }, origin: "Mover.RecoverOrphans", cycle: 3,
			wantCodes: []signalcenter.Code{CodeReleaseMoveFailed}, wantSteps: []string{"recover_orphans"},
			wantStderr: func(inbox string) []string {
				return []string{"[inbox-mover] WARN: recover-orphans: mv failed for t1.json (leaving in processing/): rename " + procPath(inbox, 3, "t1.json") + " " + filepath.Join(inbox, "t1.json") + ": file exists"}
			}},
		{name: "quarantine failed", arrange: func(t *testing.T, inbox string) []Option {
			writeItem(t, procPath(inbox, 3, "t1.json"), `{"id":"t1"}`)
			writeItem(t, filepath.Join(inbox, "quarantine"), "x")
			return nil
		}, act: func(m *Mover) { _, _ = m.Release(3, "", &Policy{Ceiling: 1}) }, origin: "Mover.Release", cycle: 3,
			wantCodes: []signalcenter.Code{CodePromoteMoveFailed, CodeQuarantineFailed}, wantSteps: []string{"mkdir", "quarantine"},
			wantStderr: func(inbox string) []string {
				return []string{
					"[inbox-mover] ERROR: promote: mkdir -p '" + filepath.Join(inbox, "quarantine") + "' failed — leaving file in processing/: mkdir " + filepath.Join(inbox, "quarantine") + ": not a directory",
					"[inbox-mover] ERROR: quarantine failed for 't1' (task-level failure #1 >= ceiling 1) — releasing to inbox root instead, it WILL be re-picked: inboxmover: mv failed: mkdir " + filepath.Join(inbox, "quarantine") + ": mkdir " + filepath.Join(inbox, "quarantine") + ": not a directory",
				}
			}},
		{name: "manifest unreadable", arrange: func(t *testing.T, inbox string) []Option {
			ws := filepath.Join(inbox, "..", "runs", "cycle-3")
			mkdirAll(t, filepath.Join(ws, "continuation-manifest.json"))
			mkdirAll(t, procPath(inbox, 3, ""))
			return []Option{WithRunWorkspace(func(int) string { return ws })}
		}, act: func(m *Mover) { _, _ = m.Release(3, "", nil) }, origin: "Mover.Release", cycle: 3,
			wantCodes: []signalcenter.Code{CodeContinuationManifestUnreadable}, wantSteps: []string{"manifest"},
			wantStderr: func(inbox string) []string {
				return []string{"[inbox-mover] WARN: release-cycle: continuation manifest unreadable for cycle 3: continuation: read manifest: read " + filepath.Join(inbox, "..", "runs", "cycle-3", "continuation-manifest.json") + ": is a directory (items release unstamped)"}
			}},
		{name: "counter reset failed", arrange: func(t *testing.T, inbox string) []Option {
			q := filepath.Join(inbox, "quarantine", "q.json")
			writeItem(t, q, `{"id":"q","failure_count":3}`)
			mkdirAll(t, tmpPathOf(q))
			return nil
		}, act: func(m *Mover) { _, _ = m.ReleaseFromQuarantine("q") }, origin: "Mover.ReleaseFromQuarantine", cycle: 0,
			wantCodes: []signalcenter.Code{CodeItemRewriteFailed}, wantSteps: []string{"counter_reset"},
			wantStderr: func(inbox string) []string {
				return []string{"[inbox-mover] WARN: quarantine-release: failure_count reset failed for 'q' (open " + tmpPathOf(filepath.Join(inbox, "quarantine", "q.json")) + ": is a directory) — released with its stale count"}
			}},
	}
}

// Test 20 — the one producer's two links: with a Center every fault is ONE
// inbox.warning WARN event under module inbox (origin, cycle, code, step,
// task_id) and no fallback line; without one the legacy line prints byte for
// byte and nothing is emitted.
func TestMover_Warn_TwoLinks(t *testing.T) {
	for _, row := range warnRows() {
		t.Run(row.name+"/wired", func(t *testing.T) {
			inbox := newInbox(t)
			var stderr strings.Builder
			rc := newRecordingCenter()
			opts := []Option{WithStderr(&stderr), WithSignals(rc.accessor())}
			if row.arrange != nil {
				opts = append(opts, row.arrange(t, inbox)...)
			}
			row.act(New(inbox, nil, opts...))
			if got := rc.codes(); len(got) != len(row.wantCodes) {
				t.Fatalf("codes = %v, want %v", got, row.wantCodes)
			}
			for i, e := range rc.events {
				if e.Code != row.wantCodes[i] || e.Module != signalcenter.ModuleInbox || e.Kind != signalcenter.KindInboxWarning ||
					e.Severity != signalcenter.SeverityWarn || e.Cycle != row.cycle || e.Fields["step"] != row.wantSteps[i] {
					t.Errorf("event %d = %+v; want %s under inbox/inbox.warning/WARN cycle=%d step=%s", i, e, row.wantCodes[i], row.cycle, row.wantSteps[i])
				}
				// Every per-item event names its task_id; the manifest fault is per cycle (workspace).
				if perItem := e.Code != CodeContinuationManifestUnreadable; perItem && e.Fields["task_id"] == "" || !perItem && e.Fields["workspace"] == "" {
					t.Errorf("event %d = %+v; want task_id on a per-item event, workspace on the manifest fault", i, e)
				}
				if i == len(rc.events)-1 && e.Origin != row.origin {
					t.Errorf("origin = %q, want %q", e.Origin, row.origin)
				}
				if e.Reason == "" || strings.HasPrefix(e.Reason, "[inbox-mover]") {
					t.Errorf("the reason is the legacy sentence without its prefix: %q", e.Reason)
				}
			}
			if lines := faultLines(stderr.String()); len(lines) != 0 {
				t.Errorf("a wired Mover prints no fallback line; got %v", lines)
			}
		})
		t.Run(row.name+"/legacy", func(t *testing.T) {
			inbox := newInbox(t)
			var stderr strings.Builder
			opts := []Option{WithStderr(&stderr)}
			if row.arrange != nil {
				opts = append(opts, row.arrange(t, inbox)...)
			}
			row.act(New(inbox, nil, opts...))
			if got, want := faultLines(stderr.String()), row.wantStderr(inbox); !equalStrings(got, want) {
				t.Errorf("fallback lines:\n got %q\nwant %q", got, want)
			}
		})
	}
}

// codeSteps is the code → fields.step table every registration doc must name.
var codeSteps = map[signalcenter.Code][]string{
	CodeClaimNotFound:                  {"locate"},
	CodeClaimRefused:                   {"route"},
	CodeClaimMoveFailed:                {"mkdir", "rename"},
	CodePromoteNotFound:                {"locate"},
	CodePromoteUnlandedSHA:             {"landing"},
	CodePromoteMoveFailed:              {"mkdir", "rename"},
	CodeLandedCheckFailed:              {"landing"},
	CodeReleaseDoubleMove:              {"release_cycle"},
	CodeReleaseMoveFailed:              {"release_cycle", "recover_orphans"},
	CodeQuarantineFailed:               {"quarantine"},
	CodeContinuationManifestUnreadable: {"manifest"},
	CodeItemRewriteFailed:              {"counter_reset", "failure_bump", "continuation_stamp"},
}

// Test 21 — every code is registered under module inbox with a doc that names
// its fields.step values; the real registry is conflict-free; no INBOX_ code
// doubles the ledger adapter's LEDGER_APPEND_FAILED.
func TestMover_Codes_RegisteredUnderInbox_DocsNameTheirSteps_NoLedgerDouble(t *testing.T) {
	if len(codeSteps) != 12 {
		t.Fatalf("the unit registers 12 codes, the table names %d", len(codeSteps))
	}
	docs := map[signalcenter.Code]string{}
	for _, cd := range signalcenter.RegisteredCodes()[signalcenter.ModuleInbox] {
		docs[cd.Code] = cd.Doc
	}
	for code, steps := range codeSteps {
		if owner, ok := signalcenter.IsRegistered(code); !ok || owner != signalcenter.ModuleInbox || !code.Valid() || !code.BelongsTo(signalcenter.ModuleInbox) {
			t.Errorf("%s: registered under %q (ok=%v)", code, owner, ok)
		}
		if docs[code] == "" {
			t.Errorf("%s: no doc", code)
		}
		for _, step := range steps {
			if !strings.Contains(docs[code], step) {
				t.Errorf("%s: the doc must name fields.step=%s: %q", code, step, docs[code])
			}
		}
	}
	for code := range docs {
		if strings.HasPrefix(string(code), "INBOX_LEDGER") {
			t.Errorf("%s doubles the ledger adapter's LEDGER_APPEND_FAILED", code)
		}
		if _, listed := codeSteps[code]; !listed {
			t.Errorf("%s is registered under inbox but not in the unit's table", code)
		}
	}
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("registry conflicts: %+v", conflicts)
	}
}

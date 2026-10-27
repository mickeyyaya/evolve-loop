package ship

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
)

type argvMatch func(args []string) bool

type gitFault struct {
	when   argvMatch
	exit   int
	err    error
	stdout string
	before func()
	limit  int
	hits   int
	calls  [][]string
	next   CmdRunner
}

func (f *gitFault) runner() CmdRunner {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" {
			f.calls = append(f.calls, append([]string(nil), args...))
		}
		if name == "git" && (f.limit == 0 || f.hits < f.limit) && f.when(gitSubArgs(args)) {
			f.hits++
			if f.before != nil {
				f.before()
			}
			if stdout != nil {
				_, _ = io.WriteString(stdout, f.stdout)
			}
			return f.exit, f.err
		}
		if f.next != nil {
			return f.next(ctx, name, cwd, args, env, stdin, stdout, stderr)
		}
		return 0, nil
	}
}

func gitSubArgs(args []string) []string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-C", "-c":
			i++
		default:
			return args[i:]
		}
	}
	return nil
}

func argvIs(want ...string) argvMatch {
	return func(args []string) bool { return reflect.DeepEqual(args, want) }
}

func argvStarts(want ...string) argvMatch {
	return func(args []string) bool { return len(args) >= len(want) && reflect.DeepEqual(args[:len(want)], want) }
}

func skipWhenRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions, so a read-only directory cannot fail a write")
	}
}

func readOnlyDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestRespond_MapsTheShipOutcomeToThePhaseVerdict(t *testing.T) {
	t.Parallel()
	req := core.PhaseRequest{Workspace: "/ws/cycle-9"}
	failure := shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip, "ship: git broke")
	cases := []struct {
		name        string
		res         RunResult
		err         error
		wantVerdict string
		wantErr     string
		wantDiag    string
		wantSignals map[string]any
		wantNext    string
		wantCommit  string
	}{
		{name: "an error fails with its ship-error and repair signals", err: failure,
			res:         RunResult{ExitCode: ExitFailure, RepairAttempted: "GIT_PUSH_REJECTED", RepairOutcome: "needs-reaudit"},
			wantVerdict: core.VerdictFAIL, wantErr: "ship: native: " + failure.Error(), wantDiag: failure.Error(),
			wantSignals: map[string]any{"ship.error_code": string(core.CodeGitIO), "ship.error_class": string(core.ShipClassTransient),
				"ship.error_stage": string(core.StageAtomicShip), "ship.debug": failure.(*core.ShipError).DebugString(),
				"ship.repair_attempted": "GIT_PUSH_REJECTED", "ship.repair_outcome": "needs-reaudit"}},
		{name: "a nonzero exit with no error fails with the logs as its diagnostic",
			res:         RunResult{ExitCode: ExitIntegrity, Logs: []string{"[ship] first", "[ship] second"}, CommitSHA: "abc"},
			wantVerdict: core.VerdictFAIL, wantErr: "ship: native exit=2", wantDiag: "[ship] first\n[ship] second"},
		{name: "a pass with no repair carries no signals", res: RunResult{ExitCode: ExitOK, CommitSHA: "c0ffee"},
			wantVerdict: core.VerdictPASS, wantNext: string(core.PhaseRetro), wantCommit: "c0ffee"},
		{name: "a self-healed pass carries the repair signals",
			res:         RunResult{ExitCode: ExitOK, CommitSHA: "beef", RepairAttempted: "GIT_PUSH_REJECTED", RepairOutcome: landingResumed},
			wantVerdict: core.VerdictPASS, wantNext: string(core.PhaseRetro), wantCommit: "beef",
			wantSignals: map[string]any{"ship.repair_attempted": "GIT_PUSH_REJECTED", "ship.repair_outcome": landingResumed}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := respond(req, tc.res, tc.err)

			if gotErr := errText(err); gotErr != tc.wantErr {
				t.Errorf("error = %q, want %q", gotErr, tc.wantErr)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("error %v does not wrap the ship error %v", err, tc.err)
			}
			if resp.Verdict != tc.wantVerdict || resp.NextPhase != tc.wantNext || resp.CommitSHA != tc.wantCommit || resp.ArtifactsDir != req.Workspace || resp.Phase != phaseName {
				t.Errorf("resp = %+v, want verdict %s next %q commit %q artifacts %q", resp, tc.wantVerdict, tc.wantNext, tc.wantCommit, req.Workspace)
			}
			if !reflect.DeepEqual(resp.Signals, tc.wantSignals) {
				t.Errorf("signals = %v, want %v", resp.Signals, tc.wantSignals)
			}
			gotDiag := ""
			if len(resp.Diagnostics) == 1 && resp.Diagnostics[0].Severity == "error" {
				gotDiag = resp.Diagnostics[0].Message
			}
			if gotDiag != tc.wantDiag || len(resp.Diagnostics) > 1 {
				t.Errorf("diagnostics = %+v, want one error diagnostic %q", resp.Diagnostics, tc.wantDiag)
			}
		})
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAdoptIntentConsumption_AcceptsOnlyWholeInboxPairs(t *testing.T) {
	t.Parallel()
	a, b := consumptionPairSlice("a.json"), consumptionPairSlice("2026-10-01T00-00-00Z-b.json")
	cases := []struct {
		name    string
		paths   []string
		wantErr string
	}{
		{name: "no paths adopts an empty set", paths: nil},
		{name: "one pair", paths: a},
		{name: "two pairs", paths: append(append([]string{}, a...), b...)},
		{name: "a lone source is not a pair", paths: a[:1], wantErr: "1 consumed path(s) are not whole inbox consumption pairs"},
		{name: "three paths are not whole pairs", paths: append(append([]string{}, a...), b[0]), wantErr: "3 consumed path(s)"},
		{name: "a swapped pair", paths: []string{a[1], a[0]}, wantErr: a[1] + " -> " + a[0] + " is not an inbox consumption pair"},
		{name: "a pair whose destination names another item", paths: []string{a[0], b[1]}, wantErr: "is not an inbox consumption pair"},
		{name: "a pair outside the inbox", paths: []string{"go/main.json", ".evolve/inbox/consumed/main.json"}, wantErr: "go/main.json -> .evolve/inbox/consumed/main.json"},
		{name: "a pair of a non-json item", paths: consumptionPairSlice("notes.md"), wantErr: "is not an inbox consumption pair"},
		{name: "a second pair that is broken", paths: append(append([]string{}, a...), b[0], a[1]), wantErr: b[0] + " -> " + a[1]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prior := []string{"prior/src.json", "prior/dst.json"}
			opts := &Options{internalConsumedPaths: prior}

			err := adoptIntentConsumption(opts, tc.paths)

			if errText(err) != tc.wantErr && (tc.wantErr == "" || err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			want := prior
			if tc.wantErr == "" {
				want = tc.paths
			}
			if len(want) == 0 && len(opts.internalConsumedPaths) == 0 {
				return
			}
			if !reflect.DeepEqual(opts.internalConsumedPaths, want) {
				t.Errorf("consumed paths = %v, want %v: a refusal keeps the sanction set, an adoption replaces it", opts.internalConsumedPaths, want)
			}
		})
	}
}

func consumptionPairSlice(name string) []string {
	src, dst := consumptionPair(name)
	return []string{src, dst}
}

func TestAdoptIntentConsumption_CopiesTheIntentPaths(t *testing.T) {
	t.Parallel()
	paths := consumptionPairSlice("x.json")
	opts := &Options{}

	if err := adoptIntentConsumption(opts, paths); err != nil {
		t.Fatal(err)
	}
	paths[0] = "mutated"

	if opts.internalConsumedPaths[0] != ".evolve/inbox/x.json" {
		t.Errorf("sanction set %v aliases the intent's slice; a later edit of the intent must not change what ship sanctions", opts.internalConsumedPaths)
	}
}

func FuzzAdoptIntentConsumption(f *testing.F) {
	f.Add(".evolve/inbox/a.json", ".evolve/inbox/consumed/a.json", false)
	f.Add(".evolve/inbox/a.json", ".evolve/inbox/consumed/b.json", false)
	f.Add("a.json", ".evolve/inbox/consumed/a.json", true)
	f.Add(".evolve/inbox/über ✓.json", ".evolve/inbox/consumed/über ✓.json", false)
	f.Add(".evolve/inbox/a.md", ".evolve/inbox/consumed/a.md", true)
	f.Fuzz(func(t *testing.T, src, dst string, odd bool) {
		paths := []string{src, dst}
		if odd {
			paths = append(paths, src)
		}
		opts := &Options{internalConsumedPaths: []string{"kept"}}

		err := adoptIntentConsumption(opts, paths)

		if err != nil {
			if !reflect.DeepEqual(opts.internalConsumedPaths, []string{"kept"}) {
				t.Fatalf("a refused adoption changed the sanction set to %v", opts.internalConsumedPaths)
			}
			return
		}
		wantSrc, wantDst := consumptionPair(filepath.Base(src))
		if odd || src != wantSrc || dst != wantDst || !strings.HasSuffix(src, ".json") || !reflect.DeepEqual(opts.internalConsumedPaths, paths) {
			t.Fatalf("adopted %v (odd=%v), want only a whole .json pair %s -> %s", paths, odd, wantSrc, wantDst)
		}
	})
}

func TestLandingComplete_IsTrueOnlyForACompleteIntentOfAPositiveCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	opts := &Options{ProjectRoot: root}
	write := func(cycle int, status landing.IntentStatus) {
		t.Helper()
		if err := writeLandingIntent(opts, landing.Intent{Cycle: cycle, Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	write(0, landing.IntentComplete)
	write(-1, landing.IntentComplete)
	write(1, landing.IntentComplete)
	write(2, landing.IntentPrepared)
	write(3, landing.IntentUnwound)
	mustWrite(t, intentPath(opts, 4), "{")
	cases := map[int]bool{-1: false, 0: false, 1: true, 2: false, 3: false, 4: false, 5: false}

	for cycle, want := range cases {
		if got := landingComplete(opts, cycle); got != want {
			t.Errorf("landingComplete(cycle %d) = %v, want %v", cycle, got, want)
		}
	}
}

func TestAppendShipJournal_AppendsOneTimestampedLinePerEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	before := time.Now().UTC().Add(-time.Second)

	for _, e := range []shipjournal.Entry{{SHA: "aaa", Class: "cycle", Cycle: 7}, {SHA: "bbb", Class: "manual"}} {
		if err := appendShipJournal(root, e); err != nil {
			t.Fatal(err)
		}
	}

	journal := readShipJournal(root)
	if len(journal) != 2 || journal["aaa"].Cycle != 7 || journal["aaa"].Class != "cycle" || journal["bbb"].Cycle != 0 {
		t.Fatalf("journal = %+v, want both entries with their class and cycle", journal)
	}
	ts, err := time.Parse(time.RFC3339, journal["bbb"].TS)
	if err != nil || ts.Before(before.Truncate(time.Second)) {
		t.Errorf("ts %q (err %v): ship stamps each entry with the current UTC time", journal["bbb"].TS, err)
	}
	raw, err := os.ReadFile(shipJournalPath(root))
	if err != nil || strings.Count(string(raw), "\n") != 2 || strings.Contains(string(raw), `"cycle":0`) {
		t.Errorf("journal bytes %q: one line per entry, a zero cycle omitted", raw)
	}
}

func TestAppendShipJournal_ReportsEveryWriteFailure(t *testing.T) {
	cases := map[string]func(t *testing.T, root string){
		"the .evolve dir is a file": func(t *testing.T, root string) {
			mustWrite(t, filepath.Join(root, ".evolve"), "not a dir")
		},
		"the journal path is a directory": func(t *testing.T, root string) {
			mustMkdir(t, shipJournalPath(root))
		},
		"the entry cannot be encoded": func(t *testing.T, root string) {
			stubMarshalJSON(t, errors.New("encode refused"))
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			arrange(t, root)

			err := appendShipJournal(root, shipjournal.Entry{SHA: "abc", Class: "cycle"})

			if err == nil || journalHasSHA(root, "abc") {
				t.Errorf("err = %v, journaled = %v: a failed journal write is an error and records nothing", err, journalHasSHA(root, "abc"))
			}
		})
	}
}

func stubMarshalJSON(t *testing.T, err error) {
	t.Helper()
	prior := marshalJSON
	marshalJSON = func(any) ([]byte, error) { return nil, err }
	t.Cleanup(func() { marshalJSON = prior })
}

func TestRunIDForShip_PrefersTheHostRunAndReadsTheStateOtherwise(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, runID, state, want string
		wantErr                  bool
	}{
		{name: "the host run wins over the state", runID: "run-host", state: `{"run_id":"run-state"}`, want: "run-host"},
		{name: "the state names the run", state: `{"run_id":"run-state"}`, want: "run-state"},
		{name: "no state file is no run", want: ""},
		{name: "a state with no run id", state: `{"cycle_id":3}`, want: ""},
		{name: "a corrupt state is an error", state: "{", wantErr: true},
		{name: "a corrupt state never hides the host run", runID: "run-host", state: "{", want: "run-host"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.state != "" {
				mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), tc.state)
			}

			got, err := runIDForShip(&Options{ProjectRoot: root, RunID: tc.runID})

			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("runIDForShip = %q, %v; want %q, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestReadLandingIntent_DistinguishesAbsentFromUnreadable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		opts       func(root string) *Options
		arrange    func(t *testing.T, opts *Options)
		wantFound  bool
		wantStatus landing.IntentStatus
		wantErr    string
	}{
		{name: "no cycle identity is no intent", opts: func(root string) *Options { return &Options{ProjectRoot: root} }},
		{name: "an unreadable cycle state is an error",
			opts: func(root string) *Options { return &Options{ProjectRoot: root} },
			arrange: func(t *testing.T, opts *Options) {
				mustWrite(t, filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json"), "{")
			}, wantErr: "the cycle identity is unreadable"},
		{name: "no intent file", opts: func(root string) *Options { return &Options{ProjectRoot: root, CycleID: 5} }},
		{name: "a prepared intent", opts: func(root string) *Options { return &Options{ProjectRoot: root, CycleID: 5} },
			arrange: func(t *testing.T, opts *Options) {
				if err := writeLandingIntent(opts, landing.Intent{Cycle: 5, CommitSHA: "abc", Status: landing.IntentPrepared}); err != nil {
					t.Fatal(err)
				}
			}, wantFound: true, wantStatus: landing.IntentPrepared},
		{name: "a corrupt intent is an error naming its path",
			opts: func(root string) *Options { return &Options{ProjectRoot: root, CycleID: 5} },
			arrange: func(t *testing.T, opts *Options) {
				mustWrite(t, intentPath(opts, 5), `{"cycle":`)
			}, wantErr: "ship: read the landing intent: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts(t.TempDir())
			if tc.arrange != nil {
				tc.arrange(t, opts)
			}

			in, found, err := readLandingIntent(opts)

			if tc.wantErr != "" {
				se := wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, tc.wantErr)
				if found || in.Status != "" {
					t.Errorf("found=%v status=%q with an error", found, in.Status)
				}
				if strings.Contains(tc.name, "path") && se.Debug["path"] != intentPath(opts, 5) {
					t.Errorf("Debug[path] = %q, want %q", se.Debug["path"], intentPath(opts, 5))
				}
				return
			}
			if err != nil || found != tc.wantFound || in.Status != tc.wantStatus {
				t.Errorf("readLandingIntent = %+v, %v, %v; want found=%v status=%q", in, found, err, tc.wantFound, tc.wantStatus)
			}
		})
	}
}

func TestJournalMinted_JournalsEachMintedCommitOnce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		opts        Options
		res         RunResult
		arrange     func(t *testing.T, root string)
		wantCycle   int
		wantEntry   bool
		wantWarn    bool
		wantEntries int
	}{
		{name: "no commit journals nothing", opts: Options{CycleID: 4}},
		{name: "a dry run journals nothing", opts: Options{CycleID: 4, DryRun: true}, res: RunResult{CommitSHA: "abc"}},
		{name: "push-only mints nothing", opts: Options{CycleID: 4, PushOnly: true}, res: RunResult{CommitSHA: "abc"}},
		{name: "a minted commit is journaled with its cycle", opts: Options{CycleID: 4, Class: ClassCycle}, res: RunResult{CommitSHA: "abc"},
			wantEntry: true, wantCycle: 4, wantEntries: 1},
		{name: "a journaled commit is never journaled twice", opts: Options{CycleID: 4, Class: ClassCycle}, res: RunResult{CommitSHA: "abc"},
			arrange: func(t *testing.T, root string) {
				if err := appendShipJournal(root, shipjournal.Entry{SHA: "abc", Class: "cycle", Cycle: 4}); err != nil {
					t.Fatal(err)
				}
			}, wantEntry: true, wantCycle: 4, wantEntries: 1},
		{name: "an unreadable cycle state still journals the commit and warns", opts: Options{Class: ClassManual}, res: RunResult{CommitSHA: "abc"},
			arrange: func(t *testing.T, root string) {
				mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), "{")
			}, wantEntry: true, wantWarn: true, wantEntries: 1},
		{name: "a journal that cannot be written warns", opts: Options{CycleID: 4, Class: ClassCycle}, res: RunResult{CommitSHA: "abc"},
			arrange: func(t *testing.T, root string) {
				mustMkdir(t, shipJournalPath(root))
			}, wantWarn: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.arrange != nil {
				tc.arrange(t, root)
			}
			opts, res := tc.opts, tc.res
			opts.ProjectRoot = root

			journalMinted(&opts, &res)

			journal := readShipJournal(root)
			entry, found := journal["abc"]
			if found != tc.wantEntry || (found && entry.Cycle != tc.wantCycle) || len(journal) != tc.wantEntries {
				t.Errorf("journal = %+v, want entry=%v cycle=%d entries=%d", journal, tc.wantEntry, tc.wantCycle, tc.wantEntries)
			}
			if got := containsLog(res, "the ship journal entry of abc is not complete"); got != tc.wantWarn {
				t.Errorf("warned = %v, want %v (logs %q)", got, tc.wantWarn, res.Logs)
			}
			raw, _ := os.ReadFile(shipJournalPath(root))
			if tc.wantEntries == 1 && strings.Count(string(raw), "abc") != 1 {
				t.Errorf("journal bytes %q hold abc %d times, want once", raw, strings.Count(string(raw), "abc"))
			}
		})
	}
}

func TestExplanationViewSHA_AnAbsentContractIsNoViewAndACorruptOneIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	sum, err := explanationViewSHA(root, 9, "run-9")
	if err != nil || sum != "" {
		t.Errorf("no activation marker = %q, %v; want no view and no error", sum, err)
	}

	mustWrite(t, filepath.Join(root, ".evolve", "build-explanation-contracts", "cycle-9.json"), "{")
	sum, err = explanationViewSHA(root, 9, "run-9")

	if err == nil || sum != "" || !strings.Contains(err.Error(), "activation marker") {
		t.Errorf("a corrupt activation marker = %q, %v; want an error and no hash", sum, err)
	}
}

func TestAuditPassed_RequiresAnUntamperedPassArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) (string, string) {
		path := filepath.Join(dir, name)
		mustWrite(t, path, body)
		sum := sha256.Sum256([]byte(body))
		return path, hex.EncodeToString(sum[:])
	}
	pass, passSHA := write("pass.md", "# Audit\n\nVerdict: PASS\n")
	fail, failSHA := write("fail.md", "# Audit\n\nVerdict: FAIL\n")
	cases := []struct {
		name  string
		entry auditEntry
		want  bool
	}{
		{name: "an untampered PASS", entry: auditEntry{ArtifactPath: pass, ArtifactSHA256: passSHA}, want: true},
		{name: "a findings exit of 1 with a PASS", entry: auditEntry{ExitCode: 1, ArtifactPath: pass, ArtifactSHA256: passSHA}, want: true},
		{name: "a FAIL verdict", entry: auditEntry{ArtifactPath: fail, ArtifactSHA256: failSHA}},
		{name: "a PASS whose bytes changed after the audit", entry: auditEntry{ArtifactPath: pass, ArtifactSHA256: failSHA}},
		{name: "a missing artifact", entry: auditEntry{ArtifactPath: filepath.Join(dir, "gone.md"), ArtifactSHA256: passSHA}},
		{name: "an auditor that crashed", entry: auditEntry{ExitCode: 2, ArtifactPath: pass, ArtifactSHA256: passSHA}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := auditPassed(&Options{}, &tc.entry); got != tc.want {
				t.Errorf("auditPassed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStrandedLanding_NeedsACycleShipWithAnAuditBinding(t *testing.T) {
	t.Parallel()
	for name, opts := range map[string]*Options{
		"a manual ship":            {Class: ClassManual, internalAuditBoundTreeSHA: strings.Repeat("a", 40)},
		"a cycle ship never bound": {Class: ClassCycle},
	} {
		t.Run(name, func(t *testing.T) {
			fault := &gitFault{when: func([]string) bool { return false }}
			opts.Runner = fault.runner()

			in, ok := strandedLanding(context.Background(), opts, &core.ShipError{Debug: map[string]string{"audited": "abc"}})

			if ok || in.CommitSHA != "" || len(fault.calls) != 0 {
				t.Errorf("stranded=%v intent=%+v git calls=%v: only an audit-bound cycle ship can hold a stranded landing", ok, in, fault.calls)
			}
		})
	}
}

func TestFinishResume_OnOriginStillRefusesAnIntentWithBrokenConsumptionPairs(t *testing.T) {
	t.Parallel()
	fault := &gitFault{when: func([]string) bool { return false }}
	opts := &Options{ProjectRoot: t.TempDir(), Runner: fault.runner(), internalConsumedPaths: []string{"kept"}}
	res := &RunResult{}
	s := newWorktreeShip(context.Background(), opts, res, "main", t.TempDir())
	in := landing.Intent{Cycle: 3, CommitSHA: "c0ffee", ConsumedPaths: []string{".evolve/inbox/a.json"}}

	err := s.finishResume(in, landing.Verdict{OnOrigin: true})

	se := wantShipErr(t, err, core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, "INTEGRITY BREACH (pre-push)")
	if se.Debug["commit"] != "c0ffee" || se.Debug["phase"] != "pre-push" || len(fault.calls) != 0 || res.CommitSHA != "" {
		t.Errorf("debug=%v git calls=%v commit=%q: a broken intent settles nothing and runs no git", se.Debug, fault.calls, res.CommitSHA)
	}
	if !reflect.DeepEqual(opts.internalConsumedPaths, []string{"kept"}) {
		t.Errorf("sanction set = %v, want it unchanged after a refusal", opts.internalConsumedPaths)
	}
}

func TestVerifyLandingCommit_AdoptsTheIntentPairsWhenNoAuditTreeIsBound(t *testing.T) {
	t.Parallel()
	opts := &Options{ProjectRoot: t.TempDir()}
	s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", t.TempDir())
	paths := consumptionPairSlice("item.json")

	err := s.verifyLandingCommit(landing.Intent{ConsumedPaths: paths})

	if err != nil || !reflect.DeepEqual(opts.internalConsumedPaths, paths) {
		t.Errorf("err=%v sanctioned=%v, want the intent's pairs %v adopted", err, opts.internalConsumedPaths, paths)
	}
}

func TestRunRepoContractGate_AnUnresolvableLandingTreeStopsTheGate(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, core.RunStateFile), `{"active_worktree":"/some/other/lane"}`)
	p := New(Config{Runner: (&gitFault{when: func([]string) bool { return false }}).runner()})
	opts := &Options{Class: ClassCycle, ProjectRoot: t.TempDir(), WorkspacePath: ws, ActiveWorktree: "/the/host/lane"}

	err := p.runRepoContractGate(context.Background(), core.PhaseRequest{Workspace: ws}, opts)

	if err == nil || !strings.HasPrefix(err.Error(), "ship repo-contract gate: ") || !strings.Contains(err.Error(), "does not match host identity") {
		t.Errorf("err = %v, want the gate to stop on the run.json identity mismatch", err)
	}
	if _, ok := core.AsShipError(err); !ok {
		t.Errorf("err %v lost its ship error code", err)
	}
}

func TestRunPushOnly_AnUnreadableBranchOrOriginStopsBeforeAnyPush(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		failing argvMatch
		wantErr string
	}{
		{name: "the branch is unreadable", failing: argvIs("rev-parse", "--abbrev-ref", "HEAD"), wantErr: "ship --push-only: resolve branch: "},
		{name: "origin has no such branch", failing: argvIs("rev-list", "origin/main..HEAD"), wantErr: "ship --push-only: no origin/main to compare against: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			branch := &gitFault{when: argvIs("rev-parse", "--abbrev-ref", "HEAD"), stdout: "main\n"}
			fault := &gitFault{when: tc.failing, exit: 128, next: branch.runner()}
			opts := &Options{ProjectRoot: t.TempDir(), PushOnly: true, Runner: fault.runner()}
			res := &RunResult{}

			err := runPushOnly(context.Background(), opts, res)

			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want prefix %q", err, tc.wantErr)
			}
			for _, call := range fault.calls {
				if len(gitSubArgs(call)) > 0 && gitSubArgs(call)[0] == "push" {
					t.Errorf("push-only pushed (%v) after it could not read its branch or origin", call)
				}
			}
			if res.CommitSHA != "" || len(res.Logs) != 0 {
				t.Errorf("result = %+v, want nothing recorded", res)
			}
		})
	}
}

type inboxFixture struct {
	root, inbox, consumed, base, src, orig string
}

func newInboxFixture(t *testing.T, id string) inboxFixture {
	t.Helper()
	root := t.TempDir()
	f := inboxFixture{root: root, inbox: filepath.Join(root, ".evolve", "inbox"), base: "2026-10-11T00-00-00Z-" + id + ".json"}
	f.consumed = filepath.Join(f.inbox, "consumed")
	f.src = filepath.Join(f.inbox, f.base)
	f.orig = `{"id":"` + id + `","title":"an item ship consumes","weight":0.25}`
	mustWrite(t, f.src, f.orig)
	return f
}

func (f inboxFixture) consumer(runner CmdRunner, bound string) (*itemConsumer, *Options, *RunResult) {
	opts := &Options{ProjectRoot: f.root, Runner: runner, internalAuditBoundTreeSHA: bound}
	res := &RunResult{}
	return &itemConsumer{ctx: context.Background(), opts: opts, res: res, root: f.root, inboxDir: f.inbox, cid: "1830"}, opts, res
}

func (f inboxFixture) wantPickable(t *testing.T, res *RunResult) {
	t.Helper()
	raw, err := os.ReadFile(f.src)
	if err != nil || string(raw) != f.orig {
		t.Errorf("root item = %q (err %v), want its original bytes %q; logs %q", raw, err, f.orig, res.Logs)
	}
	if _, err := os.Stat(filepath.Join(f.consumed, f.base)); err == nil {
		t.Errorf("a consumed record exists: a failed move leaves no half")
	}
}

func TestItemConsumerStage_EveryFailureIsLoudAndLeavesTheItemPickable(t *testing.T) {
	const id = "backfill-item"
	accept := func([]string) bool { return false }
	cases := []struct {
		name    string
		arrange func(t *testing.T, f inboxFixture) (*gitFault, string)
		warn    []string
	}{
		{name: "the item cannot be read", warn: []string{`consume read "backfill-item": read refused`},
			arrange: func(t *testing.T, f inboxFixture) (*gitFault, string) {
				prior := readInboxItem
				readInboxItem = func(string) ([]byte, error) { return nil, errors.New("read refused") }
				t.Cleanup(func() { readInboxItem = prior })
				return &gitFault{when: accept}, ""
			}},
		{name: "the item is not a JSON object", warn: []string{`consume parse "backfill-item"`, "leaving item in place"},
			arrange: func(t *testing.T, f inboxFixture) (*gitFault, string) {
				prior := readInboxItem
				readInboxItem = func(string) ([]byte, error) { return []byte(`["an array"]`), nil }
				t.Cleanup(func() { readInboxItem = prior })
				return &gitFault{when: accept}, ""
			}},
		{name: "the annotated item cannot be encoded", warn: []string{`consume marshal "backfill-item": encode refused`},
			arrange: func(t *testing.T, f inboxFixture) (*gitFault, string) {
				prior := marshalInboxItem
				marshalInboxItem = func(any, string, string) ([]byte, error) { return nil, errors.New("encode refused") }
				t.Cleanup(func() { marshalInboxItem = prior })
				return &gitFault{when: accept}, ""
			}},
		{name: "the consumed dir cannot be made", warn: []string{`consume mkdir for "backfill-item"`},
			arrange: func(t *testing.T, f inboxFixture) (*gitFault, string) {
				mustWrite(t, f.consumed, "a file where the consumed dir belongs")
				return &gitFault{when: accept}, ""
			}},
		{name: "the consumed record cannot be written", warn: []string{`consume write "backfill-item"`},
			arrange: func(t *testing.T, f inboxFixture) (*gitFault, string) {
				skipWhenRoot(t)
				mustMkdir(t, f.consumed)
				readOnlyDir(t, f.consumed)
				return &gitFault{when: accept}, ""
			}},
		{name: "the root item cannot be removed", warn: []string{`consume remove root "backfill-item"`},
			arrange: func(t *testing.T, f inboxFixture) (*gitFault, string) {
				skipWhenRoot(t)
				mustMkdir(t, f.consumed)
				readOnlyDir(t, f.inbox)
				return &gitFault{when: accept}, ""
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newInboxFixture(t, id)
			fault, bound := tc.arrange(t, f)
			c, opts, res := f.consumer(fault.runner(), bound)

			m, outcome := c.stage(id)

			if outcome != moveFailed || m.dstRel != "" {
				t.Fatalf("outcome = %v move = %+v, want moveFailed and no move", outcome, m)
			}
			for _, w := range tc.warn {
				if !containsLog(*res, w) || !strings.HasPrefix(res.Logs[0], "[ship] WARN: consume ") {
					t.Errorf("logs %q lack the WARN %q", res.Logs, w)
				}
			}
			if len(fault.calls) != 0 || len(opts.internalConsumedPaths) != 0 {
				t.Errorf("git calls %v, sanctioned %v: a move that failed before staging stages and sanctions nothing", fault.calls, opts.internalConsumedPaths)
			}
			_ = os.Chmod(f.inbox, 0o755)
			_ = os.Chmod(f.consumed, 0o755)
			f.wantPickable(t, res)
		})
	}
}

func TestItemConsumerStage_ALookupErrorIsAFailureNotAnAbsence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	c := &itemConsumer{ctx: context.Background(), opts: &Options{ProjectRoot: root}, res: &RunResult{}, root: root,
		inboxDir: filepath.Join(root, ".evolve", "inbox"), cid: "1"}

	_, outcome := c.stage("anything")

	if outcome != moveFailed || !containsLog(*c.res, `[ship] WARN: consume lookup "anything"`) {
		t.Errorf("outcome = %v logs %q: an unreadable inbox is a loud failure, never a silent already-consumed", outcome, c.res.Logs)
	}
}

func TestItemConsumerStage_ARollbackThatCannotRestoreTheItemSaysSo(t *testing.T) {
	t.Parallel()
	const id = "backfill-item"
	cases := []struct {
		name  string
		bound string
		at    argvMatch
		warn  []string
	}{
		{name: "the provenance check refuses and the inbox turned read-only", bound: strings.Repeat("b", 40), at: argvStarts("show"),
			warn: []string{`consume provenance rollback write "backfill-item"`, `consume "backfill-item" REFUSED sanctioning`}},
		{name: "the stage fails and the inbox turned read-only", at: argvStarts("add", "-A", "--"),
			warn: []string{`consume rollback write "backfill-item"`, `consume stage "backfill-item" rc=1`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			skipWhenRoot(t)
			f := newInboxFixture(t, id)
			mustMkdir(t, f.consumed)
			fault := &gitFault{when: tc.at, exit: 1, before: func() { readOnlyDir(t, f.inbox) }}
			c, opts, res := f.consumer(fault.runner(), tc.bound)

			_, outcome := c.stage(id)

			if outcome != moveFailed || fault.hits != 1 || len(opts.internalConsumedPaths) != 0 {
				t.Fatalf("outcome=%v hits=%d sanctioned=%v, want one refused git step and a failure", outcome, fault.hits, opts.internalConsumedPaths)
			}
			for _, w := range tc.warn {
				if !containsLog(*res, w) || !strings.HasPrefix(res.Logs[0], "[ship] WARN: consume ") {
					t.Errorf("logs %q lack the WARN %q", res.Logs, w)
				}
			}
			if _, err := os.Stat(filepath.Join(f.consumed, f.base)); !os.IsNotExist(err) {
				t.Errorf("consumed record still exists (stat err %v)", err)
			}
		})
	}
}

func TestItemConsumerStage_AStagedMoveCarriesTheShipAnnotation(t *testing.T) {
	t.Parallel()
	const id = "backfill-item"
	f := newInboxFixture(t, id)
	fault := &gitFault{when: func([]string) bool { return false }}
	c, _, res := f.consumer(fault.runner(), "")

	m, outcome := c.stage(id)

	src, dst := consumptionPair(f.base)
	if outcome != moveStaged || m.srcRel != src || m.dstRel != dst || string(m.raw) != f.orig {
		t.Fatalf("outcome=%v move=%+v, want the staged pair %s -> %s holding the original bytes", outcome, m, src, dst)
	}
	var doc map[string]any
	raw, err := os.ReadFile(filepath.Join(f.consumed, f.base))
	if err != nil || json.Unmarshal(raw, &doc) != nil {
		t.Fatalf("consumed record %q unreadable: %v", raw, err)
	}
	consumed, _ := doc["consumed"].(map[string]any)
	if consumed["via"] != "ship" || consumed["cycle"] != "1830" || doc["id"] != id || doc["weight"] != 0.25 {
		t.Errorf("consumed record %v, want the original fields plus consumed{via:ship, cycle:1830}", doc)
	}
	if _, err := os.Stat(f.src); !os.IsNotExist(err) || len(fault.calls) != 1 || !reflect.DeepEqual(gitSubArgs(fault.calls[0]), []string{"add", "-A", "--", src, dst}) {
		t.Errorf("root stat err %v, git calls %v: the root item moves and exactly the pair is staged; logs %q", err, fault.calls, res.Logs)
	}
}

func TestNewIntent_AnUnreadableRunIdentityPreparesNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, state string
		cycleID     int
		wantErr     []string
	}{
		{name: "no cycle id in the state", state: `{"run_id":"run-1"}`, wantErr: []string{"no readable cycle_id for the run workspace"}},
		{name: "a corrupt state with no host cycle", state: "{", wantErr: []string{"no readable cycle_id for the run workspace", "parse "}},
		{name: "a corrupt state with no host run", state: "{", cycleID: 12, wantErr: []string{"parse "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), tc.state)
			fault := &gitFault{when: func([]string) bool { return false }}
			opts := &Options{ProjectRoot: root, CycleID: tc.cycleID, Runner: fault.runner()}
			s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", t.TempDir())

			in, err := s.newIntent("c0ffee")

			for _, want := range tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to say %q", err, want)
				}
			}
			if in.CommitSHA != "" || in.Status != "" || len(fault.calls) != 0 {
				t.Errorf("intent %+v, git calls %v: an unidentified run prepares no intent and measures nothing", in, fault.calls)
			}
		})
	}
}

func TestMeasure_AnUnreadableTreeOrBranchLeavesTheIntentUnmeasured(t *testing.T) {
	t.Parallel()
	treeOK := &gitFault{when: argvIs("rev-parse", "c0ffee^{tree}"), stdout: "7ree\n"}
	cases := []struct {
		name  string
		fault *gitFault
	}{
		{name: "the commit tree is unreadable", fault: &gitFault{when: argvIs("rev-parse", "c0ffee^{tree}"), exit: 128}},
		{name: "the branch is unreadable", fault: &gitFault{when: argvIs("rev-parse", "main"), exit: 128, next: treeOK.runner()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &Options{ProjectRoot: t.TempDir(), Runner: tc.fault.runner()}
			s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", t.TempDir())
			in := landing.Intent{CommitSHA: "c0ffee", WorktreeBaseSHA: "base"}

			err := s.measure(&in)

			wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "exited 128")
			if in.CommitTree != "" || in.PreMain != "" || in.LaneTree != "" || in.WorktreeBaseSHA != "base" {
				t.Errorf("intent %+v: a failed measure records no partial measurement", in)
			}
		})
	}
}

func TestAbandonPrepared_ReturnsTheCauseAndEveryRefFailure(t *testing.T) {
	t.Parallel()
	cause := shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip, "ship: ship cannot record the landing intent")
	t.Run("an intent measured before the lane tree is the cause alone", func(t *testing.T) {
		fault := &gitFault{when: func([]string) bool { return true }, exit: 1}
		s := newWorktreeShip(context.Background(), &Options{ProjectRoot: t.TempDir(), Runner: fault.runner()}, &RunResult{}, "main", t.TempDir())

		err := s.abandonPrepared("c0ffee", landing.Intent{CommitSHA: "c0ffee"}, cause)

		if err != cause || len(fault.calls) != 0 {
			t.Errorf("err = %v, git calls %v: with no lane tree there is nothing to unwind and the ref never moved", err, fault.calls)
		}
	})
	t.Run("a lane ref that does not move joins its failure to the cause", func(t *testing.T) {
		fault := &gitFault{when: argvStarts("update-ref"), exit: 1}
		wt := t.TempDir()
		s := newWorktreeShip(context.Background(), &Options{ProjectRoot: t.TempDir(), Runner: fault.runner()}, &RunResult{}, "main", wt)

		err := s.abandonPrepared("c0ffee", landing.Intent{CommitSHA: "c0ffee", LaneTree: "7ree"}, cause)

		if !errors.Is(err, cause) || !strings.Contains(err.Error(), "git update-ref of the lane to c0ffee failed (rc=1)") {
			t.Errorf("err = %v, want the cause joined with the update-ref failure", err)
		}
		if fault.hits != 1 || !reflect.DeepEqual(fault.calls[0], []string{"-C", wt, "update-ref", "-m", "evolve ship: the landing commit", "HEAD", "c0ffee", "c0ffee^"}) {
			t.Errorf("git calls %v, want one guarded update-ref of the lane", fault.calls)
		}
	})
}

package loopchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	fixedNow  = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	rfc3339   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)
	readDirOp = regexp.MustCompile(`\b[a-z_]+ (\S+): not a directory`)
)

const commit = "cafebabe1234deadbeef"

// project builds a temp root with .evolve/state.json pinned to STALE_PIN and
// a rebuilt go/bin/evolve; returns root, evolveDir and sha256(binary).
func project(t *testing.T) (root, evolveDir, binSHA string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	writeJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{"expected_ship_sha": "STALE_PIN"})
	if err := os.MkdirAll(filepath.Join(root, "go", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "bin", "evolve"), []byte("REBUILT-BINARY-BYTES"), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("REBUILT-BINARY-BYTES"))
	return root, evolveDir, hex.EncodeToString(sum[:])
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func golden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func template(s, root, evolveDir string) string {
	s = strings.ReplaceAll(s, evolveDir, "{EVOLVE_DIR}")
	s = strings.ReplaceAll(s, root, "{ROOT}")
	// The os.ReadDir fault is a *fs.PathError whose Op is Go-version- and
	// OS-dependent (`open` darwin/Go 1.27, `fdopendir` darwin/Go 1.23,
	// `readdirent` linux — PR #599 CI); the goldens keep `open`. Same
	// canonicalization as cmd/evolve's u13Template.
	s = readDirOp.ReplaceAllString(s, "open $1: not a directory")
	return rfc3339.ReplaceAllString(s, "{TS}")
}

// signals is a recording Center with a WARN-filtered console.
type signals struct {
	center  *signalcenter.Center
	events  *[]signalcenter.Event
	console *bytes.Buffer
}

func newSignals() *signals {
	s := &signals{center: signalcenter.New(), events: &[]signalcenter.Event{}, console: &bytes.Buffer{}}
	s.center.Subscribe(func(e signalcenter.Event) { *s.events = append(*s.events, e) })
	s.center.Subscribe(signalcenter.Filter(signalcenter.StderrSink(s.console), signalcenter.SeverityWarn))
	return s
}

func (s *signals) accessor() func() *signalcenter.Center {
	return func() *signalcenter.Center { return s.center }
}

func (s *signals) codes() []string {
	var out []string
	for _, e := range *s.events {
		out = append(out, string(e.Code))
	}
	return out
}

func (s *signals) only(t *testing.T, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	if len(*s.events) != 1 || (*s.events)[0].Code != code {
		t.Fatalf("exactly one %s, got %v", code, s.codes())
	}
	return (*s.events)[0]
}

// refreshFixture scripts every seam: the running commit, a stale ahead
// check, an idle plane, a recorded rebuild/exec, a verifying provenance.
type refreshFixture struct {
	root, evolveDir string
	deps            RefreshDeps
	order           []string
	stderr          bytes.Buffer
	sig             *signals
	argv0           string
	argv, environ   []string
}

func newRefreshFixture(t *testing.T) *refreshFixture {
	t.Helper()
	f := &refreshFixture{sig: newSignals()}
	f.root, f.evolveDir, _ = project(t)
	f.deps = RefreshDeps{
		RunningCommit: func() string { return commit },
		Ahead:         func(string, string) (bool, error) { f.order = append(f.order, "ahead"); return true, nil },
		LaneActive:    func() (bool, error) { f.order = append(f.order, "lane"); return false, nil },
		Rebuild:       func(string) error { f.order = append(f.order, "rebuild"); return nil },
		ReExecTarget:  RebuiltBinary,
		Provenance: func(string) (string, phaseintegrity.ProvenanceVerified) {
			f.order = append(f.order, "provenance")
			return commit, func(c string) bool { return c == commit }
		},
		Argv: func() []string {
			f.order = append(f.order, "argv")
			return []string{"evolve", "loop", "--until-inbox-empty"}
		},
		Environ: func() []string { return []string{"K=V"} },
		Flush:   func() { f.order = append(f.order, "flush") },
		ReExec: func(argv0 string, argv, environ []string) error {
			f.order = append(f.order, "reexec")
			f.argv0, f.argv, f.environ = argv0, argv, environ
			return nil
		},
	}
	return f
}

func (f *refreshFixture) refresher(opts ...Option) *Refresher {
	all := append([]Option{WithSignals(f.sig.accessor()), WithNow(func() time.Time { return fixedNow })}, opts...)
	return NewRefresher(Roots{ProjectRoot: f.root, EvolveDir: f.evolveDir}, f.deps, &f.stderr, all...)
}

func refreshSection(t *testing.T, name string) string {
	t.Helper()
	for _, s := range strings.Split(golden(t, "stderr_refresh.golden.txt"), "== ") {
		n, body, _ := strings.Cut(s, "\n")
		if strings.HasPrefix(n, name+" ") {
			return body
		}
	}
	t.Fatalf("no golden section %q", name)
	return ""
}

// oldSentence is the golden's [chain] line for a section minus its prefix —
// the reason every replaced line keeps.
func oldSentence(t *testing.T, section string, line int) string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(refreshSection(t, section), "\n"), "\n")
	return strings.TrimPrefix(lines[line], "[chain] boundary-refresh: ")
}

// --- 37. codes ---

func TestCodes_AreRegisteredUnderLoopWithDocs(t *testing.T) {
	for _, c := range []signalcenter.Code{CodeBoundaryRefreshSkipped, CodeBoundaryRefreshAuditFailed, CodeChainInboxItemInvalid, CodeChainQuotaDefer, CodeChainInboxUnreadable, CodeChainBatchError} {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleLoop {
			t.Errorf("%s under loop: %q %v", c, m, ok)
		}
	}
	for _, d := range signalcenter.RegisteredCodes()[signalcenter.ModuleLoop] {
		if (strings.HasPrefix(string(d.Code), "LOOP_BOUNDARY_") || strings.HasPrefix(string(d.Code), "LOOP_CHAIN_")) && d.Doc == "" {
			t.Errorf("%s has no doc", d.Code)
		}
	}
}

// --- 41. every degrade branch is ONE coded WARN naming its step ---

func TestRefresh_EveryDegradeBranchIsOneCodedWarnNamingItsStep(t *testing.T) {
	cases := []struct {
		section, step string
		line          int
		fault         func(f *refreshFixture) []Option
		nextNotCalled string
	}{
		{"ahead_check_failed", "ahead_check", 0, func(f *refreshFixture) []Option {
			f.deps.Ahead = func(string, string) (bool, error) { return false, errors.New("git fetch: network unreachable") }
			return nil
		}, "lane"},
		{"lane_check_unverifiable", "lane_check", 0, func(f *refreshFixture) []Option {
			f.deps.LaneActive = func() (bool, error) { return false, errors.New("runlease: parse .lease: unexpected end of JSON input") }
			return nil
		}, "rebuild"},
		{"lane_active", "lane_active", 0, func(f *refreshFixture) []Option {
			f.deps.LaneActive = func() (bool, error) { return true, nil }
			return nil
		}, "rebuild"},
		{"breaker_refused", "breaker", 0, func(f *refreshFixture) []Option {
			writeJSON(t, filepath.Join(f.evolveDir, AttemptFile), map[string]any{"running_commit": commit, "batch": 6, "timestamp": "2026-09-14T10:00:00Z"})
			return nil
		}, "rebuild"},
		{"rebuild_failed", "rebuild", 1, func(f *refreshFixture) []Option {
			f.deps.Rebuild = func(string) error { return errors.New("make -C go build: exit status 2: build failed: syntax error") }
			return nil
		}, "provenance"},
		{"no_target", "target", 1, func(f *refreshFixture) []Option {
			if err := os.Remove(filepath.Join(f.root, "go", "bin", "evolve")); err != nil {
				t.Fatal(err)
			}
			return nil
		}, "provenance"},
		{"repin_refused", "repin", 1, func(f *refreshFixture) []Option {
			f.deps.Provenance = func(string) (string, phaseintegrity.ProvenanceVerified) {
				return commit, func(string) bool { return false }
			}
			return nil
		}, "argv"},
		{"empty_argv", "argv", 1, func(f *refreshFixture) []Option {
			f.deps.Argv = func() []string { return nil }
			return nil
		}, "flush"},
		{"arm_failed", "arm", 1, func(f *refreshFixture) []Option {
			// A directory at the marker path: unreadable (so not "already
			// attempted") and unwritable (so the arm fails).
			if err := os.Mkdir(filepath.Join(f.evolveDir, AttemptFile), 0o755); err != nil {
				t.Fatal(err)
			}
			return nil
		}, "flush"},
		{"reexec_failed", "reexec", 2, func(f *refreshFixture) []Option {
			f.deps.ReExec = func(string, []string, []string) error { return errors.New("exec format error") }
			return nil
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.section, func(t *testing.T) {
			f := newRefreshFixture(t)
			opts := c.fault(f)
			if f.refresher(opts...).Refresh(7) {
				t.Fatal("a degrade branch never reports refreshed=true")
			}
			ev := f.sig.only(t, CodeBoundaryRefreshSkipped)
			want := oldSentence(t, c.section, c.line)
			if ev.Kind != signalcenter.KindLoopWarning || ev.Severity != signalcenter.SeverityWarn || ev.Origin != "Refresher.Refresh" || ev.Module != signalcenter.ModuleLoop ||
				ev.Fields["step"] != c.step || ev.Fields["batch"] != "7" || ev.Fields["commit"] != commit || template(ev.Reason, f.root, f.evolveDir) != want {
				t.Errorf("the event: %+v\nwant reason %q", ev, want)
			}
			if c.nextNotCalled != "" && strings.Contains(strings.Join(f.order, ","), c.nextNotCalled) {
				t.Errorf("%s must not be reached after %s: %v", c.nextNotCalled, c.step, f.order)
			}
			if console := template(f.sig.console.String(), f.root, f.evolveDir); !strings.Contains(console, "LOOP_BOUNDARY_REFRESH_SKIPPED") || !strings.Contains(console, want) {
				t.Errorf("the console renders the WARN with the old sentence: %q", f.sig.console.String())
			}
			// The kept lines (before the fault) stay verbatim on stderr.
			kept := strings.Split(strings.TrimSuffix(refreshSection(t, c.section), "\n"), "\n")[:c.line]
			if got := template(f.stderr.String(), f.root, f.evolveDir); strings.TrimSuffix(got, "\n") != strings.Join(kept, "\n") {
				t.Errorf("stderr keeps the report lines that print before the fault:\n got %q\nwant %q", got, strings.Join(kept, "\n"))
			}
			if ev.Fields["error"] == "" && c.step != "lane_active" && c.step != "breaker" && c.step != "argv" {
				t.Errorf("fields.error names the cause: %+v", ev.Fields)
			}
		})
	}
}

// --- 42. the healthy and the empty-commit paths are silent ---

func TestRefresh_NotAheadAndEmptyCommitAreSilent(t *testing.T) {
	f := newRefreshFixture(t)
	f.deps.Ahead = func(string, string) (bool, error) { f.order = append(f.order, "ahead"); return false, nil }
	if f.refresher().Refresh(1) || len(*f.sig.events) != 0 || f.stderr.Len() != 0 || strings.Join(f.order, ",") != "ahead" {
		t.Errorf("not ahead: zero events, zero stderr, nothing beyond Ahead: %v %v %q", f.sig.codes(), f.order, f.stderr.String())
	}
	f = newRefreshFixture(t)
	f.deps.RunningCommit = func() string { return "" }
	f.deps.Ahead = GitAhead // the real check: an empty commit is a quiet no-op, no git subprocess
	if f.refresher().Refresh(1) || len(*f.sig.events) != 0 || f.stderr.Len() != 0 {
		t.Errorf("an empty commit is a silent no-op: %v %q", f.sig.codes(), f.stderr.String())
	}
}

// --- 43. the success path: log, arm, flush, exec — in that order ---

func TestRefresh_SuccessLogsArmsFlushesThenExecs(t *testing.T) {
	f := newRefreshFixture(t)
	var seen []string
	f.deps.Argv = func() []string {
		f.order = append(f.order, "argv")
		if _, err := os.Stat(filepath.Join(f.evolveDir, LogFile)); err == nil {
			seen = append(seen, "log-before-argv")
		}
		return []string{"evolve", "loop", "--until-inbox-empty"}
	}
	f.deps.Flush = func() {
		f.order = append(f.order, "flush")
		if _, err := os.Stat(filepath.Join(f.evolveDir, AttemptFile)); err == nil {
			seen = append(seen, "armed-before-flush")
		}
	}
	if !f.refresher().Refresh(7) {
		t.Fatalf("the refresh fires: %s / %s", f.stderr.String(), f.sig.console.String())
	}
	if got := strings.Join(f.order, ","); got != "ahead,lane,rebuild,provenance,argv,flush,reexec" {
		t.Errorf("order: %s", got)
	}
	if strings.Join(seen, ",") != "log-before-argv,armed-before-flush" {
		t.Errorf("the audit entry lands BEFORE the argv check (Q-C2) and the breaker is armed BEFORE the flush: %v", seen)
	}
	if f.argv0 != filepath.Join(f.root, "go", "bin", "evolve") || strings.Join(f.argv, " ") != "evolve loop --until-inbox-empty" || strings.Join(f.environ, ",") != "K=V" {
		t.Errorf("exec(target, argv, the injected environ): %q %v %v", f.argv0, f.argv, f.environ)
	}
	marker, _ := os.ReadFile(filepath.Join(f.evolveDir, AttemptFile))
	if string(marker) != strings.ReplaceAll(golden(t, "marker.golden.json"), "{TS}", "2026-09-14T10:00:00Z") {
		t.Errorf("marker bytes: %s", marker)
	}
	logLine, _ := os.ReadFile(filepath.Join(f.evolveDir, LogFile))
	if string(logLine) != strings.ReplaceAll(golden(t, "log_entry.golden.jsonl"), "{TS}", "2026-09-14T10:00:00Z") {
		t.Errorf("log entry bytes: %s", logLine)
	}
	if got := template(f.stderr.String(), f.root, f.evolveDir); got != refreshSection(t, "refreshed") {
		t.Errorf("the two kept report lines verbatim:\n got %q\nwant %q", got, refreshSection(t, "refreshed"))
	}
	if len(*f.sig.events) != 0 {
		t.Errorf("a successful refresh emits nothing: %v", f.sig.codes())
	}
	raw, _ := os.ReadFile(filepath.Join(f.evolveDir, "state.json"))
	if strings.Contains(string(raw), "STALE_PIN") {
		t.Errorf("the pin moved before the exec: %s", raw)
	}
}

// --- 44. the audit failure is its own code and the exec still happens ---

func TestRefresh_AuditWriteFailureIsItsOwnCodeAndTheExecStillHappens(t *testing.T) {
	f := newRefreshFixture(t)
	if err := os.Mkdir(filepath.Join(f.evolveDir, "log-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !f.refresher(WithMarkerFiles(AttemptFile, "log-dir")).Refresh(7) {
		t.Fatalf("the refresh still fires: %s", f.sig.console.String())
	}
	ev := f.sig.only(t, CodeBoundaryRefreshAuditFailed)
	if ev.Fields["batch"] != "7" || ev.Fields["path"] != filepath.Join(f.evolveDir, "log-dir") || ev.Fields["error"] == "" || ev.Kind != signalcenter.KindLoopWarning ||
		!strings.HasPrefix(ev.Reason, "WARN: audit log write failed (open boundary-refresh log: open ") || !strings.HasSuffix(ev.Reason, "is a directory) — pin already moved") {
		t.Errorf("the audit failure names the path and the error: %+v", ev)
	}
	if !strings.Contains(strings.Join(f.order, ","), "reexec") {
		t.Errorf("the exec still happens: %v", f.order)
	}
}

// --- 45. the breaker ---

func TestRefresh_BreakerRefusesTheSameCommitTwice(t *testing.T) {
	f := newRefreshFixture(t)
	r := f.refresher()
	if r.alreadyAttempted("") {
		t.Error("an empty commit is never 'already attempted'")
	}
	if wrapLogWrite(nil) != nil || wrapLogWrite(errors.New("x")) == nil {
		t.Error("wrapLogWrite passes nil through and names a failure")
	}
	if !r.Refresh(1) {
		t.Fatal("the first refresh proceeds")
	}
	if r.Refresh(2) {
		t.Error("the same commit twice is refused")
	}
	if ev := f.sig.only(t, CodeBoundaryRefreshSkipped); ev.Fields["step"] != "breaker" || !strings.Contains(ev.Reason, "(see .evolve/"+AttemptFile+")") {
		t.Errorf("step=breaker naming the marker: %+v", ev)
	}
	// A different commit re-arms; an absent or corrupt marker proceeds.
	f.deps.RunningCommit = func() string { return "0ddba11beef0" }
	f.deps.Provenance = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "0ddba11beef0", func(string) bool { return true }
	}
	if err := os.WriteFile(filepath.Join(f.root, "go", "bin", "evolve"), []byte("REBUILT-AGAIN"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !f.refresher().Refresh(3) {
		t.Error("a moved commit re-arms the breaker")
	}
	if err := os.WriteFile(filepath.Join(f.evolveDir, AttemptFile), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "go", "bin", "evolve"), []byte("REBUILT-THRICE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !f.refresher().Refresh(4) {
		t.Error("a corrupt marker never refuses a legitimate refresh")
	}
	other := f.refresher(WithMarkerFiles("other-attempt.json", "other-log.jsonl"))
	if err := os.WriteFile(filepath.Join(f.root, "go", "bin", "evolve"), []byte("REBUILT-4"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !other.Refresh(5) {
		t.Error("WithMarkerFiles points the breaker at its own files")
	}
	if _, err := os.Stat(filepath.Join(f.evolveDir, "other-attempt.json")); err != nil {
		t.Errorf("the custom marker was written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.evolveDir, "other-log.jsonl")); err != nil {
		t.Errorf("the custom log was written: %v", err)
	}
}

// --- 46-49. the git and disk defaults ---

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repo builds a one-commit repo; commitAt adds one more commit on the
// current branch and returns its SHA.
func repo(t *testing.T) (dir, commitA string) {
	t.Helper()
	dir = t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "ci@example.com")
	git(t, dir, "config", "user.name", "ci")
	git(t, dir, "config", "commit.gpgsign", "false")
	return dir, commitAt(t, dir, "a")
}

func commitAt(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", name+".txt")
	git(t, dir, "commit", "-q", "-m", "commit "+name)
	return git(t, dir, "rev-parse", "HEAD")
}

func TestGitAhead_ResolvesTheShortStampBeforeComparingAndRejectsNonAncestors(t *testing.T) {
	dir, a := repo(t)
	if ahead, err := GitAhead(dir, a[:12]); err != nil || ahead {
		t.Errorf("the 12-char stamp resolving to HEAD is not ahead: %v %v", ahead, err)
	}
	if ahead, err := GitAhead(dir, ""); err != nil || ahead {
		t.Errorf("an empty commit is a quiet no-op: %v %v", ahead, err)
	}
	commitAt(t, dir, "b")
	if ahead, err := GitAhead(dir, a[:12]); err != nil || !ahead {
		t.Errorf("HEAD moved past the stamp: %v %v", ahead, err)
	}
	if _, err := GitAhead(t.TempDir(), "deadbeef"); err == nil || !strings.Contains(err.Error(), "resolve running commit") {
		t.Errorf("a non-repo errors at the resolve step: %v", err)
	}
	// A commit that exists but is not an ancestor of HEAD.
	git(t, dir, "checkout", "-q", "-b", "side")
	side := commitAt(t, dir, "s")
	git(t, dir, "checkout", "-q", "main")
	commitAt(t, dir, "c")
	if _, err := GitAhead(dir, side); err == nil || !strings.Contains(err.Error(), "not a verifiable ancestor") {
		t.Errorf("a non-ancestor is refused: %v", err)
	}
	// HEAD unresolvable while the commit resolves: the resolve-HEAD branch.
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := GitAhead(dir, a); err == nil || !strings.Contains(err.Error(), "resolve HEAD") {
		t.Errorf("an unresolvable HEAD errors at the HEAD step: %v", err)
	}
}

func TestGitProvenance_VerifiesAncestorsOnlyAndReadsTheCommitFnLive(t *testing.T) {
	dir, a := repo(t)
	commitAt(t, dir, "b")
	running := "first"
	prov := GitProvenance(func() string { return running })
	c, verify := prov(dir)
	if c != "first" {
		t.Errorf("the commit fn's value: %q", c)
	}
	running = "second"
	if c, _ := prov(dir); c != "second" {
		t.Errorf("the commit fn is read at every call, not snapshotted: %q", c)
	}
	if verify("") || verify("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef") || verify("boundary-refresh") || !verify(a) {
		t.Error("only a real ancestor of HEAD verifies; empty, foreign and sentinel commits never do")
	}
}

func TestRebuiltBinary_MissingDirNonExecutable(t *testing.T) {
	root, _, _ := project(t)
	if target, err := RebuiltBinary(root); err != nil || target != filepath.Join(root, "go", "bin", "evolve") {
		t.Errorf("the rebuilt binary: %q %v", target, err)
	}
	if err := os.Chmod(filepath.Join(root, "go", "bin", "evolve"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RebuiltBinary(root); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Errorf("a non-executable target: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "go", "bin", "evolve")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "go", "bin", "evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := RebuiltBinary(root); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Errorf("a directory at the target: %v", err)
	}
	if _, err := RebuiltBinary(t.TempDir()); err == nil || !strings.Contains(err.Error(), "rebuilt binary") {
		t.Errorf("an absent target: %v", err)
	}
}

package subagentrun

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// fixedNow is the fixture clock every run reads (four reads: exec start, exec
// end, verify, ledger ts).
var fixedNow = time.Date(2026, 5, 23, 17, 0, 0, 0, time.UTC)

func clock() time.Time { return fixedNow }

// aaRand fills the token bytes with 0xaa → "aaaaaaaaaaaaaaaa".
func aaRand(b []byte) (int, error) {
	for i := range b {
		b[i] = 0xaa
	}
	return len(b), nil
}

const aaToken = "aaaaaaaaaaaaaaaa"

var errDepth = errors.New("subagent/run: recursion depth cap exceeded — too many nested bridge dispatches (likely a fan-out loop); inspect EVOLVE_DISPATCH_DEPTH")

// recorder collects every event a Center delivered, in order.
type recorder struct{ events []signalcenter.Event }

func recording() (*signalcenter.Center, *recorder) {
	c := signalcenter.New()
	r := &recorder{}
	c.Subscribe(func(e signalcenter.Event) { r.events = append(r.events, e) })
	return c, r
}

func (r *recorder) codes() []signalcenter.Code {
	out := make([]signalcenter.Code, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Code)
	}
	return out
}

// only asserts the recorder holds exactly one event of code and returns it.
func (r *recorder) only(t *testing.T, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	if len(r.events) != 1 || r.events[0].Code != code {
		t.Fatalf("want exactly one %s, got %v", code, r.codes())
	}
	return r.events[0]
}

// fixture is the on-disk layout every run shares.
type fixture struct{ root, ws, worktree string }

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{root: root, ws: filepath.Join(root, "ws"), worktree: filepath.Join(root, "wt")}
	for _, d := range []string{f.ws, f.worktree} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f fixture) request() Request {
	return Request{Agent: "scout", Cycle: 5, WorkspacePath: f.ws, ProfilesDir: "/p", AdaptersDir: "/a", ProjectRoot: f.root, WorktreePath: f.worktree, Prompt: strings.NewReader("hi\n")}
}

// writeArtifact materialises a sound artifact: the token in the first line,
// mtime = at.
func writeArtifact(t *testing.T, path, token string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<!-- challenge-token: "+token+" -->\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// soundAdapter writes a sound artifact and exits 0.
func soundAdapter(t *testing.T) Adapter {
	return AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
		writeArtifact(t, e.ArtifactPath, e.ChallengeToken, fixedNow)
		return 0, nil
	})
}

// scoutProfile is the profile every happy run reads.
var scoutProfile = Profile{CLI: "claude", OutputArtifact: ".evolve/runs/cycle-{cycle}/scout.md", Overrides: func(string) (string, string) { return "", "" }}

// happyDeps is a Deps whose every port succeeds.
func happyDeps(t *testing.T) Deps {
	t.Helper()
	return Deps{
		Profile:    func(string) (Profile, error) { return scoutProfile, nil },
		ResolveLLM: func(string) (LLM, error) { return LLM{CLI: "claude", ModelTier: "sonnet", Source: "profile"}, nil },
		Inspect: func(string, string) (Capability, error) {
			return Capability{BudgetNative: true, PermissionScoping: true}, nil
		},
		ResolveTier:   func(TierRequest) (string, error) { return "sonnet", nil },
		KnownRole:     func(role string) bool { return role == "scout" || role == "auditor" || role == "triage" },
		GuardDepth:    func(depth int) error { return nil },
		AdapterExists: func(string) bool { return true },
		Adapter:       soundAdapter(t),
		GitState:      func(context.Context, string) (string, string, error) { return "abc123", "def456", nil },
		RunID:         func(string) string { return "" },
	}
}

// observed builds a dispatcher over deps with the fixture clock and entropy,
// reporting into a recording Center.
func observed(t *testing.T, deps Deps, opts ...Option) (*Dispatcher, *recorder) {
	t.Helper()
	c, r := recording()
	all := append([]Option{WithClock(clock), WithRand(aaRand), WithSignals(func() *signalcenter.Center { return c })}, opts...)
	return New(deps, all...), r
}

// Test 18 — construction: the production defaults and every option observed.
func TestNew_DefaultsAndOptions(t *testing.T) {
	d := New(happyDeps(t))
	if d.SignalsWired() {
		t.Fatal("no Center ⇒ not wired")
	}
	if tok, err := MintToken(d.rand); err != nil || len(tok) != 16 {
		t.Fatalf("the default entropy is crypto/rand: %q %v", tok, err)
	}
	if d.now().IsZero() || d.stager == nil || d.openLedger == nil {
		t.Fatal("the defaults are the production collaborators")
	}
	var clockRead, randRead, statRead, readRead, hashRead, stagerRead, openRead bool
	d = New(happyDeps(t),
		WithClock(func() time.Time { clockRead = true; return fixedNow }),
		WithRand(func(b []byte) (int, error) { randRead = true; return aaRand(b) }),
		WithFS(func(p string) (time.Time, error) { statRead = true; return StatMTime(p) },
			func(p string) ([]byte, error) { readRead = true; return os.ReadFile(p) },
			func(p string) (string, error) { hashRead = true; return HashFile(p) }),
		WithStager(stagerFunc(func(prompt string) (string, func(), error) {
			stagerRead = true
			return tempFileStager{create: os.CreateTemp}.Stage(prompt)
		})),
		WithLedgerOpener(func(p string) (io.WriteCloser, error) { openRead = true; return openAppend(p) }),
	)
	f := newFixture(t)
	req := f.request()
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	if _, err := d.Dispatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if !clockRead || !randRead || !statRead || !readRead || !hashRead || !stagerRead || !openRead {
		t.Fatalf("every option is observed: clock %v rand %v stat %v read %v hash %v stager %v open %v", clockRead, randRead, statRead, readRead, hashRead, stagerRead, openRead)
	}
	c, _ := recording()
	if d := New(happyDeps(t), WithSignals(func() *signalcenter.Center { return c })); !d.SignalsWired() {
		t.Fatal("WithSignals wires the accessor")
	}
	if d := New(happyDeps(t), WithSignals(func() *signalcenter.Center { return nil })); d.SignalsWired() {
		t.Fatal("an accessor returning nil is the Null Object")
	}
}

// stagerFunc adapts a function to the PromptStager port (test double).
type stagerFunc func(prompt string) (string, func(), error)

func (f stagerFunc) Stage(prompt string) (string, func(), error) { return f(prompt) }

var _ PromptStager = stagerFunc(nil)

// Test 19 — the eleven codes are registered under the bridge module with the
// BRIDGE_SUBAGENT_ sub-prefix, documented, disjoint from the engine's six, and
// the one producer stamps kind, severity, origin, phase and the step.
func TestCodes_RegisteredUnderModuleBridgeWithTheSubagentPrefix(t *testing.T) {
	engine := map[signalcenter.Code]bool{"BRIDGE_TOKEN_RESOLVER_MISSING": true, "BRIDGE_TOKEN_RESOLVER_FAILED": true, "BRIDGE_TOKEN_USAGE_WARNING": true, "BRIDGE_CONTEXT_FILL_HIGH": true, "BRIDGE_TELEMETRY_APPEND_FAILED": true, "BRIDGE_TELEMETRY_TRIPWIRE": true}
	all := []signalcenter.Code{CodeRequestRejected, CodeResolutionFailed, CodeLLMResolveFallback, CodeWorktreeFallback, CodeGitStateUnknown, CodePrepareFailed, CodeAdapterExecFailed, CodeArtifactIntegrityFail, CodeVerdictFail, CodeArtifactHashFailed, CodeLedgerWriteFailed}
	docs := signalcenter.RegisteredCodes()[signalcenter.ModuleBridge]
	documented := map[signalcenter.Code]bool{}
	for _, d := range docs {
		if d.Doc != "" {
			documented[d.Code] = true
		}
	}
	for _, c := range all {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleBridge {
			t.Errorf("%s must be registered under module bridge (got %q, %v)", c, m, ok)
		}
		if !c.BelongsTo(signalcenter.ModuleBridge) || !strings.HasPrefix(string(c), "BRIDGE_SUBAGENT_") || engine[c] || !documented[c] {
			t.Errorf("%s: sub-prefix, engine-disjoint, documented", c)
		}
	}
	d, r := observed(t, happyDeps(t))
	d.warn(identity{agent: "auditor-worker-deep", role: "auditor", worker: "deep", runID: "run-7"}, 7, CodeVerdictFail, "why", map[string]string{"step": "verify"})
	e := r.only(t, CodeVerdictFail)
	if e.Module != signalcenter.ModuleBridge || e.Kind != signalcenter.KindBridgeWarning || e.Severity != signalcenter.SeverityWarn ||
		e.Origin != "Dispatcher.Dispatch" || e.Cycle != 7 || e.Phase != "auditor-worker-deep" || e.RunID != "run-7" || e.Reason != "why" ||
		e.Fields["step"] != "verify" || e.Fields["role"] != "auditor" || e.Fields["worker"] != "deep" {
		t.Fatalf("the producer's stamp: %+v", e)
	}
	d.warn(identity{agent: "scout", role: "scout"}, 1, CodeVerdictFail, "why", nil)
	if _, has := r.events[1].Fields["worker"]; has || r.events[1].Fields["role"] != "scout" {
		t.Fatalf("no worker field without a worker: %+v", r.events[1].Fields)
	}
}

// The request shapes are constructed positionally so a new field breaks this
// test at compile time and the host's ONE projection is revisited.
func TestRequestShapes_HaveExactlyTheDeclaredFields(t *testing.T) {
	r := Request{"a", 1, "ws", "p", "a", "c", "root", "wt", "l", strings.NewReader(""), "hint", "ovr", true, true, false, 2, "tok"}
	o := Outcome{"PASS", "claude", "sonnet", "/a", "sha", "tok", 0, 1, []string{"w"}, nil, ""}
	tr := TierRequest{"p", 1, "root", "wt", "hint", "ovr", true}
	if r.Agent != "a" || o.Verdict != "PASS" || tr.ProfilePath != "p" {
		t.Fatal("positional construction")
	}
}

// sequencer logs the order of the ports and the stdlib collaborators one run
// touches.
type sequencer struct{ log []string }

func (s *sequencer) mark(step string) { s.log = append(s.log, step) }

// Test 49 — the step order is admit → resolve (run id, profile, cli, driver,
// tier, capability) → prepare (artifact dir, token, git, prompt) → stage →
// exec → verify → hash → ledger, the stager cleanup after the exec, and the
// exec error returned last.
func TestDispatch_StepOrderIsAdmitResolvePrepareStageExecVerifyRecord(t *testing.T) {
	f := newFixture(t)
	seq := &sequencer{}
	deps := happyDeps(t)
	deps.RunID = func(string) string { seq.mark("run_id"); return "run-7" }
	deps.KnownRole = func(string) bool { seq.mark("role"); return true }
	deps.GuardDepth = func(int) error { seq.mark("depth"); return nil }
	deps.Profile = func(string) (Profile, error) { seq.mark("profile"); return scoutProfile, nil }
	deps.ResolveLLM = func(string) (LLM, error) { seq.mark("llm"); return LLM{CLI: "claude", Source: "profile"}, nil }
	deps.AdapterExists = func(string) bool { seq.mark("driver"); return true }
	deps.ResolveTier = func(TierRequest) (string, error) { seq.mark("tier"); return "sonnet", nil }
	deps.Inspect = func(string, string) (Capability, error) { seq.mark("capability"); return Capability{}, nil }
	deps.GitState = func(context.Context, string) (string, string, error) { seq.mark("git"); return "h", "t", nil }
	deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
		seq.mark("exec")
		if _, err := os.Stat(e.PromptFile); err != nil {
			t.Fatalf("the prompt is staged before the exec: %v", err)
		}
		writeArtifact(t, e.ArtifactPath, e.ChallengeToken, fixedNow)
		return -1, errors.New("crashed")
	})
	rng := func(b []byte) (int, error) { seq.mark("token"); return aaRand(b) }
	stat := func(p string) (time.Time, error) { seq.mark("verify"); return StatMTime(p) }
	hash := func(p string) (string, error) { seq.mark("hash"); return HashFile(p) }
	var promptPath string
	stager := stagerFunc(func(prompt string) (string, func(), error) {
		seq.mark("stage")
		p, cleanup, err := tempFileStager{create: os.CreateTemp}.Stage(prompt)
		promptPath = p
		return p, func() { seq.mark("cleanup"); cleanup() }, err
	})
	open := func(p string) (io.WriteCloser, error) { seq.mark("ledger"); return openAppend(p) }
	d, _ := observed(t, deps, WithRand(rng), WithFS(stat, os.ReadFile, hash), WithStager(stager), WithLedgerOpener(open))
	req := f.request()
	req.Prompt = readerFunc(func(p []byte) (int, error) { seq.mark("prompt"); return 0, io.EOF })
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	out, err := d.Dispatch(context.Background(), req)
	if err == nil || !strings.HasPrefix(err.Error(), "subagent/run: adapter exec: ") || out.ExitCode != -1 {
		t.Fatalf("the exec error returned last with the outcome populated: %v %+v", err, out)
	}
	want := "role depth run_id profile llm driver tier capability token git prompt stage exec verify hash ledger cleanup"
	if got := strings.Join(seq.log, " "); got != want {
		t.Fatalf("order:\n got %s\nwant %s", got, want)
	}
	if _, err := os.Stat(promptPath); !os.IsNotExist(err) {
		t.Fatal("the staged prompt is removed after the run")
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

// Test 50 — no Center and no options: a full run through fakes with the
// production stager and opener, the Null Object at every producer, an
// Outcome identical to the observed run's.
func TestDispatch_NoCenterIsTheNullObjectAndDefaultsAreTheProductionOnes(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.GitState = func(context.Context, string) (string, string, error) { return "", "", errors.New("no git") } // a provoked WARN into nothing
	deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
		writeArtifact(t, e.ArtifactPath, e.ChallengeToken, time.Now())
		return 3, nil
	})
	bare := New(deps, WithRand(aaRand))
	req := f.request()
	req.WorktreePath = "" // the fallback WARN into nothing
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	if bare.SignalsWired() {
		t.Fatal("not wired")
	}
	got, err := bare.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	obs, r := observed(t, deps)
	req.Prompt = strings.NewReader("hi\n")
	want, err := obs.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got.DurationMS, want.DurationMS = 0, 0
	if got.Verdict != VerdictFAIL || got.ExitCode != 3 || len(got.Warns) != 1 || got.ArtifactSHA256 == "" ||
		got.Verdict != want.Verdict || got.ArtifactSHA256 != want.ArtifactSHA256 || got.Warns[0] != want.Warns[0] || got.ChallengeToken != want.ChallengeToken {
		t.Fatalf("the same outcome with or without a Center:\n got %+v\nwant %+v", got, want)
	}
	if len(r.events) != 3 {
		t.Fatalf("the observed twin reported the git fallback, the worktree fallback and the verdict: %v", r.codes())
	}
}

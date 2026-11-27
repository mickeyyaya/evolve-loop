package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHeadlessOutcomeSpec_ClassifiesTheRecordedResults(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("claude-p")
	if err != nil || m.HeadlessOutcome == nil {
		t.Fatalf("claude-p headless_outcome: %v %v", m.HeadlessOutcome, err)
	}
	cases := []struct{ name, result, stderr, want string }{
		{"usage limit", "You've hit your limit · resets 3pm (UTC)", "", "rate_limit"},
		{"legacy usage limit", "Claude AI usage limit reached|1700000000", "", "rate_limit"},
		{"rate limit error", "API Error: rate_limit_error", "", "rate_limit"},
		{"login", "Invalid API key · Please run /login", "", "auth_recheck"},
		{"login on stderr", "", "Login expired · run /login\n", "auth_recheck"},
		{"quota on stderr", "", "you have reached your usage limit\n", "rate_limit"},
		{"quota with the error prefix on stderr", "", "Error: Claude AI usage limit reached|1700000000\n", "rate_limit"},
		{"free text about the percent left on stderr", "", "note: the dashboard shows 0% left of the monthly cap\n", ""},
		{"free text about too many requests on stderr", "", "retry helper: too many requests seen in the docs\n", ""},
		{"free text mid line on stderr", "", "the docs say you have reached your usage limit means wait\n", ""},
		{"generic", "Tool Write failed: ENOENT", "boom\n", ""},
		{"empty", "", "", ""},
	}
	for _, tc := range cases {
		got, err := m.HeadlessOutcome.Classify(HeadlessEvidence{ResultText: tc.result, Stderr: tc.stderr})
		if err != nil || got != tc.want {
			t.Errorf("%s: Classify = (%q, %v), want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestHeadlessOutcomeSpec_NilSpecClassifiesNothing(t *testing.T) {
	t.Parallel()
	var spec *HeadlessOutcomeSpec
	if got, err := spec.Classify(HeadlessEvidence{ResultText: "usage limit reached"}); got != "" || err != nil {
		t.Fatalf("nil spec = (%q, %v), want empty", got, err)
	}
}

func TestHeadlessOutcomeSpec_RejectsBrokenRules(t *testing.T) {
	t.Parallel()
	for _, rule := range []HeadlessRule{
		{Pattern: "x", Source: "result", Regex: "("},
		{Pattern: "x", Source: "result", RegexFrom: "claude-tmux"},
		{Pattern: "x", Source: "result", RegexFrom: "no-such-cli:prompt:rate_limit"},
		{Pattern: "x", Source: "result", RegexFrom: "claude-tmux:prompt:no_such_prompt"},
		{Pattern: "x", Source: "result", RegexFrom: "claude-tmux:control:no_such_control"},
		{Pattern: "x", Source: "result", RegexFrom: "claude-tmux:other:rate_limit"},
		{Pattern: "x", Source: "result", RegexFrom: "claude-p:regex:no_such_regex"},
	} {
		spec := &HeadlessOutcomeSpec{Rules: []HeadlessRule{rule}}
		if got, err := spec.Classify(HeadlessEvidence{ResultText: "anything"}); err == nil || got != "" {
			t.Errorf("rule %+v = (%q, %v), want an error", rule, got, err)
		}
	}
}

func TestReadHeadlessResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name string
		path string
		want headlessResult
	}{
		{"error result", write("e", claudePFixtureText(t, "usage-limit.jsonl")), headlessResult{text: "You've hit your limit · resets 3pm (UTC)", isError: true, found: true}},
		{"success result", write("s", claudePFixtureText(t, "success.jsonl")), headlessResult{text: "The deploy notes say you have reached your usage limit docs section.", found: true}},
		{"stream events are never evidence", write("ev", "{\"type\":\"stream_event\",\"event\":{\"text\":\"rate_limit_error\"}}\n"), headlessResult{text: "", isError: true}},
		{"a plain line before trailing events is kept", write("pe", "plain boom\n{\"type\":\"stream_event\"}\n\n"), headlessResult{text: "plain boom", isError: true}},
		{"json without a type is a plain line", write("nt", "{\"a\":1}\n"), headlessResult{text: "{\"a\":1}", isError: true}},
		{"plain text takes the last line", write("p", "a\nb\n\n"), headlessResult{text: "b", isError: true}},
		{"missing file", filepath.Join(dir, "none"), headlessResult{text: "", isError: true}},
		{"oversize file keeps the tail", write("big", strings.Repeat("x", headlessTailBytes+10)+"\nlast line\n"), headlessResult{text: "last line", isError: true}},
	}
	for _, tc := range cases {
		if got := readHeadlessResult(tc.path); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func claudePFixtureText(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "claude-p-results", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestHeadlessEvidenceFor(t *testing.T) {
	t.Parallel()
	crowd := "old one\nold two\n\nthree\nfour\nfive\nsix\nseven\n"
	cases := []struct {
		name, fixture, stderr string
		want                  HeadlessEvidence
	}{
		{"a success result carries no evidence at all", "success.jsonl", "warn\n", HeadlessEvidence{}},
		{"an error result carries its text and the stderr tail", "generic-error.jsonl", "warn\n", HeadlessEvidence{ResultText: "Tool Write failed: ENOENT", Stderr: "warn"}},
		{"only the last non-empty stderr lines count", "generic-error.jsonl", crowd, HeadlessEvidence{ResultText: "Tool Write failed: ENOENT", Stderr: "three\nfour\nfive\nsix\nseven"}},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		cfg := &Config{StdoutLog: filepath.Join(dir, "o"), StderrLog: filepath.Join(dir, "e")}
		_ = os.WriteFile(cfg.StdoutLog, []byte(claudePFixtureText(t, tc.fixture)), 0o644)
		_ = os.WriteFile(cfg.StderrLog, []byte(tc.stderr), 0o644)
		if got := headlessEvidenceFor(cfg); got != tc.want {
			t.Errorf("%s: evidence = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

type testClock struct{ ns atomic.Int64 }

func newTestClock() *testClock {
	c := &testClock{}
	c.ns.Store(time.Unix(1000, 0).UnixNano())
	return c
}

func (c *testClock) Now() time.Time { return time.Unix(0, c.ns.Load()) }

func (c *testClock) Advance(d time.Duration) { c.ns.Add(int64(d)) }

func appendFile(t *testing.T, path, body string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(body); err != nil {
		t.Fatal(err)
	}
}

func TestIdleWatch_GrowthResetsTheClockAndSilenceFires(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	appendFile(t, out, "")
	clock := newTestClock()
	asked := make(chan time.Duration)
	ticks := make(chan time.Time)
	w := newIdleWatch(idleSource{
		now:   clock.Now,
		after: func(d time.Duration) <-chan time.Time { asked <- d; return ticks },
		paths: []string{out, filepath.Join(dir, "missing")},
	})
	cancelled := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		w.watchIdle(10*time.Second, func() { close(cancelled) }, make(chan struct{}))
	}()
	if got := <-asked; got != 10*time.Second {
		t.Fatalf("first wait = %v, want 10s", got)
	}
	clock.Advance(4 * time.Second)
	ticks <- clock.Now()
	if got := <-asked; got != 6*time.Second {
		t.Fatalf("silent early tick wait = %v, want the remaining 6s", got)
	}
	appendFile(t, out, "x")
	clock.Advance(6 * time.Second)
	ticks <- clock.Now()
	if got := <-asked; got != 10*time.Second {
		t.Fatalf("wait after growth = %v, want the full 10s", got)
	}
	clock.Advance(10 * time.Second)
	ticks <- clock.Now()
	<-cancelled
	<-finished
	if !w.fired.Load() {
		t.Fatal("fired = false, want true")
	}
}

func TestIdleWatch_StopsWhenTheRunEnds(t *testing.T) {
	t.Parallel()
	w := newIdleWatch(idleSource{now: time.Now, after: func(time.Duration) <-chan time.Time { return nil }})
	done := make(chan struct{})
	close(done)
	w.watchIdle(time.Hour, func() { t.Error("cancel must not run") }, done)
	if w.fired.Load() {
		t.Fatal("fired after done")
	}
}

func TestArtifactBudget_Sources(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		cfg, deps    int
		depsExt      int
		wantInterval int
		wantExtends  int
	}{
		{"the config wins", 9, 50, 3, 9, 3},
		{"the deps default", 0, 50, 0, 50, defaultArtifactMaxExtends},
		{"the built in default", 0, 0, 0, tmuxArtifactTimeoutS, defaultArtifactMaxExtends},
	} {
		interval, extends := artifactBudget(&Config{ArtifactTimeoutS: tc.cfg}, Deps{ArtifactTimeoutS: tc.deps, ArtifactMaxExtends: tc.depsExt})
		if interval != tc.wantInterval || extends != tc.wantExtends {
			t.Errorf("%s: (%d, %d), want (%d, %d)", tc.name, interval, extends, tc.wantInterval, tc.wantExtends)
		}
	}
}

func TestHeadlessIdleBudget_IsTheFullReviewBudget(t *testing.T) {
	t.Parallel()
	got := headlessIdleBudget(&Config{ArtifactTimeoutS: 9}, Deps{ArtifactMaxExtends: 3})
	if want := 36 * time.Second; got != want {
		t.Fatalf("budget = %v, want %v", got, want)
	}
}

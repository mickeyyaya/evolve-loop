# Bidirectional Communication Channel — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a programmatic supervisor monitor a long-running tmux-REPL LLM-CLI phase via a live filtered feed and inject correlated questions, getting the matching answer back — with a read-only human debug tail.

**Architecture:** Reuse the ADR-0023 inbox for outbound asks (add a `CorrID`); wake the dormant `phasestream.Normalizer` inside a new `channel.Producer` (spawned beside the autospawn observer, single writer of `<agent>-channel.ndjson`); the tmux driver emits two structured stderr breadcrumbs (`inject_applied`/`idle_reached`) the producer turns into `correlation` envelopes that bracket the answer span. A `Supervisor` Go API asks via the inbox and reads answers from the feed; `evolve bridge watch` tails the feed read-only.

**Tech Stack:** Go (stdlib only), `go test -race -cover`, existing fakes (`deps.Tmux`, `deps.Runner`, injected `Now`/clock). Spec: `docs/superpowers/specs/2026-06-04-bidirectional-channel-design.md`.

**Coverage gate (all tasks):** every new/changed package must report **≥95% statement coverage**. Each commit runs `go test -race -cover ./internal/<pkg>/...` and the task is not done until the touched package prints `coverage: ≥95.0% of statements`. Behavioral tests only — no line-padding.

**Global rollback:** the whole feature is gated `EVOLVE_CHANNEL=1` (producer + `CorrID` plumbing) and `EVOLVE_CHANNEL_SUPERVISOR=1` (supervisor attach). With both unset the pipeline is byte-identical to today.

---

## File Structure

| File | Responsibility | New/Modify |
|---|---|---|
| `go/internal/bridge/inbox/inbox.go` | add optional `CorrID` field to `Envelope` | Modify |
| `go/internal/phasestream/envelope.go` | add `KindCorrelation` + correlation `Data` keys | Modify |
| `go/internal/phasestream/correlation.go` | breadcrumb-line detection + correlation-envelope minting + span state | New |
| `go/internal/phasestream/classify.go` | route stderr breadcrumb lines into correlation.go | Modify |
| `go/internal/bridge/driver_tmux_repl.go` | emit `inject_applied`/`idle_reached` stderr breadcrumbs | Modify |
| `go/internal/bridge/channel/feed.go` | feed path + envelope kinds shared by producer/supervisor/watch | New |
| `go/internal/bridge/channel/producer.go` | run `Normalizer.Poll` loop → feed (single writer) | New |
| `go/internal/bridge/channel/supervisor.go` | `Ask` / `Feed` / `response_timeout` / `ErrTransportNoInject` | New |
| `go/internal/bridge/channel/policy.go` | minimal default policy (stall → ask summary) | New |
| `go/internal/adapters/observer/core_adapter.go` | spawn the producer beside the observer when `EVOLVE_CHANNEL=1` | Modify |
| `go/cmd/evolve/cmd_bridge_watch.go` | `evolve bridge watch` read-only tail | New |
| `go/cmd/evolve/cmd_bridge.go` | register the `watch` subcommand | Modify |

Build order below is dependency-safe: data plumbing (1–2) → feed parsing (3) → driver breadcrumbs (4) → producer (5) → spawn wiring (6) → supervisor (7) → policy (8) → CLI (9) → e2e (10).

---

## Task 1: Add `CorrID` to the inbox envelope

**Files:**
- Modify: `go/internal/bridge/inbox/inbox.go:50-61`
- Test: `go/internal/bridge/inbox/inbox_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestEnvelope_CorrID_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	want := Envelope{Kind: KindCommand, Body: "summarize", CorrID: "c1", Source: "supervisor"}
	if err := Append(dir, "build", want, now); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := NewCursor(dir, "build").Drain()
	if err != nil || len(got) != 1 {
		t.Fatalf("drain: %v len=%d", err, len(got))
	}
	if got[0].CorrID != "c1" {
		t.Fatalf("CorrID = %q, want c1", got[0].CorrID)
	}
}
```

- [ ] **Step 2: Run it — expect FAIL** (`Envelope has no field CorrID`)

Run: `cd go && go test ./internal/bridge/inbox/ -run TestEnvelope_CorrID_RoundTrips`

- [ ] **Step 3: Add the field** (after `Source`, before `DeferCount`, line 55)

```go
	// CorrID, when set, ties this injected ask to the agent's reply in the
	// channel feed (ADR-0037). The driver echoes it in its inject_applied /
	// idle_reached breadcrumbs; the producer brackets the answer span by it.
	CorrID string `json:"corr_id,omitempty"`
```

- [ ] **Step 4: Run — expect PASS.** Then `go test -race -cover ./internal/bridge/inbox/` → confirm **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/bridge/inbox/inbox.go go/internal/bridge/inbox/inbox_test.go
git commit -m "feat(inbox): add optional CorrID for correlated asks (ADR-0037)"
```

---

## Task 2: Add the `correlation` kind + data keys to phasestream

**Files:**
- Modify: `go/internal/phasestream/envelope.go:38-52` (Kind block)
- Test: `go/internal/phasestream/envelope_test.go` (create if absent)

- [ ] **Step 1: Write the failing test**

```go
func TestKindCorrelation_Defined(t *testing.T) {
	if KindCorrelation != "correlation" {
		t.Fatalf("KindCorrelation = %q", KindCorrelation)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: KindCorrelation`)

Run: `cd go && go test ./internal/phasestream/ -run TestKindCorrelation_Defined`

- [ ] **Step 3: Add the kind** (in the `const (... )` Kind block, after `KindStall`)

```go
	KindCorrelation Kind = "correlation"
```

Document the `Data` contract in a comment above the const block:

```go
// KindCorrelation Data keys (ADR-0037):
//   sub: "request"           + corr_id, at_seq
//   sub: "response_complete" + corr_id, start_seq, end_seq
//   sub: "response_timeout"  + corr_id, waited_s
```

- [ ] **Step 4: Run — expect PASS.** `go test -cover ./internal/phasestream/` (package already ~high; keep **≥95%**).

- [ ] **Step 5: Commit**

```bash
git add go/internal/phasestream/envelope.go go/internal/phasestream/envelope_test.go
git commit -m "feat(phasestream): add correlation envelope kind (ADR-0037)"
```

---

## Task 3: Breadcrumb → correlation-envelope minting (with span state)

**Files:**
- Create: `go/internal/phasestream/correlation.go`
- Test: `go/internal/phasestream/correlation_test.go`

The driver writes these exact JSON lines to its stderr (Task 4):
`{"evolve_channel":"inject_applied","corr_id":"c1"}` and
`{"evolve_channel":"idle_reached","corr_id":"c1"}`.
`correlation.go` is a pure function over (line, currentSeq, state) → envelopes.

- [ ] **Step 1: Write the failing test**

```go
func TestCorrelation_BracketsAnswerSpan(t *testing.T) {
	c := newCorrelator()
	// inject_applied at seq 10 → a request envelope tagged at_seq=10
	reqs := c.observe([]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`), 10)
	if len(reqs) != 1 || reqs[0].sub != "request" || reqs[0].atSeq != 10 {
		t.Fatalf("request = %+v", reqs)
	}
	// idle_reached at seq 17 → response_complete span [11,17]
	done := c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"c1"}`), 17)
	if len(done) != 1 || done[0].sub != "response_complete" || done[0].startSeq != 11 || done[0].endSeq != 17 {
		t.Fatalf("done = %+v", done)
	}
}

func TestCorrelation_IgnoresNonBreadcrumb(t *testing.T) {
	c := newCorrelator()
	if got := c.observe([]byte("normal stderr line"), 3); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestCorrelation_DuplicateIdleIgnored(t *testing.T) {
	c := newCorrelator()
	c.observe([]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`), 1)
	c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"c1"}`), 5)
	if got := c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"c1"}`), 9); got != nil {
		t.Fatalf("second idle should be a no-op, got %+v", got)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: newCorrelator`)

Run: `cd go && go test ./internal/phasestream/ -run TestCorrelation`

- [ ] **Step 3: Implement `correlation.go`**

```go
package phasestream

import "encoding/json"

// breadcrumb is the driver's structured stderr marker (Task 4 emits it).
type breadcrumb struct {
	Channel string `json:"evolve_channel"`
	CorrID  string `json:"corr_id"`
}

// corrMark is the internal pre-envelope form the classifier converts to a
// KindCorrelation Envelope. sub ∈ {request, response_complete}.
type corrMark struct {
	sub      string
	corrID   string
	atSeq    int64 // request
	startSeq int64 // response_complete
	endSeq   int64 // response_complete
}

// correlator tracks the open inject per corr_id so an idle_reached can be
// bracketed into a span. One per phase (the classifier owns it).
type correlator struct {
	openAtSeq map[string]int64
}

func newCorrelator() *correlator {
	return &correlator{openAtSeq: map[string]int64{}}
}

// observe parses one stderr line. currentSeq is the classifier's seq counter
// at this line. Returns nil for non-breadcrumb lines.
func (c *correlator) observe(line []byte, currentSeq int64) []corrMark {
	var b breadcrumb
	if err := json.Unmarshal(line, &b); err != nil || b.Channel == "" || b.CorrID == "" {
		return nil
	}
	switch b.Channel {
	case "inject_applied":
		c.openAtSeq[b.CorrID] = currentSeq
		return []corrMark{{sub: "request", corrID: b.CorrID, atSeq: currentSeq}}
	case "idle_reached":
		at, ok := c.openAtSeq[b.CorrID]
		if !ok {
			return nil // no matching open inject (dup / out-of-order) → ignore
		}
		delete(c.openAtSeq, b.CorrID)
		return []corrMark{{sub: "response_complete", corrID: b.CorrID, startSeq: at + 1, endSeq: currentSeq}}
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run — expect PASS.** `go test -race -cover ./internal/phasestream/` → **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/phasestream/correlation.go go/internal/phasestream/correlation_test.go
git commit -m "feat(phasestream): bracket inject/idle breadcrumbs into correlation spans"
```

---

## Task 4: Wire the correlator into the classifier's stderr path

**Files:**
- Modify: `go/internal/phasestream/classify.go` (the `Stderr` method, near line 330 where "Plaintext infra markers (CLI error lines, tmux scrollback)" are handled) and the `Classifier` struct (add `corr *correlator`, init in `NewClassifier`)
- Test: `go/internal/phasestream/classify_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestClassifier_Stderr_EmitsCorrelationEnvelopes(t *testing.T) {
	clf := NewClassifier(Source{Phase: "build", Agent: "build"}, "trace", func() time.Time { return time.Unix(0, 0).UTC() })
	// prime seq with one content line so the request at_seq is non-zero
	_ = clf.Line([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`))
	got := clf.Stderr([]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`))
	if len(got) != 1 || got[0].Kind != KindCorrelation || got[0].Data["sub"] != "request" || got[0].Data["corr_id"] != "c1" {
		t.Fatalf("correlation envelope = %+v", got)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (correlation not emitted; `got` empty)

Run: `cd go && go test ./internal/phasestream/ -run TestClassifier_Stderr_EmitsCorrelationEnvelopes`

- [ ] **Step 3: Implement.** In the `Classifier` struct add `corr *correlator`; in `NewClassifier` set `corr: newCorrelator()`. At the top of `Stderr(raw []byte) []Envelope`, before the existing plaintext-marker handling, add:

```go
	if marks := c.corr.observe(raw, c.seq); len(marks) > 0 {
		var out []Envelope
		for _, m := range marks {
			data := map[string]any{"sub": m.sub, "corr_id": m.corrID}
			switch m.sub {
			case "request":
				data["at_seq"] = m.atSeq
			case "response_complete":
				data["start_seq"] = m.startSeq
				data["end_seq"] = m.endSeq
			}
			out = append(out, c.Emit(KindCorrelation, SeverityInfo, data))
		}
		return out
	}
```

(Use the classifier's existing seq field name — confirm via the struct; if it is `c.seq`, the snippet is correct; otherwise substitute the real field. `c.Emit` is the existing envelope minter used elsewhere in classify.go.)

- [ ] **Step 4: Run — expect PASS.** `go test -race -cover ./internal/phasestream/` → **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/phasestream/classify.go go/internal/phasestream/classify_test.go
git commit -m "feat(phasestream): route stderr breadcrumbs to correlation minting"
```

---

## Task 5: Driver emits `inject_applied` / `idle_reached` breadcrumbs

**Files:**
- Modify: `go/internal/bridge/driver_tmux_repl.go` — `injectEnvelope` (emit `inject_applied` when an idle-gated ask with `CorrID` is delivered) and the poll loop's idle transition (emit `idle_reached`)
- Test: `go/internal/bridge/driver_tmux_repl_corr_test.go` (new)

The breadcrumb writer is a tiny helper so both call sites are identical and testable:

- [ ] **Step 1: Write the failing test**

```go
func TestEmitBreadcrumb_Format(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "inject_applied", "c1")
	got := strings.TrimSpace(buf.String())
	want := `{"evolve_channel":"inject_applied","corr_id":"c1"}`
	if got != want {
		t.Fatalf("breadcrumb = %s, want %s", got, want)
	}
}

func TestEmitBreadcrumb_EmptyCorrIDNoOp(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "inject_applied", "")
	if buf.Len() != 0 {
		t.Fatalf("empty corr_id must not write, got %q", buf.String())
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: emitChannelBreadcrumb`)

Run: `cd go && go test ./internal/bridge/ -run TestEmitBreadcrumb`

- [ ] **Step 3: Implement the helper** (top of `driver_tmux_repl.go`, after imports)

```go
// emitChannelBreadcrumb writes one structured channel marker to w (the
// driver's stderr log). The producer's correlator parses these to bracket an
// injected ask's answer span (ADR-0037). Empty corrID is a no-op so
// non-correlated injects add no noise.
func emitChannelBreadcrumb(w io.Writer, channel, corrID string) {
	if corrID == "" {
		return
	}
	fmt.Fprintf(w, "{\"evolve_channel\":%q,\"corr_id\":%q}\n", channel, corrID)
}
```

- [ ] **Step 4: Run the helper tests — expect PASS.**

- [ ] **Step 5: Call it at the two sites.** In `injectEnvelope`, right after the idle-gated ask is pasted (the `injectText(...)` for `KindCommand`/`KindNudge`), add:

```go
	emitChannelBreadcrumb(deps.Stderr, "inject_applied", env.CorrID)
```

In the poll loop (`runTmuxREPL`), track the open corr_id from the last delivered ask and, when the prompt marker reappears after a busy turn (the existing idle detection used for idle-gating), emit once:

```go
	emitChannelBreadcrumb(deps.Stderr, "idle_reached", openCorrID)
	openCorrID = "" // bracket closed
```

Add an integration-style test using the existing fake tmux (toggling marker visibility) that asserts both breadcrumbs land in the captured stderr. Model it on the existing `driver_tmux_repl` tests (search `fakeTmux` in `go/internal/bridge/*_test.go`).

- [ ] **Step 6: Run — expect PASS.** `go test -race -cover ./internal/bridge/` → **≥95%** on the new lines (the file is large; the gate is on the diff — run `go test -coverprofile` and confirm the new funcs are fully covered).

- [ ] **Step 7: Commit**

```bash
git add go/internal/bridge/driver_tmux_repl.go go/internal/bridge/driver_tmux_repl_corr_test.go
git commit -m "feat(bridge): emit inject_applied/idle_reached channel breadcrumbs"
```

---

## Task 6: `channel.feed` — shared feed path + helpers

**Files:**
- Create: `go/internal/bridge/channel/feed.go`
- Test: `go/internal/bridge/channel/feed_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestFeedPath(t *testing.T) {
	if got := FeedPath("/ws", "build"); got != "/ws/build-channel.ndjson" {
		t.Fatalf("FeedPath = %q", got)
	}
	if got := FeedPath("/ws", ""); got != "/ws/agent-channel.ndjson" {
		t.Fatalf("empty agent: FeedPath = %q", got)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: FeedPath`)

Run: `cd go && go test ./internal/bridge/channel/ -run TestFeedPath`

- [ ] **Step 3: Implement**

```go
// Package channel is the bidirectional communication channel for long-running
// tmux-REPL phases (ADR-0037): a live filtered inbound feed produced beside the
// observer, plus correlated outbound asks over the ADR-0023 inbox.
package channel

import "path/filepath"

// FeedPath is the canonical per-agent live feed file. Producer (sole writer),
// Supervisor, and `evolve bridge watch` MUST all call this.
func FeedPath(workspace, agent string) string {
	if agent == "" {
		agent = "agent"
	}
	return filepath.Join(workspace, agent+"-channel.ndjson")
}
```

- [ ] **Step 4: Run — expect PASS.** `go test -cover ./internal/bridge/channel/` → **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/bridge/channel/feed.go go/internal/bridge/channel/feed_test.go
git commit -m "feat(channel): canonical live-feed path helper"
```

---

## Task 7: `channel.Producer` — Normalizer.Poll → feed (single writer)

**Files:**
- Create: `go/internal/bridge/channel/producer.go`
- Test: `go/internal/bridge/channel/producer_test.go`

The producer owns the only feed write handle. It wraps `phasestream.NewNormalizer` (StdoutPath, StderrPath, Sink=feed file) and ticks `Poll()` until ctx cancels.

- [ ] **Step 1: Write the failing test**

```go
func TestProducer_WritesContentAndCorrelation(t *testing.T) {
	ws := t.TempDir()
	stdout := filepath.Join(ws, "build-stdout.log")
	stderr := filepath.Join(ws, "build-stderr.log")
	os.WriteFile(stdout, []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`+"\n"), 0o644)
	os.WriteFile(stderr, []byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`+"\n"), 0o644)

	p := NewProducer(ProducerConfig{Workspace: ws, Agent: "build", Phase: "build", Cycle: 1,
		Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	// one poll is enough; cancel after a beat
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	data, _ := os.ReadFile(FeedPath(ws, "build"))
	if !strings.Contains(string(data), `"kind":"assistant_text"`) {
		t.Fatalf("feed missing content envelope:\n%s", data)
	}
	if !strings.Contains(string(data), `"kind":"correlation"`) {
		t.Fatalf("feed missing correlation envelope:\n%s", data)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: NewProducer`)

Run: `cd go && go test ./internal/bridge/channel/ -run TestProducer`

- [ ] **Step 3: Implement** (use the real `phasestream.NormalizerConfig` fields: `Source, TraceID, StdoutPath, StderrPath, Sink, Now`)

```go
package channel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
)

type ProducerConfig struct {
	Workspace string
	Agent     string
	Phase     string
	Cycle     int
	CLI       string
	PollEvery time.Duration   // default 2s
	Now       func() time.Time
}

type Producer struct{ cfg ProducerConfig }

func NewProducer(cfg ProducerConfig) *Producer {
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 2 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Producer{cfg: cfg}
}

// Run is the producer goroutine: it is the SOLE writer of the feed file. It
// opens the feed O_APPEND, builds one Normalizer over the phase's raw logs, and
// Polls until ctx cancels. Best-effort: open/poll errors degrade (warn) and
// never block the phase.
func (p *Producer) Run(ctx context.Context) error {
	feed, err := os.OpenFile(FeedPath(p.cfg.Workspace, p.cfg.Agent), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("channel: open feed: %w", err)
	}
	defer func() { _ = feed.Close() }()

	n := phasestream.NewNormalizer(phasestream.NormalizerConfig{
		Source:     phasestream.Source{Producer: "normalizer", CLI: p.cfg.CLI, Cycle: p.cfg.Cycle, Phase: p.cfg.Phase, Agent: p.cfg.Agent},
		TraceID:    fmt.Sprintf("cycle-%d-%s-channel", p.cfg.Cycle, p.cfg.Phase),
		StdoutPath: filepath.Join(p.cfg.Workspace, p.cfg.Phase+"-stdout.log"),
		StderrPath: filepath.Join(p.cfg.Workspace, p.cfg.Phase+"-stderr.log"),
		Sink:       feed,
		Now:        p.cfg.Now,
	})

	t := time.NewTicker(p.cfg.PollEvery)
	defer t.Stop()
	for {
		if _, err := n.Poll(); err != nil {
			fmt.Fprintf(os.Stderr, "[channel] WARN poll: %v\n", err)
		}
		select {
		case <-ctx.Done():
			_, _ = n.Poll() // final drain so the trailing answer/idle isn't lost
			return nil
		case <-t.C:
		}
	}
}
```

- [ ] **Step 4: Run — expect PASS.** Add edge tests: feed-open error path (point Workspace at a read-only dir → `Run` returns error, no panic); `EVOLVE_CHANNEL` gating is in Task 8 wiring, not here. `go test -race -cover ./internal/bridge/channel/` → **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/bridge/channel/producer.go go/internal/bridge/channel/producer_test.go
git commit -m "feat(channel): live Producer (Normalizer.Poll → feed, single writer)"
```

---

## Task 8: Spawn the producer beside the observer (gated)

**Files:**
- Modify: `go/internal/adapters/observer/core_adapter.go:120-151` (after `obs := New(...)`, inside `Start`)
- Test: `go/internal/adapters/observer/core_adapter_channel_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
func TestCoreAdapter_SpawnsProducer_WhenChannelOn(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"), 0o644)
	a := &CoreAdapter{EnvLookup: func(k string) string {
		if k == "EVOLVE_CHANNEL" { return "1" }
		return ""
	}}
	cancel := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 1})
	time.Sleep(30 * time.Millisecond)
	cancel()
	if _, err := os.Stat(channel.FeedPath(ws, "build")); err != nil {
		t.Fatalf("feed not produced: %v", err)
	}
}

func TestCoreAdapter_NoProducer_WhenChannelOff(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte("{}\n"), 0o644)
	a := &CoreAdapter{EnvLookup: func(string) string { return "" }}
	cancel := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 1})
	time.Sleep(20 * time.Millisecond)
	cancel()
	if _, err := os.Stat(channel.FeedPath(ws, "build")); !os.IsNotExist(err) {
		t.Fatalf("feed should not exist when channel off, err=%v", err)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (feed file never created)

Run: `cd go && go test ./internal/adapters/observer/ -run TestCoreAdapter_.*Producer`

- [ ] **Step 3: Implement.** In `Start`, after the observer goroutine is launched and before building the cancel closure, add (reuse the existing `get := os.Getenv / a.EnvLookup` pattern):

```go
	// ADR-0037: when EVOLVE_CHANNEL=1, spawn the live channel producer beside
	// the observer. It is the SOLE writer of <agent>-channel.ndjson. Off →
	// byte-identical to pre-channel behavior.
	var prodCancel func()
	if a.envGet("EVOLVE_CHANNEL") == "1" {
		p := channel.NewProducer(channel.ProducerConfig{
			Workspace: req.Workspace, Agent: phase, Phase: phase, Cycle: req.Cycle,
		})
		pctx, pcancel := context.WithCancel(ctx)
		wg.Add(1)
		go func() { defer wg.Done(); _ = p.Run(pctx) }()
		prodCancel = pcancel
	}
```

Add a tiny `func (a *CoreAdapter) envGet(k string) string` mirroring `resolveDuration`'s lookup, and call `prodCancel()` (nil-guarded) inside the existing `once.Do` cancel closure before `wg.Wait()`. Add the `channel` import.

- [ ] **Step 4: Run — expect PASS.** `go test -race -cover ./internal/adapters/observer/` → **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/adapters/observer/core_adapter.go go/internal/adapters/observer/core_adapter_channel_test.go
git commit -m "feat(observer): spawn channel producer beside observer when EVOLVE_CHANNEL=1"
```

---

## Task 9: `channel.Supervisor` — Ask / Feed / timeout

**Files:**
- Create: `go/internal/bridge/channel/supervisor.go`
- Test: `go/internal/bridge/channel/supervisor_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestSupervisor_Ask_ReturnsAnswerSpan(t *testing.T) {
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	s := NewSupervisor(SupervisorConfig{Workspace: ws, Agent: "build", Transport: "claude-tmux",
		Now: now, NewID: func() string { return "c1" }, PollEvery: time.Millisecond, Timeout: time.Second})

	// Simulate the producer: write the feed the supervisor will read.
	go func() {
		time.Sleep(10 * time.Millisecond)
		appendFeed(t, ws, "build",
			`{"seq":5,"kind":"correlation","data":{"sub":"request","corr_id":"c1","at_seq":5}}`,
			`{"seq":6,"kind":"assistant_text","data":{"text":"done: 3 bullets"}}`,
			`{"seq":7,"kind":"correlation","data":{"sub":"response_complete","corr_id":"c1","start_seq":6,"end_seq":7}}`)
	}()

	ans, err := s.Ask(context.Background(), "summarize")
	if err != nil { t.Fatalf("Ask: %v", err) }
	if len(ans.Events) == 0 || !strings.Contains(ans.Text(), "3 bullets") {
		t.Fatalf("answer = %+v", ans)
	}
	// And it enqueued the ask with the corr id:
	got, _ := inbox.NewCursor(ws, "build").Drain()
	if len(got) != 1 || got[0].CorrID != "c1" || got[0].Kind != inbox.KindCommand {
		t.Fatalf("inbox = %+v", got)
	}
}

func TestSupervisor_Ask_HeadlessRefused(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{Workspace: t.TempDir(), Agent: "build", Transport: "claude-p"})
	if _, err := s.Ask(context.Background(), "x"); !errors.Is(err, ErrTransportNoInject) {
		t.Fatalf("err = %v, want ErrTransportNoInject", err)
	}
}

func TestSupervisor_Ask_Timeout(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{Workspace: t.TempDir(), Agent: "build", Transport: "claude-tmux",
		NewID: func() string { return "c1" }, PollEvery: time.Millisecond, Timeout: 20 * time.Millisecond})
	if _, err := s.Ask(context.Background(), "x"); !errors.Is(err, ErrResponseTimeout) {
		t.Fatalf("err = %v, want ErrResponseTimeout", err)
	}
}
```

(`appendFeed` is a 3-line test helper that `O_APPEND`s lines to `FeedPath`.)

- [ ] **Step 2: Run — expect FAIL** (`undefined: NewSupervisor`)

Run: `cd go && go test ./internal/bridge/channel/ -run TestSupervisor`

- [ ] **Step 3: Implement** (transport check: only `*-tmux` can receive; tail the feed by byte offset for the matching `response_complete`)

```go
package channel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

var (
	ErrTransportNoInject = errors.New("channel: transport cannot receive mid-task input (headless)")
	ErrResponseTimeout   = errors.New("channel: timed out waiting for the agent's reply")
)

type SupervisorConfig struct {
	Workspace string
	Agent     string
	Transport string // driver name, e.g. "claude-tmux"
	Timeout   time.Duration // default 120s
	PollEvery time.Duration // default 500ms
	Now       func() time.Time
	NewID     func() string // injectable for tests; prod uses a random id
}

type Answer struct{ Events []map[string]any }

func (a Answer) Text() string {
	var b strings.Builder
	for _, e := range a.Events {
		if d, ok := e["data"].(map[string]any); ok {
			if t, ok := d["text"].(string); ok { b.WriteString(t) }
		}
	}
	return b.String()
}

type Supervisor struct{ cfg SupervisorConfig }

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	if cfg.Timeout <= 0 { cfg.Timeout = 120 * time.Second }
	if cfg.PollEvery <= 0 { cfg.PollEvery = 500 * time.Millisecond }
	if cfg.Now == nil { cfg.Now = time.Now }
	return &Supervisor{cfg: cfg}
}

// Ask injects a correlated question and blocks until the agent's reply span is
// observed in the feed (or Timeout / ctx). Headless transports refuse.
func (s *Supervisor) Ask(ctx context.Context, question string) (Answer, error) {
	if !strings.HasSuffix(s.cfg.Transport, "-tmux") {
		return Answer{}, ErrTransportNoInject
	}
	id := s.cfg.NewID()
	env := inbox.Envelope{Kind: inbox.KindCommand, Body: question, CorrID: id, Source: "supervisor"}
	if err := inbox.Append(s.cfg.Workspace, s.cfg.Agent, env, s.cfg.Now); err != nil {
		return Answer{}, err
	}
	return s.awaitReply(ctx, id)
}

func (s *Supervisor) awaitReply(ctx context.Context, corrID string) (Answer, error) {
	deadline := s.cfg.Now().Add(s.cfg.Timeout)
	var off int64
	var span [2]int64
	var haveSpan bool
	var collected []map[string]any
	t := time.NewTicker(s.cfg.PollEvery)
	defer t.Stop()
	for {
		lines, n := tailLines(FeedPath(s.cfg.Workspace, s.cfg.Agent), off)
		off = n
		for _, ln := range lines {
			var e map[string]any
			if json.Unmarshal([]byte(ln), &e) != nil { continue }
			if e["kind"] == "correlation" {
				if d, ok := e["data"].(map[string]any); ok && d["corr_id"] == corrID && d["sub"] == "response_complete" {
					span = [2]int64{toI64(d["start_seq"]), toI64(d["end_seq"])}
					haveSpan = true
				}
			}
		}
		if haveSpan {
			// re-read the whole feed once, collecting content envelopes in span
			collected = collectSpan(FeedPath(s.cfg.Workspace, s.cfg.Agent), span)
			return Answer{Events: collected}, nil
		}
		if !s.cfg.Now().Before(deadline) {
			return Answer{}, ErrResponseTimeout
		}
		select {
		case <-ctx.Done():
			return Answer{}, ctx.Err()
		case <-t.C:
		}
	}
}
```

Add small helpers in the same file: `tailLines(path, off) ([]string, int64)` (mirror `phasestream.tailFile`), `collectSpan(path, [2]int64) []map[string]any` (read feed, keep non-correlation envelopes whose `seq` ∈ [start,end]), `toI64(any) int64`. `Feed()` (live channel tail) can be a thin wrapper added when the policy needs it (Task 10) — keep YAGNI here.

- [ ] **Step 4: Run — expect PASS.** `go test -race -cover ./internal/bridge/channel/` → **≥95%** (cover the timeout, headless, and span-collection branches).

- [ ] **Step 5: Commit**

```bash
git add go/internal/bridge/channel/supervisor.go go/internal/bridge/channel/supervisor_test.go
git commit -m "feat(channel): Supervisor.Ask with correlated reply + timeout/headless guards"
```

---

## Task 10: Minimal default policy (stall → ask summary)

**Files:**
- Create: `go/internal/bridge/channel/policy.go`
- Test: `go/internal/bridge/channel/policy_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestStallPolicy_AsksOnStall(t *testing.T) {
	var asked []string
	pol := StallPolicy{Question: "Summarize progress + blockers in 3 bullets."}
	act := pol.OnEvent(map[string]any{"kind": "stall"})
	if act == nil || act.Question != "Summarize progress + blockers in 3 bullets." {
		t.Fatalf("expected ask action, got %+v", act)
	}
	_ = asked
	if pol.OnEvent(map[string]any{"kind": "assistant_text"}) != nil {
		t.Fatalf("non-stall must not ask")
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: StallPolicy`)

Run: `cd go && go test ./internal/bridge/channel/ -run TestStallPolicy`

- [ ] **Step 3: Implement**

```go
// Policy decides whether to ask, from one feed envelope. The default
// StallPolicy asks for a progress summary when the producer emits a stall. A
// smarter (LLM-driven) policy implements the same interface later (ADR-0037
// non-goal for v1).
type Policy interface{ OnEvent(env map[string]any) *AskAction }

type AskAction struct{ Question string }

type StallPolicy struct{ Question string }

func (p StallPolicy) OnEvent(env map[string]any) *AskAction {
	if env["kind"] == "stall" {
		return &AskAction{Question: p.Question}
	}
	return nil
}
```

- [ ] **Step 4: Run — expect PASS.** `go test -cover ./internal/bridge/channel/` → **≥95%**.

- [ ] **Step 5: Commit**

```bash
git add go/internal/bridge/channel/policy.go go/internal/bridge/channel/policy_test.go
git commit -m "feat(channel): minimal stall→summary default policy"
```

---

## Task 11: `evolve bridge watch` (read-only human debug tail)

**Files:**
- Create: `go/cmd/evolve/cmd_bridge_watch.go`
- Modify: `go/cmd/evolve/cmd_bridge.go` (register `watch` in the subcommand switch — mirror how `send` is registered)
- Test: `go/cmd/evolve/cmd_bridge_watch_test.go`

- [ ] **Step 1: Write the failing test** (render a fixture feed to expected lines; inject a one-shot reader so the test doesn't block)

```go
func TestBridgeWatch_RendersFeed(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(channelFeedForTest(ws, "build"), []byte(
		`{"seq":1,"kind":"assistant_text","data":{"text":"hello"}}`+"\n"+
		`{"seq":2,"kind":"correlation","data":{"sub":"request","corr_id":"c1","at_seq":2}}`+"\n"), 0o644)
	var out bytes.Buffer
	err := runBridgeWatchOnce(&out, ws, "build") // one-shot: read existing, no follow
	if err != nil { t.Fatalf("watch: %v", err) }
	s := out.String()
	if !strings.Contains(s, "assistant_text") || !strings.Contains(s, "hello") || !strings.Contains(s, "request corr_id=c1") {
		t.Fatalf("render =\n%s", s)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: runBridgeWatchOnce`)

Run: `cd go && go test ./cmd/evolve/ -run TestBridgeWatch`

- [ ] **Step 3: Implement.** `runBridgeWatchOnce(w io.Writer, workspace, agent string) error` reads `channel.FeedPath`, parses each NDJSON line, and prints a compact human line (`seq=%d %s %s` with correlation envelopes rendered as `correlation: <sub> corr_id=<id>`). The exported `cmdBridgeWatch(args)` parses `--workspace`/`--agent`/`--follow`, calls the one-shot for the existing content, then (when `--follow`, the default for a human) tails for new lines with a 500ms poll until SIGINT. Read-only — it never writes the feed or the inbox.

```go
func runBridgeWatchOnce(w io.Writer, workspace, agent string) error {
	data, err := os.ReadFile(channel.FeedPath(workspace, agent))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) { return nil }
		return err
	}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if ln == "" { continue }
		var e map[string]any
		if json.Unmarshal([]byte(ln), &e) != nil { continue }
		fmt.Fprintln(w, renderFeedLine(e))
	}
	return nil
}
```

`renderFeedLine` is a pure function (its own unit test): correlation → `correlation: <sub> corr_id=<id>`; else `<kind> <short data>`.

- [ ] **Step 4: Run — expect PASS.** `go test -cover ./cmd/evolve/ -run TestBridgeWatch` → the new funcs **≥95%** (cover missing-file, malformed-line, and correlation-render branches).

- [ ] **Step 5: Commit**

```bash
git add go/cmd/evolve/cmd_bridge_watch.go go/cmd/evolve/cmd_bridge.go go/cmd/evolve/cmd_bridge_watch_test.go
git commit -m "feat(cli): evolve bridge watch — read-only live channel tail"
```

---

## Task 12: End-to-end integration test + coverage sweep

**Files:**
- Test: `go/internal/bridge/channel/e2e_test.go`

- [ ] **Step 1: Write the e2e test** — drive the real producer + supervisor against a hand-written stdout/stderr log pair simulating the driver:

```go
func TestChannel_EndToEnd(t *testing.T) {
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	// supervisor asks; we then simulate the driver applying + answering + idling
	// by appending to the raw logs the producer tails.
	prod := NewProducer(ProducerConfig{Workspace: ws, Agent: "build", Phase: "build", Cycle: 1, PollEvery: time.Millisecond, Now: now})
	ctx, cancel := context.WithCancel(context.Background())
	go prod.Run(ctx)
	defer cancel()

	sup := NewSupervisor(SupervisorConfig{Workspace: ws, Agent: "build", Transport: "claude-tmux",
		Now: now, NewID: func() string { return "c1" }, PollEvery: time.Millisecond, Timeout: 2 * time.Second})

	go func() {
		// driver-side simulation: deliver, answer, idle
		appendRaw(t, ws, "build-stderr.log", `{"evolve_channel":"inject_applied","corr_id":"c1"}`)
		appendRaw(t, ws, "build-stdout.log", `{"type":"assistant","message":{"content":[{"type":"text","text":"3 bullets: a,b,c"}]}}`)
		appendRaw(t, ws, "build-stderr.log", `{"evolve_channel":"idle_reached","corr_id":"c1"}`)
	}()

	ans, err := sup.Ask(context.Background(), "summarize")
	if err != nil { t.Fatalf("Ask: %v", err) }
	if !strings.Contains(ans.Text(), "3 bullets") {
		t.Fatalf("answer = %q", ans.Text())
	}
}
```

- [ ] **Step 2: Run — expect PASS** (`go test -race ./internal/bridge/channel/ -run TestChannel_EndToEnd`). Debug ordering with the injected clock + tiny `PollEvery` until green.

- [ ] **Step 3: Coverage sweep — the gate.** Run for every touched package and confirm each prints **≥95%**:

```bash
cd go && for p in ./internal/bridge/inbox/... ./internal/phasestream/... ./internal/bridge/channel/... ./internal/adapters/observer/... ; do
  go test -race -cover "$p" || exit 1
done
go test -race ./internal/bridge/... ./cmd/evolve/...
```

If any package is <95%, add the missing branch test before proceeding. Document the final per-package numbers in the PR body as `pkg — N/N PASS, coverage X% (≥95%)`.

- [ ] **Step 4: Commit**

```bash
git add go/internal/bridge/channel/e2e_test.go
git commit -m "test(channel): end-to-end ask→answer integration + coverage sweep"
```

---

## Task 13: Graduate the design to ADR-0037 + docs

**Files:**
- Create: `docs/architecture/adr/0037-bidirectional-channel.md` (ACCEPTED; link the spec, the env vars, the file map)
- Modify: `CLAUDE.md` current-behavior table — add `EVOLVE_CHANNEL` (default `0`, opt-in) and `EVOLVE_CHANNEL_SUPERVISOR` rows

- [ ] **Step 1:** Write ADR-0037 summarizing the shipped design (reuse the spec's decisions table + file map; status ACCEPTED with commit range).
- [ ] **Step 2:** Add the two env-var rows to the CLAUDE.md table (match the existing row format).
- [ ] **Step 3: Commit**

```bash
git add docs/architecture/adr/0037-bidirectional-channel.md CLAUDE.md
git commit -m "docs(adr): ADR-0037 bidirectional channel (shipped)"
```

---

## Self-Review (run before execution)

- **Spec coverage:** every spec section maps to a task — feed file (T6), live producer (T7), spawn gating (T8), `CorrID` (T1), correlation kind/minting/wiring (T2–T4), driver breadcrumbs (T5), supervisor Ask/timeout/headless (T9), default policy (T10), `bridge watch` (T11), edge cases (timeout/headless/crash/race covered in T7/T9/T12), ADR-0036 liveness-floor untouched (observer code unchanged except an additive spawn; assert via T8's channel-off byte-identical test), ≥95% gate (every task + T12 sweep).
- **Type consistency:** `FeedPath`, `NewProducer/ProducerConfig`, `NewSupervisor/SupervisorConfig`, `Answer.Text()`, `ErrTransportNoInject`, `ErrResponseTimeout`, `Policy/AskAction/StallPolicy`, `emitChannelBreadcrumb`, `KindCorrelation` — names are used identically across tasks. `inbox.Envelope.CorrID` and the breadcrumb JSON (`evolve_channel`,`corr_id`) match between T1/T5 (driver) and T3 (parser).
- **No placeholders:** every code step shows real code against verified signatures (`inbox.Append`, `inbox.Cursor`, `phasestream.NormalizerConfig`, `CoreAdapter.Start`).
- **Open verification points (engineer confirms against compiler, not guesses):** the classifier's seq field name in T4 (`c.seq` vs actual) and the exact idle-transition hook in T5's poll loop — both are reading-not-inventing checks per Rule 8; the plan flags them explicitly rather than hiding them.

## Non-goals (unchanged from spec)

No headless duplex, no smart/LLM supervisor brain, no streaming server, no change to post-phase `events.ndjson` or the observer liveness floor.

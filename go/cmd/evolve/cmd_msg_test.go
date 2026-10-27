package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
)

const (
	msgTestCycle    = 7
	msgTestDispatch = "d-build"
	msgTestToken    = "tok-secret"
	msgTestID       = "01JC0000000000000000000001"
	msgTestOtherID  = "01JC0000000000000000000002"
	msgTestUnknown  = "01JC0000000000000000000009"
)

var msgTestNow = time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

type msgFakeWaker struct {
	armed  []wake.Targets
	waits  []time.Time
	result wake.Wake
	err    error
	onWait func()
	closed bool
}

func (w *msgFakeWaker) Arm(t wake.Targets) error {
	w.armed = append(w.armed, t)
	return nil
}

func (w *msgFakeWaker) Wait(_ context.Context, deadline time.Time) (wake.Wake, error) {
	w.waits = append(w.waits, deadline)
	if w.onWait != nil {
		w.onWait()
	}
	return w.result, w.err
}

func (w *msgFakeWaker) Close() error {
	w.closed = true
	return nil
}

type msgEnteredWaker struct {
	*wake.Waiter
	entered chan struct{}
	once    sync.Once
}

func (w *msgEnteredWaker) Wait(ctx context.Context, deadline time.Time) (wake.Wake, error) {
	w.once.Do(func() { close(w.entered) })
	return w.Waiter.Wait(ctx, deadline)
}

type msgFakeSender struct {
	mu      sync.Mutex
	sent    []envelope.Envelope
	creds   []identity.Credential
	err     error
	cursor  int64
	refused []broker.Refusal
}

func (s *msgFakeSender) Send(_ context.Context, c identity.Credential, e envelope.Envelope) (broker.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, e)
	s.creds = append(s.creds, c)
	if s.err != nil {
		return broker.Receipt{}, s.err
	}
	return broker.Receipt{ID: e.ID, Cursor: s.cursor, Refused: s.refused}, nil
}

func (s *msgFakeSender) Ack(context.Context, identity.Credential, int64) error { return nil }

func (s *msgFakeSender) envelopes() []envelope.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.sent)
}

type msgSilentNotifier struct{}

func (msgSilentNotifier) Notify(context.Context, broker.Event) {}

type msgBrokenWriter struct{ err error }

func (w msgBrokenWriter) Write([]byte) (int, error) { return 0, w.err }

type msgScenario struct {
	args        []string
	stdin       string
	sender      *msgFakeSender
	mailbox     []envelope.Envelope
	waker       *msgFakeWaker
	callerErr   error
	wakerErr    error
	rootIsFile  bool
	stdout      io.Writer
	receiptWait time.Duration
}

type msgRun struct {
	code   int
	out    string
	errOut string
	layout outbox.Layout
}

func msgLayout(t *testing.T, sc msgScenario) outbox.Layout {
	t.Helper()
	root := t.TempDir()
	if sc.rootIsFile {
		root = filepath.Join(root, "file")
		if err := os.WriteFile(root, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return outbox.Layout{Root: root, Cycle: msgTestCycle}
}

func msgWriteMailbox(t *testing.T, layout outbox.Layout, messages []envelope.Envelope) {
	t.Helper()
	var buf bytes.Buffer
	for _, m := range messages {
		line, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	box := layout.BoxDir(msgTestDispatch)
	if err := os.MkdirAll(box, 0o700); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(box, "messages.tmp")
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(box, "messages.ndjson")); err != nil {
		t.Fatal(err)
	}
}

func msgStartService(t *testing.T, layout outbox.Layout, sender outbox.Sender) {
	t.Helper()
	waker, err := wake.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	watcher := outbox.New(outbox.Config{Layout: layout, Sender: sender, Waker: waker, Notifier: msgSilentNotifier{}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- watcher.Run(ctx, msgTestDispatch) }()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = waker.Close()
	})
}

func msgEnvFor(layout outbox.Layout, sc msgScenario) msgEnv {
	env := msgEnv{
		caller: func() (msgCaller, error) {
			if sc.callerErr != nil {
				return msgCaller{}, sc.callerErr
			}
			return msgCaller{Layout: layout, Dispatch: msgTestDispatch, Token: msgTestToken}, nil
		},
		now:         time.Now,
		entropy:     rand.Reader,
		receiptWait: 30 * time.Second,
	}
	if sc.receiptWait > 0 {
		env.receiptWait = sc.receiptWait
	}
	if sc.waker != nil {
		env.now = func() time.Time { return msgTestNow }
	}
	env.waker = func() (msgWaker, error) {
		if sc.wakerErr != nil {
			return nil, sc.wakerErr
		}
		if sc.waker != nil {
			return sc.waker, nil
		}
		return wake.New(nil)
	}
	return env
}

func runMsgScenario(t *testing.T, sc msgScenario) msgRun {
	t.Helper()
	layout := msgLayout(t, sc)
	if len(sc.mailbox) > 0 {
		msgWriteMailbox(t, layout, sc.mailbox)
	}
	if sc.sender != nil {
		msgStartService(t, layout, sc.sender)
	}
	var out, errOut bytes.Buffer
	var stdout io.Writer = &out
	if sc.stdout != nil {
		stdout = sc.stdout
	}
	code := runMsgWith(msgEnvFor(layout, sc), sc.args, strings.NewReader(sc.stdin), stdout, &errOut)
	return msgRun{code: code, out: out.String(), errOut: errOut.String(), layout: layout}
}

func msgMessage(id, correlation string) envelope.Envelope {
	return envelope.Envelope{
		Schema:        envelope.SchemaVersion,
		ID:            id,
		TS:            msgTestNow,
		Kind:          envelope.KindRequest,
		From:          identity.Principal{Cycle: msgTestCycle, Dispatch: "d-scout", Phase: "scout", Role: "scout"},
		To:            []address.Address{{Selector: address.SelectDispatch, Value: msgTestDispatch}},
		Subject:       "need a plan",
		Body:          "please plan",
		CorrelationID: correlation,
		ContextID:     msgTestCycle,
	}
}

func msgAddresses(e envelope.Envelope) []string {
	out := make([]string, 0, len(e.To))
	for _, a := range e.To {
		out = append(out, a.String())
	}
	return out
}

func msgOutDirEntries(t *testing.T, layout outbox.Layout) []string {
	t.Helper()
	entries, err := os.ReadDir(layout.OutDir(msgTestDispatch))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestMsgCLI_ExitCodesMatchTheTable(t *testing.T) {
	t.Parallel()
	sendArgs := []string{"send", "--to", "role:build", "--kind", "note", "--id", msgTestID}
	waitDeadline := &msgFakeWaker{result: wake.Wake{Deadline: true}}
	hangup := &msgFakeWaker{result: wake.Wake{Hangup: true}}
	waitFails := &msgFakeWaker{err: errors.New("kqueue gone")}
	noReceipt := &msgFakeWaker{result: wake.Wake{Deadline: true}}
	rows := []struct {
		name string
		want int
		sc   msgScenario
	}{
		{"send accepted", 0, msgScenario{args: sendArgs, stdin: "hi", sender: &msgFakeSender{}}},
		{"send accepted with a partial refusal", 0, msgScenario{args: sendArgs, stdin: "hi", sender: &msgFakeSender{refused: []broker.Refusal{{Rule: "queue_cap", Reason: "full"}}}}},
		{"wait returns the queued messages", 0, msgScenario{args: []string{"wait"}, mailbox: []envelope.Envelope{msgMessage(msgTestID, "")}, waker: &msgFakeWaker{}}},
		{"reply accepted", 0, msgScenario{args: []string{"reply", msgTestID}, stdin: "ok", mailbox: []envelope.Envelope{msgMessage(msgTestID, "")}, sender: &msgFakeSender{}}},
		{"service denies", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrDenied}}},
		{"service finds it unroutable", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrUnroutable}}},
		{"service finds an id in use", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrIDInUse}}},
		{"service finds an id conflict", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrIDConflict}}},
		{"service finds an invalid envelope", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: fmt.Errorf("%w: body too long", envelope.ErrInvalid)}}},
		{"service finds the caller unauthenticated", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: identity.ErrUnauthenticated}}},
		{"service rate limits", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrRateLimited}}},
		{"service hits the hop limit", 1, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrHopLimit}}},
		{"caller has no identity", 1, msgScenario{args: sendArgs, callerErr: errors.New("EVOLVE_MSG_TOKEN is not set")}},
		{"no receipt by the deadline", 2, msgScenario{args: sendArgs, waker: noReceipt}},
		{"the outbox cannot be created", 2, msgScenario{args: sendArgs, rootIsFile: true}},
		{"the waker cannot start", 2, msgScenario{args: sendArgs, wakerErr: errors.New("no kqueue")}},
		{"wait fails in the kernel", 2, msgScenario{args: []string{"wait"}, waker: waitFails}},
		{"wait cannot start the waker", 2, msgScenario{args: []string{"wait"}, wakerErr: errors.New("no kqueue")}},
		{"the reader of stdout is gone on a wait", 3, msgScenario{args: []string{"wait"}, waker: hangup}},
		{"stdout raises EPIPE on a wait", 3, msgScenario{args: []string{"wait", "--json"}, mailbox: []envelope.Envelope{msgMessage(msgTestID, "")}, waker: &msgFakeWaker{}, stdout: msgBrokenWriter{err: syscall.EPIPE}}},
		{"stdout raises EPIPE on a send", 3, msgScenario{args: append([]string{"send", "--json"}, sendArgs[1:]...), sender: &msgFakeSender{}, stdout: msgBrokenWriter{err: syscall.EPIPE}}},
		{"recipient not found", 4, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrNotFound}}},
		{"recipient unknown", 4, msgScenario{args: sendArgs, sender: &msgFakeSender{err: broker.ErrUnknownRecipient}}},
		{"reply to an id outside the mailbox", 4, msgScenario{args: []string{"reply", msgTestUnknown}, stdin: "x", mailbox: []envelope.Envelope{msgMessage(msgTestID, "")}}},
		{"wait deadline passes", 5, msgScenario{args: []string{"wait", "--deadline", "1s"}, waker: waitDeadline}},
		{"no arguments", 10, msgScenario{}},
		{"unknown sub-verb", 10, msgScenario{args: []string{"shout"}}},
		{"send without recipients", 10, msgScenario{args: []string{"send", "--kind", "note"}}},
		{"send without a kind", 10, msgScenario{args: []string{"send", "--to", "role:build"}}},
		{"send to a malformed address", 10, msgScenario{args: []string{"send", "--to", "bogus", "--kind", "note"}}},
		{"send with a malformed id", 10, msgScenario{args: []string{"send", "--to", "role:build", "--kind", "note", "--id", "not-a-ulid"}}},
		{"send with a malformed expiry", 10, msgScenario{args: []string{"send", "--to", "role:build", "--kind", "note", "--expires", "tomorrow"}}},
		{"send tries to set the sender", 10, msgScenario{args: []string{"send", "--to", "role:build", "--kind", "note", "--from", "role:audit"}}},
		{"send with a stray argument", 10, msgScenario{args: append(slices.Clone(sendArgs), "extra")}},
		{"wait with a malformed deadline", 10, msgScenario{args: []string{"wait", "--deadline", "soon"}}},
		{"wait with a stray argument", 10, msgScenario{args: []string{"wait", "extra"}}},
		{"reply without an id", 10, msgScenario{args: []string{"reply"}}},
		{"reply with a kind outside reply and evidence", 10, msgScenario{args: []string{"reply", msgTestID, "--kind", "note"}}},
		{"reply with a malformed id", 10, msgScenario{args: []string{"reply", "not-a-ulid"}}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got := runMsgScenario(t, row.sc)
			if got.code != row.want {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", got.code, row.want, got.out, got.errOut)
			}
			if row.want != 0 && strings.TrimSpace(got.errOut) == "" && row.want != 3 {
				t.Errorf("a failure must explain itself on stderr")
			}
			if row.want == 10 && !strings.Contains(got.errOut, "usage: evolve msg") {
				t.Errorf("a usage error must print the usage, got %q", got.errOut)
			}
		})
	}
}

func TestMsgCLI_IsRegisteredInTheCommandTable(t *testing.T) {
	t.Parallel()
	var found *subcommand
	for i := range commands {
		if commands[i].Name == "msg" {
			found = &commands[i]
		}
	}
	if found == nil {
		t.Fatal("the command table has no msg verb")
	}
	var out, errOut bytes.Buffer
	if code := found.Run(nil, strings.NewReader(""), &out, &errOut); code != exitUsage {
		t.Fatalf("msg with no sub-verb exits %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "usage: evolve msg") {
		t.Errorf("usage missing, got %q", errOut.String())
	}
}

func TestMsgSend_ReadsTheBodyFromStdin(t *testing.T) {
	t.Parallel()
	body := "line one\n<tag> & \"quoted\" line two\n"
	sender := &msgFakeSender{cursor: 42}
	got := runMsgScenario(t, msgScenario{
		sender: sender,
		stdin:  body,
		args: []string{
			"send", "--to", "role:build,phase:audit", "--kind", "handoff", "--subject", "the plan",
			"--ref", "docs/a.md", "--ref", "go/b.go", "--reply-to", msgTestOtherID, "--correlation", "corr-9",
			"--expires", "2026-11-01T00:00:00Z", "--id", msgTestID, "--json",
		},
	})
	if got.code != 0 {
		t.Fatalf("exit %d: %s", got.code, got.errOut)
	}
	sent := sender.envelopes()
	if len(sent) != 1 {
		t.Fatalf("the service ingested %d drafts, want 1", len(sent))
	}
	e := sent[0]
	if e.Body != body {
		t.Errorf("body %q, want the stdin %q", e.Body, body)
	}
	if e.Kind != envelope.KindHandoff || e.Subject != "the plan" || e.ID != msgTestID {
		t.Errorf("kind/subject/id = %q/%q/%q", e.Kind, e.Subject, e.ID)
	}
	if !slices.Equal(e.Refs, []string{"docs/a.md", "go/b.go"}) {
		t.Errorf("refs %v", e.Refs)
	}
	if e.ReplyTo != msgTestOtherID || e.CorrelationID != "corr-9" {
		t.Errorf("reply_to/correlation = %q/%q", e.ReplyTo, e.CorrelationID)
	}
	if !slices.Equal(msgAddresses(e), []string{"role:build", "phase:audit"}) {
		t.Errorf("to %v", msgAddresses(e))
	}
	wantExpiry := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if e.ExpiresAt == nil || !e.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expires_at %v, want %v", e.ExpiresAt, wantExpiry)
	}
	if e.From != (identity.Principal{}) {
		t.Errorf("the sender is stamped by the service, the CLI sent %+v", e.From)
	}
	cred := sender.creds[0]
	if cred.Token != msgTestToken || cred.Location != msgTestDispatch {
		t.Errorf("credential %+v, want the token and dispatch of the caller", cred)
	}
	var receipt broker.Receipt
	if err := json.Unmarshal([]byte(got.out), &receipt); err != nil {
		t.Fatalf("--json output is not a receipt: %v: %q", err, got.out)
	}
	if receipt.ID != msgTestID || receipt.Cursor != 42 {
		t.Errorf("receipt %+v, want id %s cursor 42", receipt, msgTestID)
	}
}

func TestMsgSend_MintsAULIDWhenNoIDIsGiven(t *testing.T) {
	t.Parallel()
	sender := &msgFakeSender{}
	got := runMsgScenario(t, msgScenario{sender: sender, stdin: "x", args: []string{"send", "--to", "role:build", "--kind", "note"}})
	if got.code != 0 {
		t.Fatalf("exit %d: %s", got.code, got.errOut)
	}
	sent := sender.envelopes()
	if len(sent) != 1 || !envelope.ValidID(sent[0].ID) {
		t.Fatalf("sent %+v, want one envelope with a ULID", sent)
	}
	if !strings.Contains(got.out, sent[0].ID) {
		t.Errorf("text output %q does not name the id %s", got.out, sent[0].ID)
	}
}

func TestMsgSend_NoReceiptByTheDeadlineExitsTwo(t *testing.T) {
	t.Parallel()
	waker := &msgFakeWaker{result: wake.Wake{Deadline: true}}
	body := "needs an answer"
	got := runMsgScenario(t, msgScenario{
		waker: waker, stdin: body, receiptWait: 45 * time.Second,
		args: []string{"send", "--to", "role:build", "--kind", "request", "--id", msgTestID},
	})
	if got.code != 2 {
		t.Fatalf("exit %d, want 2: %s", got.code, got.errOut)
	}
	if len(waker.waits) != 1 || !waker.waits[0].Equal(msgTestNow.Add(45*time.Second)) {
		t.Errorf("wait deadlines %v, want one at now+45s", waker.waits)
	}
	receipts := filepath.Join(got.layout.BoxDir(msgTestDispatch), "receipts")
	if len(waker.armed) == 0 || !slices.Contains(waker.armed[0].Dirs, receipts) {
		t.Errorf("armed %+v, want the receipts dir %s", waker.armed, receipts)
	}
	if !waker.closed {
		t.Errorf("the waker must be closed")
	}
	names := msgOutDirEntries(t, got.layout)
	if !slices.Equal(names, []string{msgTestID + ".json"}) {
		t.Fatalf("outbox holds %v, want only the draft %s.json", names, msgTestID)
	}
	raw, err := os.ReadFile(filepath.Join(got.layout.OutDir(msgTestDispatch), names[0]))
	if err != nil {
		t.Fatal(err)
	}
	var draft struct {
		Type     string            `json:"type"`
		Token    string            `json:"token"`
		Envelope envelope.Envelope `json:"envelope"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&draft); err != nil {
		t.Fatalf("draft is not the outbox draft schema: %v: %s", err, raw)
	}
	if draft.Type != "send" || draft.Token != msgTestToken || draft.Envelope.Body != body {
		t.Errorf("draft %+v", draft)
	}
}

func TestMsgSend_WaitsForTheReceiptOfTheServiceNotJustAnyFile(t *testing.T) {
	t.Parallel()
	waker := &msgFakeWaker{result: wake.Wake{Changed: true}}
	calls := 0
	waker.onWait = func() {
		calls++
		if calls == 2 {
			waker.result = wake.Wake{Deadline: true}
		}
	}
	got := runMsgScenario(t, msgScenario{
		waker: waker, stdin: "x",
		args: []string{"send", "--to", "role:build", "--kind", "note", "--id", msgTestID},
	})
	if got.code != 2 {
		t.Fatalf("exit %d, want 2 after a wake with no receipt then a deadline", got.code)
	}
	if len(waker.waits) != 2 || !waker.waits[0].Equal(waker.waits[1]) {
		t.Errorf("wait deadlines %v: the deadline is one-shot and must not move", waker.waits)
	}
}

func TestMsgWait_WakesOnTheMailboxWithoutAPoll(t *testing.T) {
	t.Parallel()
	t.Run("one arm and one wait when the mailbox is empty", func(t *testing.T) {
		var layout outbox.Layout
		waker := &msgFakeWaker{result: wake.Wake{Changed: true}}
		waker.onWait = func() { msgWriteMailbox(t, layout, []envelope.Envelope{msgMessage(msgTestID, "corr-1")}) }
		sc := msgScenario{waker: waker, args: []string{"wait", "--json"}}
		layout = msgLayout(t, sc)
		var out, errOut bytes.Buffer
		code := runMsgWith(msgEnvFor(layout, sc), sc.args, strings.NewReader(""), &out, &errOut)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errOut.String())
		}
		if len(waker.armed) != 1 || !slices.Contains(waker.armed[0].Dirs, layout.BoxDir(msgTestDispatch)) {
			t.Errorf("armed %+v, want the mailbox dir %s", waker.armed, layout.BoxDir(msgTestDispatch))
		}
		if len(waker.waits) != 1 || !waker.waits[0].IsZero() {
			t.Errorf("waits %v, want exactly one with no deadline", waker.waits)
		}
		var e envelope.Envelope
		if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &e); err != nil || e.ID != msgTestID {
			t.Errorf("--json prints one envelope per line, got %q (%v)", out.String(), err)
		}
	})
	t.Run("a message already queued returns without any wait", func(t *testing.T) {
		waker := &msgFakeWaker{}
		got := runMsgScenario(t, msgScenario{waker: waker, args: []string{"wait"}, mailbox: []envelope.Envelope{msgMessage(msgTestID, "")}})
		if got.code != 0 || len(waker.waits) != 0 {
			t.Fatalf("exit %d with %d waits, want 0 with none", got.code, len(waker.waits))
		}
		if !strings.Contains(got.out, "need a plan") || !strings.Contains(got.out, "please plan") {
			t.Errorf("text output %q lacks the subject and the body", got.out)
		}
	})
	t.Run("the deadline flag becomes an absolute deadline", func(t *testing.T) {
		waker := &msgFakeWaker{result: wake.Wake{Deadline: true}}
		got := runMsgScenario(t, msgScenario{waker: waker, args: []string{"wait", "--deadline", "90s"}})
		if got.code != 5 {
			t.Fatalf("exit %d, want 5", got.code)
		}
		if len(waker.waits) != 1 || !waker.waits[0].Equal(msgTestNow.Add(90*time.Second)) {
			t.Errorf("waits %v, want now+90s", waker.waits)
		}
		if got.out != "" {
			t.Errorf("a deadline prints nothing on stdout, got %q", got.out)
		}
	})
	t.Run("the kernel wakes a blocked wait on a new mailbox file", func(t *testing.T) {
		real, err := wake.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		entered := &msgEnteredWaker{Waiter: real, entered: make(chan struct{})}
		layout := outbox.Layout{Root: t.TempDir(), Cycle: msgTestCycle}
		env := msgEnvFor(layout, msgScenario{})
		env.waker = func() (msgWaker, error) { return entered, nil }
		var out, errOut bytes.Buffer
		done := make(chan int, 1)
		go func() { done <- runMsgWith(env, []string{"wait", "--json"}, strings.NewReader(""), &out, &errOut) }()
		<-entered.entered
		msgWriteMailbox(t, layout, []envelope.Envelope{msgMessage(msgTestID, "")})
		select {
		case code := <-done:
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errOut.String())
			}
		case <-time.After(20 * time.Second):
			t.Fatal("wait did not wake on the mailbox")
		}
		if !strings.Contains(out.String(), msgTestID) {
			t.Errorf("output %q lacks the message", out.String())
		}
	})
}

func TestMsgWait_SourceHasNoPollTimer(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("cmd_msg*.go")
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"Sleep": true, "Tick": true, "After": true, "NewTicker": true, "NewTimer": true, "AfterFunc": true}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		checked++
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, isIdent := sel.X.(*ast.Ident); isIdent && pkg.Name == "time" && banned[sel.Sel.Name] {
				t.Errorf("%s uses time.%s: a wait wakes on the kernel, never on a timer", file, sel.Sel.Name)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no cmd_msg*.go source file exists to check")
	}
}

func TestMsgReply_OutOfScopeExitsFour(t *testing.T) {
	t.Parallel()
	t.Run("an id outside the mailbox never reaches the outbox", func(t *testing.T) {
		got := runMsgScenario(t, msgScenario{
			args: []string{"reply", msgTestUnknown}, stdin: "answer",
			mailbox: []envelope.Envelope{msgMessage(msgTestID, "")},
		})
		if got.code != 4 {
			t.Fatalf("exit %d, want 4: %s", got.code, got.errOut)
		}
		if names := msgOutDirEntries(t, got.layout); len(names) != 0 {
			t.Errorf("outbox holds %v, want no draft for an unknown target", names)
		}
	})
	t.Run("an empty mailbox is also out of scope", func(t *testing.T) {
		got := runMsgScenario(t, msgScenario{args: []string{"reply", msgTestID}, stdin: "answer"})
		if got.code != 4 {
			t.Fatalf("exit %d, want 4: %s", got.code, got.errOut)
		}
	})
	t.Run("the service saying not found maps to four", func(t *testing.T) {
		got := runMsgScenario(t, msgScenario{
			args: []string{"reply", msgTestID}, stdin: "answer",
			mailbox: []envelope.Envelope{msgMessage(msgTestID, "")},
			sender:  &msgFakeSender{err: broker.ErrNotFound},
		})
		if got.code != 4 {
			t.Fatalf("exit %d, want 4: %s", got.code, got.errOut)
		}
	})
}

func TestMsgReply_AnswersTheSenderWithTheCorrelationID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		args            []string
		correlation     string
		wantKind        envelope.Kind
		wantCorrelation string
	}{
		{"default kind keeps the correlation", []string{"reply", msgTestID}, "corr-1", envelope.KindReply, "corr-1"},
		{"evidence kind", []string{"reply", msgTestID, "--kind", "evidence"}, "corr-1", envelope.KindEvidence, "corr-1"},
		{"no correlation falls back to the request id", []string{"reply", msgTestID}, "", envelope.KindReply, msgTestID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sender := &msgFakeSender{}
			got := runMsgScenario(t, msgScenario{
				args: tc.args, stdin: "the answer\n", sender: sender,
				mailbox: []envelope.Envelope{msgMessage(msgTestOtherID, "other"), msgMessage(msgTestID, tc.correlation)},
			})
			if got.code != 0 {
				t.Fatalf("exit %d: %s", got.code, got.errOut)
			}
			sent := sender.envelopes()
			if len(sent) != 1 {
				t.Fatalf("the service ingested %d drafts, want 1", len(sent))
			}
			e := sent[0]
			if e.Kind != tc.wantKind || e.ReplyTo != msgTestID || e.CorrelationID != tc.wantCorrelation || e.Body != "the answer\n" {
				t.Errorf("kind/reply_to/correlation/body = %q/%q/%q/%q", e.Kind, e.ReplyTo, e.CorrelationID, e.Body)
			}
			if !slices.Equal(msgAddresses(e), []string{"dispatch:d-scout"}) {
				t.Errorf("to %v, want the dispatch that wrote the request", msgAddresses(e))
			}
			if !envelope.ValidID(e.ID) || e.ID == msgTestID {
				t.Errorf("a reply carries its own fresh ULID, got %q", e.ID)
			}
		})
	}
}

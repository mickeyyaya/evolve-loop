package outbox_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/memstore"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
)

const cycle = 1850

var (
	key   = []byte("0123456789abcdef0123456789abcdef")
	start = time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	scout = identity.Principal{Cycle: cycle, Dispatch: "r/1850/scout/p11n1", Agent: "scout", Phase: "scout", Role: "scout"}
	build = identity.Principal{Cycle: cycle, Dispatch: "r/1850/build/p13n1", Agent: "builder", Phase: "build", Role: "build"}
)

type roster []broker.Member

func (r roster) Members(context.Context, int) ([]broker.Member, error) { return r, nil }

type recorder struct {
	events []broker.Event
	onSent func()
}

func (r *recorder) Notify(_ context.Context, e broker.Event) {
	r.events = append(r.events, e)
	if e.Kind == broker.EventSent && r.onSent != nil {
		r.onSent()
	}
}

type fixture struct {
	t      *testing.T
	root   string
	auth   *identity.Authority
	store  *memstore.Store
	svc    *broker.Service
	events *recorder
	ids    envelope.IDSource
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := func() time.Time { return start }
	auth, err := identity.NewAuthority(key, clock, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := policy.Compile(policy.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: t.TempDir(), auth: auth, store: memstore.New(), events: &recorder{},
		ids: envelope.IDSource{Now: clock, Entropy: rand.Reader}}
	svc, err := broker.New(broker.Config{
		Cycle: cycle, Auth: auth, Policy: engine, Store: f.store,
		Roster:   roster{{Principal: scout, State: broker.MemberLive}, {Principal: build, State: broker.MemberLive}},
		Notifier: f.events, IDs: f.ids, Now: clock,
		Envelope: envelope.Limits{BodyBytes: 8192, SubjectBytes: 256, Kinds: envelope.DefaultKinds()},
		Limits:   broker.Limits{SenderPerMinute: 30, RepeatWindow: time.Minute, RecipientQueue: 50, MaxHops: 4, CycleBudget: 200},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return f
}

func (f *fixture) layout() outbox.Layout { return outbox.Layout{Root: f.root, Cycle: cycle} }

func (f *fixture) watcher(w outbox.Waker) *outbox.Watcher {
	return outbox.New(outbox.Config{Layout: f.layout(), Sender: f.svc, Waker: w, Notifier: f.events})
}

func (f *fixture) token(p identity.Principal) string {
	f.t.Helper()
	tok, err := f.auth.Mint(p, start.Add(time.Hour))
	if err != nil {
		f.t.Fatal(err)
	}
	return tok
}

func (f *fixture) newID() string {
	f.t.Helper()
	id, err := f.ids.New()
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) sendDraft(p identity.Principal, token string) (id string, path string) {
	f.t.Helper()
	addr, err := address.Parse("phase:build")
	if err != nil {
		f.t.Fatal(err)
	}
	id = f.newID()
	draft := map[string]any{
		"type":  "send",
		"token": token,
		"envelope": envelope.Envelope{
			Kind: envelope.KindHandoff, To: []address.Address{addr}, Subject: "s", Body: "b",
		},
	}
	path = filepath.Join(f.layout().OutDir(p.Dispatch), id+".json")
	if err := atomicwrite.JSON(path, draft); err != nil {
		f.t.Fatal(err)
	}
	return id, path
}

func (f *fixture) receipt(p identity.Principal, id string) outbox.Answer {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.layout().BoxDir(p.Dispatch), "receipts", id+".json"))
	if err != nil {
		f.t.Fatalf("receipt of %s: %v", id, err)
	}
	var a outbox.Answer
	if err := json.Unmarshal(data, &a); err != nil {
		f.t.Fatal(err)
	}
	return a
}

type scripted struct {
	armErr  error
	waits   []func() (wake.Wake, error)
	armed   []wake.Targets
	onArmed func()
}

func (s *scripted) Arm(t wake.Targets) error {
	s.armed = append(s.armed, t)
	if s.onArmed != nil {
		s.onArmed()
	}
	return s.armErr
}

func (s *scripted) Wait(context.Context, time.Time) (wake.Wake, error) {
	next := s.waits[0]
	s.waits = s.waits[1:]
	return next()
}

func ctxStop(cancel context.CancelFunc) func() (wake.Wake, error) {
	return func() (wake.Wake, error) { cancel(); return wake.Wake{}, context.Canceled }
}

type waitSignal struct {
	outbox.Waker
	waiting chan struct{}
}

func (w *waitSignal) Wait(ctx context.Context, deadline time.Time) (wake.Wake, error) {
	w.waiting <- struct{}{}
	return w.Waker.Wait(ctx, deadline)
}

func TestOutbox_AKernelWakeIngestsADraft(t *testing.T) {
	f := newFixture(t)
	kernel, err := wake.New(nil)
	if err != nil {
		t.Skipf("no kernel wake on this host: %v", err)
	}
	defer kernel.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	waiting := make(chan struct{}, 8)
	f.events.onSent = cancel
	done := make(chan error, 1)
	go func() { done <- f.watcher(&waitSignal{Waker: kernel, waiting: waiting}).Run(ctx, scout.Dispatch) }()
	<-waiting

	id, path := f.sendDraft(scout, f.token(scout))

	if err := <-done; err != nil {
		t.Errorf("Run = %v, want nil after the stop", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("draft stat = %v, want it deleted after the ingest", err)
	}
	if got := f.receipt(scout, id); got.Receipt == nil || got.Receipt.ID != id {
		t.Errorf("answer = %+v, want a receipt for %s", got, id)
	}
}

func TestOutbox_ThePathNamesTheDispatch(t *testing.T) {
	f := newFixture(t)
	id, _ := f.sendDraft(scout, f.token(build))
	stop := &scripted{}
	ctx, cancel := context.WithCancel(context.Background())
	stop.waits = []func() (wake.Wake, error){ctxStop(cancel)}

	if err := f.watcher(stop).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	got := f.receipt(scout, id)
	if got.Receipt != nil || got.Error == "" {
		t.Errorf("answer = %+v, want the build token refused in the outbox of scout", got)
	}
	if len(f.events.events) == 0 || f.events.events[0].Kind != broker.EventDenied {
		t.Errorf("events = %+v, want msg.denied", f.events.events)
	}
	if b, _ := f.store.Read(context.Background(), cycle, 0); len(b.Records) != 0 {
		t.Errorf("store holds %d records, want none", len(b.Records))
	}
}

func TestLayout_LocateAcceptsOnlyTheExactForm(t *testing.T) {
	f := newFixture(t)
	l := f.layout()
	id := f.newID()
	good := filepath.Join(l.OutDir(scout.Dispatch), id+".json")
	mustWrite := func(p string) string {
		t.Helper()
		if err := atomicwrite.Bytes(p, []byte("{}")); err != nil {
			t.Fatal(err)
		}
		return p
	}
	written := mustWrite(good)
	cycleDir := filepath.Dir(filepath.Dir(l.OutDir(scout.Dispatch)))
	otherCycle := outbox.Layout{Root: f.root, Cycle: 1851}
	nonCanon := filepath.Join(cycleDir, "c2NvdXQ=", "out", id+".json")
	altKey := base64.RawURLEncoding.EncodeToString([]byte("x"))
	link := filepath.Join(f.root, "link.json")
	if err := os.Symlink(written, link); err != nil {
		t.Fatal(err)
	}
	bare := outbox.Layout{Root: t.TempDir(), Cycle: cycle}
	cases := map[string]string{
		"no cycle dir":     written,
		"missing file":     filepath.Join(l.OutDir(scout.Dispatch), f.newID()+".json"),
		"other cycle":      mustWrite(filepath.Join(otherCycle.OutDir(scout.Dispatch), id+".json")),
		"not a ulid":       mustWrite(filepath.Join(l.OutDir(scout.Dispatch), "draft.json")),
		"wrong subdir":     mustWrite(filepath.Join(l.BoxDir(scout.Dispatch), id+".json")),
		"too deep":         mustWrite(filepath.Join(l.OutDir(scout.Dispatch), "x", id+".json")),
		"outside the tree": mustWrite(filepath.Join(f.root, id+".json")),
		"bad key":          mustWrite(filepath.Join(cycleDir, "@@", "out", id+".json")),
		"non canonical":    mustWrite(nonCanon),
		"not json":         mustWrite(filepath.Join(cycleDir, altKey, "out", id+".txt")),
	}

	for name, p := range cases {
		layout := l
		if name == "no cycle dir" {
			layout = bare
		}
		if got, _, _, err := layout.Locate(p); err == nil {
			t.Errorf("%s: Locate(%s) = %q, want a refusal", name, p, got)
		}
	}
	dispatch, gotID, _, err := l.Locate(link)
	if err != nil || dispatch != scout.Dispatch || gotID != id {
		t.Errorf("Locate(symlink to the draft) = %q, %q, %v, want the resolved location", dispatch, gotID, err)
	}
}

func TestOutbox_AHalfWrittenDraftIsNeverRead(t *testing.T) {
	f := newFixture(t)
	dir := f.layout().OutDir(scout.Dispatch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	half := filepath.Join(dir, "."+f.newID()+".json.1234.tmp")
	if err := os.WriteFile(half, []byte(`{"type":"send","tok`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{}
	s.waits = []func() (wake.Wake, error){
		func() (wake.Wake, error) {
			if r, _ := os.ReadDir(filepath.Join(f.layout().BoxDir(scout.Dispatch), "receipts")); len(r) != 0 {
				t.Errorf("receipts = %d, want none for a half-written draft", len(r))
			}
			f.sendDraft(scout, f.token(scout))
			return wake.Wake{Changed: true}, nil
		},
		ctxStop(cancel),
	}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if _, err := os.Stat(half); err != nil {
		t.Errorf("half draft stat = %v, want it left alone", err)
	}
	if recs, _ := os.ReadDir(filepath.Join(f.layout().BoxDir(scout.Dispatch), "receipts")); len(recs) != 1 {
		t.Errorf("receipts = %d, want 1 for the renamed draft", len(recs))
	}
}

func TestOutbox_TheReceiptLandsInTheMailbox(t *testing.T) {
	f := newFixture(t)
	id, _ := f.sendDraft(scout, f.token(scout))
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	got := f.receipt(scout, id)
	if got.Receipt == nil || got.Receipt.ID != id || len(got.Receipt.Delivered) != 1 || got.Receipt.Delivered[0] != build {
		t.Errorf("answer = %+v, want the receipt of %s delivered to build", got, id)
	}
	if recs, _ := f.store.Read(context.Background(), cycle, 0); len(recs.Records) != 1 {
		t.Errorf("store holds %d records, want 1", len(recs.Records))
	}
	if len(s.armed) != 1 || len(s.armed[0].Dirs) != 1 || s.armed[0].Dirs[0] != f.layout().OutDir(scout.Dispatch) {
		t.Errorf("armed = %+v, want the own outbox only", s.armed)
	}
}

func TestOutbox_AnAckDraftAcksTheQueue(t *testing.T) {
	f := newFixture(t)
	addr, _ := address.Parse("phase:build")
	_, err := f.svc.Send(context.Background(), identity.Credential{Token: f.token(scout), Location: scout.Dispatch},
		envelope.Envelope{Kind: envelope.KindHandoff, To: []address.Address{addr}, Subject: "s", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	ack := func(cursor int64) string {
		id := f.newID()
		draft := map[string]any{"type": "ack", "token": f.token(build), "cursor": cursor}
		if err := atomicwrite.JSON(filepath.Join(f.layout().OutDir(build.Dispatch), id+".json"), draft); err != nil {
			t.Fatal(err)
		}
		return id
	}
	ok, bad := ack(1), ack(99)
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, build.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if got := f.receipt(build, ok); got.Error != "" || got.Receipt != nil {
		t.Errorf("ack answer = %+v, want an empty success", got)
	}
	if got := f.receipt(build, bad); !strings.Contains(got.Error, "cursor") {
		t.Errorf("ack answer = %+v, want the cursor error", got)
	}
}

func TestOutbox_ABadDraftGetsAnErrorAnswer(t *testing.T) {
	f := newFixture(t)
	dir := f.layout().OutDir(scout.Dispatch)
	put := func(body string) string {
		id := f.newID()
		if err := atomicwrite.Bytes(filepath.Join(dir, id+".json"), []byte(body)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	junk := put("not json")
	unknown := put(`{"type":"send","token":"x","extra":1}`)
	kind := put(`{"type":"shout","token":"x"}`)
	unreadable := put("{}")
	if err := os.Chmod(filepath.Join(dir, unreadable+".json"), 0); err != nil {
		t.Fatal(err)
	}
	asDir := f.newID()
	if err := os.Mkdir(filepath.Join(dir, asDir+".json"), 0o700); err != nil {
		t.Fatal(err)
	}
	big := put(`{"type":"send","token":"` + string(make([]byte, outbox.MaxDraftBytes)) + `"}`)
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	for name, id := range map[string]string{"junk": junk, "unknown field": unknown, "unknown type": kind, "oversize": big, "unreadable": unreadable, "directory": asDir} {
		if got := f.receipt(scout, id); got.Error == "" {
			t.Errorf("%s: answer = %+v, want an error", name, got)
		}
	}
}

func TestOutbox_RunFailsLoudly(t *testing.T) {
	f := newFixture(t)
	boom := errors.New("boom")
	t.Run("arm", func(t *testing.T) {
		err := f.watcher(&scripted{armErr: boom}).Run(context.Background(), scout.Dispatch)
		if !errors.Is(err, boom) {
			t.Errorf("Run = %v, want the arm error", err)
		}
	})
	t.Run("wait", func(t *testing.T) {
		s := &scripted{waits: []func() (wake.Wake, error){func() (wake.Wake, error) { return wake.Wake{}, boom }}}
		if err := f.watcher(s).Run(context.Background(), scout.Dispatch); !errors.Is(err, boom) {
			t.Errorf("Run = %v, want the wait error", err)
		}
	})
	t.Run("outbox is a file", func(t *testing.T) {
		blocked := newFixture(t)
		if err := os.MkdirAll(filepath.Dir(blocked.layout().OutDir(scout.Dispatch)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(blocked.layout().OutDir(scout.Dispatch), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := blocked.watcher(&scripted{}).Run(context.Background(), scout.Dispatch); err == nil {
			t.Error("Run = nil, want the mkdir error")
		}
	})
	t.Run("outbox removed under the watcher", func(t *testing.T) {
		gone := newFixture(t)
		s := &scripted{}
		s.onArmed = func() { _ = os.RemoveAll(gone.layout().OutDir(scout.Dispatch)) }
		if err := gone.watcher(s).Run(context.Background(), scout.Dispatch); err == nil {
			t.Error("Run = nil, want the read-dir error")
		}
	})
}

func TestOutbox_APathOutsideTheFormIsDeniedAndRemoved(t *testing.T) {
	f := newFixture(t)
	dir := f.layout().OutDir(scout.Dispatch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(f.root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	id := f.newID()
	real := filepath.Join(target, id+".json")
	if err := os.WriteFile(real, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(dir, id+".json")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(dir, id+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("link stat = %v, want it removed", err)
	}
	if len(f.events.events) != 1 || f.events.events[0].Rule != outbox.RulePath {
		t.Errorf("events = %+v, want one msg.denied with rule %s", f.events.events, outbox.RulePath)
	}
}

func TestOutbox_ASymlinkToAnotherOutboxIsDeniedAndNeverAttributed(t *testing.T) {
	f := newFixture(t)
	_, victim := f.sendDraft(build, f.token(build))
	dir := f.layout().OutDir(scout.Dispatch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	id := f.newID()
	link := filepath.Join(dir, id+".json")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("link stat = %v, want it removed", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("victim draft stat = %v, want it untouched", err)
	}
	if len(f.events.events) != 1 || f.events.events[0].Rule != outbox.RulePath {
		t.Errorf("events = %+v, want one msg.denied with rule %s", f.events.events, outbox.RulePath)
	}
	if b, _ := f.store.Read(context.Background(), cycle, 0); len(b.Records) != 0 {
		t.Errorf("store holds %d records, want none", len(b.Records))
	}
}

func TestOutbox_ADraftThatVanishesIsSkippedWithoutAnEvent(t *testing.T) {
	f := newFixture(t)
	dir := f.layout().OutDir(scout.Dispatch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(dir, f.newID()+".json")
	if err := os.Symlink(filepath.Join(f.root, "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v, want the watcher to survive a vanished draft", err)
	}

	if len(f.events.events) != 0 {
		t.Errorf("events = %+v, want none for a vanished draft", f.events.events)
	}
}

func TestOutbox_AReceiptThatCannotBeWrittenIsReportedAndTheWatcherGoesOn(t *testing.T) {
	f := newFixture(t)
	id, path := f.sendDraft(scout, f.token(scout))
	if err := os.MkdirAll(f.layout().BoxDir(scout.Dispatch), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.layout().BoxDir(scout.Dispatch), "receipts"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v, want the watcher to go on", err)
	}

	var rules []string
	for _, e := range f.events.events {
		rules = append(rules, e.Rule)
	}
	if !slices.Contains(rules, outbox.RuleReceipt) {
		t.Errorf("events rules = %v, want %s for draft %s", rules, outbox.RuleReceipt, id)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("draft stat = %v, want it kept for a retry", err)
	}
}

func TestOutbox_ADraftThatIsNotARegularFileGetsAnErrorAnswer(t *testing.T) {
	f := newFixture(t)
	dir := f.layout().OutDir(scout.Dispatch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	id := f.newID()
	if err := syscall.Mkfifo(filepath.Join(dir, id+".json"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if got := f.receipt(scout, id); !strings.Contains(got.Error, "regular file") {
		t.Errorf("answer = %+v, want the regular-file refusal", got)
	}
}

func TestOutbox_ADraftThatCannotBeRemovedIsReportedAndTheWatcherGoesOn(t *testing.T) {
	f := newFixture(t)
	f.sendDraft(scout, f.token(scout))
	dir := f.layout().OutDir(scout.Dispatch)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	ctx, cancel := context.WithCancel(context.Background())
	s := &scripted{waits: []func() (wake.Wake, error){ctxStop(cancel)}}

	if err := f.watcher(s).Run(ctx, scout.Dispatch); err != nil {
		t.Fatalf("Run = %v, want the watcher to go on", err)
	}

	last := f.events.events[len(f.events.events)-1]
	if last.Rule != outbox.RuleRemove {
		t.Errorf("last event = %+v, want rule %s", last, outbox.RuleRemove)
	}
}

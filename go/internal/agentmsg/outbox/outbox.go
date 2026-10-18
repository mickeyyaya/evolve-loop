package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
)

const (
	MaxDraftBytes = 256 << 10
	RulePath      = "outbox.path"
	RuleReceipt   = "outbox.receipt"
	RuleRemove    = "outbox.remove"
	typeSend      = "send"
	typeAck       = "ack"
)

type Sender interface {
	Send(ctx context.Context, c identity.Credential, e envelope.Envelope) (broker.Receipt, error)
	Ack(ctx context.Context, c identity.Credential, cursor int64) error
}

type Waker interface {
	Arm(t wake.Targets) error
	Wait(ctx context.Context, deadline time.Time) (wake.Wake, error)
}

type Config struct {
	Layout   Layout
	Sender   Sender
	Waker    Waker
	Notifier broker.Notifier
}

type Watcher struct{ cfg Config }

func New(cfg Config) *Watcher { return &Watcher{cfg: cfg} }

type Answer struct {
	Receipt *broker.Receipt `json:"receipt,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type draft struct {
	Type     string            `json:"type"`
	Token    string            `json:"token"`
	Envelope envelope.Envelope `json:"envelope"`
	Cursor   int64             `json:"cursor"`
}

func (w *Watcher) Run(ctx context.Context, dispatch string) error {
	dir := w.cfg.Layout.OutDir(dispatch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("outbox: %w", err)
	}
	if err := w.cfg.Waker.Arm(wake.Targets{Dirs: []string{dir}}); err != nil {
		return fmt.Errorf("outbox: arm %s: %w", dir, err)
	}
	for {
		if err := w.drain(ctx, dispatch, dir); err != nil {
			return err
		}
		if _, err := w.cfg.Waker.Wait(ctx, time.Time{}); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("outbox: wait: %w", err)
		}
	}
}

func (w *Watcher) drain(ctx context.Context, own, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("outbox: %w", err)
	}
	for _, e := range entries {
		if !draftName(e.Name()) {
			continue
		}
		w.ingest(ctx, own, filepath.Join(dir, e.Name()))
	}
	return nil
}

func draftName(name string) bool {
	id, ok := strings.CutSuffix(name, draftSuffix)
	return ok && envelope.ValidID(id)
}

func (w *Watcher) ingest(ctx context.Context, own, path string) {
	dispatch, id, resolved, err := w.cfg.Layout.Locate(path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil && dispatch != own {
		err = fmt.Errorf("outbox: the draft of %q lies in the outbox of %q", dispatch, own)
	}
	if err != nil {
		w.deny(ctx, RulePath, err)
		w.remove(ctx, path)
		return
	}
	d, err := readDraft(resolved)
	receipt := filepath.Join(w.cfg.Layout.BoxDir(dispatch), "receipts", id+draftSuffix)
	if err := atomicwrite.JSON(receipt, w.answer(ctx, dispatch, id, d, err)); err != nil {
		w.deny(ctx, RuleReceipt, err)
		return
	}
	w.remove(ctx, path)
}

func (w *Watcher) deny(ctx context.Context, rule string, err error) {
	w.cfg.Notifier.Notify(ctx, broker.Event{Kind: broker.EventDenied, Cycle: w.cfg.Layout.Cycle, Rule: rule, Reason: err.Error()})
}

func (w *Watcher) remove(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		w.deny(ctx, RuleRemove, err)
	}
}

func (w *Watcher) answer(ctx context.Context, dispatch, id string, d draft, readErr error) Answer {
	if readErr != nil {
		return Answer{Error: readErr.Error()}
	}
	cred := identity.Credential{Token: d.Token, Location: dispatch}
	switch d.Type {
	case typeSend:
		if d.Envelope.ID == "" {
			d.Envelope.ID = id
		}
		r, err := w.cfg.Sender.Send(ctx, cred, d.Envelope)
		if err != nil {
			return Answer{Error: err.Error()}
		}
		return Answer{Receipt: &r}
	case typeAck:
		if err := w.cfg.Sender.Ack(ctx, cred, d.Cursor); err != nil {
			return Answer{Error: err.Error()}
		}
		return Answer{}
	}
	return Answer{Error: fmt.Sprintf("outbox: unknown draft type %q", d.Type)}
}

func readDraft(path string) (draft, error) {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("a draft is a regular file, got mode %s", info.Mode().Type())
	}
	if err != nil {
		return draft{}, fmt.Errorf("outbox: %w", err)
	}
	data, err := readLimited(path)
	if err != nil {
		return draft{}, fmt.Errorf("outbox: %w", err)
	}
	if len(data) > MaxDraftBytes {
		return draft{}, fmt.Errorf("outbox: a draft is at most %d bytes", MaxDraftBytes)
	}
	var d draft
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return draft{}, fmt.Errorf("outbox: malformed draft: %w", err)
	}
	return d, nil
}

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, MaxDraftBytes+1))
}

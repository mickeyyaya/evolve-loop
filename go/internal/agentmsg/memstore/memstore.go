package memstore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
)

var ErrCursor = errors.New("memstore: cursor")

type Store struct {
	mu   sync.Mutex
	logs map[int][]broker.Record
	dead map[int][]broker.Dead
}

func New() *Store {
	return &Store{logs: map[int][]broker.Record{}, dead: map[int][]broker.Dead{}}
}

func (s *Store) Append(ctx context.Context, cycle int, r broker.Record) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r = cloneRecord(r)
	r.Cursor = int64(len(s.logs[cycle]))
	s.logs[cycle] = append(s.logs[cycle], r)
	return r.Cursor, nil
}

func (s *Store) Read(ctx context.Context, cycle int, from int64) (broker.Batch, error) {
	if err := ctx.Err(); err != nil {
		return broker.Batch{}, err
	}
	if from < 0 {
		return broker.Batch{}, fmt.Errorf("%w: %d is below 0", ErrCursor, from)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	log := s.logs[cycle]
	end := int64(len(log))
	var out []broker.Record
	for _, r := range log[min(from, end):] {
		out = append(out, cloneRecord(r))
	}
	return broker.Batch{Records: out, Next: end}, nil
}

func (s *Store) DeadLetter(ctx context.Context, cycle int, d broker.Dead) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d.Envelope = cloneEnvelope(d.Envelope)
	s.dead[cycle] = append(s.dead[cycle], d)
	return nil
}

func (s *Store) Dead(cycle int) []broker.Dead {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.dead[cycle])
}

func cloneRecord(r broker.Record) broker.Record {
	r.Envelope = cloneEnvelope(r.Envelope)
	r.Recipients = slices.Clone(r.Recipients)
	return r
}

func cloneEnvelope(e envelope.Envelope) envelope.Envelope {
	e.To = slices.Clone(e.To)
	e.Refs = slices.Clone(e.Refs)
	if e.ExpiresAt != nil {
		at := *e.ExpiresAt
		e.ExpiresAt = &at
	}
	return e
}

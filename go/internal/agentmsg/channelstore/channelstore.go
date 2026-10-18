package channelstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
)

const sourceMsg = "msg"

var (
	ErrConfig = errors.New("channelstore: config")
	ErrCursor = errors.New("channelstore: cursor")
)

type eventLog interface {
	Append(records []channel.Record) (int64, error)
	Read(from int64) (channel.Batch, error)
	Segments() ([]channel.Segment, error)
}

type opener func(root, name string, cfg channel.Config) (eventLog, error)

type Store struct {
	root string
	cfg  channel.Config
	open opener
	mu   sync.Mutex
	logs map[string]eventLog
}

func New(root string, cfg channel.Config) (*Store, error) {
	if root == "" || cfg.SegmentBytes < 1 || cfg.LockDeadline <= 0 {
		return nil, fmt.Errorf("%w: a root, a segment size above 0 and a lock deadline above 0 are required", ErrConfig)
	}
	return &Store{root: root, cfg: cfg, open: openLog, logs: map[string]eventLog{}}, nil
}

func openLog(root, name string, cfg channel.Config) (eventLog, error) {
	return channel.New(root, name, cfg)
}

func (s *Store) Append(ctx context.Context, cycle int, r broker.Record) (int64, error) {
	return s.write(ctx, cycleChannel(cycle), r.Envelope.From.Dispatch, r)
}

func (s *Store) DeadLetter(ctx context.Context, cycle int, d broker.Dead) error {
	_, err := s.write(ctx, deadChannel(cycle), d.Envelope.From.Dispatch, d)
	return err
}

func (s *Store) Read(ctx context.Context, cycle int, from int64) (broker.Batch, error) {
	if err := ctx.Err(); err != nil {
		return broker.Batch{}, err
	}
	if from < 0 {
		return broker.Batch{}, fmt.Errorf("%w: %d is below 0", ErrCursor, from)
	}
	l, err := s.logOf(cycleChannel(cycle))
	if err != nil {
		return broker.Batch{}, err
	}
	in, err := l.Read(from)
	if err != nil {
		if channelGone(l, err) {
			return broker.Batch{}, nil
		}
		return broker.Batch{}, err
	}
	return toBatch(in), nil
}

func cycleChannel(cycle int) string { return "msg." + strconv.Itoa(cycle) }

func deadChannel(cycle int) string { return "msg.dead." + strconv.Itoa(cycle) }

func (s *Store) write(ctx context.Context, name, dispatch string, payload any) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	message, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("channelstore: encode: %w", err)
	}
	l, err := s.logOf(name)
	if err != nil {
		return 0, err
	}
	cursor, err := l.Append([]channel.Record{{Source: sourceMsg, Dispatch: dispatch, Message: message}})
	if err != nil && !errors.Is(err, channel.ErrRotate) {
		return 0, err
	}
	return cursor, nil
}

func (s *Store) logOf(name string) (eventLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.logs[name]; ok {
		return l, nil
	}
	l, err := s.open(s.root, name, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("channelstore: %w", err)
	}
	s.logs[name] = l
	return l, nil
}

func channelGone(l eventLog, readErr error) bool {
	if !errors.Is(readErr, fs.ErrNotExist) {
		return false
	}
	_, err := l.Segments()
	return errors.Is(err, fs.ErrNotExist)
}

func toBatch(in channel.Batch) broker.Batch {
	out := broker.Batch{Next: in.Next}
	ends := endsOf(in)
	for i, rec := range in.Records {
		if rec.Source == channel.SourceWatch {
			out.Gaps = append(out.Gaps, broker.Gap{From: rec.Gap.From, To: rec.Gap.To, Reason: rec.Gap.Reason})
			continue
		}
		if stored, ok := decode(rec); ok {
			out.Records = append(out.Records, stored)
			continue
		}
		out.Gaps = append(out.Gaps, broker.Gap{From: rec.Cursor, To: ends[i], Reason: channel.ReasonMalformed})
	}
	return out
}

func decode(rec channel.Record) (broker.Record, bool) {
	var stored broker.Record
	if json.Unmarshal(rec.Message, &stored) != nil {
		return broker.Record{}, false
	}
	stored.Cursor = rec.Cursor
	return stored, true
}

func endsOf(in channel.Batch) []int64 {
	ends := make([]int64, len(in.Records))
	end := in.Next
	for i := len(in.Records) - 1; i >= 0; i-- {
		ends[i] = end
		end = startOf(in.Records[i])
	}
	return ends
}

func startOf(rec channel.Record) int64 {
	if rec.Source == channel.SourceWatch {
		return rec.Gap.From
	}
	return rec.Cursor
}

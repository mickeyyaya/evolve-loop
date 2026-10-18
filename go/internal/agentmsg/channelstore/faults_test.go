package channelstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
)

var errBoom = errors.New("boom")

type fakeLog struct {
	appendCursor int64
	appendErr    error
	readErr      error
	segmentsErr  error
}

func (f fakeLog) Append([]channel.Record) (int64, error) { return f.appendCursor, f.appendErr }

func (f fakeLog) Read(from int64) (channel.Batch, error) {
	return channel.Batch{Next: from}, f.readErr
}

func (f fakeLog) Segments() ([]channel.Segment, error) { return nil, f.segmentsErr }

func storeOver(t *testing.T, open opener) *Store {
	t.Helper()
	s, err := New(t.TempDir(), testConfig(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	s.open = open
	return s
}

func fixedLog(l eventLog) opener {
	return func(string, string, channel.Config) (eventLog, error) { return l, nil }
}

func TestChannelStore_ARotationErrorIsNotALoss(t *testing.T) {
	t.Parallel()
	s := storeOver(t, fixedLog(fakeLog{appendCursor: 42, appendErr: fmt.Errorf("rotate: %w", channel.ErrRotate)}))

	cursor, err := s.Append(context.Background(), cycle, record("a"))

	if err != nil || cursor != 42 {
		t.Fatalf("Append over a failed rotation = %d, %v, want cursor 42 and no error: the record is on disk", cursor, err)
	}
}

func TestChannelStore_AnAppendErrorIsReturnedWithNoCursor(t *testing.T) {
	t.Parallel()
	s := storeOver(t, fixedLog(fakeLog{appendCursor: 7, appendErr: errBoom}))

	cursor, appendErr := s.Append(context.Background(), cycle, record("a"))
	deadErr := s.DeadLetter(context.Background(), cycle, broker.Dead{})

	if !errors.Is(appendErr, errBoom) || cursor != 0 || !errors.Is(deadErr, errBoom) {
		t.Fatalf("Append = %d, %v and DeadLetter = %v, want errBoom from each and no cursor", cursor, appendErr, deadErr)
	}
}

func TestChannelStore_AnOpenErrorIsReturnedByEachCall(t *testing.T) {
	t.Parallel()
	s := storeOver(t, func(string, string, channel.Config) (eventLog, error) { return nil, errBoom })

	_, appendErr := s.Append(context.Background(), cycle, record("a"))
	_, readErr := s.Read(context.Background(), cycle, 0)
	deadErr := s.DeadLetter(context.Background(), cycle, broker.Dead{})

	for name, err := range map[string]error{"Append": appendErr, "Read": readErr, "DeadLetter": deadErr} {
		if !errors.Is(err, errBoom) {
			t.Fatalf("%s error = %v, want errBoom", name, err)
		}
	}
}

func TestChannelStore_AReadErrorIsReturnedAndKeepsNoBatch(t *testing.T) {
	t.Parallel()
	s := storeOver(t, fixedLog(fakeLog{readErr: errBoom}))

	b, err := s.Read(context.Background(), cycle, 3)

	if !errors.Is(err, errBoom) || b.Next != 0 || len(b.Records) != 0 {
		t.Fatalf("Read = %+v, %v, want errBoom and an empty batch", b, err)
	}
}

func TestChannelStore_OnlyAMissingDirectoryReadsAsEmpty(t *testing.T) {
	t.Parallel()
	notExist := fmt.Errorf("read segment: %w", fs.ErrNotExist)
	cases := map[string]struct {
		log     fakeLog
		isEmpty bool
	}{
		"the directory is gone":                    {fakeLog{readErr: notExist, segmentsErr: notExist}, true},
		"the tail segment is gone":                 {fakeLog{readErr: notExist}, false},
		"the directory cannot be read":             {fakeLog{readErr: notExist, segmentsErr: errBoom}, false},
		"the read fails and the directory is gone": {fakeLog{readErr: errBoom, segmentsErr: notExist}, false},
	}
	for name, c := range cases {
		s := storeOver(t, fixedLog(c.log))

		b, err := s.Read(context.Background(), cycle, 9)

		if c.isEmpty && (err != nil || b.Next != 0 || len(b.Records) != 0) {
			t.Fatalf("Read when %s = %+v, %v, want an empty batch", name, b, err)
		}
		if !c.isEmpty && err == nil {
			t.Fatalf("Read when %s = %+v, %v, want the error: a lost tail is never an empty channel", name, b, err)
		}
	}
}

func TestChannelStore_OneLogServesEachChannelEvenUnderConcurrentFirstUse(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	opened := map[string]int{}
	s := storeOver(t, func(root, name string, cfg channel.Config) (eventLog, error) {
		mu.Lock()
		opened[name]++
		mu.Unlock()
		return channel.New(root, name, cfg)
	})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Read(context.Background(), cycle, 0); err != nil {
				t.Errorf("Read error = %v", err)
			}
		}()
	}
	wg.Wait()

	for _, c := range []int{5, 5, 6} {
		if _, err := s.Append(context.Background(), c, record("a")); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeadLetter(context.Background(), cycle, broker.Dead{Envelope: record("a").Envelope}); err != nil {
		t.Fatal(err)
	}

	want := map[string]int{"msg.5": 1, "msg.6": 1, "msg.dead.5": 1}
	if !reflect.DeepEqual(opened, want) {
		t.Fatalf("logs opened = %v, want %v: one Log for each channel keeps the single waiting lock call of the channel package", opened, want)
	}
}

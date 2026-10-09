package reader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func nextErr(t *testing.T, r *Reader) error {
	t.Helper()
	_, err := r.Next(context.Background(), time.Time{})
	return err
}

func asFile(t *testing.T, path string) {
	t.Helper()
	mustRemove(t, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func unreadable(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory without the read bit")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestReader_AChannelPathThatIsAFileIsAnError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"cycle", "loop"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, s := newFixture(t), newScript()
			asFile(t, f.dir(name))
			r := f.reader(s, []string{"cycle"}, Since{})

			err := nextErr(t, r)

			if !errors.Is(err, syscall.ENOTDIR) || len(s.arms) != 0 {
				t.Errorf("Next = %v after %d arms, want ENOTDIR before any arm", err, len(s.arms))
			}
		})
	}
}

func TestReader_ACatalogNameThatIsNotAChannelNameIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.readerOver([]Channel{{Name: "loop"}, {Name: "Bad"}}, s, []string{"*"}, Since{})

	err := nextErr(t, r)

	if err == nil || err.Error() != `reader: channel: name "Bad" is not dot-separated tokens of [a-z0-9_-]` {
		t.Errorf("Next = %v, want the refusal of the name", err)
	}
}

func TestReader_AStartThatCannotBeReadIsAnError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"cycle", "loop"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, s := newFixture(t), newScript()
			if err := os.MkdirAll(filepath.Join(f.dir(name), "seg-00000000000000000000.ndjson"), 0o755); err != nil {
				t.Fatal(err)
			}
			r := f.reader(s, []string{"cycle"}, Since{})

			err := nextErr(t, r)

			if !errors.Is(err, syscall.EISDIR) || len(s.arms) != 0 {
				t.Errorf("Next = %v after %d arms, want EISDIR before any arm", err, len(s.arms))
			}
		})
	}
}

func TestReader_ATimeStartThatCannotBeReadIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	if err := os.MkdirAll(filepath.Join(f.dir("cycle"), "seg-00000000000000000000.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceTime, at: time.Now()})

	if err := nextErr(t, r); !errors.Is(err, syscall.EISDIR) {
		t.Errorf("Next = %v, want EISDIR", err)
	}
}

func TestReader_AnUnlistableChannelAtTheArmIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	unreadable(t, f.dir("cycle"))
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceAll})

	err := nextErr(t, r)

	if !errors.Is(err, fs.ErrPermission) || len(s.arms) != 0 {
		t.Errorf("Next = %v after %d arms, want a permission error before any arm", err, len(s.arms))
	}
}

func TestReader_AChannelPathThatBecomesAFileIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		asFile(t, f.dir("cycle"))
		return wake.Wake{Changed: true}, nil
	})

	if err := nextErr(t, r); !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("Next = %v, want ENOTDIR", err)
	}
}

func TestReader_ADirectoryThatCannotBeMadeAtTheArmIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		mustRemove(t, f.dir("cycle"))
		if err := os.Symlink(filepath.Join(f.root, "absent"), f.dir("cycle")); err != nil {
			t.Fatal(err)
		}
		return wake.Wake{Changed: true}, nil
	})

	err := nextErr(t, r)

	if !errors.Is(err, fs.ErrExist) || len(s.arms) != 1 {
		t.Errorf("Next = %v after %d arms, want EEXIST and no second arm", err, len(s.arms))
	}
}

func TestReader_AWildcardRootThatBecomesAFileAfterTheArmIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"*"}, Since{})
	s.onArm = func(n int) {
		if n == 2 {
			asFile(t, f.root)
		}
	}
	s.then(func() (wake.Wake, error) { return wake.Wake{Changed: true}, nil })

	err := nextErr(t, r)

	if !errors.Is(err, syscall.ENOTDIR) || !strings.HasPrefix(err.Error(), "reader: list channels: ") {
		t.Errorf("Next = %v, want the list error of the root", err)
	}
}

func TestReader_ADirectoryGoneBetweenTheArmAndTheReadIsMadeAgain(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	read := r.read
	wiped := false
	r.read = func(l *channel.Log, from, maxBytes int64) (channel.Batch, error) {
		if !wiped && filepath.Base(l.Dir()) == "cycle" {
			wiped = true
			mustRemove(t, l.Dir())
		}
		return read(l, from, maxBytes)
	}

	items := drain(t, r)

	if describe(items) != "" || len(s.arms) != 2 {
		t.Errorf("items = %s after %d arms, want nothing and a second arm of the made directory", describe(items), len(s.arms))
	}
	if _, err := os.Stat(f.dir("cycle")); err != nil {
		t.Errorf("the channel directory is not there again: %v", err)
	}
}

func TestReader_ADirectoryThatCannotBeMadeAtTheReadIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	read := r.read
	r.read = func(l *channel.Log, from, maxBytes int64) (channel.Batch, error) {
		if filepath.Base(l.Dir()) == "cycle" {
			asFile(t, l.Dir())
			return channel.Batch{Next: from}, fmt.Errorf("list segments: %w", fs.ErrNotExist)
		}
		return read(l, from, maxBytes)
	}

	err := nextErr(t, r)

	if !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("Next = %v, want the make error", err)
	}
}

func TestLastWill_ALoopChannelThatBecomesUnlistableIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		unreadable(t, f.dir("loop"))
		return wake.Wake{Exited: []int{1}}, nil
	})

	if err := nextErr(t, r); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("Next = %v, want a permission error", err)
	}
}

func TestLastWill_ALoopThatStartsAndExitsBetweenTwoWakesIsNoLoss(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("loop", loopStarted(loopPid, 1, procA), loopExit(loopPid, 2))
		return wake.Wake{Changed: true}, nil
	})
	s.then(func() (wake.Wake, error) { return wake.Wake{Changed: true}, nil })

	items := drain(t, r)

	if describe(items) != "loop:loop.started#1 loop:loop.exit#2" || s.count("start:9055") != 0 {
		t.Errorf("items = %s after %d start reads, want the two records, no loss and no read", describe(items), s.count("start:9055"))
	}
	for _, a := range s.arms {
		if len(a.Pids) != 0 {
			t.Errorf("arm %+v, want no exit watch", a)
		}
	}
}

func failOnce(r *Reader, name string) {
	read := r.read
	failed := false
	r.read = func(l *channel.Log, from, maxBytes int64) (channel.Batch, error) {
		if !failed && filepath.Base(l.Dir()) == name {
			failed = true
			return channel.Batch{Next: from}, syscall.EIO
		}
		return read(l, from, maxBytes)
	}
}

func TestReader_AFailedReadOfALaterChannelLosesNoItemOfAnEarlierOne(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1))
	warn := sealed(2)
	warn.Signal.Severity = signalcenter.SeverityWarn
	f.append("errors", warn)
	r := f.reader(s, []string{"cycle", "errors"}, Since{mode: sinceAll})
	failOnce(r, "errors")

	items, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, syscall.EIO) || len(items) != 0 {
		t.Fatalf("first Next = %s, %v, want EIO and no item", describe(items), err)
	}
	if got := describe(drain(t, r)); got != "cycle:cycle.sealed#1 errors:cycle.sealed#2" {
		t.Errorf("retried items = %s, want both records: a failed read moves no cursor", got)
	}
}

func TestLastWill_AFailedChannelReadLosesNoLastWill(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(func() (wake.Wake, error) {
		failOnce(r, "errors")
		return wake.Wake{Exited: []int{loopPid}}, nil
	})

	items, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, syscall.EIO) || len(items) != 0 {
		t.Fatalf("first Next = %s, %v, want EIO and no item", describe(items), err)
	}
	wantLoss(t, drain(t, r), loopPid, "crash", false)
}

func TestReader_NextReturnsItemsOrAnErrorNeverBoth(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("loop", loopStarted(loopPid, 1, procA))
		return wake.Wake{Changed: true}, nil
	})
	s.onArm = func(n int) {
		if n == 3 {
			s.armErr = wake.ErrRefused
		}
	}

	items, err := r.Next(context.Background(), time.Time{})
	again, againErr := r.Next(context.Background(), time.Time{})

	if err != nil || describe(items) != "loop:loop.started#1" {
		t.Errorf("first Next = %s, %v, want the record and no error", describe(items), err)
	}
	if !errors.Is(againErr, wake.ErrRefused) || len(again) != 0 {
		t.Errorf("second Next = %s, %v, want ErrRefused and no item", describe(again), againErr)
	}
}

package gc

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func stubExec(handle func(args []string) (stdout string, code int, err error)) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		out, code, err := handle(args)
		io.WriteString(stdout, out)
		return code, err
	}
}

func failOn(first string, code int, err error) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == first {
			return code, err
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
}

func TestParseReflogStamp(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"HEAD@{1791774224}", 1791774224, false},
		{"  HEAD@{0}\n", 0, false},
		{"HEAD@{}", 0, true},
		{"HEAD@{12", 0, true},
		{"12}", 0, true},
		{"HEAD@{-5}", 0, true},
		{"HEAD@{12ab}", 0, true},
		{"HEAD@{1 2}", 0, true},
		{"HEAD@{99999999999999999999999}", 0, true},
		{"", 0, true},
		{"refs/heads/main@{5}", 0, true},
	}
	for _, c := range cases {
		got, err := parseReflogStamp(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseReflogStamp(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got.Unix() != c.want {
			t.Errorf("parseReflogStamp(%q) = %d, want %d", c.in, got.Unix(), c.want)
		}
	}
}

func FuzzParseReflogStamp(f *testing.F) {
	for _, seed := range []string{"HEAD@{1791774224}", "HEAD@{}", "", "HEAD@{-1}", "HEAD@{9999999999999999999999}", "HEAD@{1}}", "\x00HEAD@{1}"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := parseReflogStamp(in)
		if err != nil {
			return
		}
		want := strings.TrimSpace(in)
		if !strings.HasPrefix(want, "HEAD@{") || !strings.HasSuffix(want, "}") {
			t.Fatalf("accepted %q, which is not HEAD@{<seconds>}", in)
		}
		digits := want[len("HEAD@{") : len(want)-1]
		if got.Unix() < 0 || digits == "" || strings.Trim(digits, "0123456789") != "" {
			t.Fatalf("accepted %q as %d with digits %q", in, got.Unix(), digits)
		}
	})
}

func TestLastActivity_TheNewestOfTheHeadCommitAndTheHeadReflogWins(t *testing.T) {
	e := newDevEnv(t)
	longAgo := e.real.Add(-100 * 24 * time.Hour).Truncate(time.Second)
	soon := e.real.Add(5 * 24 * time.Hour).Truncate(time.Second)

	oldCommit := e.addDev("old-commit")
	e.commitDatedIn(oldCommit, "x.txt", "x\n", longAgo)
	got, err := lastActivity(context.Background(), realGit(oldCommit))
	if err != nil {
		t.Fatal(err)
	}
	if got.Before(e.real.Add(-time.Minute)) {
		t.Errorf("a 100-day-old HEAD commit with a fresh reflog entry: last activity = %s, want the reflog time near %s", got, e.real)
	}

	futureCommit := e.addDev("future-commit")
	e.commitDatedIn(futureCommit, "y.txt", "y\n", soon)
	got, err = lastActivity(context.Background(), realGit(futureCommit))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(soon) {
		t.Errorf("a HEAD commit dated after the reflog entry: last activity = %s, want the commit time %s", got, soon)
	}
}

func TestLastActivity_TheIndexFileTimeIsNotASignal(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("index-time")
	before, err := lastActivity(context.Background(), realGit(dir))
	if err != nil {
		t.Fatal(err)
	}
	e.touchIndex(dir, e.real.Add(500*24*time.Hour))
	after, err := lastActivity(context.Background(), realGit(dir))
	if err != nil || !after.Equal(before) {
		t.Fatalf("last activity moved from %s to %s (err %v) when only the index time changed", before, after, err)
	}
}

func TestLastActivity_FailuresSurface(t *testing.T) {
	boom := errors.New("git vanished")
	cases := []struct {
		name   string
		handle func(args []string) (string, int, error)
	}{
		{"log fails", func(args []string) (string, int, error) { return "", 0, boom }},
		{"commit time is not a number", func(args []string) (string, int, error) { return "later\n", 0, nil }},
		{"reflog fails", func(args []string) (string, int, error) {
			if args[0] == "log" {
				return "1700000000\n", 0, nil
			}
			return "", 0, boom
		}},
		{"reflog stamp is malformed", func(args []string) (string, int, error) {
			if args[0] == "log" {
				return "1700000000\n", 0, nil
			}
			return "HEAD~1\n", 0, nil
		}},
	}
	for _, c := range cases {
		if _, err := lastActivity(context.Background(), gitexec.Git{Dir: t.TempDir(), Exec: stubExec(c.handle)}); err == nil {
			t.Errorf("%s: lastActivity = nil error, want an error", c.name)
		}
	}
}

func TestLastActivity_AnEmptyReflogFallsBackToTheCommitTime(t *testing.T) {
	exec := stubExec(func(args []string) (string, int, error) {
		if args[0] == "log" {
			return "1700000000\n", 0, nil
		}
		return "", 0, nil
	})
	got, err := lastActivity(context.Background(), gitexec.Git{Dir: t.TempDir(), Exec: exec})
	if err != nil || got.Unix() != 1700000000 {
		t.Fatalf("lastActivity = %v, %v; want the commit time 1700000000", got, err)
	}
}

func TestLastActivity_AnOlderReflogEntryDoesNotLowerTheCommitTime(t *testing.T) {
	exec := stubExec(func(args []string) (string, int, error) {
		if args[0] == "log" {
			return "1700000100\n", 0, nil
		}
		return "HEAD@{1700000000}\n", 0, nil
	})
	got, err := lastActivity(context.Background(), gitexec.Git{Dir: t.TempDir(), Exec: exec})
	if err != nil || got.Unix() != 1700000100 {
		t.Fatalf("lastActivity = %v, %v; want the later commit time 1700000100", got, err)
	}
}

package integration

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	recordedPRListEmpty   = "[]\n"
	recordedPRList        = `[{"number":1295},{"number":1294}]` + "\n"
	recordedPRListHeads   = `[{"headRefName":"feat/wave-integration-i13","number":1295},{"headRefName":"fix/guard-denial-ends-the-dispatch-chain","number":1294}]` + "\n"
	recordedRunListEmpty  = "[]\n"
	recordedRunList       = `[{"databaseId":41780640701},{"databaseId":41780468625}]` + "\n"
	recordedRunViewOK     = `{"conclusion":"success"}` + "\n"
	recordedRunViewActive = `{"conclusion":"","status":"in_progress"}` + "\n"
)

type ghScript struct {
	calls [][]string
	reply func(args []string) (string, int, error)
}

func (s *ghScript) run(_ context.Context, _ string, args ...string) (string, int, error) {
	s.calls = append(s.calls, args)
	return s.reply(args)
}

func has(args []string, words ...string) bool {
	for i := range args {
		if len(args)-i >= len(words) && slices.Equal(args[i:i+len(words)], words) {
			return true
		}
	}
	return false
}

func conclusionJSON(c string) string { return `{"conclusion":"` + c + `"}` + "\n" }

func runListJSON(id string) string { return `[{"databaseId":` + id + `}]` + "\n" }

func okReply(id string) func([]string) (string, int, error) {
	return replyWith(id, "success")
}

func replyWith(id, conclusion string) func([]string) (string, int, error) {
	return func(args []string) (string, int, error) {
		switch {
		case has(args, "pr", "list"):
			return recordedPRListEmpty, 0, nil
		case has(args, "pr", "create"):
			return "https://github.com/o/r/pull/42\n", 0, nil
		case has(args, "run", "list"):
			return runListJSON(id), 0, nil
		case has(args, "run", "view"):
			return conclusionJSON(conclusion), 0, nil
		}
		return "", 0, nil
	}
}

func newGhCI(t *testing.T, s *ghScript) GhCI {
	t.Helper()
	return GhCI{Repo: Repo{Dir: t.TempDir(), Git: func(context.Context, string, string, ...string) (string, error) { return "", nil }}, Gh: s.run}
}

func TestGhCIWatchRunsOneBlockingWatchChildAndNeverCallsTheAPI(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: okReply("777")}
	v, err := newGhCI(t, s).Watch(context.Background(), PR{Number: 42, Tip: sha('a')})
	if err != nil || v != Green {
		t.Fatalf("Watch = %v, %v, want green", v, err)
	}
	watches := 0
	for _, c := range s.calls {
		if has(c, "run", "watch") {
			watches++
			if !slices.Contains(c, "--exit-status") || !slices.Contains(c, "777") {
				t.Errorf("watch call %v lacks --exit-status or the run id", c)
			}
		}
		if slices.Contains(c, "api") {
			t.Errorf("call %v uses gh api; the waker must not poll", c)
		}
	}
	if watches != 1 {
		t.Errorf("watch calls = %d, want exactly 1", watches)
	}
}

func TestGhCIWatchReadsTheVerdictFromTheRunConclusion(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		name       string
		watch      func() (string, int, error)
		view       func() (string, int, error)
		want       Verdict
		fail       bool
		wantViewed bool
	}{
		{"green", func() (string, int, error) { return "", 0, nil }, nil, Green, false, true},
		{"red", func() (string, int, error) { return "", 1, nil }, func() (string, int, error) { return conclusionJSON("failure"), 0, nil }, Red, false, true},
		{"gh failed in watch and in view", func() (string, int, error) { return "", 1, nil }, func() (string, int, error) { return "", 1, nil }, "", true, true},
		{"cancelled run", func() (string, int, error) { return "", 1, nil }, func() (string, int, error) { return conclusionJSON("cancelled"), 0, nil }, "", true, true},
		{"run still active", func() (string, int, error) { return "", 0, nil }, func() (string, int, error) { return recordedRunViewActive, 0, nil }, "", true, true},
		{"view is not json", func() (string, int, error) { return "", 0, nil }, func() (string, int, error) { return "oops", 0, nil }, "", true, true},
		{"other exit", func() (string, int, error) { return "", 4, nil }, nil, "", true, false},
		{"exec error", func() (string, int, error) { return "", 0, boom }, nil, "", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			base := okReply("9")
			viewed := false
			s := &ghScript{}
			s.reply = func(args []string) (string, int, error) {
				switch {
				case has(args, "run", "watch"):
					return c.watch()
				case has(args, "run", "view") && c.view != nil:
					viewed = true
					return c.view()
				case has(args, "run", "view"):
					viewed = true
				}
				return base(args)
			}
			v, err := newGhCI(t, s).Watch(context.Background(), PR{Tip: sha('a')})
			if (err != nil) != c.fail || v != c.want || viewed != c.wantViewed {
				t.Errorf("Watch = %q, %v viewed=%v, want %q fail=%v viewed=%v", v, err, viewed, c.want, c.fail, c.wantViewed)
			}
		})
	}
}

func TestGhCIReadsTheRecordedOutputOfGh(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: func(args []string) (string, int, error) {
		switch {
		case has(args, "run", "list"):
			return recordedRunList, 0, nil
		case has(args, "run", "view"):
			return recordedRunViewOK, 0, nil
		}
		return "", 0, nil
	}}
	if v, err := newGhCI(t, s).Watch(context.Background(), PR{Tip: sha('a')}); err != nil || v != Green {
		t.Fatalf("Watch on recorded output = %q, %v, want green", v, err)
	}
	if !has(s.calls[1], "watch", "41780640701") {
		t.Errorf("watch call %v must name the first run of the recorded list", s.calls[1])
	}
}

func TestGhCIFindsNoRunIsAnError(t *testing.T) {
	t.Parallel()
	for _, reply := range []func([]string) (string, int, error){
		func([]string) (string, int, error) { return recordedRunListEmpty, 0, nil },
		func([]string) (string, int, error) { return "\n", 0, nil },
		func([]string) (string, int, error) { return "", 1, nil },
		func([]string) (string, int, error) { return "", 0, errors.New("boom") },
		func([]string) (string, int, error) { return "not-json", 0, nil },
	} {
		s := &ghScript{reply: reply}
		ci := newGhCI(t, s)
		if _, err := ci.Watch(context.Background(), PR{Tip: sha('a')}); err == nil {
			t.Error("Watch without a usable run returned nil")
		}
		if err := ci.Rerun(context.Background(), PR{Tip: sha('a')}); err == nil {
			t.Error("Rerun without a usable run returned nil")
		}
	}
}

func TestGhCIWaitsForTheRunToRegisterAndGivesUp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		empty   int
		settle  func(*int) func(context.Context) error
		wantErr bool
		settles int
	}{
		{"registers after two waits", 2, func(n *int) func(context.Context) error { return func(context.Context) error { *n++; return nil } }, false, 2},
		{"never registers", 99, func(n *int) func(context.Context) error { return func(context.Context) error { *n++; return nil } }, true, registerTries - 1},
		{"no settle", 1, func(*int) func(context.Context) error { return nil }, true, 0},
		{"settle fails", 1, func(n *int) func(context.Context) error {
			return func(context.Context) error { *n++; return errors.New("cancelled") }
		}, true, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			settles, empty := 0, c.empty
			s := &ghScript{reply: func(args []string) (string, int, error) {
				if has(args, "run", "list") && empty > 0 {
					empty--
					return recordedRunListEmpty, 0, nil
				}
				return okReply("5")(args)
			}}
			ci := newGhCI(t, s)
			ci.Settle = c.settle(&settles)
			_, err := ci.Watch(context.Background(), PR{Tip: sha('a')})
			if (err != nil) != c.wantErr || settles != c.settles {
				t.Errorf("err %v settles %d, want fail=%v settles=%d", err, settles, c.wantErr, c.settles)
			}
		})
	}
}

func TestGhCIRerunReRunsTheFailedJobs(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: okReply("55")}
	if err := newGhCI(t, s).Rerun(context.Background(), PR{Tip: sha('a')}); err != nil {
		t.Fatal(err)
	}
	last := s.calls[len(s.calls)-1]
	if !has(last, "run", "rerun", "55") || !slices.Contains(last, "--failed") {
		t.Errorf("last call %v, want gh run rerun 55 --failed", last)
	}
	s.reply = func(args []string) (string, int, error) {
		if has(args, "run", "rerun") {
			return "", 1, nil
		}
		return okReply("55")(args)
	}
	if err := newGhCI(t, s).Rerun(context.Background(), PR{Tip: sha('a')}); err == nil {
		t.Error("a failed rerun returned nil")
	}
}

func TestGhCIOpenPushesWithALeaseThenCreatesThePR(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: okReply("1")}
	var gitCalls [][]string
	ci := GhCI{Repo: Repo{Git: func(_ context.Context, _, _ string, args ...string) (string, error) {
		gitCalls = append(gitCalls, args)
		return "", nil
	}}, Gh: s.run}
	b := Batch{Wave: "1", Index: 2, Base: sha('0'), Branch: "integration/w1-b2", Tip: sha('a')}
	pr, err := ci.Open(context.Background(), b)
	if err != nil || pr.Number != 42 || pr.Tip != b.Tip {
		t.Fatalf("Open = %+v, %v, want PR 42 at the tip", pr, err)
	}
	if len(gitCalls) != 1 || !slices.Contains(gitCalls[0], "--force-with-lease") || !slices.Contains(gitCalls[0], b.Tip+":refs/heads/"+b.Branch) {
		t.Errorf("git calls %v, want one lease push of the tip", gitCalls)
	}
	list := s.calls[0]
	if !has(list, "--state", "open") || !has(list, "--head", b.Branch) {
		t.Errorf("list call %v must look for an open PR of the batch branch", list)
	}
}

func TestGhCIOpenReusesAnOpenPRFromTheRecordedList(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: func(args []string) (string, int, error) {
		if has(args, "pr", "list") {
			return recordedPRList, 0, nil
		}
		return "", 0, nil
	}}
	b := Batch{Wave: "1", Index: 1, Base: sha('0'), Branch: "integration/w1-b1", Tip: sha('a')}
	pr, err := newGhCI(t, s).Open(context.Background(), b)
	if err != nil || pr.Number != 1295 {
		t.Fatalf("Open = %+v, %v, want PR 1295", pr, err)
	}
	for _, c := range s.calls {
		if has(c, "pr", "create") {
			t.Errorf("created a second PR: %v", c)
		}
	}
}

func TestGhCIOpenFailures(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	b := Batch{Wave: "1", Index: 1, Base: sha('0'), Branch: "integration/w1-b1", Tip: sha('a')}
	forCreate := func(out string, code int, err error) func([]string) (string, int, error) {
		return func(args []string) (string, int, error) {
			if has(args, "pr", "create") {
				return out, code, err
			}
			return recordedPRListEmpty, 0, nil
		}
	}
	cases := map[string]func(args []string) (string, int, error){
		"list":        func(args []string) (string, int, error) { return "", 0, boom },
		"list exit":   func(args []string) (string, int, error) { return "", 1, nil },
		"list shape":  func(args []string) (string, int, error) { return "x", 0, nil },
		"list number": func(args []string) (string, int, error) { return `[{"number":"x"}]`, 0, nil },
		"create":      forCreate("", 0, boom),
		"create exit": forCreate("", 1, nil),
		"create url":  forCreate("no url here", 0, nil),
		"create huge": forCreate("https://github.com/o/r/pull/99999999999999999999\n", 0, nil),
	}
	for name, reply := range cases {
		s := &ghScript{reply: reply}
		if _, err := newGhCI(t, s).Open(context.Background(), b); err == nil {
			t.Errorf("%s: Open returned nil", name)
		}
	}
	bad := Batch{Branch: "x"}
	if _, err := newGhCI(t, &ghScript{reply: okReply("1")}).Open(context.Background(), bad); err == nil {
		t.Error("Open of an invalid batch returned nil")
	}
	ci := GhCI{Repo: Repo{Git: func(context.Context, string, string, ...string) (string, error) { return "", boom }}, Gh: (&ghScript{reply: okReply("1")}).run}
	if _, err := ci.Open(context.Background(), b); !errors.Is(err, boom) {
		t.Errorf("push failure = %v, want boom", err)
	}
}

func TestNoProductionCodeInThePackageSleepsOrTicks(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), f, src, 0); err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"time.Sleep", "time.Tick", "time.NewTicker", "time.After(", "\"api\""} {
			if strings.Contains(string(src), banned) {
				t.Errorf("%s uses %s; the CI waker must block on gh run watch and not poll", f, banned)
			}
		}
	}
}

func TestSysGhReportsAMissingBinaryAsAnError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, _, err := SysGh()(context.Background(), t.TempDir(), "run", "watch"); err == nil {
		t.Error("SysGh with no gh on PATH returned nil")
	}
}

func TestGhHostListsCommentsAndClosesThroughGh(t *testing.T) {
	t.Parallel()
	var calls [][]string
	gh := func(_ context.Context, _ string, args ...string) (string, int, error) {
		calls = append(calls, args)
		if args[1] == "list" {
			return recordedPRListHeads, 0, nil
		}
		return "", 0, nil
	}
	h := GhHost{CI: GhCI{Gh: gh}}
	ready, err := h.Ready(context.Background())
	if err != nil || !slices.Equal(ready, []int{1295, 1294}) {
		t.Fatalf("Ready = %v, %v", ready, err)
	}
	if err := h.Comment(context.Background(), 5, "brief"); err != nil {
		t.Fatal(err)
	}
	if err := h.Close(context.Background(), 5, "landed"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"pr", "list", "--state", "open", "--base", mainBranch, "--search", "draft:false", "--limit", "100", "--json", "number,headRefName"},
		{"pr", "comment", "5", "--body", "brief"},
		{"pr", "close", "5", "--comment", "landed"},
	}
	for i, w := range want {
		if strings.Join(calls[i], " ") != strings.Join(w, " ") {
			t.Errorf("call %d = %v, want %v", i, calls[i], w)
		}
	}
}

func TestGhHostReadyLeavesOutTheBatchPRsOfTheTrain(t *testing.T) {
	t.Parallel()
	gh := func(context.Context, string, ...string) (string, int, error) {
		return `[{"number":7,"headRefName":"integration/wpr1-b1"},{"number":8,"headRefName":"feat/x"}]`, 0, nil
	}
	ready, err := (GhHost{CI: GhCI{Gh: gh}}).Ready(context.Background())
	if err != nil || !slices.Equal(ready, []int{8}) {
		t.Fatalf("Ready = %v, %v, want only PR 8", ready, err)
	}
}

func TestGhHostReadyFailsLoudlyWhenGhMayHaveCutTheList(t *testing.T) {
	t.Parallel()
	var items []string
	for i := 1; i <= readyLimit; i++ {
		items = append(items, `{"number":`+strings.Repeat("1", 1)+`,"headRefName":"f"}`)
	}
	gh := func(context.Context, string, ...string) (string, int, error) {
		return "[" + strings.Join(items, ",") + "]", 0, nil
	}
	if _, err := (GhHost{CI: GhCI{Gh: gh}}).Ready(context.Background()); err == nil {
		t.Error("a full page of PRs may be cut and must fail loudly")
	}
}

func TestGhHostRefusesWhatGhRefuses(t *testing.T) {
	t.Parallel()
	bad := func(context.Context, string, ...string) (string, int, error) { return "x\n", 0, nil }
	if _, err := (GhHost{CI: GhCI{Gh: bad}}).Ready(context.Background()); err == nil {
		t.Error("a non-JSON answer must fail")
	}
	down := func(context.Context, string, ...string) (string, int, error) { return "", 1, nil }
	h := GhHost{CI: GhCI{Gh: down}}
	if _, err := h.Ready(context.Background()); err == nil {
		t.Error("Ready hid a gh failure")
	}
	if h.Comment(context.Background(), 1, "b") == nil || h.Close(context.Background(), 1, "b") == nil {
		t.Error("Comment and Close hid a gh failure")
	}
}

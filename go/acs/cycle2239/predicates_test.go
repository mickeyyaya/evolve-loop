//go:build acs

package cycle2239

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	settleCap     = 100000
	batchBase     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	batchTip      = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cycleLanded   = 7
	runIdentifier = "4242"
)

type ghFake struct {
	view    func(n int) string
	calls   [][]string
	viewed  int
	settles int
}

func hasWords(args []string, words ...string) bool {
	for i := range args {
		if len(args)-i >= len(words) && slices.Equal(args[i:i+len(words)], words) {
			return true
		}
	}
	return false
}

func (g *ghFake) run(_ context.Context, _ string, args ...string) (string, int, error) {
	g.calls = append(g.calls, args)
	switch {
	case hasWords(args, "pr", "list"):
		return "[]\n", 0, nil
	case hasWords(args, "pr", "create"):
		return "https://github.com/o/r/pull/42\n", 0, nil
	case hasWords(args, "run", "list"):
		return `[{"databaseId":` + runIdentifier + `}]` + "\n", 0, nil
	case hasWords(args, "run", "view"):
		g.viewed++
		return g.view(g.viewed), 0, nil
	}
	return "", 0, nil
}

func (g *ghFake) countOf(words ...string) int {
	n := 0
	for _, c := range g.calls {
		if hasWords(c, words...) {
			n++
		}
	}
	return n
}

func runView(status, conclusion string) string {
	return `{"conclusion":"` + conclusion + `","status":"` + status + `"}` + "\n"
}

func pendingThen(pending int, status, final string) func(int) string {
	return func(n int) string {
		if n <= pending {
			return runView(status, "")
		}
		return runView("completed", final)
	}
}

func gitNoop(context.Context, string, string, ...string) (string, error) { return "", nil }

func (g *ghFake) ci(t *testing.T, settle func(context.Context) error) integration.GhCI {
	t.Helper()
	return integration.GhCI{
		Repo:   integration.Repo{Dir: t.TempDir(), Git: gitNoop},
		Gh:     g.run,
		Settle: settle,
	}
}

func (g *ghFake) countingSettle(cancelAt int, cancel context.CancelFunc) func(context.Context) error {
	return func(c context.Context) error {
		g.settles++
		if g.settles >= settleCap {
			return errors.New("test cap: the run was polled without end")
		}
		if cancelAt > 0 && g.settles >= cancelAt {
			cancel()
			return c.Err()
		}
		return nil
	}
}

type mainFake struct {
	head   string
	landed []string
}

func (m *mainFake) Head(context.Context) (string, error) { return m.head, nil }

func (m *mainFake) FastForward(_ context.Context, tip string) error {
	m.landed = append(m.landed, tip)
	m.head = tip
	return nil
}

func merger(ci integration.CI, m *mainFake, ejected *[]int) integration.Merger {
	batch := integration.Batch{Wave: "1", Index: 1, Base: batchBase, Branch: "integration/w1-b1", Tip: batchTip}
	return integration.Merger{
		CI:   ci,
		Main: m,
		Compose: func(_ context.Context, recs []integration.DeliverableRecord) (integration.Composition, error) {
			return integration.Composition{Batch: batch, Kept: recs}, nil
		},
		Eject: func(_ context.Context, rec integration.DeliverableRecord, _ error) error {
			*ejected = append(*ejected, rec.Ref())
			return nil
		},
	}
}

func oneRecord() []integration.DeliverableRecord {
	return []integration.DeliverableRecord{{Cycle: cycleLanded}}
}

func TestC2239_001_ABatchWaitsOutARunInProgressAndLandsOnItsSuccess(t *testing.T) {
	for _, status := range []string{"in_progress", "queued", "waiting"} {
		t.Run(status, func(t *testing.T) {
			g := &ghFake{view: pendingThen(3, status, "success")}
			main := &mainFake{head: batchBase}
			var ejected []int
			ci := g.ci(t, nil)
			ci.Settle = g.countingSettle(0, func() {})
			out, err := merger(ci, main, &ejected).Land(context.Background(), oneRecord())
			if err != nil {
				t.Fatalf("Land on a run that was %s and then succeeded = %v, want nil", status, err)
			}
			if !slices.Equal(out.Landed, []int{cycleLanded}) || !slices.Equal(main.landed, []string{batchTip}) {
				t.Errorf("landed %v, main fast-forwards %v, want [%d] and [%s]", out.Landed, main.landed, cycleLanded, batchTip)
			}
			if len(ejected) != 0 || out.Flaked || out.ConfirmedRed {
				t.Errorf("ejected %v flaked %v red %v, want a clean landing", ejected, out.Flaked, out.ConfirmedRed)
			}
			if g.countOf("run", "rerun") != 0 {
				t.Errorf("a run still in progress was rerun")
			}
			if g.viewed < 4 {
				t.Errorf("run view called %d times, want the run read again until it completed (4)", g.viewed)
			}
			if g.settles == 0 {
				t.Errorf("the batch never waited through Settle while the run was %s", status)
			}
		})
	}
}

func TestC2239_002_ARunStillInProgressPastTheDeadlineEndsWithANamedTimeout(t *testing.T) {
	always := func(int) string { return runView("in_progress", "") }
	t.Run("the settle wait reports the deadline", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		g := &ghFake{view: always}
		main := &mainFake{head: batchBase}
		var ejected []int
		ci := g.ci(t, nil)
		ci.Settle = g.countingSettle(3, cancel)
		out, err := merger(ci, main, &ejected).Land(ctx, oneRecord())
		if !errors.Is(err, integration.ErrRunTimeout) {
			t.Fatalf("Land = %v, want integration.ErrRunTimeout", err)
		}
		if len(out.Landed) != 0 || len(main.landed) != 0 || len(ejected) != 0 {
			t.Errorf("landed %v main %v ejected %v, want nothing landed and nothing ejected on a timeout", out.Landed, main.landed, ejected)
		}
	})
	t.Run("the context ends while the settle wait returns nil", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		g := &ghFake{view: always}
		ci := g.ci(t, nil)
		inner := g.countingSettle(0, cancel)
		ci.Settle = func(c context.Context) error {
			if err := inner(c); err != nil {
				return err
			}
			if g.settles == 3 {
				cancel()
			}
			return nil
		}
		verdict, err := ci.Watch(ctx, integration.PR{Number: 42, Tip: batchTip})
		if !errors.Is(err, integration.ErrRunTimeout) || verdict != "" {
			t.Fatalf("Watch = %q, %v, want no verdict and integration.ErrRunTimeout", verdict, err)
		}
	})
	t.Run("a background context still ends the wait", func(t *testing.T) {
		g := &ghFake{view: always}
		ci := g.ci(t, nil)
		ci.Settle = g.countingSettle(0, func() {})
		verdict, err := ci.Watch(context.Background(), integration.PR{Number: 42, Tip: batchTip})
		if !errors.Is(err, integration.ErrRunTimeout) || verdict != "" {
			t.Fatalf("Watch = %q, %v after %d waits, want no verdict and integration.ErrRunTimeout", verdict, err, g.settles)
		}
		if g.settles >= settleCap-1 {
			t.Errorf("waited %d times, the wait has no bound of its own", g.settles)
		}
	})
	t.Run("no way to wait is a timeout and never a green", func(t *testing.T) {
		g := &ghFake{view: always}
		verdict, err := g.ci(t, nil).Watch(context.Background(), integration.PR{Number: 42, Tip: batchTip})
		if err == nil || verdict != "" {
			t.Fatalf("Watch = %q, %v, want an error and no verdict", verdict, err)
		}
	})
}

func TestC2239_003_ACompletedRunWithAnEmptyConclusionIsStillRefused(t *testing.T) {
	g := &ghFake{view: func(int) string { return runView("completed", "") }}
	main := &mainFake{head: batchBase}
	var ejected []int
	ci := g.ci(t, nil)
	ci.Settle = g.countingSettle(0, func() {})
	verdict, err := ci.Watch(context.Background(), integration.PR{Number: 42, Tip: batchTip})
	if err == nil || verdict != "" {
		t.Fatalf("Watch = %q, %v, want an error and no verdict", verdict, err)
	}
	if !strings.Contains(err.Error(), "neither green nor red") || !strings.Contains(err.Error(), runIdentifier) {
		t.Errorf("error %q lacks the refusal text and the run id", err)
	}
	if errors.Is(err, integration.ErrRunTimeout) {
		t.Errorf("a completed run was reported as a timeout: %v", err)
	}
	if g.settles != 0 {
		t.Errorf("waited %d times on a run that had completed", g.settles)
	}
	out, landErr := merger(ci, main, &ejected).Land(context.Background(), oneRecord())
	if landErr == nil || len(out.Landed) != 0 || len(main.landed) != 0 {
		t.Errorf("Land = %v landed %v main %v, want a refusal with nothing landed", landErr, out.Landed, main.landed)
	}
}

func TestC2239_004_ARunInProgressThatThenFailsIsRedAndNeverGreen(t *testing.T) {
	g := &ghFake{view: pendingThen(2, "in_progress", "failure")}
	ci := g.ci(t, nil)
	ci.Settle = g.countingSettle(0, func() {})
	verdict, err := ci.Watch(context.Background(), integration.PR{Number: 42, Tip: batchTip})
	if err != nil || verdict != integration.Red {
		t.Fatalf("Watch = %q, %v, want red", verdict, err)
	}
	if g.settles == 0 {
		t.Errorf("the failing run was read once and never waited for")
	}
}

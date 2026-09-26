package panestream

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestSignalCenter_BusyOf_AllProfilesMatrix(t *testing.T) {
	sc := NewLivenessCenter()
	cases := []struct {
		name         string
		profile      PaneProfile
		busyRendered string
		idleRendered string
	}{
		{
			name:         "claude",
			profile:      Profiles["claude"],
			busyRendered: "⏺ thinking\nesc to interrupt\n❯ \n",
			idleRendered: "⏺ done\n❯ \n",
		},
		{
			name:         "codex",
			profile:      Profiles["codex"],
			busyRendered: "some output\nesc to interrupt\n› \n",
			idleRendered: "some output\n› \n",
		},
		{
			name:         "agy",
			profile:      Profiles["agy"],
			busyRendered: "working\nesc to cancel\n> \n",
			idleRendered: "done\n> \n",
		},
		{
			name:         "ollama",
			profile:      Profiles["ollama"],
			busyRendered: "thinking\nesc to interrupt\n>>> Send a message\n",
			idleRendered: ">>> Send a message\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name+"/busy", func(t *testing.T) {
			want := PaneBusy(c.busyRendered, c.profile)
			if !want {
				t.Fatalf("fixture bug: PaneBusy(busyRendered) = false for profile %s, fixture must be genuinely busy", c.name)
			}
			if got := sc.BusyOf(c.busyRendered, c.profile); got != want {
				t.Errorf("BusyOf(%s busy fixture) = %v, want %v (PaneBusy oracle)", c.name, got, want)
			}
		})
		t.Run(c.name+"/idle", func(t *testing.T) {
			want := PaneBusy(c.idleRendered, c.profile)
			if want {
				t.Fatalf("fixture bug: PaneBusy(idleRendered) = true for profile %s, fixture must be genuinely idle", c.name)
			}
			if got := sc.BusyOf(c.idleRendered, c.profile); got != want {
				t.Errorf("BusyOf(%s idle fixture) = %v, want %v (PaneBusy oracle)", c.name, got, want)
			}
		})
	}
}

func TestSignalCenter_BusyOf_ZeroValueProfileWithAffordanceLine(t *testing.T) {
	sc := NewLivenessCenter()
	zero := PaneProfile{}
	rendered := "some agent output\nesc to interrupt\n"

	want := PaneBusy(rendered, zero)
	if !want {
		t.Fatalf("fixture bug: PaneBusy(rendered, zero-value profile) = false, want true (affordance line present)")
	}
	if got := sc.BusyOf(rendered, zero); got != want {
		t.Errorf("BusyOf(zero-value profile, affordance present) = %v, want %v", got, want)
	}
}

func TestSignalCenter_BusyOf_ANSIWrappedAffordanceLine(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]
	rendered := "⏺ thinking\n\x1b[31mesc to interrupt\x1b[0m\n❯ \n"

	want := PaneBusy(rendered, p)
	if !want {
		t.Fatalf("fixture bug: PaneBusy(ANSI-wrapped affordance) = false, want true")
	}
	if got := sc.BusyOf(rendered, p); got != want {
		t.Errorf("BusyOf(ANSI-wrapped affordance line) = %v, want %v — ANSI must be stripped identically to PaneBusy", got, want)
	}
}

func TestSignalCenter_BusyOf_OllamaEmptyPaneReadsBusy(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["ollama"]

	want := PaneBusy("", p)
	if !want {
		t.Fatalf("contract assumption violated: PaneBusy(\"\", ollama-profile) = false; this test's premise (empty pane lacks the IdlePlaceholder substring) no longer holds — re-derive from the current PaneBusy/Profiles definitions")
	}
	if got := sc.BusyOf("", p); got != want {
		t.Errorf("BusyOf(\"\", ollama-profile) = %v, want %v (PaneBusy oracle) — BusyOf must delegate exactly, including this edge case", got, want)
	}
}

func TestSignalCenter_BusyOf_OllamaIdlePlaceholderBoundary(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["ollama"]

	cases := []struct {
		name     string
		rendered string
	}{
		{"placeholder_present_no_affordance", ">>> Send a message\n"},
		{"placeholder_absent_no_affordance", "thinking...\n>>> \n"},
		{"placeholder_present_and_affordance", "esc to interrupt\n>>> Send a message\n"},
		{"placeholder_absent_and_affordance", "esc to interrupt\n>>> \n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := PaneBusy(c.rendered, p)
			if got := sc.BusyOf(c.rendered, p); got != want {
				t.Errorf("BusyOf(%s) = %v, want %v (PaneBusy oracle)", c.name, got, want)
			}
		})
	}
}

func TestSignalCenter_BusyOf_LargeRenderedPaneBuried(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]

	const numFillerLines = 5000
	buildPane := func(affordanceAtLine int) string {
		var b strings.Builder
		for i := 0; i < numFillerLines; i++ {
			if i == affordanceAtLine {
				b.WriteString("esc to interrupt\n")
				continue
			}
			fmt.Fprintf(&b, "scrollback content line %d\n", i)
		}
		b.WriteString("❯ \n")
		return b.String()
	}

	busyRendered := buildPane(numFillerLines - 10) // buried near the end
	idleRendered := buildPane(-1)                  // no affordance anywhere

	if want := PaneBusy(busyRendered, p); !want {
		t.Fatalf("fixture bug: PaneBusy(large buried-affordance pane) = false, want true")
	} else if got := sc.BusyOf(busyRendered, p); got != want {
		t.Errorf("BusyOf(large buried-affordance pane) = %v, want %v", got, want)
	}

	if want := PaneBusy(idleRendered, p); want {
		t.Fatalf("fixture bug: PaneBusy(large affordance-free pane) = true, want false")
	} else if got := sc.BusyOf(idleRendered, p); got != want {
		t.Errorf("BusyOf(large affordance-free pane) = %v, want %v", got, want)
	}
}

func TestSignalCenter_BusyOf_NilReceiverMatchesNonNilForBusyInput(t *testing.T) {
	var nilCenter *LivenessCenter
	freshCenter := NewLivenessCenter()
	p := Profiles["claude"]
	rendered := "⏺ thinking\nesc to interrupt\n❯ \n"

	oracle := PaneBusy(rendered, p)
	if !oracle {
		t.Fatalf("fixture bug: PaneBusy(rendered) = false, want true")
	}

	nilGot := nilCenter.BusyOf(rendered, p)
	freshGot := freshCenter.BusyOf(rendered, p)

	if nilGot != oracle {
		t.Errorf("nil-receiver BusyOf(busy input) = %v, want %v (PaneBusy oracle) — nil-safety must not mean 'always false'", nilGot, oracle)
	}
	if freshGot != oracle {
		t.Errorf("non-nil BusyOf(busy input) = %v, want %v (PaneBusy oracle)", freshGot, oracle)
	}
	if nilGot != freshGot {
		t.Errorf("nil-receiver BusyOf = %v but non-nil BusyOf = %v for identical input — nil-receiver-safety must be value-transparent, not merely panic-free", nilGot, freshGot)
	}
}

func TestSignalCenter_BusyOf_RepeatedCallsIdempotentNoStateLeak(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]
	busy := "⏺ thinking\nesc to interrupt\n❯ \n"
	idle := "⏺ done\n❯ \n"

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		if i%2 == 0 {
			if got := sc.BusyOf(busy, p); !got {
				t.Fatalf("iteration %d: BusyOf(busy) = false, want true (state leaked from a prior call?)", i)
			}
		} else {
			if got := sc.BusyOf(idle, p); got {
				t.Fatalf("iteration %d: BusyOf(idle) = true, want false (state leaked from a prior call?)", i)
			}
		}
	}

	if got := len(sc.sessions); got != 0 {
		t.Errorf("after %d BusyOf calls: len(sc.sessions) = %d, want 0 (BusyOf must never create session state)", iterations, got)
	}
}

func TestSignalCenter_BusyOf_ConcurrentWithObserveAggregateRegisterHandler(t *testing.T) {
	const numProducers = 12
	const observesPerProducer = 50
	const busyOfIterations = 300

	sc := NewLivenessCenter()
	claudeProfile := Profiles["claude"]
	busyRendered := "⏺ thinking\nesc to interrupt\n❯ \n"
	idleRendered := "⏺ done\n❯ \n"

	var wg sync.WaitGroup

	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("busyof-sess-%d", id)
			for j := 0; j < observesPerProducer; j++ {
				sc.Observe(key, fmt.Sprintf("⏺ producer-%d tick-%d\n❯ \n", id, j), claudeProfile)
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 4; i++ {
			name := fmt.Sprintf("busyof-handler-%d", i)
			sc.RegisterHandler(name, func() LivenessProbe { return NewDefaultDetector(0) })
		}
	}()

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < 100; i++ {
			state := sc.Aggregate()
			if !validAggregateStates[state] {
				t.Errorf("Aggregate() returned invalid state %v under concurrency (with BusyOf traffic interleaved)", state)
				return
			}
			key := fmt.Sprintf("busyof-sess-%d", i%numProducers)
			_ = sc.Busy(key)
			_ = sc.Changed(key)
		}
	}()

	busyOfDone := make(chan struct{})
	go func() {
		defer close(busyOfDone)
		for i := 0; i < busyOfIterations; i++ {
			if i%2 == 0 {
				if got := sc.BusyOf(busyRendered, claudeProfile); !got {
					t.Errorf("concurrent BusyOf(busy) iteration %d = false, want true", i)
					return
				}
			} else {
				if got := sc.BusyOf(idleRendered, claudeProfile); got {
					t.Errorf("concurrent BusyOf(idle) iteration %d = true, want false", i)
					return
				}
			}
		}
	}()

	wg.Wait()
	<-readerDone
	<-busyOfDone

	if got := len(sc.sessions); got != numProducers {
		t.Errorf("after mixed-op stress with BusyOf interleaved: len(sc.sessions) = %d, want %d", got, numProducers)
	}
}

package panestream

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// signalcenter_busyof_amplify_test.go — Test-Amplification pass for cycle 434
// slice S4 (s4-complete-residual-busy-callsites, ADR-0068). Written black-box
// against the TDD contract (test-report.md) and the eval acceptance criteria
// (s4-complete-residual-busy-callsites.md) WITHOUT reading signalcenter.go's
// diff — only the documented signature is used:
//
//	func (sc *SignalCenter) BusyOf(rendered string, profile PaneProfile) bool
//
// Contract pinned here (from the spec, not the implementation):
//  1. BusyOf delegates to the SAME PaneBusy(rendered, profile) definition
//     (test-report.md: "delegates to the same PaneBusy definition; no
//     Observe, no per-session state") — every case below is a DIFFERENTIAL
//     oracle against the real, pre-existing panedelta.go:PaneBusy, so any
//     future divergence (e.g. a "helpful" special case added only to BusyOf)
//     fails here even though it might look like an improvement.
//  2. Nil-receiver-safe (build-report.md: "safe on a nil *SignalCenter
//     receiver ... never dereferences sc").
//  3. Stateless: never mutates sc.sessions (build-report.md).
//
// These tests target gaps NOT already named in build-report.md's Self-Verify
// Evidence list (TestSignalCenter_BusyOf_MatchesStandalonePaneBusy,
// _EmptyPaneUnknownProfileNoPanic, _StatelessNoSessionMutation,
// _NilReceiverSafe) — breadth across all four shipped CLI profiles, the
// ANSI/large-input/ollama-placeholder edge cases those single-fixture names
// suggest are not yet covered, and NEW-method safety under the established
// ParallelEvaluate mixed-op concurrency stress (signalcenter_parallelevaluate_test.go,
// cycle 433) which predates BusyOf and so never exercised it.

// TestSignalCenter_BusyOf_AllProfilesMatrix (breadth, differential):
// every shipped CLI profile (claude, codex, agy, ollama) must have BusyOf
// agree with the real PaneBusy for both a busy and an idle fixture. The
// existing MatchesStandalonePaneBusy test (per build-report.md) is singular
// in name; this closes the per-profile breadth gap explicitly.
func TestSignalCenter_BusyOf_AllProfilesMatrix(t *testing.T) {
	sc := NewSignalCenter()
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

// TestSignalCenter_BusyOf_ZeroValueProfileWithAffordanceLine (edge,
// differential): an unrecognized/zero-value PaneProfile combined with a
// NON-empty pane that DOES contain the interrupt affordance must still read
// busy=true — Rule 1 (affordance-line detection) is documented as
// profile-independent (classify.go: "the only profile-specific busy signal
// is ollama's IdlePlaceholder"). This is distinct from the existing
// EmptyPaneUnknownProfileNoPanic test, which (per its name) pairs an
// unknown profile with an EMPTY pane; an unknown profile must not be
// conflated with "always reads idle."
func TestSignalCenter_BusyOf_ZeroValueProfileWithAffordanceLine(t *testing.T) {
	sc := NewSignalCenter()
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

// TestSignalCenter_BusyOf_ANSIWrappedAffordanceLine (adversarial): a real
// tmux capture-pane wraps text in ANSI SGR sequences. PaneBusy strips ANSI
// before line-splitting (panedelta.go: "clean := stripANSI(rendered)"); a
// BusyOf that read raw bytes without going through that same path would
// falsely report idle on live terminal output.
func TestSignalCenter_BusyOf_ANSIWrappedAffordanceLine(t *testing.T) {
	sc := NewSignalCenter()
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

// TestSignalCenter_BusyOf_OllamaEmptyPaneReadsBusy (surprising edge,
// differential): PaneBusy's Rule 2 for a profile with a non-empty
// IdlePlaceholder is "busy if the placeholder is ABSENT" — an EMPTY rendered
// pane trivially does not contain the placeholder, so ollama's empty-pane
// case reads BUSY, the opposite of the intuitive "empty = idle" assumption
// the EmptyPaneUnknownProfileNoPanic test name suggests for an unknown
// (non-ollama-shaped) profile. Pinning this exact behavior guards against a
// future BusyOf edit that "fixes" this by short-circuiting on empty input —
// which would break the documented single-definition delegation contract.
func TestSignalCenter_BusyOf_OllamaEmptyPaneReadsBusy(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["ollama"]

	want := PaneBusy("", p)
	if !want {
		t.Fatalf("contract assumption violated: PaneBusy(\"\", ollama-profile) = false; this test's premise (empty pane lacks the IdlePlaceholder substring) no longer holds — re-derive from the current PaneBusy/Profiles definitions")
	}
	if got := sc.BusyOf("", p); got != want {
		t.Errorf("BusyOf(\"\", ollama-profile) = %v, want %v (PaneBusy oracle) — BusyOf must delegate exactly, including this edge case", got, want)
	}
}

// TestSignalCenter_BusyOf_OllamaIdlePlaceholderBoundary (edge matrix,
// differential): exhaustively covers the interaction of PaneBusy's two
// rules for the one profile where both are simultaneously reachable
// (ollama has both an IdlePlaceholder AND can carry an affordance line).
func TestSignalCenter_BusyOf_OllamaIdlePlaceholderBoundary(t *testing.T) {
	sc := NewSignalCenter()
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

// TestSignalCenter_BusyOf_LargeRenderedPaneBuried (limit/large-scale):
// a multi-thousand-line rendered pane (large tmux scrollback capture) with
// the single busy affordance line buried deep inside must still be detected
// correctly — and the negative case (no affordance anywhere in a
// large pane) must not false-positive.
func TestSignalCenter_BusyOf_LargeRenderedPaneBuried(t *testing.T) {
	sc := NewSignalCenter()
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

// TestSignalCenter_BusyOf_NilReceiverMatchesNonNilForBusyInput
// (nil-safety, differential): build-report.md's NilReceiverSafe test (by
// name) proves BusyOf does not PANIC on a nil receiver; it does not by
// itself prove the nil receiver returns the CORRECT VALUE for genuinely
// busy input. A nil-safe method that silently always returned false would
// pass a no-panic check while breaking autorespond.go's busy-gate (which
// calls exactly `ar.deps.LivenessCenter.BusyOf(...)` where LivenessCenter is
// nil on the majority of production paths per test-report.md's design
// rationale) — this closes that gap.
func TestSignalCenter_BusyOf_NilReceiverMatchesNonNilForBusyInput(t *testing.T) {
	var nilCenter *SignalCenter
	freshCenter := NewSignalCenter()
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

// TestSignalCenter_BusyOf_RepeatedCallsIdempotentNoStateLeak (stateless,
// stronger than a single-call check): alternates busy/idle inputs across
// many calls on the SAME *SignalCenter and asserts every call is correct in
// isolation — a hidden memoization or accumulator bug would surface as
// cross-call contamination that a single before/after StatelessNoSessionMutation
// assertion (per build-report.md's name, presumably one Observe-vs-no-Observe
// check) would not catch. Also re-verifies the sessions map stays untouched
// after sustained BusyOf traffic.
func TestSignalCenter_BusyOf_RepeatedCallsIdempotentNoStateLeak(t *testing.T) {
	sc := NewSignalCenter()
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

// TestSignalCenter_BusyOf_ConcurrentWithObserveAggregateRegisterHandler
// (-race, S5-forward-looking): BusyOf is the newest method on SignalCenter
// and postdates the established ParallelEvaluate mixed-op stress harness
// (signalcenter_parallelevaluate_test.go, cycle 433's s5-parallelevaluate-stress-race,
// "written against the ALREADY-SHIPPED SignalCenter") — that harness never
// exercised BusyOf because it did not exist yet. The cycle-434 goal's own S5
// slice ("concurrency hardening ... under ParallelEvaluate") is still
// upcoming; this pins BusyOf's concurrency safety now, before S5 formally
// starts, using the same mixed-op shape (concurrent Observe producers +
// concurrent RegisterHandler + concurrent Aggregate/Busy/Changed readers)
// plus concurrent BusyOf calls interleaved throughout.
func TestSignalCenter_BusyOf_ConcurrentWithObserveAggregateRegisterHandler(t *testing.T) {
	const numProducers = 12
	const observesPerProducer = 50
	const busyOfIterations = 300

	sc := NewSignalCenter()
	claudeProfile := Profiles["claude"]
	busyRendered := "⏺ thinking\nesc to interrupt\n❯ \n"
	idleRendered := "⏺ done\n❯ \n"

	var wg sync.WaitGroup

	// Producers: distinct session keys, mirrors the established stress shape.
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

	// Concurrent RegisterHandler, overlapping the producers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 4; i++ {
			name := fmt.Sprintf("busyof-handler-%d", i)
			sc.RegisterHandler(name, func() LivenessProbe { return NewDefaultDetector(0) })
		}
	}()

	// Reader: concurrent Aggregate/Busy/Changed on the producers' keys.
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

	// The new method under test: concurrent, alternating busy/idle BusyOf
	// calls on the SAME shared center, asserting correctness on every call
	// (not merely "did not panic" / "did not race").
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

	// Post-condition (white-box, same package): every producer session key
	// recorded exactly once — BusyOf traffic interleaved throughout must not
	// have perturbed the sessions map (it must never touch it at all).
	if got := len(sc.sessions); got != numProducers {
		t.Errorf("after mixed-op stress with BusyOf interleaved: len(sc.sessions) = %d, want %d", got, numProducers)
	}
}

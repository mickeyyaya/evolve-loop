package panestream

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSignalCenter_ObserveAndAggregate(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude-repl-v1"]
	sc.Observe("sess-1", "⏺ thinking\n(4s · ↓ 50 tokens)\n❯ \n", p)
	sc.Observe("sess-1", "⏺ thinking\n(4s · ↓ 100 tokens)\n❯ \n", p)
	state := sc.Aggregate()
	if state == 0 {
		t.Errorf("Aggregate after two Observes: got zero LivenessState (unset), want a valid state")
	}
}

func TestSignalCenter_AggregateIsDeterministic(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude-repl-v1"]
	sc.Observe("sess-1", "⏺ line A\n❯ \n", p)
	sc.Observe("sess-1", "⏺ line A\n⏺ line B\n❯ \n", p)
	s1 := sc.Aggregate()
	s2 := sc.Aggregate()
	if s1 != s2 {
		t.Errorf("Aggregate not deterministic: first call=%v second call=%v", s1, s2)
	}
}

func TestSignalCenter_UnknownProfileFallsToDefault(t *testing.T) {
	sc := NewLivenessCenter()
	unknown := PaneProfile{Name: "unknown-cli-xyz", BoundaryMarker: "$"}
	sc.Observe("sess-x", "some content\n$ \n", unknown)
	sc.Observe("sess-x", "some content\nnew line\n$ \n", unknown)
	state := sc.Aggregate()
	if state == 0 {
		t.Errorf("UnknownProfile: Aggregate returned zero state, expected DefaultDetector fallback to return a valid state")
	}
}

func TestSignalCenter_EmptyCenter_DefinedState(t *testing.T) {
	sc := NewLivenessCenter()
	if got := sc.Aggregate(); got != 0 {
		t.Errorf("Aggregate on an empty center = %v, want the zero LivenessState", got)
	}
}

func TestSignalCenter_ParallelProducerRaceClean(t *testing.T) {
	const numProducers = 8
	sc := NewLivenessCenter()
	claudeProfile := Profiles["claude-repl-v1"]
	sharedKey := "shared-sess"

	var wg sync.WaitGroup
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("sess-%d", id)
			if id%2 == 0 {
				key = sharedKey
			}
			sc.Observe(key, fmt.Sprintf("content-%d\n❯ \n", id), claudeProfile)
		}(i)
	}

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < 8; i++ {
			_ = sc.Aggregate()
		}
	}()

	wg.Wait()
	<-readerDone
	if got := sc.Aggregate(); got == 0 {
		t.Errorf("Aggregate after parallel producers = 0 (unset); want a valid LivenessState")
	}
}

func TestSignalCenter_RegisterHandlerRoutesNewCLI(t *testing.T) {
	sc := NewLivenessCenter()
	factoryCalled := false
	sc.RegisterHandler("fake-cli", func() LivenessProbe {
		factoryCalled = true
		return NewDefaultDetector(0)
	})
	fakeProfile := PaneProfile{Name: "fake-cli", BoundaryMarker: "»"}
	sc.Observe("sess-1", "content\n» \n", fakeProfile)
	if !factoryCalled {
		t.Errorf("RegisterHandler: factory not called when Observe used 'fake-cli' profile (not routed through registry)")
	}
}

func TestSignalCenter_UnregisteredProfileFallsToDefault(t *testing.T) {
	sc := NewLivenessCenter()
	p := PaneProfile{Name: "no-such-cli", BoundaryMarker: "~"}
	sc.Observe("sess-1", "content\n~ \n", p)
	sc.Observe("sess-1", "content\nnew line\n~ \n", p)
	state := sc.Aggregate()
	if state == 0 {
		t.Errorf("UnregisteredProfile: Aggregate returned zero state (expected DefaultDetector fallback to return valid state)")
	}
}

func TestSignalCenter_RegisterEmptyOrDuplicateNoPanic(t *testing.T) {
	sc := NewLivenessCenter()
	emptyCalls, firstCalls, secondCalls := 0, 0, 0
	sc.RegisterHandler("", func() LivenessProbe { emptyCalls++; return NewDefaultDetector(0) })
	sc.RegisterHandler("claude", func() LivenessProbe { firstCalls++; return NewClaudeDetector(0) })
	sc.RegisterHandler("claude", func() LivenessProbe { secondCalls++; return NewClaudeDetector(0) })
	sc.Observe("a", "content\n~ \n", PaneProfile{Name: "", BoundaryMarker: "~"})
	sc.Observe("b", "content\n~ \n", PaneProfile{Name: "claude", BoundaryMarker: "~"})
	if emptyCalls != 0 {
		t.Errorf("empty-name factory called %d times, want 0", emptyCalls)
	}
	if firstCalls != 0 || secondCalls != 1 {
		t.Errorf("duplicate registration calls first=%d second=%d, want 0 and 1", firstCalls, secondCalls)
	}
}

func TestSignalCenter_RegisterConcurrent(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude-repl-v1"]
	var calls atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sc.RegisterHandler(fmt.Sprintf("cli-%d", id), func() LivenessProbe {
				calls.Add(1)
				return NewDefaultDetector(0)
			})
			sc.Observe(fmt.Sprintf("sess-%d", id), fmt.Sprintf("content-%d\n❯ \n", id), p)
		}(i)
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		sc.Observe(fmt.Sprintf("fresh-%d", i), "content\n~ \n", PaneProfile{Name: fmt.Sprintf("cli-%d", i), BoundaryMarker: "~"})
	}
	if got := calls.Load(); got != 8 {
		t.Errorf("registered factories invoked %d times, want 8", got)
	}
}

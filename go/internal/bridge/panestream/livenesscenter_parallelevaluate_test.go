package panestream

import (
	"fmt"
	"sync"
	"testing"
)

// validAggregateStates is every value Aggregate may return: 0 for an empty center plus the five states.
var validAggregateStates = map[LivenessState]bool{
	0:                       true,
	LivenessIdle:            true,
	LivenessBusyButStagnant: true,
	LivenessHung:            true,
	LivenessConverging:      true,
	LivenessExhausted:       true,
}

func TestSignalCenter_ParallelEvaluateStress_MixedOpsRaceClean(t *testing.T) {
	const numProducers = 16
	const observesPerProducer = 100
	const readerIterations = 200

	sc := NewLivenessCenter()
	p := Profiles["claude"]

	var wg sync.WaitGroup

	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("pe-sess-%d", id)
			for j := 0; j < observesPerProducer; j++ {
				content := fmt.Sprintf("⏺ producer-%d tick-%d\n❯ \n", id, j)
				sc.Observe(key, content, p)
			}
		}(i)
	}

	const numHandlers = 4
	for i := 0; i < numHandlers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("pe-handler-%d", id)
			sc.RegisterHandler(name, func() LivenessProbe { return NewDefaultDetector(0) })
		}(i)
	}

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < readerIterations; i++ {
			state := sc.Aggregate()
			if !validAggregateStates[state] {
				t.Errorf("Aggregate() returned invalid state %v under concurrency, want one of {0,Idle,BusyButStagnant,Hung,Converging}", state)
				return
			}
			key := fmt.Sprintf("pe-sess-%d", i%numProducers)
			_ = sc.Busy(key)
			_ = sc.Changed(key)
		}
	}()

	wg.Wait()
	<-readerDone

	// Reading sc.sessions without the lock is safe: wg.Wait and <-readerDone order every write before it.
	if got := len(sc.sessions); got != numProducers {
		t.Errorf("after stress: len(sc.sessions) = %d, want %d (a session key was lost or duplicated under concurrency)", got, numProducers)
	}
}

func TestSignalCenter_ObserveAggregateSameKeyRaceClean(t *testing.T) {
	const iterations = 500

	sc := NewLivenessCenter()
	p := Profiles["claude"]
	key := "shared-key"
	sc.Observe(key, "⏺ priming\n❯ \n", p) // create the key before racing reads against it

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			sc.Observe(key, fmt.Sprintf("⏺ tick-%d\n❯ \n", i), p)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			state := sc.Aggregate()
			if !validAggregateStates[state] {
				t.Errorf("Aggregate() returned invalid state %v during same-key concurrent Observe/Aggregate, want one of {0,Idle,BusyButStagnant,Hung,Converging}", state)
				return
			}
			_ = sc.Busy(key)
			_ = sc.Changed(key)
		}
	}()
	wg.Wait()
}

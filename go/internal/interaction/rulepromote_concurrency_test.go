package interaction_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

func TestPromoteRule_ConcurrentSameID_NoLostWrite(t *testing.T) {
	const iters = 200
	const writers = 16
	regex := "Rate this session before exiting"
	note := "justification: " + strings.Repeat("x", 512)

	for i := 0; i < iters; i++ {
		dir := t.TempDir()
		// The start barrier maximizes overlap, so a shared temp file would collide within the iterations.
		start := make(chan struct{})
		errs := make([]error, writers)
		var wg sync.WaitGroup
		wg.Add(writers)
		for w := 0; w < writers; w++ {
			go func(idx int) {
				defer wg.Done()
				<-start
				_, errs[idx] = interaction.PromoteRule(dir, regex, "1,Enter", note, healthyCorpus)
			}(w)
		}
		close(start)
		wg.Wait()

		for w, err := range errs {
			if err != nil {
				t.Fatalf("iter %d writer %d: PromoteRule failed (lost write under shared temp collision): %v", i, w, err)
			}
		}
		rules := interaction.LoadRules(dir, healthyCorpus)
		if len(rules) != 1 || rules[0].Regex != regex {
			t.Fatalf("iter %d: promoted rule did not round-trip cleanly: %+v", i, rules)
		}
	}
}

func TestEnforceRule_ConcurrentSameID_NoLostWrite(t *testing.T) {
	const iters = 200
	const writers = 16
	regex := "Rate this session before exiting"

	for i := 0; i < iters; i++ {
		dir := t.TempDir()
		id, err := interaction.PromoteRule(dir, regex, "1,Enter", "n", healthyCorpus)
		if err != nil {
			t.Fatalf("iter %d setup PromoteRule: %v", i, err)
		}
		start := make(chan struct{})
		errs := make([]error, writers)
		var wg sync.WaitGroup
		wg.Add(writers)
		for w := 0; w < writers; w++ {
			go func(idx int) {
				defer wg.Done()
				<-start
				errs[idx] = interaction.EnforceRule(dir, id, healthyCorpus)
			}(w)
		}
		close(start)
		wg.Wait()

		for w, err := range errs {
			if err != nil {
				t.Fatalf("iter %d writer %d: EnforceRule failed (lost flip under shared temp collision): %v", i, w, err)
			}
		}
		rules := interaction.LoadRules(dir, healthyCorpus)
		if len(rules) != 1 || rules[0].Stage != interaction.RuleStageEnforce {
			t.Fatalf("iter %d: flip did not converge to enforce: %+v", i, rules)
		}
	}
}

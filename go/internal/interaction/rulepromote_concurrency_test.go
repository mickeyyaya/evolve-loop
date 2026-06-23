package interaction_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
)

// TestPromoteRule_ConcurrentSameID_NoLostWrite is the N14 (ADR-0049) regression
// for the interaction-rule registry — the twin of recovery's PromoteSignature
// fix. Two or more concurrent fleet cycles that promote the SAME novel rule
// resolve to the same content-hash rule-<id>.yaml and, before the fix, to the
// SAME non-unique temp path (path + ".tmp"). Their os.WriteFile/os.Rename calls
// then interleave on one shared temp: whoever renames first moves the inode out
// from under the others, so the losers' rename fails with ENOENT (a lost
// promotion) and a partial temp can be renamed over the target (a torn rule that
// every later boot replays). The atomicwrite SSOT gives each caller a UNIQUE
// temp (os.CreateTemp), so concurrent same-target writers never collide — every
// call succeeds and the final file is a complete, parseable rule. A start
// barrier maximizes overlap so the pre-fix bug trips across the iterations.
func TestPromoteRule_ConcurrentSameID_NoLostWrite(t *testing.T) {
	const iters = 200
	const writers = 16
	regex := "Rate this session before exiting"
	note := "justification: " + strings.Repeat("x", 512)

	for i := 0; i < iters; i++ {
		dir := t.TempDir()
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
		// The durable rule must round-trip: a torn write fails to parse (LoadRules
		// drops it) or carries the wrong pattern.
		rules := interaction.LoadRules(dir, healthyCorpus)
		if len(rules) != 1 || rules[0].Regex != regex {
			t.Fatalf("iter %d: promoted rule did not round-trip cleanly: %+v", i, rules)
		}
	}
}

// TestEnforceRule_ConcurrentSameID_NoLostWrite covers the shadow→enforce flip,
// a read-modify-write that also used the shared path+".tmp". Concurrent flips of
// ONE rule must all succeed and converge to stage enforce, never tear the file.
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

package recovery

import (
	"strings"
	"sync"
	"testing"
)

// TestPromoteSignature_ConcurrentSameSubstr_NoTornWriteOrLostError is the N14
// (ADR-0049) regression. Two or more concurrent fleet cycles that classify the
// SAME novel fatal pane resolve to the SAME content-addressed sig-<hash>.yaml —
// and, before the fix, to the SAME non-unique temp path (path + ".tmp"). Their
// os.WriteFile/os.Rename calls then interleave on one shared temp file: whichever
// renames first moves the inode out from under the others, so the losers' rename
// fails with ENOENT (a lost promotion) and a partially-written temp can be
// renamed over the target (a torn registry entry that poisons the deterministic
// frontier for every later boot).
//
// Under fleet mode the whole-cycle project lock is skipped, so these promotions
// genuinely overlap. The fix routes the write through the atomicwrite SSOT, which
// gives every caller a UNIQUE temp (os.CreateTemp), so concurrent same-target
// writers never collide on the temp file — every call succeeds and the final file
// is always a complete, parseable entry. A start barrier maximizes overlap so the
// pre-fix bug trips across the iterations.
func TestPromoteSignature_ConcurrentSameSubstr_NoTornWriteOrLostError(t *testing.T) {
	const iters = 300
	const writers = 16
	// A padded note widens the write window (multiple write() syscalls), making
	// both the torn-content and the rename-ENOENT races more likely to surface.
	substr := "novel fatal pane requiring promotion 0xCAFE — concurrent classification"
	note := "justification: " + strings.Repeat("x", 1024)

	for i := 0; i < iters; i++ {
		dir := t.TempDir()
		sig := FatalSignature{Substr: substr, Cause: CauseModelInvalid, Note: note}

		start := make(chan struct{})
		errs := make([]error, writers)
		var wg sync.WaitGroup
		wg.Add(writers)
		for w := 0; w < writers; w++ {
			go func(idx int) {
				defer wg.Done()
				<-start
				_, errs[idx] = PromoteSignature(dir, sig)
			}(w)
		}
		close(start)
		wg.Wait()

		for w, err := range errs {
			if err != nil {
				t.Fatalf("iter %d writer %d: PromoteSignature failed (lost promotion under temp-file collision): %v", i, w, err)
			}
		}

		// The durable entry must round-trip: a torn write would fail to parse
		// (Detect would miss) or carry the wrong substr.
		cause, matched, ok := SeedDetectorWithPromotions(dir).Detect("⏺ " + substr + " ⏺")
		if !ok {
			t.Fatalf("iter %d: promoted signature did not round-trip — torn or missing sig-*.yaml", i)
		}
		if cause != CauseModelInvalid || matched != substr {
			t.Fatalf("iter %d: torn registry entry: cause=%q matched=%q want %q/%q", i, cause, matched, CauseModelInvalid, substr)
		}
	}
}

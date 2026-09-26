package recovery

import (
	"strings"
	"sync"
	"testing"
)

func TestPromoteSignature_ConcurrentSameSubstr_NoTornWriteOrLostError(t *testing.T) {
	const iters = 300
	const writers = 16
	// A padded note spans several write() calls, widening the race window.
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

		cause, matched, ok := SeedDetectorWithPromotions(dir).Detect("⏺ " + substr + " ⏺")
		if !ok {
			t.Fatalf("iter %d: promoted signature did not round-trip — torn or missing sig-*.yaml", i)
		}
		if cause != CauseModelInvalid || matched != substr {
			t.Fatalf("iter %d: torn registry entry: cause=%q matched=%q want %q/%q", i, cause, matched, CauseModelInvalid, substr)
		}
	}
}

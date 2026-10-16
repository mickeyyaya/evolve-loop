package interaction

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestAppendLedgerLine_ConcurrentAppendsKeepEveryLineIntact(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	const writers = 32
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			appendLedgerLine(ws, Outcome{Event: Event{Phase: "build", DecisionID: fmt.Sprintf("d%02d", i)}, Result: "answered"})
		}(i)
	}
	wg.Wait()

	f, err := os.Open(ledgerPath(ws, "build"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var out Outcome
		if err := json.Unmarshal(sc.Bytes(), &out); err != nil {
			t.Fatalf("torn ledger line %q: %v", sc.Text(), err)
		}
		seen[out.DecisionID] = true
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != writers {
		t.Fatalf("ledger kept %d of %d appended outcomes", len(seen), writers)
	}
}

package marketplacepoll

import (
	"bytes"
	"testing"
	"time"
)

func TestResult_NamedType(t *testing.T) {
	m := makeMarketplace(t, "1.2.3")
	now, _ := stubClock(time.Now())
	var buf bytes.Buffer

	got, err := Run(Options{
		Target:         "1.2.3",
		MarketplaceDir: m,
		MaxWait:        5 * time.Second,
		PollInterval:   1 * time.Second,
		Now:            now,
		Sleep:          func(time.Duration) {},
		Pull:           func(string) error { return nil },
		ReleaseSh:      func(_, _ string) error { return nil },
		Stderr:         &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := Result{
		Converged:      true,
		Attempts:       1,
		Elapsed:        0,
		FinalVersion:   "1.2.3",
		ReleaseShRunOK: true,
	}
	if got != want {
		t.Errorf("Result = %+v, want %+v", got, want)
	}
}

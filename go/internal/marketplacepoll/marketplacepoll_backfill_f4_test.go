package marketplacepoll

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPollForConvergence_AnUnreadableMarketplaceIsNamedAsSuchWhileWaitingAndAtTimeout(t *testing.T) {
	var logs []string
	logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }
	clock := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	sleeps := 0
	sleep := func(d time.Duration) { sleeps++; clock = clock.Add(d) }
	opts := Options{MarketplaceDir: t.TempDir(), Target: "9.9.9", MaxWait: 30 * time.Second, PollInterval: 20 * time.Second}

	res, err := pollForConvergence(opts, now, sleep, func(string) error { return errors.New("pull failed") }, logf)

	if !errors.Is(err, ErrTimeout) || res.Converged || res.FinalVersion != "" || res.Attempts != 3 || sleeps != 2 {
		t.Fatalf("res=%+v err=%v sleeps=%d", res, err, sleeps)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "attempt 1: marketplace at v<empty>; waiting 20s") {
		t.Errorf("a wait line must call the unreadable version <empty>:\n%s", joined)
	}
	if !strings.Contains(joined, "TIMEOUT: marketplace still at v<unreadable> after 30s; expected v9.9.9") {
		t.Errorf("the timeout line must call the unreadable version <unreadable>:\n%s", joined)
	}
}

package ciwatch

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestEarlierAttemptFailures_ReadsEachEarlierTryOfARun(t *testing.T) {
	calls := fakeGH(t, func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "/artifacts"):
			return []byte(`{"total_count":0,"artifacts":[]}`), nil
		case strings.Contains(joined, "--json"):
			return []byte(`{"attempt":1,"conclusion":"failure","status":"completed","databaseId":7}`), nil
		case strings.Contains(joined, "--log-failed"):
			return []byte("go-test\tRun tests\t2026-11-27T00:00:00Z --- FAIL: TestA (0.00s)\n"), nil
		}
		return []byte("[]"), nil
	})
	got, err := EarlierAttemptFailures(context.Background(), ".", 7, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(*calls, func(a []string) bool { return slices.Contains(a, "--attempt") && slices.Contains(a, "1") }) {
		t.Fatalf("attempt 1 was not read: %v", *calls)
	}
	if len(got) == 0 {
		t.Fatalf("no failing test read: %v", got)
	}
}

func TestEarlierAttemptFailures_AFailedReadIsReturned(t *testing.T) {
	fakeGH(t, func([]string) ([]byte, error) { return nil, errors.New("HTTP 502") })
	if _, err := EarlierAttemptFailures(context.Background(), ".", 7, 3); err == nil {
		t.Fatal("a failed gh read must return an error")
	}
}

func TestEarlierAttemptFailures_NoEarlierTryReadsNothing(t *testing.T) {
	calls := fakeGH(t, func([]string) ([]byte, error) { return nil, errors.New("must not run") })
	got, err := EarlierAttemptFailures(context.Background(), ".", 7, 1)
	if err != nil || len(got) != 0 || len(*calls) != 0 {
		t.Fatalf("got %v %v calls %v", got, err, *calls)
	}
}

package bridge

import (
	"errors"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func stubRandRead(t *testing.T, fn func([]byte) (int, error)) {
	t.Helper()
	orig := randRead
	randRead = fn
	t.Cleanup(func() { randRead = orig })
}

func TestRandomStamp_HexEncodesSixEntropyBytes(t *testing.T) {
	stubRandRead(t, func(b []byte) (int, error) {
		copy(b, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0xff, 0xee})
		return len(b), nil
	})

	if got := randomStamp(); got != "0102030405ff" {
		t.Fatalf("randomStamp() = %q, want the six entropy bytes as hex %q", got, "0102030405ff")
	}
}

func TestRandomStamp_RealEntropyNeverRepeatsAcrossCalls(t *testing.T) {
	seen := map[string]bool{}
	hex12 := regexp.MustCompile(`^[0-9a-f]{12}$`)
	for range 64 {
		s := randomStamp()
		if !hex12.MatchString(s) || seen[s] {
			t.Fatalf("randomStamp() = %q (seen before: %v), want a fresh 12-hex stamp", s, seen[s])
		}
		seen[s] = true
	}
}

func TestRandomStamp_EntropyFailureFallsBackToTheBase36Clock(t *testing.T) {
	stubRandRead(t, func([]byte) (int, error) { return 0, errors.New("no entropy") })
	before := time.Now().UnixNano()

	got := randomStamp()

	after := time.Now().UnixNano()
	n, err := strconv.ParseInt(got, 36, 64)
	if err != nil || n < before || n > after {
		t.Fatalf("randomStamp() = %q (%d, %v), want a base36 UnixNano within [%d, %d]", got, n, err, before, after)
	}
}

func TestMintDispatchID_StillParsesWhenEntropyFails(t *testing.T) {
	stubRandRead(t, func([]byte) (int, error) { return 0, errors.New("no entropy") })

	first := mintDispatchID(&Config{RunID: "R", Cycle: 7, Agent: "audit"}, 99)
	second := mintDispatchID(&Config{RunID: "R", Cycle: 7, Agent: "audit"}, 99)

	got, ok := proctree.ParseDispatchID(first)
	if !ok || got.Owner != 99 || got.Cycle != 7 || got.Agent != "audit" {
		t.Fatalf("ParseDispatchID(%q) = %+v, %v; want the launch fields", first, got, ok)
	}
	if first == second {
		t.Fatalf("two launches minted the same id %q while entropy failed", first)
	}
}

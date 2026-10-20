package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

const ledgerZeroSeed = subagentrun.LedgerZeroSeed

func readChainLink(ledgerPath string) (prevHash string, entrySeq int, err error) {
	return subagentrun.ChainLink(ledgerPath)
}

func sha256Hex(s string) string { return subagentrun.SHA256Hex(s) }

func TestWriteFanoutLedgerEntry_ThroughALinkedLedgerWritesTheTipBesideTheRealLedger(t *testing.T) {
	t.Parallel()
	host := t.TempDir()
	lane := t.TempDir()
	real := filepath.Join(host, "ledger.jsonl")
	if err := os.WriteFile(real, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(lane, "ledger.jsonl")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := WriteFanoutLedgerEntry(link, FanoutLedgerEntry{Cycle: 1, Agent: "scout", ChallengeToken: "t", WorkerNames: []string{"w"}, WorkerCount: 1}, nil); err != nil {
		t.Fatalf("WriteFanoutLedgerEntry: %v", err)
	}

	if _, err := os.Stat(filepath.Join(host, "ledger.tip")); err != nil {
		t.Fatalf("the tip must advance beside the real ledger: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lane, "ledger.tip")); err == nil {
		t.Fatal("a tip beside the link forks the chain: the host tip stays stale")
	}
	if _, err := os.Stat(filepath.Join(host, "ledger.lock")); err != nil {
		t.Fatalf("the fan-out append must serialize on the real ledger's lock: %v", err)
	}
}

func TestWriteFanoutLedgerEntry_AWorkersEncodeFailureWritesNoLine(t *testing.T) {
	orig := marshalWorkers
	marshalWorkers = func(any) ([]byte, error) { return nil, errors.New("encode refused") }
	t.Cleanup(func() { marshalWorkers = orig })
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")

	err := WriteFanoutLedgerEntry(ledger, FanoutLedgerEntry{Cycle: 1, Agent: "scout", WorkerNames: []string{"w"}}, nil)

	if err == nil || !strings.Contains(err.Error(), "marshal workers") {
		t.Fatalf("err = %v, want the workers encode failure", err)
	}
	if _, statErr := os.Stat(ledger); statErr == nil {
		t.Fatal("no ledger line may be written when the workers list cannot be encoded")
	}
}

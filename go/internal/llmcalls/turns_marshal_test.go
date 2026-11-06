package llmcalls

import (
	"errors"
	"os"
	"testing"
)

func TestAppendTurns_MarshalFailureReturnsAnErrorAndWritesNothing(t *testing.T) {
	ws := t.TempDir()
	failure := errors.New("marshal refused")
	original := marshalTurn
	marshalTurn = func(any) ([]byte, error) { return nil, failure }
	defer func() { marshalTurn = original }()

	err := AppendTurns(ws, "call-m", numberedTurns(2))

	if !errors.Is(err, failure) {
		t.Fatalf("AppendTurns error = %v, want it to wrap %v", err, failure)
	}
	if _, statErr := os.Stat(TurnsPath(ws)); !os.IsNotExist(statErr) {
		t.Errorf("stat %s: %v, want the sidecar absent after a marshal failure", TurnsPath(ws), statErr)
	}
}

package cyclecost

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestSummarizeCycle_StatFailureWrapsItsCause(t *testing.T) {
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := SummarizeCycle(filepath.Join(file, "cycle-1"), 1)
	if errors.Is(err, ErrNoWorkspace) || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("err = %v, want a wrapped ENOTDIR", err)
	}
}

func TestSummarizeCycle_GlobFailureWrapsItsCause(t *testing.T) {
	sentinel := errors.New("glob sentinel")
	prev := globFn
	defer func() { globFn = prev }()
	globFn = func(string) ([]string, error) { return nil, sentinel }
	if _, err := SummarizeCycle(t.TempDir(), 1); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the glob error wrapped", err)
	}
}

func TestSummarizeCycle_SidecarOnlyWorkspaceIsSummarized(t *testing.T) {
	ws := t.TempDir()
	writeUsageSidecar(t, ws, "build", 0.5, 1, 2, 3, 4)
	s, err := SummarizeCycle(ws, 7)
	if err != nil {
		t.Fatalf("err = %v, want a summary from the sidecar alone", err)
	}
	if len(s.Phases) != 1 || s.Phases[0].Phase != "build" || s.Total.CostUSD != 0.5 || s.Cycle != 7 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestParseEventsLog_NestedResultKindIsNotTheResult(t *testing.T) {
	ws := t.TempDir()
	writeLog(t, ws, "build-events.ndjson", resultEnvelope(1.5, 1, 2, 3, 4)+"\n"+`{"kind":"assistant_text","data":{"inner":{"kind":"result"}}}`+"\n")
	pc, ok := parseEventsLog(filepath.Join(ws, "build-events.ndjson"))
	if !ok || pc.CostUSD != 1.5 || pc.InputTokens != 1 {
		t.Fatalf("pc = %+v ok = %v, want the real result envelope", pc, ok)
	}
}

func TestParseEventsLog_TypeMismatchedResultDoesNotReplaceTheLastGoodOne(t *testing.T) {
	ws := t.TempDir()
	writeLog(t, ws, "build-events.ndjson", resultEnvelope(1.5, 1, 2, 3, 4)+"\n"+`{"kind":"result","data":{"cost_usd":"free"}}`+"\n")
	pc, ok := parseEventsLog(filepath.Join(ws, "build-events.ndjson"))
	if !ok || pc.CostUSD != 1.5 || pc.OutputTokens != 2 {
		t.Fatalf("pc = %+v ok = %v, want the last well-typed result envelope", pc, ok)
	}
}

func TestParseEventsLog_ScannerErrorDiscardsAnEarlierResult(t *testing.T) {
	prev := maxScannerBufBytes
	defer func() { maxScannerBufBytes = prev }()
	maxScannerBufBytes = 1024
	ws := t.TempDir()
	writeLog(t, ws, "build-events.ndjson", resultEnvelope(1.5, 1, 2, 3, 4)+"\n"+strings.Repeat("x", 2048)+"\n")
	if pc, ok := parseEventsLog(filepath.Join(ws, "build-events.ndjson")); ok {
		t.Fatalf("pc = %+v ok = true, want ok=false on a scanner error", pc)
	}
}

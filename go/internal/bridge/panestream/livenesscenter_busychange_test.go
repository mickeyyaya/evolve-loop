package panestream

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestSignalCenter_BusyProjection(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]

	busyPane := "⏺ working\n⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n❯ \n"
	sc.Observe("sess-1", busyPane, p)
	if !sc.Busy("sess-1") {
		t.Fatal("Busy(sess-1) = false after Observe with a busy-affordance pane, want true")
	}
	if sc.Busy("sess-1") != PaneBusy(busyPane, p) {
		t.Fatal("Busy projection disagrees with standalone PaneBusy for the same pane (must be the folded function, not a reimplementation)")
	}

	idlePane := "⏺ done\n❯ \n"
	sc.Observe("sess-1", idlePane, p)
	if sc.Busy("sess-1") {
		t.Fatal("Busy(sess-1) = true after Observe with a quiet pane, want false")
	}
	if sc.Busy("sess-1") != PaneBusy(idlePane, p) {
		t.Fatal("Busy projection disagrees with standalone PaneBusy for the idle pane")
	}
}

func TestSignalCenter_ChangedProjection(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]

	sc.Observe("sess-1", "❯ starting task\n⏺ line one\n❯ \n", p)
	sc.Observe("sess-1", "❯ starting task\n⏺ line one\n⏺ line two (new content)\n❯ \n", p)
	if !sc.Changed("sess-1") {
		t.Fatal("Changed(sess-1) = false, want true after a content-bearing second Observe")
	}

	sc.Observe("sess-1", "❯ starting task\n⏺ line one\n⏺ line two (new content)\n❯ \n", p) // identical content again
	if sc.Changed("sess-1") {
		t.Fatal("Changed(sess-1) = true after an identical-content Observe, want false")
	}
}

func TestSignalCenter_ChangedIgnoresChrome(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]

	sc.Observe("sess-1", "⏺ working on it\n✻ Schlepping… (4s · ↑ 1.2k tokens)\n❯ \n", p)
	sc.Observe("sess-1", "⏺ working on it\n✻ Schlepping… (9s · ↑ 4.7k tokens)\n❯ \n", p)
	if sc.Changed("sess-1") {
		t.Fatal("Changed(sess-1) = true for a chrome-only delta (ticking spinner/token counter), want false")
	}
}

func TestSignalCenter_UnknownKeyIsQuiet(t *testing.T) {
	sc := NewLivenessCenter()
	if sc.Busy("") {
		t.Error(`Busy("") on an empty center = true, want false`)
	}
	if sc.Busy("never-observed") {
		t.Error(`Busy("never-observed") on an empty center = true, want false`)
	}
	if sc.Changed("") {
		t.Error(`Changed("") on an empty center = true, want false`)
	}
	if sc.Changed("never-observed") {
		t.Error(`Changed("never-observed") on an empty center = true, want false`)
	}
}

func TestSignalCenter_ProjectionsConcurrent(t *testing.T) {
	const numProducers = 8
	sc := NewLivenessCenter()
	p := Profiles["claude"]
	sharedKey := "shared-sess"

	var wg sync.WaitGroup
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("sess-%d", id)
			if id%2 == 0 {
				key = sharedKey
			}
			sc.Observe(key, fmt.Sprintf("⏺ content-%d\n❯ \n", id), p)
		}(i)
	}

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < 8; i++ {
			_ = sc.Busy(sharedKey)
			_ = sc.Changed(sharedKey)
			_ = sc.Aggregate()
		}
	}()

	wg.Wait()
	<-readerDone
}

var funcDefRE = regexp.MustCompile(`(?m)^func\s+(cleanPane|PaneHasSubstantiveChange)\s*\(`)

func countChromeParseFuncDefs(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		n += len(funcDefRE.FindAllString(string(b), -1))
	}
	return n
}

func TestSignalCenter_SingleDefinitionAntiDuplication(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	panestreamDir := filepath.Dir(thisFile)
	bridgeDir := filepath.Dir(panestreamDir)

	inPanestream := countChromeParseFuncDefs(t, panestreamDir)
	inBridge := countChromeParseFuncDefs(t, bridgeDir)

	if inPanestream != 2 {
		t.Errorf("panestream package: want exactly 2 func defs (cleanPane + PaneHasSubstantiveChange) after relocation, got %d", inPanestream)
	}
	if inBridge != 0 {
		t.Errorf("bridge package (stopreview.go): want 0 remaining cleanPane/PaneHasSubstantiveChange defs after relocation, got %d (duplication — single-source-with-projection violated)", inBridge)
	}
}

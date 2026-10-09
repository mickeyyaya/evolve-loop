package structure

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var pollFreeDirs = []string{
	filepath.Join("internal", "events"),
}

var pollTimers = []string{"Sleep", "Tick", "NewTicker", "After", "NewTimer"}

func pollTimerUses(root string, dirs []string) ([]string, []string, error) {
	files, err := SourceFiles(root, dirs...)
	if err != nil {
		return nil, nil, err
	}
	var uses []string
	for _, rel := range files {
		found, err := SelectorUses(root, rel, "time", pollTimers...)
		if err != nil {
			return nil, nil, err
		}
		uses = append(uses, found...)
	}
	return files, uses, nil
}

func TestNoPollTimerInTheEventChannelSources(t *testing.T) {
	t.Parallel()
	files, uses, err := pollTimerUses(filepath.Join("..", ".."), pollFreeDirs)
	if err != nil {
		t.Fatalf("pollTimerUses = %v", err)
	}
	if !slices.Contains(files, filepath.Join("internal", "events", "wake", "wake.go")) {
		t.Fatalf("scanned %q, want the events tree with internal/events/wake/wake.go", files)
	}
	if len(uses) > 0 {
		t.Errorf("%d poll timer(s) in %v; a wait arms a kernel watch and a one-shot deadline:\n%s", len(uses), pollFreeDirs, strings.Join(uses, "\n"))
	}
}

func TestPollTimerUses_FindsEachBannedTimeMember(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/events/x/x.go": "package x\nimport \"time\"\nfunc f() {\n\ttime.Sleep(1)\n\t<-time.Tick(1)\n\t_ = time.NewTicker(1)\n\t<-time.After(1)\n\t_ = time.NewTimer(1)\n\t_ = time.AfterFunc\n}\n",
	})

	_, uses, err := pollTimerUses(root, []string{filepath.Join("internal", "events")})

	want := []string{"Sleep", "Tick", "NewTicker", "After", "NewTimer"}
	if err != nil || len(uses) != len(want) {
		t.Errorf("pollTimerUses = %q, %v, want one use for each of %v", uses, err, want)
	}
}

func TestPollTimerUses_AnAbsentDirectoryOrABadFileIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"internal/events/bad.go": "package"})

	_, _, absent := pollTimerUses(root, []string{"internal/gone"})
	_, _, bad := pollTimerUses(root, []string{"internal/events"})

	if absent == nil || bad == nil {
		t.Errorf("pollTimerUses = %v, %v, want an error for an absent directory and for a bad file", absent, bad)
	}
}

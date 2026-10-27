package selfheal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func LoadSignals(channelsRoot string, names []string, since time.Time) ([]signalcenter.Event, error) {
	var out []signalcenter.Event
	for _, name := range names {
		log, err := channel.New(channelsRoot, name, channel.Config{})
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(log.Dir()); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		batch, err := log.Read(0)
		if err != nil {
			return nil, err
		}
		for _, r := range batch.Records {
			if r.Signal != nil && !signalTime(*r.Signal).Before(since) {
				out = append(out, *r.Signal)
			}
		}
	}
	return out, nil
}

func signalTime(e signalcenter.Event) time.Time {
	at, _ := time.Parse(time.RFC3339Nano, e.TS)
	return at
}

func LoadShipped(journalPath string) (map[int]bool, error) {
	raw, err := os.ReadFile(journalPath)
	if errors.Is(err, fs.ErrNotExist) {
		return map[int]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ship journal: %w", err)
	}
	shipped := map[int]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for line := 1; scanner.Scan(); line++ {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var entry struct {
			Cycle int `json:"cycle"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("ship journal %s line %d: %w", journalPath, line, err)
		}
		if entry.Cycle > 0 {
			shipped[entry.Cycle] = true
		}
	}
	return shipped, nil
}

var cycleDir = regexp.MustCompile(`^cycle-(\d+)$`)

func LoadCycles(runsDir string, limit int, shipped map[int]bool) ([]Cycle, []string, error) {
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("runs: %w", err)
	}
	var ids []int
	for _, e := range entries {
		if m := cycleDir.FindStringSubmatch(e.Name()); m != nil && e.IsDir() {
			id, _ := strconv.Atoi(m[1])
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	ids = ids[len(ids)-min(len(ids), max(limit, 0)):]
	var cycles []Cycle
	var warnings []string
	for _, id := range ids {
		c, ok, err := loadCycle(filepath.Join(runsDir, "cycle-"+strconv.Itoa(id)), id)
		if err != nil {
			warnings = append(warnings, err.Error())
		}
		if ok {
			c.Shipped = shipped[id]
			cycles = append(cycles, c)
		}
	}
	return cycles, warnings, nil
}

type runFile struct {
	StartedAt           time.Time `json:"started_at"`
	AuditRepairAttempts int       `json:"audit_repair_attempts"`
}

type triageFile struct {
	TopN []struct {
		ID string `json:"id"`
	} `json:"top_n"`
}

type failureDecisionFile struct {
	Level string `json:"level"`
}

type defectFile struct {
	Entries []struct {
		Text   string `json:"text"`
		Status string `json:"status"`
	} `json:"entries"`
}

func loadCycle(dir string, id int) (Cycle, bool, error) {
	var run runFile
	if found, err := readOptionalJSON(filepath.Join(dir, "run.json"), &run); !found || err != nil {
		return Cycle{}, false, err
	}
	c := Cycle{ID: id, Started: run.StartedAt, RepairAttempts: run.AuditRepairAttempts}
	var triage triageFile
	var defects defectFile
	var decision failureDecisionFile
	err := errors.Join(
		ignoreFound(readOptionalJSON(filepath.Join(dir, "triage-decision.json"), &triage)),
		ignoreFound(readOptionalJSON(filepath.Join(dir, "defect-ledger.json"), &defects)),
		ignoreFound(readOptionalJSON(filepath.Join(dir, "failure-decision.json"), &decision)),
	)
	c.FailureLevel = decision.Level
	if len(triage.TopN) > 0 {
		c.Item = triage.TopN[0].ID
	}
	for _, e := range defects.Entries {
		if e.Status == "OPEN" {
			c.ProtectedFindings = appendNew(c.ProtectedFindings, protectedIn(e.Text)...)
		}
	}
	return c, true, err
}

func readOptionalJSON(path string, v any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err == nil {
		err = json.Unmarshal(raw, v)
	}
	if err != nil {
		return true, fmt.Errorf("%s: %w", path, err)
	}
	return true, nil
}

func ignoreFound(_ bool, err error) error { return err }

var protectedWord = regexp.MustCompile(`(?i)(?:^|[^a-z])protected\b`)

func protectedIn(text string) []string {
	if paths := guards.ProtectedPathsIn(text); len(paths) > 0 {
		return paths
	}
	if paths := guards.PathsIn(text); protectedWord.MatchString(text) && len(paths) > 0 {
		return paths[:1]
	}
	return nil
}

func LoadUsage(evolveDir string, declared func(family string) bool) ([]UsageReading, error) {
	all, err := usageprobe.LoadObservations(evolveDir)
	if err != nil {
		return nil, err
	}
	out := make([]UsageReading, 0, len(all))
	for family, obs := range all {
		out = append(out, UsageReading{Family: family, Declared: declared(family), ObservedAt: obs.ObservedAt, Windows: len(obs.Windows), Error: obs.Error})
	}
	slices.SortFunc(out, func(a, b UsageReading) int { return strings.Compare(a.Family, b.Family) })
	return out, nil
}

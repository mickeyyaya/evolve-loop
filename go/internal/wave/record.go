package wave

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const wavesDir = "waves"

var (
	ErrNumberTaken = errors.New("wave number is not more than the last recorded wave")
	recordFile     = regexp.MustCompile(`^wave-(\d+)\.json$`)
)

type Record struct {
	Number     int        `json:"number"`
	RunID      string     `json:"run_id"`
	GoalHash   string     `json:"goal_hash"`
	GoalPath   string     `json:"goal_path"`
	GoalBytes  int        `json:"goal_bytes"`
	MainSHA    string     `json:"main_sha,omitempty"`
	CycleFloor int        `json:"cycle_floor"`
	PID        int        `json:"pid,omitempty"`
	LogPath    string     `json:"log_path,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	Outcome    string     `json:"outcome,omitempty"`
}

type Store struct {
	dir string
}

func NewStore(evolveDir string) Store { return Store{dir: filepath.Join(evolveDir, wavesDir)} }

func (s Store) recordPath(number int) string {
	return filepath.Join(s.dir, fmt.Sprintf("wave-%d.json", number))
}

func (s Store) Records() ([]Record, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list wave records: %w", err)
	}
	var out []Record
	for _, e := range entries {
		m := recordFile.FindStringSubmatch(e.Name())
		if m == nil || e.IsDir() {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		r, err := s.readRecord(n)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func (s Store) readRecord(number int) (Record, error) {
	b, err := os.ReadFile(s.recordPath(number))
	if err != nil {
		return Record{}, fmt.Errorf("read wave record %d: %w", number, err)
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return Record{}, fmt.Errorf("parse wave record %s: %w", s.recordPath(number), err)
	}
	return r, nil
}

func (s Store) Save(r Record) error {
	if err := atomicwrite.JSON(s.recordPath(r.Number), r); err != nil {
		return fmt.Errorf("write wave record %d: %w", r.Number, err)
	}
	return nil
}

func NextNumber(records []Record, requested int) (int, error) {
	last := 0
	if len(records) > 0 {
		last = records[len(records)-1].Number
	}
	switch {
	case requested > last:
		return requested, nil
	case requested > 0:
		return 0, fmt.Errorf("%w: requested %d, last %d", ErrNumberTaken, requested, last)
	default:
		return last + 1, nil
	}
}

func (s Store) WriteGoal(number int, text string) (string, error) {
	path := filepath.Join(s.dir, fmt.Sprintf("wave-%d-goal.md", number))
	if err := atomicwrite.Bytes(path, []byte(text)); err != nil {
		return "", fmt.Errorf("write wave %d goal: %w", number, err)
	}
	return path, nil
}

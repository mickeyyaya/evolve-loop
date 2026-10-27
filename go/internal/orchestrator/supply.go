package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

const (
	ClassAdmissible       = "admissible"
	ClassWaitingCell      = "waiting_cell"
	ClassWaitingDeps      = "waiting_deps"
	ClassConsoleRoute     = "console_route"
	ClassConsoleKind      = "console_kind"
	ClassConsoleProtected = "console_protected"

	SupplyFile = "supply.json"

	supplyKeep     = 12
	consoleShown   = 8
	supplyLockWait = 10 * time.Second
)

var errCensusClock = errors.New("orchestrator: the census needs a clock")

func Classes() []string {
	return []string{ClassAdmissible, ClassWaitingCell, ClassWaitingDeps, ClassConsoleRoute, ClassConsoleKind, ClassConsoleProtected}
}

type Refusal struct {
	ID     string `json:"id"`
	Class  string `json:"class"`
	Reason string `json:"reason"`
}

type Proposal struct {
	Item    string `json:"item"`
	Console string `json:"console"`
	Lane    string `json:"lane"`
}

type Proposer func(candidates []inboxbatch.Item, admissible, window int) []Proposal

type Supply struct {
	At        time.Time      `json:"at"`
	Window    int            `json:"window"`
	Counts    map[string]int `json:"counts"`
	Refusals  []Refusal      `json:"refusals,omitempty"`
	Proposals []Proposal     `json:"proposals,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
}

type Census struct {
	Inbox      inboxmover.Options
	Rank       inboxrank.Inputs
	Protected  func(path string) bool
	Graph      admission.Graph
	Text       func(id string) string
	InFlight   []admission.Holding
	Optimistic bool
	Window     int
	Propose    Proposer
	Companions config.Stage
	Path       string
	Publish    Publish
	Now        func() time.Time
}

func (c Census) Count() (Supply, error) {
	if c.Now == nil {
		return Supply{}, errCensusClock
	}
	scan, err := inboxbatch.ScanDir(c.Inbox.InboxDir)
	if err != nil {
		return Supply{}, fmt.Errorf("orchestrator: census: %w", err)
	}
	menu := inboxmover.RankLaneMenu(c.Inbox, scan.Items, c.Protected, c.Rank, c.Companions)
	s := Supply{At: c.Now().UTC(), Window: c.Window, Counts: map[string]int{}}
	for _, class := range Classes() {
		s.Counts[class] = 0
	}
	for _, w := range scan.Warnings {
		s.Warnings = append(s.Warnings, w.String())
	}
	for _, would := range menu.WouldDefer {
		s.Warnings = append(s.Warnings, "would defer "+would)
	}
	protected := s.refuse(menu.Console, menu.ConsoleReasons, consoleClass)
	s.refuse(menu.Waiting, menu.WaitingReasons, func(string) string { return ClassWaitingDeps })
	c.admit(&s, menu.Ranked)
	if admissible := s.Counts[ClassAdmissible]; c.Propose != nil && admissible < 2*c.Window {
		s.Proposals = c.Propose(protected, admissible, c.Window)
	}
	return s, nil
}

func (c Census) Take() (Supply, error) {
	s, err := c.Count()
	if err != nil {
		return s, err
	}
	recordErr := AppendSupply(c.Path, s)
	fields := s.countFields()
	console := s.consoleSummary()
	if console != "" {
		fields["console"] = console
	}
	c.Publish.emit("Census.Take", CodeSupply, signalcenter.SeverityInfo,
		fmt.Sprintf("supply: %d admissible of %d item(s), window %d", s.Counts[ClassAdmissible], s.Total(), s.Window), fields)
	if s.Counts[ClassAdmissible] < s.Window {
		c.Publish.emit("Census.Take", CodeSupplyShort, signalcenter.SeverityInfo,
			fmt.Sprintf("supply is short: %d admissible, window %d", s.Counts[ClassAdmissible], s.Window), s.countFields())
	}
	return s, recordErr
}

func (c Census) admit(s *Supply, ranked []inboxrank.Ranked) {
	todos := make([]fleet.Todo, len(ranked))
	for i, r := range ranked {
		todos[i] = fleet.Todo{ID: r.Item.ID, Files: r.Item.DeclaredPaths()}
	}
	if c.Graph == nil {
		s.Counts[ClassAdmissible] = len(todos)
		if len(todos) > 0 {
			s.Warnings = append(s.Warnings, "the module graph is not loaded, so every ready item counts as admissible")
		}
		return
	}
	admitted, waiting := admission.Admitter{Graph: c.Graph, Text: c.Text, Optimistic: c.Optimistic}.Admit(c.InFlight, todos, len(todos))
	s.Counts[ClassAdmissible] = len(admitted)
	for _, d := range waiting {
		s.Counts[ClassWaitingCell]++
		s.Refusals = append(s.Refusals, Refusal{ID: d.ID, Class: ClassWaitingCell, Reason: "cells " + strings.Join(d.Cells, ",") + " held by " + d.BlockedBy})
	}
}

func (s *Supply) refuse(items []inboxbatch.Item, reasons []string, classOf func(reason string) string) []inboxbatch.Item {
	var protected []inboxbatch.Item
	for i, it := range items {
		reason := strings.TrimPrefix(reasons[i], it.ID+": ")
		class := classOf(reason)
		s.Counts[class]++
		s.Refusals = append(s.Refusals, Refusal{ID: it.ID, Class: class, Reason: reason})
		if class == ClassConsoleProtected {
			protected = append(protected, it)
		}
	}
	return protected
}

func (s Supply) Total() int {
	total := 0
	for _, n := range s.Counts {
		total += n
	}
	return total
}

func (s Supply) Measure() wavemeasure.Supply {
	c := s.Counts
	return wavemeasure.Supply{
		Admissible: c[ClassAdmissible], WaitingCell: c[ClassWaitingCell], WaitingDeps: c[ClassWaitingDeps],
		ConsoleRoute: c[ClassConsoleRoute], ConsoleKind: c[ClassConsoleKind], ConsoleProtected: c[ClassConsoleProtected],
	}
}

func (s Supply) countFields() map[string]string {
	fields := map[string]string{"window": strconv.Itoa(s.Window)}
	for class, n := range s.Counts {
		fields[class] = strconv.Itoa(n)
	}
	return fields
}

func (s Supply) consoleSummary() string {
	var shown []string
	more := 0
	for _, r := range s.Refusals {
		switch {
		case !strings.HasPrefix(r.Class, "console_"):
		case len(shown) < consoleShown:
			shown = append(shown, r.ID+": "+r.Reason)
		default:
			more++
		}
	}
	line := strings.Join(shown, "; ")
	if more > 0 {
		line += " (+" + strconv.Itoa(more) + " more)"
	}
	return line
}

func consoleClass(reason string) string {
	switch {
	case strings.HasPrefix(reason, "route:"):
		return ClassConsoleRoute
	case strings.HasPrefix(reason, "kind:"):
		return ClassConsoleKind
	}
	return ClassConsoleProtected
}

type supplyStore struct {
	lock  func(path string, wait time.Duration, onSettled func()) (release func(), err error)
	write func(path string, v any) error
	wait  time.Duration
}

var recordStore = supplyStore{lock: flock.LockWithin, write: atomicwrite.JSON, wait: supplyLockWait}

func AppendSupply(path string, s Supply) error { return recordStore.append(path, s) }

func (st supplyStore) append(path string, s Supply) error {
	release, err := st.lock(path+".lock", st.wait, func() {})
	if err != nil {
		return fmt.Errorf("orchestrator: lock the supply record: %w", err)
	}
	defer release()
	history, err := ReadSupply(path)
	if err != nil {
		return err
	}
	history = append(history, s)
	if len(history) > supplyKeep {
		history = history[len(history)-supplyKeep:]
	}
	if err := st.write(path, history); err != nil {
		return fmt.Errorf("orchestrator: write the supply record: %w", err)
	}
	return nil
}

func ReadSupply(path string) ([]Supply, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("orchestrator: read the supply record: %w", err)
	}
	var history []Supply
	if err := json.Unmarshal(raw, &history); err != nil {
		return nil, fmt.Errorf("orchestrator: parse the supply record %s: %w", path, err)
	}
	return history, nil
}

func SupplyIn(history []Supply, from, to time.Time) (Supply, bool) {
	for i := len(history) - 1; i >= 0; i-- {
		if at := history[i].At; !at.Before(from) && (to.IsZero() || at.Before(to)) {
			return history[i], true
		}
	}
	return Supply{}, false
}

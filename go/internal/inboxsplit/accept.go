package inboxsplit

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotPending    = errors.New("inboxsplit: no pending item with that id")
	ErrHasDependents = errors.New("inboxsplit: a pending item depends on the original")
	ErrRetireFailed  = errors.New("inboxsplit: both parts are filed and routed, but the original could not be retired")
)

type Inbox interface {
	Item(id string) (Item, error)
	Dependents(id string) ([]string, error)
	File(raw []byte) error
	RouteLane(id, reason string) error
	Withdraw(id, reason string) error
	Retire(id, resolution string) error
}

type Result struct {
	OriginalID string
	ConsoleID  string
	LaneID     string
}

func Accept(in Inbox, id string, isProtected func(path string) bool, laneAfterConsole bool) (Result, error) {
	item, err := in.Item(id)
	if err != nil {
		return Result{}, err
	}
	dependents, err := in.Dependents(id)
	if err != nil {
		return Result{}, err
	}
	if len(dependents) > 0 {
		return Result{}, fmt.Errorf("%w: %s; edit their deps first", ErrHasDependents, strings.Join(dependents, ", "))
	}
	p, err := Propose(item, isProtected, laneAfterConsole)
	if err != nil {
		return Result{}, err
	}
	if err := fileParts(in, p); err != nil {
		return Result{}, err
	}
	resolution := fmt.Sprintf("split into %s (console) and %s (lane)", p.ConsoleID, p.LaneID)
	if err := in.Retire(id, resolution); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrRetireFailed, err)
	}
	return Result{OriginalID: id, ConsoleID: p.ConsoleID, LaneID: p.LaneID}, nil
}

func fileParts(in Inbox, p Proposal) error {
	if err := in.File(encode(p.Console)); err != nil {
		return fmt.Errorf("file the console part %s: %w", p.ConsoleID, err)
	}
	if err := in.File(encode(p.Lane)); err != nil {
		return rollback(in, p, fmt.Errorf("file the lane part %s: %w", p.LaneID, err), p.ConsoleID)
	}
	reason := "split of " + p.OriginalID + ": the files that are not protected"
	if err := in.RouteLane(p.LaneID, reason); err != nil {
		return rollback(in, p, fmt.Errorf("route the lane part %s: %w", p.LaneID, err), p.LaneID, p.ConsoleID)
	}
	return nil
}

func rollback(in Inbox, p Proposal, cause error, filed ...string) error {
	errs := []error{cause}
	for _, id := range filed {
		if err := in.Withdraw(id, "split of "+p.OriginalID+" failed: "+cause.Error()); err != nil {
			errs = append(errs, fmt.Errorf("withdraw %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

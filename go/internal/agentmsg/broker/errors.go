package broker

import (
	"errors"
	"fmt"
)

var (
	ErrConfig           = errors.New("broker: config")
	ErrDenied           = errors.New("broker: denied")
	ErrUnroutable       = errors.New("broker: unroutable")
	ErrUnknownRecipient = fmt.Errorf("%w: unknown recipient", ErrNotFound)
	ErrNotFound         = errors.New("broker: not found")
	ErrCursor           = errors.New("broker: cursor")
	ErrIDConflict       = errors.New("broker: id conflict")
	ErrIDInUse          = errors.New("broker: id in use")
	ErrHopLimit         = errors.New("broker: hop limit")
	ErrBudget           = errors.New("broker: cycle budget")
	ErrRateLimited      = errors.New("broker: rate limited")
	ErrRepeat           = errors.New("broker: repeat")
	ErrQueueFull        = errors.New("broker: queue full")
)

const (
	RuleAuthenticate = "authenticate"
	RuleValidate     = "validate"
	RuleLimit        = "limit"
	RuleRoster       = "roster"
	RuleQueueCap     = "queue_cap"
	ReasonExpired    = "expired"
	ReasonGap        = "gap"
)

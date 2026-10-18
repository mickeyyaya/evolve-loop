package channel

import (
	"encoding/json"
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const MaxRecordBytes = signalcenter.MaxLineBytes + 512

const MaxMessageBytes = 64 << 10

const SourceWatch = "watch"

const (
	ReasonReset        = "reset"
	ReasonMalformed    = "malformed"
	ReasonRetention    = "retention"
	ReasonQueueFull    = "queue_full"
	ReasonWriteError   = "write_error"
	ReasonLockDeadline = "lock_deadline"
)

var (
	ErrRecordTooLong = errors.New("channel: record line too long")
	ErrInvalidRecord = errors.New("channel: a record needs exactly one of signal, gap and message, and a source that is not watch")
)

type Gap struct {
	Reason   string                `json:"reason"`
	Severity signalcenter.Severity `json:"severity,omitempty"`
	PID      int                   `json:"pid,omitempty"`
	FirstSeq uint64                `json:"first_seq,omitempty"`
	LastSeq  uint64                `json:"last_seq,omitempty"`
	Dropped  int                   `json:"dropped,omitempty"`
	From     int64                 `json:"from"`
	To       int64                 `json:"to"`
}

type Record struct {
	Cursor   int64               `json:"-"`
	Source   string              `json:"source"`
	Dispatch string              `json:"dispatch,omitempty"`
	Signal   *signalcenter.Event `json:"signal,omitempty"`
	Gap      *Gap                `json:"gap,omitempty"`
	Message  json.RawMessage     `json:"message,omitempty"`
}

type Batch struct {
	Records []Record
	Next    int64
}

func (r Record) hasOnePayload() bool {
	held := 0
	if r.Signal != nil {
		held++
	}
	if r.Gap != nil {
		held++
	}
	if r.hasMessage() {
		held++
	}
	return held == 1
}

func (r Record) hasMessage() bool {
	return len(r.Message) > 0 && string(r.Message) != "null"
}

func (r Record) lineLimit() int {
	if r.hasMessage() {
		return MaxMessageBytes
	}
	return MaxRecordBytes
}

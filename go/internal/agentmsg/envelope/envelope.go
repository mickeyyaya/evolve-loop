package envelope

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
)

const SchemaVersion = "msg/1.0"

type Kind string

const (
	KindNote      Kind = "note"
	KindRequest   Kind = "request"
	KindReply     Kind = "reply"
	KindHandoff   Kind = "handoff"
	KindFinding   Kind = "finding"
	KindEvidence  Kind = "evidence"
	KindBroadcast Kind = "broadcast"
)

var (
	ErrInvalid = errors.New("envelope: invalid")
	ErrID      = errors.New("envelope: id")
)

type Limits struct {
	BodyBytes    int
	SubjectBytes int
	Kinds        []Kind
}

type Envelope struct {
	Schema        string             `json:"schema"`
	ID            string             `json:"id"`
	TS            time.Time          `json:"ts"`
	Kind          Kind               `json:"kind"`
	From          identity.Principal `json:"from"`
	To            []address.Address  `json:"to"`
	Subject       string             `json:"subject,omitempty"`
	Body          string             `json:"body,omitempty"`
	Refs          []string           `json:"refs,omitempty"`
	ReplyTo       string             `json:"reply_to,omitempty"`
	CorrelationID string             `json:"correlation_id,omitempty"`
	ExpiresAt     *time.Time         `json:"expires_at,omitempty"`
	Hop           int                `json:"hop"`
	ContextID     int                `json:"context_id"`
}

func DefaultKinds() []Kind {
	return []Kind{KindNote, KindRequest, KindReply, KindHandoff, KindFinding, KindEvidence, KindBroadcast}
}

func (e Envelope) Validate(l Limits) error {
	checks := []func(Limits) error{e.checkHeader, e.checkRecipients, e.checkText, e.checkRefs, e.checkCorrelation}
	for _, check := range checks {
		if err := check(l); err != nil {
			return err
		}
	}
	return nil
}

func (e Envelope) checkHeader(l Limits) error {
	if e.Schema != SchemaVersion {
		return fmt.Errorf("%w: schema %q, want %q", ErrInvalid, e.Schema, SchemaVersion)
	}
	if !slices.Contains(l.Kinds, e.Kind) {
		return fmt.Errorf("%w: kind %q is not registered", ErrInvalid, e.Kind)
	}
	if e.ID != "" && !ValidID(e.ID) {
		return fmt.Errorf("%w: id %q is not a ULID", ErrInvalid, e.ID)
	}
	if e.Hop < 0 {
		return fmt.Errorf("%w: hop %d", ErrInvalid, e.Hop)
	}
	return nil
}

func (e Envelope) checkRecipients(Limits) error {
	if len(e.To) == 0 {
		return fmt.Errorf("%w: no recipient", ErrInvalid)
	}
	for _, a := range e.To {
		if _, err := a.MarshalText(); err != nil {
			return fmt.Errorf("%w: recipient: %v", ErrInvalid, err)
		}
	}
	return nil
}

func (e Envelope) checkText(l Limits) error {
	if len(e.Body) > l.BodyBytes {
		return fmt.Errorf("%w: body has %d bytes, the cap is %d", ErrInvalid, len(e.Body), l.BodyBytes)
	}
	if len(e.Subject) > l.SubjectBytes || strings.ContainsAny(e.Subject, "\r\n") {
		return fmt.Errorf("%w: a subject is one line of at most %d bytes", ErrInvalid, l.SubjectBytes)
	}
	return nil
}

func (e Envelope) checkRefs(Limits) error {
	for _, ref := range e.Refs {
		inCycle := ref != "" && filepath.IsLocal(ref) && filepath.Clean(ref) == ref
		if !inCycle {
			return fmt.Errorf("%w: ref %q is not a clean path inside the cycle", ErrInvalid, ref)
		}
	}
	return nil
}

func (e Envelope) checkCorrelation(Limits) error {
	needsCorrelation := e.Kind == KindRequest || e.Kind == KindReply
	if needsCorrelation && e.CorrelationID == "" {
		return fmt.Errorf("%w: a %s carries a correlation_id", ErrInvalid, e.Kind)
	}
	if e.Kind == KindRequest && e.ExpiresAt == nil {
		return fmt.Errorf("%w: a request carries expires_at", ErrInvalid)
	}
	return nil
}

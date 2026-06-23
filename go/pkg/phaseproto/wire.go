// Package phaseproto is the wire-protocol bridge that lets the Go
// orchestrator drive out-of-process phase agents (Node/Python/Go
// subprocesses) through the same core.PhaseRunner interface used for
// in-process runners.
//
// Wire shape: one JSON Envelope per line on stdin/stdout. The Envelope
// carries a versioned header (Version, Kind, CorrelationID), a
// payload as json.RawMessage so the inner core.PhaseRequest /
// core.PhaseResponse schema can evolve independently, and a structured
// WireError for the failure path.
//
// Why json.RawMessage for Payload: defers inner-payload decoding to
// the receiver, preserves byte-for-byte ordering across middleware,
// and lets a relay process pass envelopes through without knowing the
// inner schema.
package phaseproto

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// WireVersion is the on-wire protocol version. Bumping this is a
// coordinated rollout: all subprocess agents must be redeployed before
// orchestrators speaking the new version.
const WireVersion = 1

// marshalJSON is a testable seam over json.Marshal; lets tests drive
// the defensive error-wrap branch in EncodeRequest/EncodeResponse.
var marshalJSON = json.Marshal

// Kind constants — the three envelope kinds the protocol distinguishes.
const (
	KindRequest  = "request"
	KindResponse = "response"
	KindError    = "error"
)

// Envelope is the single per-line frame. Payload XOR Error: a request
// or response carries Payload; an error envelope carries Error.
type Envelope struct {
	Version       int             `json:"v"`
	Kind          string          `json:"kind"`
	CorrelationID string          `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Error         *WireError      `json:"error,omitempty"`
}

// WireError is the structured failure value. Code is the load-bearing
// field — failure-adapter dispatches on it; Message is for humans;
// Retryable steers the orchestrator's retry policy without parsing
// English.
type WireError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// Error implements the error interface.
func (e *WireError) Error() string {
	if e == nil {
		return "<nil WireError>"
	}
	return fmt.Sprintf("phaseproto: %s: %s", e.Code, e.Message)
}

// Is reports whether target is a *WireError with the same Code.
// Lets callers write `errors.Is(err, &WireError{Code: "BUDGET_EXCEEDED"})`.
func (e *WireError) Is(target error) bool {
	var other *WireError
	if !errors.As(target, &other) {
		return false
	}
	return e.Code == other.Code
}

// EncodeRequest builds a request envelope around a core.PhaseRequest.
func EncodeRequest(correlationID string, req core.PhaseRequest) (Envelope, error) {
	raw, err := marshalJSON(req)
	if err != nil {
		return Envelope{}, fmt.Errorf("phaseproto: encode request: %w", err)
	}
	return Envelope{
		Version:       WireVersion,
		Kind:          KindRequest,
		CorrelationID: correlationID,
		Payload:       raw,
	}, nil
}

// EncodeResponse builds a response envelope around a core.PhaseResponse.
func EncodeResponse(correlationID string, resp core.PhaseResponse) (Envelope, error) {
	raw, err := marshalJSON(resp)
	if err != nil {
		return Envelope{}, fmt.Errorf("phaseproto: encode response: %w", err)
	}
	return Envelope{
		Version:       WireVersion,
		Kind:          KindResponse,
		CorrelationID: correlationID,
		Payload:       raw,
	}, nil
}

// EncodeError builds an error envelope.
func EncodeError(correlationID string, werr *WireError) Envelope {
	return Envelope{
		Version:       WireVersion,
		Kind:          KindError,
		CorrelationID: correlationID,
		Error:         werr,
	}
}

// DecodeRequest extracts a core.PhaseRequest from a request envelope.
// Validates the envelope first; rejects wrong kinds.
func DecodeRequest(env Envelope) (core.PhaseRequest, error) {
	if err := Validate(env); err != nil {
		return core.PhaseRequest{}, err
	}
	if env.Kind != KindRequest {
		return core.PhaseRequest{}, fmt.Errorf("phaseproto: expected request envelope, got kind=%q", env.Kind)
	}
	var req core.PhaseRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		return core.PhaseRequest{}, fmt.Errorf("phaseproto: decode request payload: %w", err)
	}
	return req, nil
}

// DecodeResponse extracts a core.PhaseResponse from a response envelope.
func DecodeResponse(env Envelope) (core.PhaseResponse, error) {
	if err := Validate(env); err != nil {
		return core.PhaseResponse{}, err
	}
	if env.Kind != KindResponse {
		return core.PhaseResponse{}, fmt.Errorf("phaseproto: expected response envelope, got kind=%q", env.Kind)
	}
	var resp core.PhaseResponse
	if err := json.Unmarshal(env.Payload, &resp); err != nil {
		return core.PhaseResponse{}, fmt.Errorf("phaseproto: decode response payload: %w", err)
	}
	return resp, nil
}

// Validate enforces structural invariants common to all envelope kinds.
// Returns descriptive errors so log readers can spot wire mismatches.
func Validate(env Envelope) error {
	if env.Version <= 0 || env.Version > WireVersion {
		return fmt.Errorf("phaseproto: unsupported wire version %d (this build speaks %d)", env.Version, WireVersion)
	}
	switch env.Kind {
	case KindRequest, KindResponse, KindError:
		// ok
	default:
		return fmt.Errorf("phaseproto: unknown envelope kind %q", env.Kind)
	}
	if env.CorrelationID == "" {
		return errors.New("phaseproto: envelope missing correlation_id")
	}
	switch env.Kind {
	case KindRequest, KindResponse:
		if env.Error != nil {
			return fmt.Errorf("phaseproto: %s envelope must not carry error field", env.Kind)
		}
		if len(env.Payload) == 0 {
			return fmt.Errorf("phaseproto: %s envelope missing payload", env.Kind)
		}
	case KindError:
		if env.Error == nil {
			return errors.New("phaseproto: error envelope missing error field")
		}
		if len(env.Payload) != 0 {
			return errors.New("phaseproto: error envelope must not carry payload")
		}
	}
	return nil
}

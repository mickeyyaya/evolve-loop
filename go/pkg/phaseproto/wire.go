// Package phaseproto is the wire protocol between the orchestrator and out-of-process phase agents.
// See docs/architecture/packages/pkg-phaseproto.md.
package phaseproto

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const WireVersion = 1

var marshalJSON = json.Marshal

const (
	KindRequest  = "request"
	KindResponse = "response"
	KindError    = "error"
)

type Envelope struct {
	Version       int             `json:"v"`
	Kind          string          `json:"kind"`
	CorrelationID string          `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Error         *WireError      `json:"error,omitempty"`
}

type WireError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *WireError) Error() string {
	if e == nil {
		return "<nil WireError>"
	}
	return fmt.Sprintf("phaseproto: %s: %s", e.Code, e.Message)
}

func (e *WireError) Is(target error) bool {
	var other *WireError
	if !errors.As(target, &other) {
		return false
	}
	return e.Code == other.Code
}

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

func EncodeError(correlationID string, werr *WireError) Envelope {
	return Envelope{
		Version:       WireVersion,
		Kind:          KindError,
		CorrelationID: correlationID,
		Error:         werr,
	}
}

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

func Validate(env Envelope) error {
	if env.Version <= 0 || env.Version > WireVersion {
		return fmt.Errorf("phaseproto: unsupported wire version %d (this build speaks %d)", env.Version, WireVersion)
	}
	switch env.Kind {
	case KindRequest, KindResponse, KindError:
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

package phaseproto

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Wire-protocol version pin. The wire format is the load-bearing contract
// between the orchestrator and out-of-process phase agents; bumping this
// requires a coordinated rollout, so we pin it in the test as well.
const wantWireVersion = 1

func TestEnvelope_RoundTripRequest(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       42,
		ProjectRoot: "/tmp/proj",
		Workspace:   "/tmp/proj/.evolve",
		Worktree:    "/tmp/proj/.evolve/worktrees/abc",
		GoalHash:    "deadbeefcafebabe",
		Context:     map[string]string{"k": "v"},
		Budget:      core.BudgetEnvelope{MaxUSD: 2.50, BatchCapUSD: 20.0},
		Env:         map[string]string{"FOO": "bar"},
	}
	env, err := EncodeRequest("corr-1", req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if env.Version != wantWireVersion {
		t.Errorf("Version=%d, want %d", env.Version, wantWireVersion)
	}
	if env.Kind != KindRequest {
		t.Errorf("Kind=%q, want %q", env.Kind, KindRequest)
	}
	if env.CorrelationID != "corr-1" {
		t.Errorf("CorrelationID=%q, want corr-1", env.CorrelationID)
	}
	if env.Error != nil {
		t.Errorf("Error must be nil on request, got %+v", env.Error)
	}
	if len(env.Payload) == 0 {
		t.Fatal("Payload empty")
	}

	// Wire bytes must be deterministic JSON (encoding/json default ordering).
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal envelope: %v", err)
	}
	var back Envelope
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal envelope: %v", err)
	}

	got, err := DecodeRequest(back)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if got.Cycle != req.Cycle || got.GoalHash != req.GoalHash || got.Budget.MaxUSD != req.Budget.MaxUSD {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", got, req)
	}
	if got.Context["k"] != "v" || got.Env["FOO"] != "bar" {
		t.Errorf("map round-trip lost data: ctx=%v env=%v", got.Context, got.Env)
	}
}

func TestEnvelope_RoundTripResponse(t *testing.T) {
	resp := core.PhaseResponse{
		Phase:        "build",
		Verdict:      core.VerdictPASS,
		ArtifactsDir: "/tmp/art",
		NextPhase:    "audit",
		CostUSD:      0.42,
		Tokens:       core.TokenUsage{Input: 1000, Output: 200},
		DurationMS:   1234,
		Diagnostics:  []core.Diagnostic{{Severity: "info", Message: "ok"}},
	}
	env, err := EncodeResponse("corr-2", resp)
	if err != nil {
		t.Fatalf("EncodeResponse: %v", err)
	}
	if env.Kind != KindResponse {
		t.Errorf("Kind=%q, want %q", env.Kind, KindResponse)
	}

	raw, _ := json.Marshal(env)
	var back Envelope
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	got, err := DecodeResponse(back)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	if got.Verdict != core.VerdictPASS || got.NextPhase != "audit" || got.CostUSD != 0.42 {
		t.Errorf("response round-trip mismatch: %+v", got)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Message != "ok" {
		t.Errorf("diagnostics lost: %+v", got.Diagnostics)
	}
}

func TestEnvelope_ErrorEnvelope(t *testing.T) {
	werr := &WireError{
		Code:      "BUDGET_EXCEEDED",
		Message:   "phase exceeded $2.00 cap",
		Retryable: false,
	}
	env := EncodeError("corr-3", werr)
	if env.Kind != KindError {
		t.Errorf("Kind=%q, want %q", env.Kind, KindError)
	}
	if env.Error == nil || env.Error.Code != "BUDGET_EXCEEDED" {
		t.Errorf("Error not populated: %+v", env.Error)
	}
	if env.Payload != nil {
		t.Errorf("error envelope must have nil Payload, got %s", env.Payload)
	}

	raw, _ := json.Marshal(env)
	var back Envelope
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Error.Retryable != false {
		t.Errorf("Retryable lost on round-trip")
	}
}

func TestValidate_RejectsBadEnvelopes(t *testing.T) {
	cases := []struct {
		name    string
		env     Envelope
		wantSub string
	}{
		{
			name:    "zero_version",
			env:     Envelope{Version: 0, Kind: KindRequest, CorrelationID: "x", Payload: json.RawMessage(`{}`)},
			wantSub: "version",
		},
		{
			name:    "future_version",
			env:     Envelope{Version: 999, Kind: KindRequest, CorrelationID: "x", Payload: json.RawMessage(`{}`)},
			wantSub: "version",
		},
		{
			name:    "unknown_kind",
			env:     Envelope{Version: 1, Kind: "gibberish", CorrelationID: "x", Payload: json.RawMessage(`{}`)},
			wantSub: "kind",
		},
		{
			name:    "missing_correlation_id",
			env:     Envelope{Version: 1, Kind: KindRequest, Payload: json.RawMessage(`{}`)},
			wantSub: "correlation_id",
		},
		{
			name:    "request_with_error",
			env:     Envelope{Version: 1, Kind: KindRequest, CorrelationID: "x", Payload: json.RawMessage(`{}`), Error: &WireError{Code: "x"}},
			wantSub: "request",
		},
		{
			name:    "error_with_payload",
			env:     Envelope{Version: 1, Kind: KindError, CorrelationID: "x", Payload: json.RawMessage(`{}`), Error: &WireError{Code: "x"}},
			wantSub: "payload",
		},
		{
			name:    "error_without_error_field",
			env:     Envelope{Version: 1, Kind: KindError, CorrelationID: "x"},
			wantSub: "error",
		},
		{
			name:    "request_with_empty_payload",
			env:     Envelope{Version: 1, Kind: KindRequest, CorrelationID: "x"},
			wantSub: "payload",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.env)
			if err == nil {
				t.Fatalf("Validate accepted bad envelope: %+v", tc.env)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantSub) {
				t.Errorf("err=%q missing substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestValidate_AcceptsGoodEnvelopes(t *testing.T) {
	cases := []Envelope{
		{Version: 1, Kind: KindRequest, CorrelationID: "x", Payload: json.RawMessage(`{}`)},
		{Version: 1, Kind: KindResponse, CorrelationID: "x", Payload: json.RawMessage(`{}`)},
		{Version: 1, Kind: KindError, CorrelationID: "x", Error: &WireError{Code: "X", Message: "y"}},
	}
	for i, env := range cases {
		if err := Validate(env); err != nil {
			t.Errorf("[%d] Validate rejected good envelope: %v", i, err)
		}
	}
}

func TestDecodeRequest_RejectsWrongKind(t *testing.T) {
	env := Envelope{Version: 1, Kind: KindResponse, CorrelationID: "x", Payload: json.RawMessage(`{}`)}
	if _, err := DecodeRequest(env); err == nil {
		t.Error("DecodeRequest must reject response kind")
	}
	env2 := Envelope{Version: 1, Kind: KindError, CorrelationID: "x", Error: &WireError{Code: "X"}}
	if _, err := DecodeResponse(env2); err == nil {
		t.Error("DecodeResponse must reject error kind")
	}
}

func TestDecodeRequest_MalformedPayload(t *testing.T) {
	env := Envelope{Version: 1, Kind: KindRequest, CorrelationID: "x", Payload: json.RawMessage(`not-json`)}
	if _, err := DecodeRequest(env); err == nil {
		t.Error("DecodeRequest must reject malformed JSON")
	}
}

func TestWireError_IsError(t *testing.T) {
	werr := &WireError{Code: "X", Message: "boom"}
	// Must implement the error interface so callers can return it as `error`.
	var _ error = werr
	if !strings.Contains(werr.Error(), "boom") {
		t.Errorf("Error()=%q must contain message", werr.Error())
	}
	if !strings.Contains(werr.Error(), "X") {
		t.Errorf("Error()=%q must contain code", werr.Error())
	}

	// errors.Is should match by Code.
	other := &WireError{Code: "X", Message: "different"}
	if !errors.Is(werr, other) {
		t.Error("errors.Is must match by Code")
	}
	notSame := &WireError{Code: "Y"}
	if errors.Is(werr, notSame) {
		t.Error("errors.Is must NOT match different Code")
	}
}

func TestWireVersion_ConstantPinned(t *testing.T) {
	if WireVersion != wantWireVersion {
		t.Errorf("WireVersion bumped to %d without coordinated rollout (was %d)", WireVersion, wantWireVersion)
	}
}

func TestWireError_NilReceiver(t *testing.T) {
	var nilWE *WireError
	got := nilWE.Error()
	if !strings.Contains(got, "nil") {
		t.Errorf("nil receiver Error()=%q, want substring 'nil'", got)
	}
}

func TestWireError_IsAgainstNonWireError(t *testing.T) {
	werr := &WireError{Code: "X"}
	// errors.Is must return false when target is not a *WireError.
	if errors.Is(werr, errors.New("plain")) {
		t.Error("Is must return false for non-*WireError target")
	}
}

func TestDecodeResponse_MalformedPayload(t *testing.T) {
	env := Envelope{Version: 1, Kind: KindResponse, CorrelationID: "x", Payload: json.RawMessage(`not-json`)}
	if _, err := DecodeResponse(env); err == nil {
		t.Error("DecodeResponse must reject malformed JSON")
	}
}

// Drive the defensive json.Marshal error branch via the testable seam.
func TestEncodeRequest_MarshalError(t *testing.T) {
	saved := marshalJSON
	t.Cleanup(func() { marshalJSON = saved })
	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("synthetic marshal failure")
	}
	if _, err := EncodeRequest("x", core.PhaseRequest{}); err == nil {
		t.Error("EncodeRequest must propagate marshal failure")
	} else if !strings.Contains(err.Error(), "synthetic") {
		t.Errorf("err=%q must wrap synthetic failure", err.Error())
	}
}

func TestEncodeResponse_MarshalError(t *testing.T) {
	saved := marshalJSON
	t.Cleanup(func() { marshalJSON = saved })
	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("synthetic marshal failure")
	}
	if _, err := EncodeResponse("x", core.PhaseResponse{}); err == nil {
		t.Error("EncodeResponse must propagate marshal failure")
	}
}

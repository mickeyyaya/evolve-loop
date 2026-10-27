package agentrequest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

const roleArg = "role"

type What struct {
	Verb string            `json:"verb"`
	Args map[string]string `json:"args"`
}

type ExpectedOutput struct {
	Artifact string `json:"artifact"`
	Schema   string `json:"schema,omitempty"`
}

type Request struct {
	Schema         string         `json:"schema"`
	Kind           string         `json:"kind"`
	What           What           `json:"what"`
	Why            string         `json:"why"`
	Inputs         []string       `json:"inputs"`
	ExpectedOutput ExpectedOutput `json:"expected_output"`
}

func (r Request) Role() string { return r.What.Args[roleArg] }

func parse(raw []byte) (Request, error) {
	var r Request
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Request{}, fmt.Errorf("request: %w", err)
	}
	if dec.More() {
		return Request{}, errors.New("request: trailing data after the JSON object")
	}
	switch {
	case r.Schema != agentclass.RequestSchema:
		return Request{}, fmt.Errorf("request: schema %q is not %s", r.Schema, agentclass.RequestSchema)
	case strings.TrimSpace(r.Why) == "":
		return Request{}, errors.New("request: the why is empty")
	case strings.TrimSpace(r.ExpectedOutput.Artifact) == "":
		return Request{}, errors.New("request: the expected_output names no artifact")
	}
	return r, nil
}

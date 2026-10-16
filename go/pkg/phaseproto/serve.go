package phaseproto

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type Handler func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error)

func ServeStdio(in io.Reader, out io.Writer, h Handler) error {
	br := bufio.NewReader(in)
	line, err := br.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return fmt.Errorf("phaseproto: read request: %w", err)
	}
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return fmt.Errorf("phaseproto: unmarshal request envelope: %w", err)
	}
	req, err := DecodeRequest(env)
	if err != nil {
		return err
	}

	resp, hErr := h(context.Background(), req)
	var outEnv Envelope
	if hErr != nil {
		var werr *WireError
		if !errors.As(hErr, &werr) {
			werr = &WireError{Code: CodeHandlerError, Message: hErr.Error(), Retryable: false}
		}
		outEnv = EncodeError(env.CorrelationID, werr)
	} else {
		outEnv, err = EncodeResponse(env.CorrelationID, resp)
		if err != nil {
			return err
		}
	}
	raw, err := json.Marshal(outEnv)
	if err != nil {
		return fmt.Errorf("phaseproto: marshal response envelope: %w", err)
	}
	if _, err := out.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("phaseproto: write response: %w", err)
	}
	return nil
}

package gaterun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

const (
	MethodTrackedFiles = "Lane.TrackedFiles"
	MethodTrackedTree  = "Lane.TrackedTree"
	MethodTrackedSet   = "Lane.TrackedSet"
	MethodList         = "Toolchain.List"
	MethodTest         = "Toolchain.Test"
)

type treeParams struct {
	Root   string `json:"root"`
	RelDir string `json:"relDir"`
}

type setParams struct {
	Root   string `json:"root"`
	RelDir string `json:"relDir"`
	Ext    string `json:"ext"`
}

type toolParams struct {
	Call gatecontract.Call `json:"call"`
}

type Broker struct {
	Root    string
	Scratch string
	Ports   gatecontract.Ports
}

func (b Broker) Handle(ctx context.Context, req Frame) Frame {
	result, err := b.dispatch(ctx, req)
	return encodeResponse(req, result, err)
}

func encodeResponse(req Frame, result any, err error) Frame {
	resp := Frame{Kind: KindResponse, ID: req.ID}
	if err != nil {
		resp.Message = err.Error()
		return resp
	}
	raw, err := json.Marshal(result)
	if err != nil {
		resp.Message = fmt.Sprintf("encode the result of %s: %v", req.Method, err)
		return resp
	}
	resp.Result = raw
	return resp
}

func (b Broker) dispatch(ctx context.Context, req Frame) (any, error) {
	switch req.Method {
	case MethodTrackedFiles:
		return withLane(b, req, func(p treeParams, lane gatecontract.Lane) (any, error) {
			if err := b.checkLane(p.Root, p.RelDir); err != nil {
				return nil, err
			}
			return lane.TrackedFiles(p.Root, p.RelDir)
		})
	case MethodTrackedTree:
		return withLane(b, req, func(p treeParams, lane gatecontract.Lane) (any, error) {
			if err := b.checkLane(p.Root, p.RelDir); err != nil {
				return nil, err
			}
			return lane.TrackedTree(p.Root, p.RelDir)
		})
	case MethodTrackedSet:
		return withLane(b, req, func(p setParams, lane gatecontract.Lane) (any, error) {
			if err := b.checkLane(p.Root, p.RelDir); err != nil {
				return nil, err
			}
			return lane.TrackedSet(p.Root, p.RelDir, p.Ext)
		})
	case MethodList:
		return withToolchain(b, req, func(p toolParams, tc gatecontract.Toolchain) (any, error) {
			call, err := b.checkTool("list", p.Call)
			if err != nil {
				return nil, err
			}
			return tc.List(ctx, call)
		})
	case MethodTest:
		return withToolchain(b, req, func(p toolParams, tc gatecontract.Toolchain) (any, error) {
			call, err := b.checkTool("test", p.Call)
			if err != nil {
				return nil, err
			}
			return tc.Test(ctx, call)
		})
	}
	return nil, fmt.Errorf("unknown function %q: the broker serves only the functions of Lane and Toolchain", req.Method)
}

func withLane[P any](b Broker, req Frame, fn func(P, gatecontract.Lane) (any, error)) (any, error) {
	if b.Ports.Lane == nil {
		return nil, errors.New("no port: the broker holds no Lane")
	}
	return withParams(req, func(p P) (any, error) { return fn(p, b.Ports.Lane) })
}

func withToolchain[P any](b Broker, req Frame, fn func(P, gatecontract.Toolchain) (any, error)) (any, error) {
	if b.Ports.Toolchain == nil {
		return nil, errors.New("no port: the broker holds no Toolchain")
	}
	return withParams(req, func(p P) (any, error) { return fn(p, b.Ports.Toolchain) })
}

func withParams[P any](req Frame, fn func(P) (any, error)) (any, error) {
	var params P
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("bad params for %s: %w", req.Method, err)
	}
	return fn(params)
}

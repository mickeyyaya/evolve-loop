package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type readResult struct {
	Events   []signalcenter.Event `json:"events"`
	Next     int64                `json:"next"`
	Skipped  int                  `json:"skipped"`
	Err      string               `json:"err"`
	NotExist bool                 `json:"not_exist"`
}

type mergeResult struct {
	Merged                    json.RawMessage `json:"merged"`
	InputsAfterCall           json.RawMessage `json:"inputs_after_call"`
	InputsAfterMutatingResult json.RawMessage `json:"inputs_after_mutating_result"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "streamprobe:", err)
		os.Exit(3)
	}
}

func run(args []string) error {
	switch {
	case len(args) == 3 && args[0] == "read":
		from, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return err
		}
		return emit(read(args[1], from))
	case len(args) == 1 && args[0] == "merge":
		return mergeStdin()
	default:
		return errors.New("usage: streamprobe read <path> <from> | streamprobe merge")
	}
}

func read(path string, from int64) readResult {
	var chunk signalcenter.StreamChunk
	var err error
	chunk, err = signalcenter.ReadStream(path, from)
	r := readResult{Events: chunk.Events, Next: chunk.Next, Skipped: chunk.Skipped}
	if err != nil {
		r.Err = err.Error()
		r.NotExist = errors.Is(err, fs.ErrNotExist)
	}
	return r
}

func mergeStdin() error {
	var streams [][]signalcenter.Event
	if err := json.NewDecoder(os.Stdin).Decode(&streams); err != nil {
		return err
	}
	merged := signalcenter.MergeByTS(streams...)
	var r mergeResult
	var err error
	if r.Merged, err = json.Marshal(merged); err != nil {
		return err
	}
	if r.InputsAfterCall, err = json.Marshal(streams); err != nil {
		return err
	}
	for i := range merged {
		merged[i].Reason = "mutated through the merged slice"
	}
	if r.InputsAfterMutatingResult, err = json.Marshal(streams); err != nil {
		return err
	}
	return emit(r)
}

func emit(v any) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}

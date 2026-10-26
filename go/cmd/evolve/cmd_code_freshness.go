package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const codeExitStale = 21

type codeIndexFreshness struct {
	Head   string
	Main   string
	Behind int
}

func (f codeIndexFreshness) stale() bool { return f.Head != f.Main }

func (h codeIndexHost) freshness(ctx context.Context, indexDir string) (codeIndexFreshness, error) {
	if err := h.present(indexDir); err != nil {
		return codeIndexFreshness{}, err
	}
	return h.compare(ctx, indexDir)
}

func (h codeIndexHost) compare(ctx context.Context, indexDir string) (codeIndexFreshness, error) {
	heads, err := h.git(ctx, indexDir, "rev-parse", "HEAD", codeIndexMainRef)
	if err != nil {
		return codeIndexFreshness{}, err
	}
	lines := strings.Fields(heads)
	if len(lines) != 2 {
		return codeIndexFreshness{}, fmt.Errorf("git rev-parse HEAD %s in %s: want two object names, got %q", codeIndexMainRef, indexDir, strings.TrimSpace(heads))
	}
	f := codeIndexFreshness{Head: lines[0], Main: lines[1]}
	if !f.stale() {
		return f, nil
	}
	count, err := h.git(ctx, indexDir, "rev-list", "--count", "HEAD.."+codeIndexMainRef)
	if err != nil {
		return codeIndexFreshness{}, err
	}
	if f.Behind, err = strconv.Atoi(strings.TrimSpace(count)); err != nil {
		return codeIndexFreshness{}, fmt.Errorf("git rev-list --count in %s: %w", indexDir, err)
	}
	return f, nil
}

type codeFreshnessCheck struct {
	fresh    *codeIndexFreshness
	warnings []string
	refused  string
}

func checkCodeFreshness(ctx context.Context, idx codeIndexHandle, verb string) codeFreshnessCheck {
	f, err := idx.freshness(ctx)
	switch {
	case errors.Is(err, errCodeIndexAbsent):
		return codeFreshnessCheck{}
	case err != nil:
		return codeFreshnessCheck{warnings: []string{fmt.Sprintf("evolve code: WARN: the freshness of the code index is unknown: %v", err)}}
	case !f.stale():
		return codeFreshnessCheck{fresh: &f}
	}
	gap := fmt.Sprintf("the code index is %d commit(s) behind %s (index %s, %s %s)", f.Behind, codeIndexMainRef, shortSHA(f.Head), codeIndexMainRef, shortSHA(f.Main))
	if f.Behind == 0 {
		gap = fmt.Sprintf("the code index is not at %s but misses none of its commits (index %s, %s %s)", codeIndexMainRef, shortSHA(f.Head), codeIndexMainRef, shortSHA(f.Main))
	}
	if idx.refuseStale {
		return codeFreshnessCheck{fresh: &f, refused: fmt.Sprintf("evolve code %s: refused: %s and code_index.stale is refuse; run evolve code refresh first", verb, gap)}
	}
	return codeFreshnessCheck{fresh: &f, warnings: []string{fmt.Sprintf("evolve code: WARN: %s; an answer can miss newer code; evolve code refresh moves the index", gap)}}
}

type codeEnvelope struct {
	Schema   string             `json:"schema"`
	OK       bool               `json:"ok"`
	Data     any                `json:"data"`
	Error    *codeEnvelopeError `json:"error"`
	Warnings []string           `json:"warnings"`
}

type codeEnvelopeError struct {
	Code    int    `json:"code"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type codeEnvelopeWriter struct {
	schema         string
	json           bool
	warnings       []string
	stdout, stderr io.Writer
}

func (w codeEnvelopeWriter) fail(rc int, kind, message string) int {
	fmt.Fprintln(w.stderr, message)
	firstLine, _, _ := strings.Cut(message, "\n")
	w.emit(codeEnvelope{Error: &codeEnvelopeError{Code: rc, Kind: kind, Message: firstLine}})
	return rc
}

func (w codeEnvelopeWriter) ok(data any) int {
	w.emit(codeEnvelope{OK: true, Data: data})
	return 0
}

func (w codeEnvelopeWriter) emit(e codeEnvelope) {
	if !w.json {
		return
	}
	e.Schema, e.Warnings = w.schema, append([]string{}, w.warnings...)
	if err := json.NewEncoder(w.stdout).Encode(e); err != nil {
		fmt.Fprintf(w.stderr, "evolve code: write the JSON envelope: %v\n", err)
	}
}

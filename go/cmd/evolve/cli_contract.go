package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func coreVerbOf(name string, rest []string) (coreVerb, bool) {
	if name == "loop" {
		if _, status := loopStatusArgs(rest); status {
			return lookupCoreVerb("loop status")
		}
	}
	if len(rest) > 0 {
		if v, ok := lookupCoreVerb(name + " " + rest[0]); ok {
			return v, true
		}
	}
	return lookupCoreVerb(name)
}

func jsonRequested(args []string) bool {
	for _, a := range args {
		switch a {
		case "--":
			return false
		case "--json", "-json", "--json=true", "-json=true":
			return true
		}
	}
	return false
}

func humanWriter(stdout io.Writer, asJSON bool) io.Writer {
	if asJSON {
		return io.Discard
	}
	return stdout
}

func printVerbJSON(verb string, v any, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "evolve %s: encode the JSON result: %v\n", verb, err)
		return cliExitIO
	}
	return cliExitOK
}

type errorLineWriter struct {
	w       io.Writer
	pending []byte
	line    string
}

func (e *errorLineWriter) Write(p []byte) (int, error) {
	e.pending = append(e.pending, p...)
	for {
		line, rest, found := bytes.Cut(e.pending, []byte("\n"))
		if !found {
			break
		}
		e.pending = rest
		e.note(string(line))
	}
	return e.w.Write(p)
}

func (e *errorLineWriter) message() string {
	e.note(string(e.pending))
	e.pending = nil
	return e.line
}

func (e *errorLineWriter) note(line string) {
	t := strings.TrimSpace(line)
	indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
	if t == "" || indented || strings.HasSuffix(t, ":") || strings.Contains(t, "WARN") || strings.HasPrefix(t, "usage:") {
		return
	}
	e.line = t
}

type envelopeSink struct {
	verb           coreVerb
	stdout, stderr io.Writer
}

func (s envelopeSink) write(rc int, data json.RawMessage, message string, warnings ...string) int {
	e := codeEnvelope{Schema: s.verb.schema(), OK: rc == cliExitOK, Data: data, Warnings: append([]string{}, warnings...)}
	if rc != cliExitOK {
		kind, known := s.verb.exitKind(rc)
		if !known {
			kind = "unlisted"
			e.Warnings = append(e.Warnings, fmt.Sprintf("exit code %d is not in the exit table of cli-contract.md §3", rc))
		}
		e.Error = &codeEnvelopeError{Code: rc, Kind: kind, Message: cmp.Or(message, fmt.Sprintf("evolve %s exited %d (%s)", s.verb.path, rc, kind))}
	}
	if err := json.NewEncoder(s.stdout).Encode(e); err != nil {
		fmt.Fprintf(s.stderr, "evolve %s: write the JSON envelope: %v\n", s.verb.path, err)
		return max(rc, cliExitIO)
	}
	return rc
}

const (
	nonJSONWarning  = "the verb printed text that is not JSON on stdout; the text went to stderr"
	manyJSONWarning = "the verb printed more than one JSON value; data is the list of the values"
)

func runEnveloped(v coreVerb, run func(stdout, stderr io.Writer) int, stdout, stderr io.Writer) int {
	sink := envelopeSink{verb: v, stdout: stdout, stderr: stderr}
	errs := &errorLineWriter{w: stderr}
	if v.stream {
		lines := &envelopeLineWriter{sink: sink}
		rc := run(lines, errs)
		lines.flush()
		if lines.err != nil {
			return max(rc, cliExitIO)
		}
		if rc == cliExitOK && lines.emitted > 0 {
			return rc
		}
		return sink.write(rc, nil, errs.message())
	}
	var out bytes.Buffer
	rc := run(&out, errs)
	data, warning := envelopeData(out.Bytes(), stderr)
	if warning != "" {
		return sink.write(rc, data, errs.message(), warning)
	}
	return sink.write(rc, data, errs.message())
}

func envelopeData(out []byte, stderr io.Writer) (json.RawMessage, string) {
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, ""
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var values []json.RawMessage
	for {
		var v json.RawMessage
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_, _ = stderr.Write(out)
			return nil, nonJSONWarning
		}
		values = append(values, v)
	}
	if len(values) == 1 {
		return values[0], ""
	}
	list := bytes.NewBufferString("[")
	for i, v := range values {
		if i > 0 {
			list.WriteByte(',')
		}
		list.Write(v)
	}
	list.WriteByte(']')
	return list.Bytes(), manyJSONWarning
}

type envelopeLineWriter struct {
	sink    envelopeSink
	pending []byte
	emitted int
	err     error
}

var errEnvelopeStdout = errors.New("the JSON envelope cannot be written to stdout")

func (w *envelopeLineWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for w.err == nil {
		line, rest, found := bytes.Cut(w.pending, []byte("\n"))
		if !found {
			return len(p), nil
		}
		w.pending = rest
		w.emit(line)
	}
	return 0, w.err
}

func (w *envelopeLineWriter) flush() {
	if w.err == nil {
		w.emit(w.pending)
	}
	w.pending = nil
}

func (w *envelopeLineWriter) emit(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	if !json.Valid(line) {
		_, _ = fmt.Fprintf(w.sink.stderr, "%s\n", line)
		return
	}
	if w.sink.write(cliExitOK, line, "") != cliExitOK {
		w.err = errEnvelopeStdout
		return
	}
	w.emitted++
}

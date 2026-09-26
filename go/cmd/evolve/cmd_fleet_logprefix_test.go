package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestPrefixLineWriter_AttributesAndSerializes(t *testing.T) {
	var mu sync.Mutex
	var sink bytes.Buffer
	a := &prefixLineWriter{w: &sink, prefix: "[a] ", mu: &mu}
	b := &prefixLineWriter{w: &sink, prefix: "[b] ", mu: &mu}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			fmt.Fprintf(a, "msg-%d\n", i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			fmt.Fprintf(b, "msg-%d\n", i)
		}
	}()
	wg.Wait()

	lines := strings.Split(strings.TrimRight(sink.String(), "\n"), "\n")
	if len(lines) != 100 {
		t.Fatalf("got %d lines, want 100 whole lines", len(lines))
	}
	for _, ln := range lines {
		if !strings.HasPrefix(ln, "[a] msg-") && !strings.HasPrefix(ln, "[b] msg-") {
			t.Errorf("torn or misattributed line: %q", ln)
		}
	}
}

func TestPrefixLineWriter_BuffersPartialUntilNewline(t *testing.T) {
	var mu sync.Mutex
	var sink bytes.Buffer
	w := &prefixLineWriter{w: &sink, prefix: "[x] ", mu: &mu}
	if _, err := io.WriteString(w, "partial"); err != nil {
		t.Fatal(err)
	}
	if sink.Len() != 0 {
		t.Errorf("partial line emitted early: %q", sink.String())
	}
	w.Flush()
	if got := sink.String(); got != "[x] partial\n" {
		t.Errorf("Flush = %q, want %q", got, "[x] partial\n")
	}
}

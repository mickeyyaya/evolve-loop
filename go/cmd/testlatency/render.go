package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type outputEvent struct {
	Output string `json:"Output"`
}

func renderOutput(r io.Reader, w, diag io.Writer) {
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var e outputEvent
		if len(line) == 0 || line[0] != '{' || json.Unmarshal(line, &e) != nil {
			fmt.Fprintf(w, "%s\n", line)
			continue
		}
		fmt.Fprint(w, e.Output)
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(diag, "testlatency: render: %v\n", err)
	}
}

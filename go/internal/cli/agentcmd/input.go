package agentcmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type fileList []string

func (f *fileList) String() string { return strings.Join(*f, ",") }

func (f *fileList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

const maxInputBytes = 8 << 20

func composeInput(input string, files []string, stdin io.Reader) (string, error) {
	body, err := primaryInput(input, len(files) > 0, stdin)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(body)
	for _, path := range files {
		raw, err := readFile(path, "--file "+path)
		if err != nil {
			return "", err
		}
		f := fence(raw)
		fmt.Fprintf(&b, "\n\n### %s\n%s\n%s\n%s\n", path, f, raw, f)
		if b.Len() > maxInputBytes {
			return "", refusal{exitInput, fmt.Sprintf("the input sources together are larger than %d bytes", maxInputBytes)}
		}
	}
	if strings.TrimSpace(b.String()) == "" {
		return "", refusal{exitInput, "the input is empty: pipe it on stdin, or give --input FILE or --file PATH"}
	}
	return b.String(), nil
}

func primaryInput(input string, hasFiles bool, stdin io.Reader) (string, error) {
	if input != "" && input != "-" {
		return readFile(input, "--input "+input)
	}
	if isTerminal(stdin) {
		if hasFiles && input == "" {
			return "", nil
		}
		return "", refusal{exitUsage, "stdin is a terminal: pipe the input, or give --input FILE or --file PATH"}
	}
	return readLimited(stdin, "stdin")
}

func readFile(path, what string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", refusal{exitInput, fmt.Sprintf("read %s: %v", what, err)}
	}
	defer func() { _ = f.Close() }()
	return readLimited(f, what)
}

func readLimited(r io.Reader, what string) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxInputBytes+1))
	if err != nil {
		return "", refusal{exitInput, fmt.Sprintf("read %s: %v", what, err)}
	}
	if len(raw) > maxInputBytes {
		return "", refusal{exitInput, fmt.Sprintf("%s is larger than %d bytes", what, maxInputBytes)}
	}
	return string(raw), nil
}

func fence(content string) string {
	longest, run := 0, 0
	for _, c := range content {
		if c != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	return strings.Repeat("`", max(3, longest+1))
}

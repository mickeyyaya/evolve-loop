package inbox

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
)

type Cursor struct {
	path   string
	offset int64
}

func NewCursor(workspace, agent string) *Cursor {
	return &Cursor{path: Path(workspace, agent)}
}

func (c *Cursor) Offset() int64 { return c.offset }

func (c *Cursor) SetOffset(n int64) { c.offset = n }

func (c *Cursor) Drain() ([]Envelope, error) {
	fi, err := os.Stat(c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if fi.Size() < c.offset {
		c.offset = 0
	}
	if fi.Size() == c.offset {
		return nil, nil
	}
	f, err := os.Open(c.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(c.offset, io.SeekStart); err != nil {
		return nil, err
	}

	var envs []Envelope
	var consumed int64
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break
		}
		consumed += int64(len(line))
		var env Envelope
		if jerr := json.Unmarshal(line[:len(line)-1], &env); jerr != nil {
			continue
		}
		envs = append(envs, env)
	}
	c.offset += consumed
	return envs, nil
}

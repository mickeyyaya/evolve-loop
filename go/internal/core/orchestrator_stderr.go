package core

import (
	"io"
	"os"
)

func WithStderr(w io.Writer) Option {
	return func(o *Orchestrator) {
		if w != nil {
			o.stderr = w
		}
	}
}

func (o *Orchestrator) errOut() io.Writer {
	if o == nil || o.stderr == nil {
		return os.Stderr
	}
	return o.stderr
}

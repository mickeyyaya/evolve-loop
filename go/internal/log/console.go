package log

import (
	"fmt"
	"io"
	"os"
)

type Console struct {
	Out   io.Writer
	Err   io.Writer
	Quiet bool
}

func Default() Console {
	return Console{Out: os.Stdout, Err: os.Stderr}
}

func Diag() Console {
	return Console{Out: os.Stderr, Err: os.Stderr}
}

func (c Console) Infof(format string, args ...any) {
	if c.Quiet || c.Out == nil {
		return
	}
	fmt.Fprintf(c.Out, format, args...)
}

func (c Console) Warnf(format string, args ...any) {
	if c.Err == nil {
		return
	}
	fmt.Fprintf(c.Err, format, args...)
}

func (c Console) Errorf(format string, args ...any) {
	if c.Err == nil {
		return
	}
	fmt.Fprintf(c.Err, format, args...)
}

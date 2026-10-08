package ship

import "io"

type packLog struct {
	notes io.Writer
	raw   io.Writer
}

func (l packLog) Write(p []byte) (int, error) { return l.notes.Write(p) }

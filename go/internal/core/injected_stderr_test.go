package core

import (
	"bytes"
	"sync"
)

type injectedStderr struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *injectedStderr) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *injectedStderr) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

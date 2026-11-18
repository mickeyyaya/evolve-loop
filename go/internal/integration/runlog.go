package integration

import "sync"

type RunLog struct {
	mu  sync.Mutex
	ids map[string]string
}

func NewRunLog() *RunLog {
	return &RunLog{ids: map[string]string{}}
}

func (l *RunLog) note(tip, id string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ids[tip] = id
}

func (l *RunLog) Run(tip string) string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ids[tip]
}

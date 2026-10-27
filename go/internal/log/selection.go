package log

import (
	"context"
	"io"
	"log/slog"
	"sync"
)

type Selection struct {
	Run       string
	Cycle     int
	Phase     string
	Dispatch  string
	Cat       string
	Component string
	Code      string
}

type Sinks struct {
	JSON   io.Writer
	Text   io.Writer
	Level  string
	Render func(slog.Record) string
}

type selectionKey struct{}

func WithSelection(ctx context.Context, s Selection) context.Context {
	return context.WithValue(ctx, selectionKey{}, s)
}

func NewSelectionLogger(s Sinks) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(s.Level), ReplaceAttr: renameTime}
	var handlers []slog.Handler
	if s.JSON != nil {
		handlers = append(handlers, slog.NewJSONHandler(s.JSON, opts))
	}
	if s.Text != nil {
		handlers = append(handlers, textHandler(s, opts))
	}
	return slog.New(selectionHandler{handlers: handlers})
}

func textHandler(s Sinks, opts *slog.HandlerOptions) slog.Handler {
	if s.Render == nil {
		return slog.NewTextHandler(s.Text, opts)
	}
	return &lineHandler{mu: &sync.Mutex{}, w: s.Text, render: s.Render, level: opts.Level.Level()}
}

func renameTime(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		a.Key = "ts"
	}
	return a
}

func (s Selection) attrs() []slog.Attr {
	var out []slog.Attr
	add := func(key, value string) {
		if value != "" {
			out = append(out, slog.String(key, value))
		}
	}
	add("run", s.Run)
	if s.Cycle != 0 {
		out = append(out, slog.Int("cycle", s.Cycle))
	}
	add("phase", s.Phase)
	add("dispatch", s.Dispatch)
	add("cat", s.Cat)
	add("component", s.Component)
	add("code", s.Code)
	return out
}

type selectionHandler struct {
	handlers []slog.Handler
}

func (h selectionHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, next := range h.handlers {
		if next.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h selectionHandler) Handle(ctx context.Context, r slog.Record) error {
	if s, ok := ctx.Value(selectionKey{}).(Selection); ok {
		present := map[string]bool{}
		r.Attrs(func(a slog.Attr) bool {
			present[a.Key] = true
			return true
		})
		for _, a := range s.attrs() {
			if !present[a.Key] {
				r.AddAttrs(a)
			}
		}
	}
	var first error
	for _, next := range h.handlers {
		if !next.Enabled(ctx, r.Level) {
			continue
		}
		if err := next.Handle(ctx, r.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h selectionHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return selectionHandler{handlers: mapHandlers(h.handlers, func(next slog.Handler) slog.Handler { return next.WithAttrs(attrs) })}
}

func (h selectionHandler) WithGroup(name string) slog.Handler {
	return selectionHandler{handlers: mapHandlers(h.handlers, func(next slog.Handler) slog.Handler { return next.WithGroup(name) })}
}

func mapHandlers(in []slog.Handler, f func(slog.Handler) slog.Handler) []slog.Handler {
	out := make([]slog.Handler, len(in))
	for i, next := range in {
		out[i] = f(next)
	}
	return out
}

type lineHandler struct {
	mu     *sync.Mutex
	w      io.Writer
	render func(slog.Record) string
	level  slog.Level
}

func (h *lineHandler) Enabled(_ context.Context, level slog.Level) bool { return level >= h.level }

func (h *lineHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, h.render(r)+"\n")
	return err
}

func (h *lineHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *lineHandler) WithGroup(string) slog.Handler { return h }

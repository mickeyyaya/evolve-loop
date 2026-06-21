// Package render provides the HTML template engine for the landing site:
// shared helper functions and strict execution so missing data fails the build
// instead of silently emitting <no value>.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
)

// FuncMap is the set of helpers available to every template.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"dict": dict,
		"inc":  func(i int) int { return i + 1 },
		"json": jsonScript,
	}
}

// jsonScript marshals a value to a JSON literal a template can embed for client
// JS, e.g. <script type="application/json">{{json .X}}</script>. json.Marshal
// HTML-escapes <, >, and & so the data can't break out of the script tag.
func jsonScript(v any) (template.JS, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return template.JS(b), nil
}

// dict builds a map from alternating key/value arguments, letting a template
// pass structured data to a partial: {{template "x" dict "label" .L "href" .H}}.
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments (%d)", len(pairs))
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: argument %d is not a string key", i)
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// New returns a configured root template: shared helpers + strict missing-key
// handling so referencing absent data is a hard error.
func New() *template.Template {
	return template.New("site").Funcs(FuncMap()).Option("missingkey=error")
}

// Render parses one template string and executes it with data. Convenience for
// inline templates and tests; the build uses New().ParseFiles for real pages.
func Render(text string, data any) ([]byte, error) {
	t, err := New().Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	return buf.Bytes(), nil
}

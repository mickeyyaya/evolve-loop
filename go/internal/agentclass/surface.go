package agentclass

import (
	"cmp"
	"fmt"
)

const (
	argsSchema = "args/1"
	textSchema = "text/1"
)

type Cell struct {
	Cycle     int    `json:"cycle"`
	Phase     string `json:"phase"`
	Workspace string `json:"workspace"`
}

type RequestRef struct {
	Kind   string `json:"kind"`
	Schema string `json:"schema"`
	Path   string `json:"path"`
}

type SurfaceVerb struct {
	Verb       string      `json:"verb"`
	Class      string      `json:"class"`
	Input      string      `json:"input,omitempty"`
	Output     string      `json:"output,omitempty"`
	OnlyWith   string      `json:"only_with,omitempty"`
	DeniedWith string      `json:"denied_with,omitempty"`
	Request    *RequestRef `json:"request,omitempty"`
}

type Capabilities struct {
	Cell         *Cell          `json:"cell"`
	Verbs        []SurfaceVerb  `json:"verbs"`
	DefaultClass string         `json:"default_class"`
	Caps         map[string]any `json:"caps"`
}

func Surface(cell *Cell, output func(path string) string) Capabilities {
	workspace := ""
	if cell != nil {
		workspace = cell.Workspace
	}
	verbs := make([]SurfaceVerb, 0, len(table))
	for _, v := range table {
		row := SurfaceVerb{Verb: v.Path, Class: v.Class}
		switch v.Class {
		case Allowed:
			row.Input, row.Output = argsSchema, cmp.Or(output(v.Path), textSchema)
			row.OnlyWith, row.DeniedWith = v.OnlyWith, v.DeniedWith
		case Request:
			row.Request = &RequestRef{Kind: v.Kind, Schema: RequestSchema, Path: RequestPath(workspace, v.Kind)}
		}
		verbs = append(verbs, row)
	}
	return Capabilities{Cell: cell, Verbs: verbs, DefaultClass: Denied, Caps: map[string]any{}}
}

func Brief(workspace string) string {
	return fmt.Sprintf(`## The evolve verbs of this cell

Run evolve agent capabilities --json first. It gives the class of each evolve verb.
Run a class A verb when it helps your task. For a class R verb, write a request to %s (schema %s). Do not run a class D verb.
`, RequestPath(workspace, "<kind>"), RequestSchema)
}

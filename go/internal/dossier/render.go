package dossier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// RenderJSON serialises d to indented JSON bytes with a trailing newline.
func RenderJSON(d *Dossier) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("dossier: RenderJSON: nil dossier")
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("dossier: RenderJSON: %w", err)
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("dossier: RenderJSON: %w", err)
	}
	return append(raw, '\n'), nil
}

// ParseJSON deserialises raw JSON bytes into a Dossier.
func ParseJSON(data []byte) (*Dossier, error) {
	var d Dossier
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("dossier: ParseJSON: %w", err)
	}
	return &d, nil
}

var markdownTmpl = template.Must(template.New("dossier-md").Parse(`# Cycle {{.Cycle}} Dossier

**Goal:** {{.Goal}}
**Final verdict:** {{.FinalVerdict}}
{{- if .RunID}}
**Run ID:** {{.RunID}}
{{- end}}
{{- if .CommitSHA}}
**Commit:** {{.CommitSHA}}
{{- end}}
{{- if .StartedAt}}
**Started:** {{.StartedAt}}
{{- end}}
{{- if .EndedAt}}
**Ended:** {{.EndedAt}}
{{- end}}

## Phases

| Phase | Verdict | Key Findings |
|-------|---------|--------------|
{{- range .Phases}}
| {{.Name}} | {{.Verdict}} | {{.KeyFindings}} |
{{- end}}
{{- if .Defects}}

## Defects

{{range .Defects}}- **{{.ID}}**{{if .Severity}} ({{.Severity}}){{end}}: {{.Summary}}{{if .Fix}} — fix: {{.Fix}}{{end}}
{{end}}{{- end}}
{{- if .Lessons}}

## Lessons

{{range .Lessons}}- **{{.ID}}**: {{.Pattern}}{{if .PreventiveAction}} — {{.PreventiveAction}}{{end}}
{{end}}{{- end}}
{{- if .Carryover}}

## Carryover

{{range .Carryover}}- **{{.ID}}**{{if .Priority}} ({{.Priority}}){{end}}: {{.Action}}
{{end}}{{- end}}
`))

// RenderMarkdown renders d as a human-readable markdown document.
func RenderMarkdown(d *Dossier) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("dossier: RenderMarkdown: nil dossier")
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("dossier: RenderMarkdown: %w", err)
	}
	var buf bytes.Buffer
	if err := markdownTmpl.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("dossier: RenderMarkdown: %w", err)
	}
	return buf.Bytes(), nil
}

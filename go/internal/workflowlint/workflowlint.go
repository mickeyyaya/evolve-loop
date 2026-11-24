// Package workflowlint checks that YAML files parse and that GitHub
// workflow files keep the keys a runner needs.
// See docs/architecture/packages/internal-workflowlint.md.
package workflowlint

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
)

const workflowDir = ".github/workflows/"

type File struct {
	Path string
	Data []byte
}

type Finding struct {
	Path string
	Line int
	Msg  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Msg)
}

func Check(files []File) []Finding {
	var findings []Finding
	for _, file := range files {
		findings = append(findings, checkFile(file)...)
	}
	return findings
}

func checkFile(file File) []Finding {
	docs, syntax := yamlcodec.DecodeAll(file.Data)
	if syntax != nil {
		return []Finding{{Path: file.Path, Line: syntax.Line, Msg: "invalid YAML: " + syntax.Msg}}
	}
	if !strings.HasPrefix(file.Path, workflowDir) {
		return nil
	}
	if len(docs) == 0 {
		return []Finding{{Path: file.Path, Line: 1, Msg: "workflow file has no document"}}
	}
	var findings []Finding
	for _, doc := range docs {
		findings = append(findings, checkWorkflow(file.Path, doc)...)
	}
	return findings
}

func checkWorkflow(path string, doc yamlcodec.Node) []Finding {
	if doc.Kind() != yamlcodec.KindMapping {
		return []Finding{{Path: path, Line: doc.Line(), Msg: "workflow must be a mapping"}}
	}
	var findings []Finding
	if _, ok := doc.Get("on"); !ok {
		findings = append(findings, Finding{Path: path, Line: doc.Line(), Msg: "missing key on"})
	}
	jobs, ok := doc.Get("jobs")
	if !ok {
		return append(findings, Finding{Path: path, Line: doc.Line(), Msg: "missing key jobs"})
	}
	if jobs.Kind() != yamlcodec.KindMapping || jobs.Len() == 0 {
		return append(findings, Finding{Path: path, Line: jobs.Line(), Msg: "jobs must be a non-empty mapping"})
	}
	for _, name := range jobs.Keys() {
		job, _ := jobs.Get(name)
		if !hasRunner(job) {
			findings = append(findings, Finding{Path: path, Line: job.Line(), Msg: fmt.Sprintf("job %s needs runs-on or uses", name)})
		}
	}
	return findings
}

func hasRunner(job yamlcodec.Node) bool {
	_, runsOn := job.Get("runs-on")
	_, uses := job.Get("uses")
	return runsOn || uses
}

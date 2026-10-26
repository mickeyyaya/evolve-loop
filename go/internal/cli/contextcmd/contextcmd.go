package contextcmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const (
	SchemaIndex = "evolve.context.index/1"
	SchemaOpen  = "evolve.context.open/1"
	SchemaStats = "evolve.context.stats/1"
	exitFailed  = 1
	exitIO      = 2
	exitUsage   = 10
	exitInput   = 11
	usage       = "usage: evolve context index [--layer L1|L2] [--required] [--json] [--workspace W] | open <id> [--json] [--workspace W] | stats [--phase P] [--json] [--project-root R]"
)

type Deps struct {
	Env func(string) string
}

type envError struct {
	Code    int    `json:"code"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type envelope struct {
	Schema   string    `json:"schema"`
	OK       bool      `json:"ok"`
	Data     any       `json:"data"`
	Error    *envError `json:"error"`
	Warnings []string  `json:"warnings"`
}

type failure struct {
	code int
	err  error
}

func (f failure) Error() string { return f.err.Error() }

var kindOfExit = map[int]string{exitFailed: "failed", exitIO: "io", exitUsage: "usage", exitInput: "input"}

type options struct {
	required         bool
	layer, workspace string
	positional       []string
}

func New(d Deps) func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return func(args []string, _ io.Reader, stdout, stderr io.Writer) int {
		asJSON := slices.Contains(args, "--json") || slices.Contains(args, "-json")
		schema, data, err := run(args, d.Env)
		return report(stdout, stderr, asJSON, schema, data, err)
	}
}

func run(args []string, env func(string) string) (string, any, error) {
	if len(args) == 0 {
		return SchemaIndex, nil, failure{exitUsage, errors.New(usage)}
	}
	verb := args[0]
	schema := map[string]string{"index": SchemaIndex, "open": SchemaOpen, "stats": SchemaStats}[verb]
	if schema == "" {
		return SchemaIndex, nil, failure{exitUsage, fmt.Errorf("unknown verb %q; %s", verb, usage)}
	}
	if verb == "stats" {
		data, err := stats(args[1:])
		return schema, data, err
	}
	opts, err := parse(args[1:])
	if err != nil {
		return schema, nil, failure{exitUsage, err}
	}
	root, ws, err := cellOf(opts.workspace, env)
	if err != nil {
		return schema, nil, failure{exitUsage, err}
	}
	if verb == "open" && len(opts.positional) != 1 {
		return schema, nil, failure{exitUsage, fmt.Errorf("open takes one entry id; %s", usage)}
	}
	idx, err := inputindex.Read(ws)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return schema, nil, failure{exitIO, err}
	case err != nil:
		return schema, nil, failure{exitInput, err}
	case verb == "index":
		return schema, filtered(idx, opts), nil
	}
	res, err := open(idx, root, opts.positional[0])
	return schema, res, err
}

type openResult struct {
	Entry inputindex.Entry `json:"entry"`
	Body  string           `json:"body"`
}

func open(idx inputindex.Index, root, id string) (any, error) {
	e, body, err := idx.Open(root, id)
	switch {
	case errors.Is(err, inputindex.ErrUnknownEntry), errors.Is(err, inputindex.ErrOutsideRoot):
		return nil, failure{exitInput, err}
	case errors.Is(err, inputindex.ErrChangedBody):
		return nil, failure{exitFailed, err}
	case err != nil:
		return nil, failure{exitIO, err}
	}
	return openResult{Entry: e, Body: string(body)}, nil
}

func flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("json", false, "")
	return fs
}

func parseAll(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, fmt.Errorf("%w; %s", err, usage)
		}
		args = fs.Args()
		if len(args) > 0 {
			positional, args = append(positional, args[0]), args[1:]
		}
	}
	return positional, nil
}

type statsView struct {
	Stats []inputindex.KindStat `json:"stats"`
}

func stats(args []string) (any, error) {
	fs := flagSet()
	phase := fs.String("phase", "", "")
	root := fs.String("project-root", ".", "")
	if _, err := parseAll(fs, args); err != nil {
		return nil, failure{exitUsage, err}
	}
	records, err := inputindex.ReadAccess(*root)
	if err != nil {
		return nil, failure{exitIO, err}
	}
	return statsView{Stats: inputindex.Stats(records, *phase)}, nil
}

func parse(args []string) (options, error) {
	var o options
	fs := flagSet()
	fs.BoolVar(&o.required, "required", false, "")
	fs.StringVar(&o.layer, "layer", "", "")
	fs.StringVar(&o.workspace, "workspace", "", "")
	positional, err := parseAll(fs, args)
	if err != nil {
		return o, err
	}
	o.positional = positional
	if o.layer != "" && o.layer != inputindex.LayerUpstream && o.layer != inputindex.LayerRecovery {
		return o, fmt.Errorf("layer %q: the layers are L1 and L2", o.layer)
	}
	return o, nil
}

func cellOf(workspace string, env func(string) string) (root, ws string, err error) {
	if workspace != "" {
		ws, _ = filepath.Abs(workspace)
		runs := filepath.Dir(ws)
		if filepath.Base(runs) != "runs" || filepath.Base(filepath.Dir(runs)) != ".evolve" {
			return "", "", fmt.Errorf("the workspace %s is not a run directory <root>/.evolve/runs/<dir>", ws)
		}
		return filepath.Dir(filepath.Dir(runs)), ws, nil
	}
	root = env("EVOLVE_PROJECT_ROOT")
	id, ok := proctree.ParseDispatchID(env(ipcenv.DispatchIDKey))
	if root == "" || !ok {
		return "", "", errors.New("no cell: this process has no dispatch markers; pass --workspace <root>/.evolve/runs/<dir>")
	}
	return root, filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(id.Cycle)), nil
}

type indexView struct {
	inputindex.Index
	Listed int `json:"listed"`
}

func filtered(idx inputindex.Index, o options) indexView {
	out := indexView{Index: idx, Listed: len(idx.Entries)}
	out.Entries = slices.DeleteFunc(slices.Clone(idx.Entries), func(e inputindex.Entry) bool {
		return (o.layer != "" && e.Layer != o.layer) || (o.required && !e.Required)
	})
	return out
}

func report(stdout, stderr io.Writer, asJSON bool, schema string, data any, err error) int {
	var f failure
	code := 0
	if errors.As(err, &f) {
		code = f.code
	}
	if asJSON {
		e := envelope{Schema: schema, OK: code == 0, Data: data, Warnings: []string{}}
		if code != 0 {
			e.Error = &envError{Code: code, Kind: kindOfExit[code], Message: err.Error()}
		}
		raw, _ := json.Marshal(e)
		fmt.Fprintln(stdout, string(raw))
		return code
	}
	if code != 0 {
		fmt.Fprintf(stderr, "evolve context: %v\n", err)
		return code
	}
	writeText(stdout, data)
	return 0
}

func writeText(w io.Writer, data any) {
	switch d := data.(type) {
	case openResult:
		fmt.Fprint(w, d.Body)
	case statsView:
		for _, st := range d.Stats {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\t%.2f\t%d required\n", st.Phase, st.Kind, st.Layer, st.Opened, st.Listed, st.Rate, st.Required)
		}
	case indexView:
		fmt.Fprintf(w, "cycle %d phase %s attempt %d: %d of %d rows\n", d.Cycle, d.Phase, d.Attempt, len(d.Entries), d.Listed)
		for _, e := range d.Entries {
			mark := "-"
			if e.Required {
				mark = "required"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d tokens\t%s\t%s\t%s\n", e.ID, e.Layer, e.Kind, e.Tokens, mark, e.Path, e.Summary)
		}
	}
}

package gatehermetic

import (
	"fmt"
	"go/types"
	"slices"
	"strings"
)

type Step struct {
	Func string
	Edge string
}

type Refusal struct {
	Root  Root
	Class string
	Site  string
	Chain []Step
}

type link struct {
	from string
	kind string
}

type walker struct {
	g     *Graph
	rules Rules
	cache map[string]*facts
}

type rootWalk struct {
	w       *walker
	root    Root
	parent  map[string]link
	queue   []string
	pkgSeen map[string]bool
	live    map[string]bool
	seen    map[string]bool
	out     []Refusal
}

func Refusals(g Graph, rules Rules) ([]Refusal, error) {
	if err := validate(g, rules); err != nil {
		return nil, err
	}
	w := &walker{g: &g, rules: rules, cache: map[string]*facts{}}
	var out []Refusal
	for i, root := range g.roots {
		out = append(out, w.walk(root, g.rootKeys[i])...)
	}
	return out, nil
}

func validate(g Graph, rules Rules) error {
	for _, e := range rules.Exec {
		p := g.program.Packages[e.Package]
		if e.Param == "" || p == nil {
			continue
		}
		if sig := entrySignature(p.Types, e.Name); sig != nil && paramIndex(sig, e.Param) < 0 {
			return fmt.Errorf("gatehermetic: the exec entry %s.%s has no parameter %q in the loaded signature", e.Package, e.Name, e.Param)
		}
	}
	return nil
}

func entrySignature(pkg *types.Package, name string) *types.Signature {
	head, method, isMethod := strings.Cut(name, ".")
	obj := pkg.Scope().Lookup(head)
	if obj != nil && isMethod {
		obj, _, _ = types.LookupFieldOrMethod(obj.Type(), true, pkg, method)
	}
	if obj == nil {
		return nil
	}
	t := obj.Type()
	if _, ok := obj.(*types.TypeName); ok {
		t = t.Underlying()
	}
	sig, _ := t.(*types.Signature)
	return sig
}

func (w *walker) walkable(path string) bool {
	return w.g.pkgs[path] != nil && !slices.Contains(w.rules.Exempt, path) && !slices.Contains(w.rules.Boundary, path)
}

func (w *walker) factsOf(key string) *facts {
	if f, ok := w.cache[key]; ok {
		return f
	}
	src := w.g.sources[key]
	f := &facts{}
	s := &scanner{g: w.g, rules: &w.rules, info: src.pkg.Info, out: f, callsOnly: src.callsOnly, sig: src.sig}
	for _, n := range src.nodes {
		s.scan(n)
	}
	w.cache[key] = f
	return f
}

func (w *walker) walk(root Root, key string) []Refusal {
	r := &rootWalk{
		w: w, root: root, parent: map[string]link{key: {}}, queue: []string{key},
		pkgSeen: map[string]bool{}, live: map[string]bool{}, seen: map[string]bool{},
	}
	for i := 0; i < len(r.queue); i++ {
		r.process(r.queue[i])
	}
	sortRefusals(r.out)
	return r.out
}

func (r *rootWalk) process(key string) {
	g := r.w.g
	f := r.w.factsOf(key)
	r.reachPackage(g.sources[key].pkg.Path, key)
	for _, fd := range f.findings {
		r.refuse(fd.class, g.program.Fset.Position(fd.pos).String(), key)
	}
	for _, e := range f.edges {
		r.enqueue(e.key, e.kind, key)
	}
	for _, v := range f.vars {
		r.reachPackage(v.pkg, key)
		r.enqueue(v.key, "value", key)
	}
	for _, p := range f.pkgs {
		r.reachPackage(p, key)
	}
	for _, t := range f.types {
		r.mention(t, key)
	}
}

func (r *rootWalk) enqueue(key, kind, from string) {
	src, ok := r.w.g.sources[key]
	if _, seen := r.parent[key]; !ok || seen || !r.w.walkable(src.pkg.Path) {
		return
	}
	r.parent[key] = link{from: from, kind: kind}
	r.queue = append(r.queue, key)
}

func (r *rootWalk) reachPackage(path, from string) {
	if !r.w.walkable(path) || r.pkgSeen[path] {
		return
	}
	r.pkgSeen[path] = true
	pi := r.w.g.pkgs[path]
	for _, k := range pi.inits {
		r.enqueue(k, "init", from)
	}
	r.enqueue(pi.pkgInit, "init", from)
	if file := pi.excluded(); file != "" {
		r.refuse(classExec, file+":1:1", from)
	}
	if pi.bodyless.IsValid() {
		r.refuse(classExec, r.w.g.program.Fset.Position(pi.bodyless).String(), from)
	}
}

func (r *rootWalk) mention(t types.Type, from string) {
	switch x := types.Unalias(t).(type) {
	case *types.Named:
		r.liveNamed(x, from)
	case *types.Pointer:
		r.mention(x.Elem(), from)
	case *types.Slice:
		r.mention(x.Elem(), from)
	case *types.Array:
		r.mention(x.Elem(), from)
	case *types.Chan:
		r.mention(x.Elem(), from)
	case *types.Map:
		r.mention(x.Key(), from)
		r.mention(x.Elem(), from)
	case *types.Struct:
		for i := range x.NumFields() {
			r.mention(x.Field(i).Type(), from)
		}
	}
}

func (r *rootWalk) liveNamed(n *types.Named, from string) {
	for i := range n.TypeArgs().Len() {
		r.mention(n.TypeArgs().At(i), from)
	}
	n = n.Origin()
	pkg := n.Obj().Pkg()
	if pkg == nil || !r.w.walkable(pkg.Path()) {
		return
	}
	key := pkg.Path() + "." + n.Obj().Name()
	if r.live[key] {
		return
	}
	r.live[key] = true
	for i := range n.NumMethods() {
		r.enqueue(n.Method(i).FullName(), "live-type", from)
	}
	r.mention(n.Underlying(), from)
}

func (r *rootWalk) refuse(class, site, at string) {
	id := class + "|" + site
	if r.seen[id] {
		return
	}
	r.seen[id] = true
	var steps []Step
	for k := at; ; {
		l := r.parent[k]
		steps = append(steps, Step{Func: k, Edge: l.kind})
		if l.from == "" {
			break
		}
		k = l.from
	}
	slices.Reverse(steps)
	r.out = append(r.out, Refusal{Root: r.root, Class: class, Site: site, Chain: steps})
}

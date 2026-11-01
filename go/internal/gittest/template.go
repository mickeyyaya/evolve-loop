package gittest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
)

const (
	templatePrefix   = "gittest-template-"
	bareTemplateName = "gittest/bare"
)

var templates = &templateRegistry{byName: map[string]*template{}}

var errBuilderAborted = errors.New("the builder stopped its goroutine before it returned")

type templateRegistry struct {
	mu     sync.Mutex
	byName map[string]*template
	sweep  sync.Once
}

type template struct {
	once sync.Once
	dir  string
	err  error
}

type builderFailure struct{ message string }

type builderTB struct{ testing.TB }

func (builderTB) Fatalf(format string, args ...any) {
	panic(builderFailure{message: fmt.Sprintf(format, args...)})
}

func FromTemplate(tb testing.TB, name string, build func(r *Repo) error) *Repo {
	tb.Helper()
	return copyOfTemplate(tb, name, (*Repo).initWorkTree, build)
}

func BareFromTemplate(tb testing.TB) *Repo {
	tb.Helper()
	return copyOfTemplate(tb, bareTemplateName, (*Repo).initBare, func(*Repo) error { return nil })
}

func copyOfTemplate(tb testing.TB, name string, initRepo func(*Repo), build func(r *Repo) error) *Repo {
	tb.Helper()
	tpl := templates.entry(name)
	tpl.once.Do(func() {
		defer func() {
			if tpl.dir == "" && tpl.err == nil {
				tpl.err = errBuilderAborted
			}
		}()
		tpl.dir, tpl.err = templates.build(tb, initRepo, build)
	})
	if tpl.err != nil {
		tb.Fatalf("gittest: template %q: %v", name, tpl.err)
	}
	r := newRepo(tb)
	if err := os.CopyFS(r.Dir, os.DirFS(tpl.dir)); err != nil {
		tb.Fatalf("gittest: copy of template %q: %v", name, err)
	}
	return r
}

func RemoveTemplates() error {
	templates.mu.Lock()
	defer templates.mu.Unlock()
	var errs []error
	for _, tpl := range templates.byName {
		if tpl.dir != "" {
			errs = append(errs, os.RemoveAll(tpl.dir))
		}
	}
	templates.byName = map[string]*template{}
	return errors.Join(errs...)
}

func (g *templateRegistry) entry(name string) *template {
	g.mu.Lock()
	defer g.mu.Unlock()
	tpl, ok := g.byName[name]
	if !ok {
		tpl = &template{}
		g.byName[name] = tpl
	}
	return tpl
}

func (g *templateRegistry) build(tb testing.TB, initRepo func(*Repo), build func(r *Repo) error) (dir string, err error) {
	g.sweep.Do(func() { sweepDeadTemplates(os.TempDir()) })
	if dir, err = os.MkdirTemp("", fmt.Sprintf("%s%d-", templatePrefix, os.Getpid())); err != nil {
		return "", err
	}
	r := &Repo{Dir: dir, tb: builderTB{TB: tb}}
	defer func() {
		switch fail := recover().(type) {
		case nil:
		case builderFailure:
			err = fmt.Errorf("%s", fail.message)
		default:
			err = fmt.Errorf("builder panicked: %v", fail)
		}
	}()
	initRepo(r)
	return dir, build(r)
}

func (r *Repo) Write(rel, body string) {
	r.tb.Helper()
	path := filepath.Join(r.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.tb.Fatalf("gittest: create directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		r.tb.Fatalf("gittest: write %s: %v", rel, err)
	}
}

func sweepDeadTemplates(tmp string) {
	stale, _ := filepath.Glob(filepath.Join(tmp, templatePrefix+"*"))
	for _, dir := range stale {
		pidText, _, _ := strings.Cut(strings.TrimPrefix(filepath.Base(dir), templatePrefix), "-")
		pid, err := strconv.Atoi(pidText)
		if err != nil || processAlive(pid) {
			continue
		}
		_ = os.RemoveAll(dir)
	}
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	return err == nil && signalSaysAlive(proc.Signal(syscall.Signal(0)))
}

func signalSaysAlive(err error) bool {
	return err == nil || errors.Is(err, syscall.EPERM)
}

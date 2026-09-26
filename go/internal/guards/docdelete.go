package guards

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// DocDelete denies Bash rm and mv commands that would remove content from docs/ or knowledge-base/.
// Its one exception is a Build retracting the active cycle's own explanation draft that was never
// committed.
type DocDelete struct {
	allow   bool
	storage core.Storage
}

// NewDocDelete returns a DocDelete guard; allow (workflow.allow_doc_delete) disables it, and storage
// names the active cycle whose draft may be retracted.
func NewDocDelete(allow bool, storage core.Storage) *DocDelete {
	return &DocDelete{allow: allow, storage: storage}
}

// Name reports "docdelete".
func (d *DocDelete) Name() string { return "docdelete" }

const docRoot = `(docs|knowledge-base)`

var (
	rmDocsRe   = regexp.MustCompile(`(?m)\brm\b[^\n]*(^|[\s'"/{,():])` + docRoot + `([/\s'"},)]|$)`)
	cdDocsRe   = regexp.MustCompile(`^(cd|pushd)\s(|[^\n]*[\s'"/])` + docRoot + `([/\s'"]|$)`)
	docDirRe   = regexp.MustCompile(`(^|/)` + docRoot + `(/|$)`)
	mvDocsRe   = regexp.MustCompile(`(?m)\bmv\b[ \t]+([^\s]+)[ \t]+([^\s]+)`)
	docPathRe  = regexp.MustCompile(`(^|/)` + docRoot + `/`)
	docsDestRe = regexp.MustCompile(`(^|/)docs/`)
)

const archiveAdvice = "archive committed content with `git mv <file> docs/private/research/archived-YYYY-MM-DD/<file>`, which keeps the archived copy staged"

// Decide denies an rm that could remove content under docs/ or knowledge-base/ (removesUnderADocRoot),
// unless it retracts only the active cycle's own explanation draft, and denies an mv of doc content
// outside docs/. The doc roots match case-folded, because a case-insensitive filesystem resolves DOCS/ to
// docs/. A doc root anywhere on an rm's line counts, even in another command: a command substitution can
// feed the rm a path that only another command names.
func (d *DocDelete) Decide(ctx context.Context, in core.GuardInput) core.GuardDecision {
	if d.allow || in.ToolName != "Bash" {
		return core.GuardDecision{Allow: true}
	}
	cmd := cmdString(in)
	if cmd == "" {
		return core.GuardDecision{Allow: true}
	}
	folded := strings.ToLower(cmd)
	if d.removesUnderADocRoot(ctx, in.CWD, folded) && !d.retractsOwnDraft(ctx, in.CWD, cmd) {
		return core.GuardDecision{
			Allow:  false,
			Reason: "rm against docs/ or knowledge-base/ is forbidden — " + archiveAdvice + "; a Build may `git rm -f` its cycle's own explanation draft that was never committed; set workflow.allow_doc_delete=true to bypass",
		}
	}
	for _, m := range mvDocsRe.FindAllStringSubmatch(cmd, -1) {
		if src, dst := m[1], m[2]; isDocPath(strings.ToLower(src)) && !isDocsDest(dst) {
			return core.GuardDecision{
				Allow:  false,
				Reason: "mv out of docs//knowledge-base is a deletion — " + archiveAdvice,
			}
		}
	}
	return core.GuardDecision{Allow: true}
}

// removesUnderADocRoot reports an rm that names a doc root, or an rm or mv that runs inside one: entered
// with cd, pushd or git -C, or already the shell's working directory. Operands relative to a doc root never
// name it.
func (d *DocDelete) removesUnderADocRoot(ctx context.Context, cwd, folded string) bool {
	if rmDocsRe.MatchString(folded) {
		return true
	}
	entered, removes := false, false
	for _, c := range splitShellCommands(folded) {
		if cdDocsRe.MatchString(c.text) {
			entered = true
		}
		if removesOrMoves(c.words) {
			if entered || gitDirIsADocRoot(c.words) {
				return true
			}
			removes = true
		}
	}
	return removes && cwdInsideADocRoot(ctx, cwd)
}

func cwdInsideADocRoot(ctx context.Context, cwd string) bool {
	if cwd == "" {
		return false
	}
	prefix, err := gitexec.Default(cwd).Output(ctx, "rev-parse", "--show-prefix")
	return err == nil && docDirRe.MatchString("/"+strings.ToLower(prefix))
}

func removesOrMoves(words []string) bool {
	if _, isRm := rmArgs(words); isRm || (len(words) > 0 && words[0] == "mv") {
		return true
	}
	if len(words) < 2 || words[0] != "git" {
		return false
	}
	sub := gitSubcommand(words[1:])
	return sub == "rm" || sub == "mv"
}

// gitDirIsADocRoot reports `git -C <dir>` naming a doc root; the words are case-folded, so -C reads -c.
func gitDirIsADocRoot(words []string) bool {
	if len(words) == 0 || words[0] != "git" {
		return false
	}
	for i := 1; i+1 < len(words); i++ {
		if words[i] == "-c" && docDirRe.MatchString(words[i+1]) {
			return true
		}
	}
	return false
}

// retractsOwnDraft reports whether every rm in cmd removes only the active cycle's explanation draft,
// which HEAD never held. The host fixes the draft's name (explanationdocs.DocumentPath), so the proof
// compares one exact path instead of interpreting the shell: an expansion, a pathspec or another
// spelling is not that path.
func (d *DocDelete) retractsOwnDraft(ctx context.Context, cwd, cmd string) bool {
	draft, ok := d.activeDraft(ctx)
	if !ok {
		return false
	}
	removes := false
	for _, c := range splitShellCommands(cmd) {
		operands, isRm := rmArgs(c.words)
		if !isRm {
			if rmDocsRe.MatchString(strings.ToLower(c.text)) {
				return false
			}
			continue
		}
		for _, o := range operands {
			if o != draft {
				return false
			}
			removes = true
		}
	}
	return removes && neverCommitted(ctx, cwd, draft)
}

func (d *DocDelete) activeDraft(ctx context.Context) (string, bool) {
	if d.storage == nil {
		return "", false
	}
	cs, err := d.storage.ReadCycleState(ctx)
	if err != nil {
		return "", false
	}
	draft, err := explanationdocs.DocumentPath(cs.CycleID, cs.RunID)
	return draft, err == nil
}

// rmArgs returns the operands of a plain `rm` or `git rm`, and false for any other command.
func rmArgs(words []string) ([]string, bool) {
	switch {
	case len(words) > 0 && words[0] == "rm":
		words = words[1:]
	case len(words) > 1 && words[0] == "git" && words[1] == "rm":
		words = words[2:]
	default:
		return nil, false
	}
	var operands []string
	for i, w := range words {
		if w == "--" {
			return append(operands, words[i+1:]...), true
		}
		if !strings.HasPrefix(w, "-") {
			operands = append(operands, w)
		}
	}
	return operands, true
}

// neverCommitted reports whether cwd is the repository root, HEAD holds nothing named draft (compared
// case-folded), and every directory on draft's path is a real directory: a symlinked parent would resolve
// the name somewhere else.
func neverCommitted(ctx context.Context, cwd, draft string) bool {
	if cwd == "" {
		return false
	}
	git := gitexec.Default(cwd)
	if prefix, err := git.Output(ctx, "rev-parse", "--show-prefix"); err != nil || prefix != "" {
		return false
	}
	listing, err := git.Output(ctx, "ls-tree", "-r", "-z", "--name-only", "--full-tree", "HEAD")
	if err != nil {
		return false
	}
	folded := strings.ToLower(draft)
	for _, p := range strings.Split(strings.ToLower(listing), "\x00") {
		if p == folded || strings.HasPrefix(p, folded+"/") {
			return false
		}
	}
	for dir := path.Dir(draft); dir != "."; dir = path.Dir(dir) {
		info, err := os.Lstat(filepath.Join(cwd, filepath.FromSlash(dir)))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func isDocPath(p string) bool {
	return docPathRe.MatchString(p)
}

// isDocsDest reports whether an mv destination stays under docs/, the single documentation root.
// knowledge-base/ is not a destination: its research/ subtree is retired and cycles/ is runtime state.
func isDocsDest(p string) bool {
	return docsDestRe.MatchString(p)
}

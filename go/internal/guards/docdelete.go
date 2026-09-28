package guards

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// DocDelete denies Bash commands that would remove content from docs/ or knowledge-base/, or move it out.
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

const docRoot = `(?i:docs|knowledge-base)`

var (
	rmDocsRe      = regexp.MustCompile(`(?im)\brm\b[^\n]*(^|[\s'"/{,():])` + docRoot + `([/\s'"},)]|$)`)
	cdDocsRe      = regexp.MustCompile(`(?i)^(cd|pushd)\s(|[^\n]*[\s'"/])` + docRoot + `([/\s'"]|$)`)
	docDirRe      = regexp.MustCompile(`(^|/)` + docRoot + `(/|$)`)
	wordDocRootRe = regexp.MustCompile(`(^|[/:(){,])` + docRoot + `([/,}]|$)`)
	mvDocsRe      = regexp.MustCompile(`(?im)\bmv\b[ \t]+([^\s]+)[ \t]+([^\s]+)`)
	docPathRe     = regexp.MustCompile(`(^|/)` + docRoot + `/`)
	docsDestRe    = regexp.MustCompile(`(^|/)docs/`)
)

const archiveAdvice = "archive committed content with `git mv <file> docs/private/research/archived-YYYY-MM-DD/<file>`, which keeps the archived copy staged"

// Decide denies a removal that could reach content under docs/ or knowledge-base/ (removesUnderADocRoot),
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
	cmds := splitShellCommands(cmd)
	if d.removesUnderADocRoot(ctx, in.CWD, cmd, cmds) && !d.retractsOwnDraft(ctx, in.CWD, cmds) {
		return core.GuardDecision{
			Allow:  false,
			Reason: "rm against docs/ or knowledge-base/ is forbidden — " + archiveAdvice + "; a Build may `git rm -f` its cycle's own explanation draft that was never committed; set workflow.allow_doc_delete=true to bypass",
		}
	}
	if movesDocContentOut(cmd, cmds) {
		return core.GuardDecision{
			Allow:  false,
			Reason: "mv out of docs//knowledge-base is a deletion — " + archiveAdvice,
		}
	}
	return core.GuardDecision{Allow: true}
}

func movesDocContentOut(cmd string, cmds []shellCommand) bool {
	for _, m := range mvDocsRe.FindAllStringSubmatch(cmd, -1) {
		if src, dst := m[1], m[2]; isDocPath(src) && !isDocsDest(dst) {
			return true
		}
	}
	for _, c := range cmds {
		if anyCommand(c.words, movesOutOfDocs) {
			return true
		}
	}
	return false
}

func movesOutOfDocs(name string, args []string) bool {
	srcs, dst, ok := moveOperands(name, args)
	return ok && namesADocRoot(srcs) && !isDocsDest(dst)
}

func (d *DocDelete) removesUnderADocRoot(ctx context.Context, cwd, cmd string, cmds []shellCommand) bool {
	if rmDocsRe.MatchString(cmd) {
		return true
	}
	entered, touches := false, false
	for _, c := range cmds {
		if cdDocsRe.MatchString(c.text) {
			entered = true
		}
		deletes := removes(c.words)
		if !deletes && !moves(c.words) {
			continue
		}
		if entered || gitDirIsADocRoot(c.words) || (deletes && namesADocRoot(c.words[1:])) {
			return true
		}
		touches = true
	}
	return touches && cwdInsideADocRoot(ctx, cwd)
}

func namesADocRoot(words []string) bool {
	for _, w := range words {
		if wordDocRootRe.MatchString(w) {
			return true
		}
	}
	return false
}

func cwdInsideADocRoot(ctx context.Context, cwd string) bool {
	if cwd == "" {
		return false
	}
	prefix, err := gitexec.Default(cwd).Output(ctx, "rev-parse", "--show-prefix")
	return err == nil && docDirRe.MatchString("/"+strings.ToLower(prefix))
}

func removes(words []string) bool {
	return anyCommand(words, removesAs)
}

func removesAs(name string, args []string) bool {
	switch name {
	case "rm", "unlink":
		return true
	case "find":
		return slices.Contains(args, "-delete") || execsARemoval(args)
	case "git":
		sub := gitSubcommand(args)
		return sub == "rm" || sub == "clean"
	}
	return false
}

var findExecActions = map[string]bool{"-exec": true, "-execdir": true, "-ok": true, "-okdir": true}

func execsARemoval(args []string) bool {
	for i := 1; i < len(args); i++ {
		if !findExecActions[args[i-1]] {
			continue
		}
		if anyCommand(args[i:], isRemovalProgram) {
			return true
		}
	}
	return false
}

func isRemovalProgram(name string, _ []string) bool {
	return name == "rm" || name == "unlink"
}

func moves(words []string) bool {
	return anyCommand(words, isMoveProgram)
}

func isMoveProgram(name string, args []string) bool {
	_, ok := moveArgs(name, args)
	return ok
}

func moveArgs(name string, args []string) ([]string, bool) {
	switch name {
	case "mv":
		return args, true
	case "git":
		if i := gitSubcommandIndex(args); i >= 0 && args[i] == "mv" {
			return args[i+1:], true
		}
	}
	return nil, false
}

func moveOperands(name string, words []string) (srcs []string, dst string, ok bool) {
	args, ok := moveArgs(name, words)
	if !ok {
		return nil, "", false
	}
	var operands []string
	for i := 0; i < len(args); i++ {
		switch w := args[i]; {
		case w == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(w, "-") && w != "-":
			target, width := moveOption(args[i:])
			if target != "" {
				dst = target
			}
			i += width - 1
		default:
			operands = append(operands, w)
		}
	}
	if dst != "" {
		return operands, dst, len(operands) > 0
	}
	if len(operands) < 2 {
		return nil, "", false
	}
	return operands[:len(operands)-1], operands[len(operands)-1], true
}

func moveOption(args []string) (target string, width int) {
	w := args[0]
	if strings.HasPrefix(w, "--") {
		return moveLongOption(args)
	}
	for j := 1; j < len(w); j++ {
		switch w[j] {
		case 't':
			return optionValue(w[j+1:], args)
		case 'S':
			_, width = optionValue(w[j+1:], args)
			return "", width
		}
	}
	return "", 1
}

func moveLongOption(args []string) (target string, width int) {
	name, value, hasValue := strings.Cut(args[0][2:], "=")
	isTarget := isLongOption(name, "target-directory", 1)
	switch {
	case hasValue && isTarget:
		return value, 1
	case hasValue:
		return "", 1
	case isTarget:
		return optionValue("", args)
	case isLongOption(name, "suffix", 2):
		_, width = optionValue("", args)
		return "", width
	}
	return "", 1
}

func isLongOption(name, option string, minLength int) bool {
	return len(name) >= minLength && strings.HasPrefix(option, name)
}

func optionValue(attached string, args []string) (string, int) {
	switch {
	case attached != "":
		return attached, 1
	case len(args) > 1:
		return args[1], 2
	}
	return "", 1
}

func gitDirIsADocRoot(words []string) bool {
	return anyCommand(words, gitDirIsADocRootAs)
}

func gitDirIsADocRootAs(name string, args []string) bool {
	if name != "git" {
		return false
	}
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-C" && docDirRe.MatchString(args[i+1]) {
			return true
		}
	}
	return false
}

// retractsOwnDraft reports whether every rm in cmd removes only the active cycle's explanation draft,
// which HEAD never held. The host fixes the draft's name (explanationdocs.DocumentPath), so the proof
// compares one exact path instead of interpreting the shell: an expansion, a pathspec or another
// spelling is not that path.
func (d *DocDelete) retractsOwnDraft(ctx context.Context, cwd string, cmds []shellCommand) bool {
	draft, ok := d.activeDraft(ctx)
	if !ok {
		return false
	}
	retracts := false
	for _, c := range cmds {
		operands, isRm := rmArgs(c.words)
		if !isRm {
			if removes(c.words) || rmDocsRe.MatchString(c.text) {
				return false
			}
			continue
		}
		for _, o := range operands {
			if o != draft {
				return false
			}
			retracts = true
		}
	}
	return retracts && neverCommitted(ctx, cwd, draft)
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

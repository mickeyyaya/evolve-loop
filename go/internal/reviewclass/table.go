package reviewclass

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
)

const (
	seatSimplifierName   = "code-simplifier"
	seatCodeName         = "code-reviewer"
	seatGoName           = "go-reviewer"
	seatArchitectureName = "architecture-reviewer"
	seatDocsName         = "docs-review"
	seatSecurityName     = "security"
	seatAdversarialName  = "adversarial-review"
	lensGoTests          = "golang-test-review"
	planSeats            = "S6"
)

var (
	simplifierSeat   = Seat{Any: []string{seatSimplifierName}}
	goOrCodeSeat     = Seat{Any: []string{seatGoName, seatCodeName}}
	architectureSeat = Seat{Any: []string{seatArchitectureName}}
	docsSeat         = Seat{Any: []string{seatDocsName}}
	codeSeat         = Seat{Any: []string{seatCodeName}}
	goTestsSeat      = Seat{Any: []string{seatGoName}, Lens: lensGoTests}
	securitySeat     = Seat{Any: []string{seatSecurityName}, Planned: planSeats}
	adversarialSeat  = Seat{Any: []string{seatAdversarialName}, Tier: TierDeep, OtherFamily: true, Planned: planSeats}
)

var (
	commentEquivalence  = Check{ID: "comment-equivalence", Rule: "commentaudit.Equivalent proves each changed Go file comment-only"}
	steLint             = Check{ID: "ste-lint", Rule: "evolve docs ste-lint --strict --added"}
	docsIndexCheck      = Check{ID: "docs-index-check", Rule: "evolve docs index check"}
	committedDocGuard   = Check{ID: "committed-doc-guard", Rule: "the committed-doc deletion guard refuses a deleted document"}
	inboxLint           = Check{ID: "inbox-lint", Rule: "evolve inbox lint"}
	inboxIntegrity      = Check{ID: "inbox-integrity", Rule: "each removed pending item has a consumed, claimed or moved copy, or the commit names it in Closes-Inbox", Planned: "S2"}
	derivedCheck        = Check{ID: "derived-check", Rule: "the Check verb of each derived.Catalog entry whose output changed"}
	floorListRegression = Check{ID: "floor-list-regression", Rule: "the floor lists drop no package and lower no floor (core/floor_lists_regression.go)"}
	goTestChecks        = Check{ID: "go-checks", Rule: "gofmt, go vet, golangci-lint and go test -count=1 of the changed packages"}
	testFlags           = Check{ID: "test-flags", Rule: "gateclass flags removed-guard-test and added-skip are 0"}
	lanes               = Check{ID: "lanes", Rule: "the CI test lanes of the changed packages"}
	coverChanged        = Check{ID: "cover-changed", Rule: "cover-changed of the changed packages"}
	apicover            = Check{ID: "apicover", Rule: "apicover -enforce"}
	personaStrip        = Check{ID: "persona-strip", Rule: "no directive under the reference sections of a persona", Planned: "S2"}
	skillsCheck         = Check{ID: "skills-check", Rule: "evolve skills check"}
	guidanceCheck       = Check{ID: "guidance-check", Rule: "evolve guidance check"}
	profileLoad         = Check{ID: "profile-load", Rule: "each changed profile and phase spec loads"}
	permissionWidening  = Check{ID: "permission-widening", Rule: "a wider allowed_tools, write_subpaths or env_passthrough names a consumer", Planned: "S2"}
	requiredNeeds       = Check{ID: "required-needs", Rule: "each new job is in the needs of the required aggregator", Planned: "S5"}
	ciRoutesGenerated   = Check{ID: "ci-routes-generated", Rule: "the CI path routes are generated from this registry", Planned: "S5"}
	gateFlags           = Check{ID: "gate-flags", Rule: "each gateclass.Classify flag is 0 or acknowledged by Gate-change: <pattern> <reason>"}
	guardMutation       = Check{ID: "guard-mutation", Rule: "diff-scoped mutation of the changed guard lines, with the signal sender injected", Planned: "S11"}
	operatorApproved    = Check{ID: "operator-approved", Rule: "refuse unless the commit has Operator-approved: <item>", Planned: "S4"}
)

var (
	goCodeSeats  = []Seat{simplifierSeat, goOrCodeSeat}
	goT1Seats    = []Seat{simplifierSeat, goOrCodeSeat, architectureSeat}
	t0Seats      = []Seat{simplifierSeat, goOrCodeSeat, architectureSeat, adversarialSeat}
	goCodeChecks = []Check{lanes, coverChanged, apicover}
	t0Checks     = []Check{lanes, coverChanged, apicover, gateFlags, guardMutation}
	securityWhen = []string{"sandbox-rule", "dispatch-credential", "policy-parser"}
)

var (
	gateInputs    = []string{"go/.apicover-enforce", "go/.cover-strict", "go/.darwin-set", "go/.test-latency.json"}
	personaDirs   = []string{"skills/", "guidance/rules/"}
	promptDirs    = []string{"agents/", ".evolve/phases/"}
	notDocDirs    = []string{"agents/", "skills/", "guidance/", ".evolve/phases/"}
	packageDirs   = []string{"go/internal/", "go/cmd/"}
	manifestGlobs = []string{".evolve/profiles/*.json", ".evolve/phases/*/*.json", "go/internal/bridge/manifests/*.json"}
	manifestDirs  = []string{".claude-plugin/"}
	inboxDirs     = []string{".evolve/inbox/"}
	workflowDirs  = []string{".github/workflows/"}
)

var table = []Class{
	{ID: C0, Name: "Comment-only", Detect: "a Go file that commentaudit.Equivalent proves comment-only; it replaces the other classes of the file (rule R4)", Checks: []Check{commentEquivalence}, Cap: "$0"},
	{ID: C1, Name: "Docs", Detect: "*.md outside " + listOf(notDocDirs) + ", and not a whole generated file", Seats: []Seat{docsSeat}, Checks: []Check{steLint, docsIndexCheck, committedDocGuard}, Cap: "$1", match: onPath(isDoc)},
	{ID: C2, Name: "Inbox data", Detect: "under " + listOf(inboxDirs), Checks: []Check{inboxLint, inboxIntegrity}, Cap: "$0", match: onPath(underAny(inboxDirs))},
	{ID: C3, Name: "Generated outputs", Detect: "an output of a derived.Catalog entry", Checks: []Check{derivedCheck}, Cap: "$0", match: onPath(isGenerated)},
	{ID: C4, Name: "Gate inputs", Detect: "one of " + listOf(gateInputs), Seats: []Seat{codeSeat}, Checks: []Check{floorListRegression}, Cap: "$1", match: onPath(isGateInput)},
	{ID: C5, Name: "Go tests only", Detect: "*_test.go, or a file under a testdata/ directory in go/, that gateclass.IsGuardTest does not name", Seats: []Seat{goTestsSeat}, Checks: []Check{goTestChecks, testFlags}, Cap: "$1", match: onPath(isTestOnly)},
	{ID: C6, Name: "Go T2", Detect: "a Go file, or a file under " + listOf(packageDirs) + ", that is not test-only and has gateclass.TierOf T2", Seats: goCodeSeats, Checks: goCodeChecks, Cap: "$2", match: onPath(isCodeOf(gateclass.T2))},
	{ID: C7, Name: "Go T1", Detect: "a Go file, or a file under " + listOf(packageDirs) + ", that is not test-only and has gateclass.TierOf T1", Seats: goT1Seats, Checks: goCodeChecks, Cap: "$4", match: onPath(isCodeOf(gateclass.T1))},
	{ID: C8, Name: "Persona or prompt", Detect: "*.md under " + listOf(promptDirs) + ", or a file under " + listOf(personaDirs), Seats: []Seat{architectureSeat, docsSeat}, Checks: []Check{personaStrip, skillsCheck, guidanceCheck, steLint}, Cap: "$3", match: onPath(isPersona)},
	{ID: C9, Name: "Manifests and profiles", Detect: "a match of " + listOf(manifestGlobs) + ", or a file under " + listOf(manifestDirs), Seats: []Seat{codeSeat, securitySeat}, Checks: []Check{profileLoad, skillsCheck, permissionWidening}, Cap: "$2", match: onPath(isManifest)},
	{ID: C10, Name: "CI workflows", Detect: "under " + listOf(workflowDirs), Seats: []Seat{codeSeat, architectureSeat}, Checks: []Check{requiredNeeds, ciRoutesGenerated}, Cap: "$3", match: onPath(underAny(workflowDirs))},
	{ID: C11, Name: "T0 protected", Detect: "the Protected predicate of the Surface (guards.IsProtectedSurface in production)", Seats: t0Seats, SecurityWhen: securityWhen, Checks: t0Checks, Cap: "about $5.6 to $6.8", match: Surface.isProtected},
	{ID: C12, Name: "Stage flip", Detect: "a gateclass stage-off-enforce flag in the file", Seats: t0Seats, SecurityWhen: securityWhen, Checks: []Check{operatorApproved}, Cap: "C11"},
	{ID: C13, Name: "Default", Detect: "a changed path that matches no other class (fail closed)", Seats: goCodeSeats, Checks: goCodeChecks, Cap: "$2"},
}

var knownDefaults = []string{
	"knowledge-base/",
	"docs/architecture/phase-descriptor.schema.json",
	"docs/architecture/phase-registry.json",
	"docs/assets/",
	"docs/explain/",
	"docs/private/",
	"docs/reports/",
	"docs/research/",
	".agents/skills/",
	".evolve/commit-prefix-scope.json",
	".evolve/inbox-parked/",
	".evolve/instincts/",
	".evolve/naming.json",
	".evolve/phase-inventory.json",
	".evolve/plugin/",
	"adapters/",
	"bin/",
	"examples/",
	"guidance/index.json",
	"guidance/index.md",
	"landing/",
	"schemas/",
	"go/.cover-split",
	"go/.docgo-allow-missing",
	"go/.gitignore",
	"go/.golangci.yml",
	"go/Makefile",
	"go/acs/",
	"go/evolve",
	"go/go.mod",
	"go/go.sum",
	"go/vendor/",
	"go/vulncheck-allowlist.json",
	".editorconfig",
	".gitattributes",
	".gitignore",
	".goreleaser.yml",
	"LICENSE",
	"NOTICE",
	"eslint.config.mjs",
	"install.sh",
}

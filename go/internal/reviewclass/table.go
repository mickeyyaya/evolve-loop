package reviewclass

import "github.com/mickeyyaya/evolve-loop/go/internal/guards"

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
	securityWhen = []guards.GuaranteeClass{guards.SandboxGuarantee, guards.DispatchCredentialGuarantee, guards.PolicyGuarantee}
)

var table = []Class{
	{ID: C0, Name: "Comment-only", Detect: "a Go file that commentaudit.Equivalent proves comment-only (rule R4)", Checks: []Check{commentEquivalence}, Cap: "$0"},
	{ID: C1, Name: "Docs", Detect: "*.md outside agents/, skills/ and guidance/, and not a whole generated file", Seats: []Seat{docsSeat}, Checks: []Check{steLint, docsIndexCheck, committedDocGuard}, Cap: "$1"},
	{ID: C2, Name: "Inbox data", Detect: ".evolve/inbox/**", Checks: []Check{inboxLint, inboxIntegrity}, Cap: "$0"},
	{ID: C3, Name: "Generated outputs", Detect: "an output of a derived.Catalog entry", Checks: []Check{derivedCheck}, Cap: "$0"},
	{ID: C4, Name: "Gate inputs", Detect: "go/.apicover-enforce, go/.cover-strict, go/.darwin-set, go/.test-latency.json", Seats: []Seat{codeSeat}, Checks: []Check{floorListRegression}, Cap: "$1"},
	{ID: C5, Name: "Go tests only", Detect: "*_test.go or go/**/testdata/, not a guard test (gateclass.IsGuardTest)", Seats: []Seat{goTestsSeat}, Checks: []Check{goTestChecks, testFlags}, Cap: "$1"},
	{ID: C6, Name: "Go T2", Detect: "Go code or a guard test of tier T2 (gateclass.TierOf)", Seats: goCodeSeats, Checks: goCodeChecks, Cap: "$2"},
	{ID: C7, Name: "Go T1", Detect: "a file under go/ of tier T1 (core, bridge, cliroute, gc) that is not test-only: Go code, a guard test or a data file", Seats: goT1Seats, Checks: goCodeChecks, Cap: "$4"},
	{ID: C8, Name: "Persona or prompt", Detect: "agents/*.md, skills/**, guidance/rules/**", Seats: []Seat{architectureSeat, docsSeat}, Checks: []Check{personaStrip, skillsCheck, guidanceCheck, steLint}, Cap: "$3"},
	{ID: C9, Name: "Manifests and profiles", Detect: ".evolve/profiles/*.json, .evolve/phases/**.json, go/internal/bridge/manifests/*.json, .claude-plugin/**", Seats: []Seat{codeSeat, securitySeat}, Checks: []Check{profileLoad, skillsCheck, permissionWidening}, Cap: "$2"},
	{ID: C10, Name: "CI workflows", Detect: ".github/workflows/**", Seats: []Seat{codeSeat, architectureSeat}, Checks: []Check{requiredNeeds, ciRoutesGenerated}, Cap: "$3"},
	{ID: C11, Name: "T0 protected", Detect: "guards.IsProtectedSurface", Seats: t0Seats, SecurityWhen: securityWhen, Checks: t0Checks, Cap: "about $5.6 to $6.8"},
	{ID: C12, Name: "Stage flip", Detect: "a gateclass stage-off-enforce flag", Seats: t0Seats, SecurityWhen: securityWhen, Checks: []Check{operatorApproved}, Cap: "C11"},
	{ID: C13, Name: "Default", Detect: "a changed path that matches no other class (fail closed)", Seats: goCodeSeats, Checks: goCodeChecks, Cap: "$2"},
}

var gateInputs = []string{"go/.apicover-enforce", "go/.cover-strict", "go/.darwin-set", "go/.test-latency.json"}

var knownDefaults = []string{
	"knowledge-base/",
	"docs/",
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
	"go/cmd/evolve/templates/",
	"go/evolve",
	"go/go.mod",
	"go/go.sum",
	"go/internal/",
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

package overlap

type Tier string

const (
	T1 Tier = "T1"
	T2 Tier = "T2"
	T3 Tier = "T3"
	T4 Tier = "T4"
)

type Rule string

const (
	RuleConflict           Rule = "conflict"
	RuleBaseNotAncestor    Rule = "base_not_ancestor"
	RuleAuditedTreeMissing Rule = "audited_tree_missing"
	RuleBookkeepingPeer    Rule = "bookkeeping_peer"
	RuleEmptyPeer          Rule = "empty_peer"
	RuleCompile            Rule = "compile"
	RuleSharedPath         Rule = "shared_path"
	RulePackageEdge        Rule = "package_edge"
	RuleBuildZone          Rule = "build_zone"
	RuleUnknown            Rule = "unknown"
	RuleDerived            Rule = "derived"
	RuleDisjoint           Rule = "disjoint"
)

type MergeClass string

const (
	MergeClean           MergeClass = "clean"
	MergeDerivedConflict MergeClass = "derived"
	MergeGenuineConflict MergeClass = "genuine"
)

type Side string

const (
	SideLane Side = "lane"
	SidePeer Side = "peer"
)

type Catalogs interface {
	Bookkeeping(path string) bool
	BuildZone(path string) bool
	GateZone(path string) bool
	DerivedOutput(side Side, path string) bool
	DataRead(path string) bool
	Fired(lane, peer, conflicted []string) []string
}

type Package struct {
	ImportPath string   `json:"import_path"`
	Dir        string   `json:"dir"`
	Files      []string `json:"files"`
	Deps       []string `json:"deps"`
}

type Module struct {
	Path     string    `json:"path"`
	Packages []Package `json:"packages"`
}

type BlobPair struct {
	Lane string `json:"lane"`
	Peer string `json:"peer"`
}

type Input struct {
	Lane               []string
	Peer               []string
	Merge              MergeClass
	Conflicted         []string
	BaseNotAncestor    bool
	AuditedTreeMissing bool
	CompileRed         bool
	DeletedAtC         []string
	Module             Module
	Failures           []string
	Catalogs           Catalogs
	Blobs              map[string]BlobPair
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Evidence struct {
	SharedPaths     []string `json:"shared_paths"`
	EdgesLaneToPeer []Edge   `json:"edges_lane_to_peer"`
	EdgesPeerToLane []Edge   `json:"edges_peer_to_lane"`
	BuildZone       []string `json:"build_zone"`
	GateZone        []string `json:"gate_zone"`
	DataEdges       []string `json:"data_edges"`
	Derived         []string `json:"derived"`
	Unknown         []string `json:"unknown"`
}

type Selection struct {
	LanePackages []string `json:"lane_packages"`
	PeerPackages []string `json:"peer_packages"`
}

type Proof struct {
	Tier           Tier      `json:"tier"`
	Rules          []Rule    `json:"rules"`
	Evidence       Evidence  `json:"evidence"`
	EvidenceDigest string    `json:"evidence_digest"`
	Selection      Selection `json:"selection"`
}

package overlap

func Prove(in Input) Proof {
	if rules := preconditionRules(in); len(rules) > 0 {
		return Proof{Tier: T4, Rules: rules}
	}
	idx := newModuleIndex(in.Module, in.DeletedAtC)
	lane := classifySide(in.Catalogs, idx, SideLane, in.Lane)
	peer := classifySide(in.Catalogs, idx, SidePeer, in.Peer)
	ev := gatherEvidence(in, idx, lane, peer)
	proof := Proof{
		Evidence:       ev,
		EvidenceDigest: evidenceDigest(ev, edgePeerPaths(peer, ev), in.Blobs),
		Selection:      Selection{LanePackages: sortedKeys(lane.pkgs), PeerPackages: sortedKeys(peer.pkgs)},
	}
	proof.Tier, proof.Rules = decide(in, peer, ev)
	return proof
}

func preconditionRules(in Input) []Rule {
	var rules []Rule
	if in.Merge == MergeGenuineConflict {
		rules = append(rules, RuleConflict)
	}
	if in.BaseNotAncestor {
		rules = append(rules, RuleBaseNotAncestor)
	}
	if in.AuditedTreeMissing {
		rules = append(rules, RuleAuditedTreeMissing)
	}
	return rules
}

func decide(in Input, peer sideView, ev Evidence) (Tier, []Rule) {
	if len(peer.paths) == 0 {
		return T1, []Rule{RuleEmptyPeer}
	}
	if peer.onlyBookkeeping() {
		return T1, []Rule{RuleBookkeepingPeer}
	}
	if in.CompileRed {
		return T4, []Rule{RuleCompile}
	}
	return stepThree(ev)
}

func stepThree(ev Evidence) (Tier, []Rule) {
	checks := []struct {
		rule  Rule
		tier  Tier
		fired bool
	}{
		{RuleSharedPath, T3, len(ev.SharedPaths) > 0},
		{RulePackageEdge, T3, len(ev.EdgesLaneToPeer)+len(ev.EdgesPeerToLane) > 0},
		{RuleBuildZone, T3, len(ev.BuildZone) > 0},
		{RuleUnknown, T3, len(ev.Unknown) > 0},
		{RuleDerived, T2, len(ev.Derived) > 0},
	}
	tier, rules := T1, []Rule{}
	for _, c := range checks {
		if c.fired {
			rules = append(rules, c.rule)
			tier = stricter(tier, c.tier)
		}
	}
	if len(rules) == 0 {
		return T1, []Rule{RuleDisjoint}
	}
	return tier, rules
}

var tierRank = map[Tier]int{T1: 1, T2: 2, T3: 3, T4: 4}

func stricter(a, b Tier) Tier {
	if tierRank[b] > tierRank[a] {
		return b
	}
	return a
}

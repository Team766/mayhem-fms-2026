//go:build custom

package game

// HasRankingPointFoul reports whether this alliance committed any ranking-point foul whose rule
// number matches one of the given ones. Call it on the OPPONENT's score to award a bonus ranking
// point — e.g. in game/custom_scoring_logic.go:
//
//	func ComputeSafetyRP(score, opponentScore Score, summary ScoreSummary) bool {
//	    return opponentScore.HasRankingPointFoul("G418", "G428")
//	}
//
// A foul only counts when its rule is flagged is-ranking-point in game/custom_rules.go (mirroring
// standard FRC bonus-RP fouls), so keep that flag in sync with the rule numbers you pass here.
func (s *Score) HasRankingPointFoul(ruleNumbers ...string) bool {
	for _, foul := range s.Fouls {
		rule := foul.Rule()
		if rule == nil || !rule.IsRankingPoint {
			continue
		}
		for _, ruleNumber := range ruleNumbers {
			if rule.RuleNumber == ruleNumber {
				return true
			}
		}
	}
	return false
}

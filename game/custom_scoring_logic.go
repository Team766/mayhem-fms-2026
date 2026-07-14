//go:build custom

package game

// Custom scoring logic for the active custom game. Hand-written; never generated or touched by
// `go generate`. Every logic_func named in custom_game.yaml's ranking_points needs a matching func
// here with the signature:
//
//	func(score, opponentScore Score, summary ScoreSummary) bool
//
//   - summary carries this alliance's point totals — the phase totals (summary.AutoPoints, …), the
//     per-group/status point fields (e.g. summary.ShipPoints), and MatchPoints/FoulPoints/Score.
//     Prefer these over re-deriving from raw counts so the logic can't drift from the generated
//     point math. Note that the ranking-point fields (summary.<X>RankingPoint) and
//     summary.BonusRankingPoints are NOT yet populated when these funcs run — they are computed from
//     the results of these funcs — so don't read them here.
//   - score / opponentScore are the raw per-element counts, for thresholds the summary doesn't
//     expose (and for cross-alliance logic). The opponent's *summary* is deliberately not passed —
//     it would recurse back through this same logic.

func ComputeAutonBonusRP(score, opponentScore Score, summary ScoreSummary) bool {
	if summary.AutoPoints >= 20 {
		return true
	}
	if opponentScore.HasRankingPointFoul("MA2603") {
		return true
	}
	return false
}

func ComputeScoringBonusRP(score, opponentScore Score, summary ScoreSummary) bool {
	// count up:
	// score.TeleopShelfL1Count, score.TeleopShelfL2Count, score.TeleopShelfStackedCount
	// score.TeleopShelfL1CountGolden, score.TeleopShelfL2CountGolden, score.TeleopShelfStackedCountGolden
	return false
}

func ComputeEndgameBonusRP(score, opponentScore Score, summary ScoreSummary) bool {
	// check:
	// score.AnyBalanceBeamStatus, opponent.HasRankingPointFoul("", "")
	return false
}

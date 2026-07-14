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

// ComputeAutonRP: alliance places more than 2 game pieces on Structure 1 (either level) during auto.
// (Could also be written against the summary, e.g. `return summary.AutoPoints >= 9`.)
func ComputeAutonRP(score, opponentScore Score, summary ScoreSummary) bool {
	return score.AutoStructure1Level1Count+score.AutoStructure1Level2Count > 2
}

// ComputeScoringRP: alliance places 10 or more game pieces on Structure 1 during teleop.
func ComputeScoringRP(score, opponentScore Score, summary ScoreSummary) bool {
	return score.TeleopStructure1Level1Count+score.TeleopStructure1Level2Count >= 10
}

// ComputeEndgameRP: alliance parks at least two of three robots.
func ComputeEndgameRP(score, opponentScore Score, summary ScoreSummary) bool {
	parked := 0
	for _, p := range score.ParkStatuses {
		if p {
			parked++
		}
	}
	return parked >= 2
}

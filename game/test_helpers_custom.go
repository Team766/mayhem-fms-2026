//go:build custom

package game

func TestScore1() *Score {
	fouls := []Foul{
		{1, true, 25, 16},
		{2, false, 1868, 13},
		{3, false, 1868, 13},
		{4, true, 25, 15},
		{5, true, 25, 15},
		{6, true, 25, 15},
		{7, true, 25, 15},
	}
	return &Score{
		Fouls:     fouls,
		PlayoffDq: false,
	}
}

func TestScore2() *Score {
	return &Score{
		Fouls:     []Foul{},
		PlayoffDq: false,
	}
}

// TestRanking1/TestRanking2 are shared fixtures for the custom build (api, model, and report tests).
// They deliberately set only build-independent RankingFields — RankingPoints, the win/loss record,
// and Played — and leave the configured ranking_tiebreaker columns at their zero value, so this file
// compiles for any custom_game.yaml. Tests that need specific tiebreaker values (the rankings
// report) are generated from the config and set those columns themselves.
func TestRanking1() *Ranking {
	return &Ranking{
		TeamId: 254,
		Rank:   1,
		RankingFields: RankingFields{
			RankingPoints: 20,
			Random:        0.254,
			Wins:          3,
			Losses:        2,
			Ties:          1,
			Played:        10,
		},
	}
}

func TestRanking2() *Ranking {
	return &Ranking{
		TeamId: 1114,
		Rank:   2,
		RankingFields: RankingFields{
			RankingPoints: 18,
			Random:        0.1114,
			Wins:          1,
			Losses:        3,
			Ties:          2,
			Played:        10,
		},
	}
}

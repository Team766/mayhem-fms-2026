//go:build custom

package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Hand-written, config-agnostic framework tests. Config-specific correctness — point math, the
// tiebreak cascade, ranking sort, and the Score mutators — is owned by the generated_*_test.go
// files (regenerated per custom_game.yaml). Everything here uses only always-present fields.

func TestAddScoreSummary(t *testing.T) {
	fields := &RankingFields{}
	own := &ScoreSummary{Score: 15, BonusRankingPoints: 1}
	opponent := &ScoreSummary{Score: 10}

	fields.AddScoreSummary(own, opponent, false)

	assert.Equal(t, 1, fields.Played)
	assert.Equal(t, 1, fields.Wins)
	assert.Equal(t, 4, fields.RankingPoints) // 3 for the win + 1 bonus RP
}

func TestGetAllRulesCustom(t *testing.T) {
	rules := GetAllRules()
	assert.NotEmpty(t, rules)
	for id, rule := range rules {
		assert.NotNil(t, GetRuleById(id))
		assert.Equal(t, rule, GetRuleById(id))
	}
}

func TestFoulPointValueCustom(t *testing.T) {
	fMajor := Foul{IsMajor: true}
	fMinor := Foul{IsMajor: false}

	assert.Equal(t, MajorFoulPoints, fMajor.PointValue())
	assert.Equal(t, MinorFoulPoints, fMinor.PointValue())
}

func TestHasRankingPointFoul(t *testing.T) {
	// Derive a ranking-point rule and a non-ranking-point rule from whatever custom_rules.go ships,
	// so the test doesn't hardcode a rule number.
	var rpRule, plainRule *Rule
	for _, r := range GetAllRules() {
		if r.IsRankingPoint {
			if rpRule == nil {
				rpRule = r
			}
		} else if plainRule == nil {
			plainRule = r
		}
	}
	if rpRule == nil {
		t.Skip("no is-ranking-point rule defined in custom_rules.go")
	}

	scored := &Score{Fouls: []Foul{{RuleId: rpRule.Id}}}
	assert.True(t, scored.HasRankingPointFoul(rpRule.RuleNumber))
	assert.True(t, scored.HasRankingPointFoul("ZZZ", rpRule.RuleNumber)) // varargs membership

	assert.False(t, scored.HasRankingPointFoul("ZZZ"))                 // not in the set
	assert.False(t, (&Score{}).HasRankingPointFoul(rpRule.RuleNumber)) // no fouls
	if plainRule != nil {
		// A non-ranking-point foul must not count even if its number is passed.
		notRp := &Score{Fouls: []Foul{{RuleId: plainRule.Id}}}
		assert.False(t, notRp.HasRankingPointFoul(plainRule.RuleNumber))
	}
}

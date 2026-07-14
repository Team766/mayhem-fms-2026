package main

import (
	"github.com/stretchr/testify/assert"
	"go/ast"
	"go/parser"
	"go/token"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTemplates(t *testing.T) {
	paths := []string{
		"../../game/custom_game.yaml",
		"../../game/examples/high_seas_havoc.yaml",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			data, err := os.ReadFile(p)
			assert.Nil(t, err)

			var yamlData GameYAML
			err = yaml.Unmarshal(data, &yamlData)
			assert.Nil(t, err)

			validationErrors := validateGameYAML(&yamlData)
			assert.Empty(t, validationErrors)
		})
	}
}

// testGameYAML is a small, self-contained, valid config the validation tests mutate. Using it
// instead of the shipped game/custom_game.yaml keeps these unit tests independent of whichever game
// is configured — a user who swaps in their own yaml doesn't break the generator's own tests.
func testGameYAML() *GameYAML {
	return &GameYAML{
		Game:          GameInfo{Name: "Test Game"},
		Fouls:         FoulConfig{MinorFoulPoints: 5, MajorFoulPoints: 15},
		GamePieces:    []GamePiece{{ID: "cube", DisplayName: "Cube"}},
		ScoringGroups: []ScoringGroup{{ID: "rack", DisplayName: "Rack"}},
		ScoringCounts: []ScoringCount{
			{ID: "rack_low", DisplayName: "Rack Low", GamePiece: "cube", ScoringGroup: "rack",
				Phases: []PhasePoints{{Phase: "auto", Points: 3}, {Phase: "teleop", Points: 2}}},
			{ID: "rack_high", DisplayName: "Rack High", GamePiece: "cube", ScoringGroup: "rack",
				Phases: []PhasePoints{{Phase: "teleop", Points: 5}}},
		},
		Statuses: []Status{
			{ID: "park", DisplayName: "Park", Phases: []PhasePoints{{Phase: "endgame", Points: 2}}},
			{ID: "climb", DisplayName: "Climb", Phases: []PhasePoints{{Phase: "endgame"}}, Values: []StatusValue{
				{ID: "none", DisplayName: "None", Points: 0}, {ID: "high", DisplayName: "High", Points: 5}}},
		},
		RPs:                []RankingPoint{{ID: "auto_rp", DisplayName: "Auto RP", LogicFunc: "ComputeAutoRp"}},
		RankingTiebreakers: []Tiebreaker{{Metric: "total_points"}, {Metric: "auto_points"}},
		PlayoffTiebreakers: []Tiebreaker{{Metric: "auto_points"}, {Metric: "total_points"}},
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		modify        func(*GameYAML)
		expectedError string
	}{
		{
			name: "missing game name",
			modify: func(y *GameYAML) {
				y.Game.Name = ""
			},
			expectedError: "game.name is required",
		},
		{
			name: "invalid minor foul points",
			modify: func(y *GameYAML) {
				y.Fouls.MinorFoulPoints = 0
			},
			expectedError: "fouls.minor_foul_points must be > 0",
		},
		{
			name: "invalid major foul points",
			modify: func(y *GameYAML) {
				y.Fouls.MajorFoulPoints = -1
			},
			expectedError: "fouls.major_foul_points must be > 0",
		},
		{
			name: "missing scoring count id",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].ID = ""
			},
			expectedError: "scoring_counts[0]: id is required",
		},
		{
			name: "bad scoring count phase",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].Phases[0].Phase = "invalid_phase"
			},
			expectedError: "unknown phase 'invalid_phase'",
		},
		{
			name: "scoring count with no phases",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].Phases = nil
			},
			expectedError: "at least one phase is required",
		},
		{
			name: "scoring count with duplicate phase",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].Phases = []PhasePoints{
					{Phase: "auto", Points: 5},
					{Phase: "auto", Points: 3},
				}
			},
			expectedError: "duplicate phase 'auto'",
		},
		{
			name: "scoring count phase with non-positive points",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].Phases = []PhasePoints{{Phase: "auto", Points: 0}}
			},
			expectedError: "points must be > 0",
		},
		{
			name: "scoring count in both teleop and endgame rejected",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].Phases = []PhasePoints{{Phase: "teleop", Points: 2}, {Phase: "endgame", Points: 3}}
			},
			expectedError: "cannot be scored in both teleop and endgame",
		},
		{
			name: "unknown scoring_group reference",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].ScoringGroup = "nonexistent"
			},
			expectedError: "unknown scoring_group 'nonexistent'",
		},
		{
			name: "missing game_piece rejected",
			modify: func(y *GameYAML) {
				y.ScoringCounts[0].GamePiece = ""
			},
			expectedError: "game_piece is required",
		},
		{
			name: "enum status with too few values",
			modify: func(y *GameYAML) {
				y.Statuses = []Status{
					{
						ID:     "bad_status",
						Phases: []PhasePoints{{Phase: "auto"}},
						Values: []StatusValue{
							{ID: "one", DisplayName: "One"},
						},
					},
				}
			},
			expectedError: "enum status requires at least 2 values",
		},
		{
			name: "status with teleop phase rejected",
			modify: func(y *GameYAML) {
				y.Statuses[0].Phases = []PhasePoints{{Phase: "teleop", Points: 3}}
			},
			expectedError: "only auto and endgame are supported for statuses",
		},
		{
			name: "status with more than one phase rejected",
			modify: func(y *GameYAML) {
				y.Statuses[0].Phases = []PhasePoints{{Phase: "auto", Points: 3}, {Phase: "endgame", Points: 3}}
			},
			expectedError: "exactly one phase is required",
		},
		{
			name: "unknown tiebreaker metric",
			modify: func(y *GameYAML) {
				y.RankingTiebreakers = append(y.RankingTiebreakers, Tiebreaker{Metric: "nonexistent"})
			},
			expectedError: "unknown metric 'nonexistent'",
		},
		{
			name: "duplicate id across sections",
			modify: func(y *GameYAML) {
				// A scoring count reusing the status id "park".
				y.ScoringCounts = append(y.ScoringCounts, ScoringCount{ID: "park", GamePiece: y.GamePieces[0].ID, Phases: []PhasePoints{{Phase: "auto", Points: 5}}})
			},
			expectedError: "duplicate id: 'park'",
		},
		{
			name: "id collides with a built-in summary field",
			modify: func(y *GameYAML) {
				// "match" -> MatchPoints, which already exists as a built-in ScoreSummary field.
				y.Statuses[0].ID = "match"
			},
			expectedError: "collides with a built-in field",
		},
		{
			name: "two ids generate the same summary field",
			modify: func(y *GameYAML) {
				// "Rack" CamelCases to the same field as scoring_group "rack" (-> RackPoints), yet is a
				// distinct raw id, so the dup-id check misses it.
				y.Statuses = append(y.Statuses, Status{ID: "Rack", Phases: []PhasePoints{{Phase: "auto", Points: 3}}})
			},
			expectedError: "collides with scoring group 'rack'",
		},
		{
			name: "duplicate ranking tiebreaker metric",
			modify: func(y *GameYAML) {
				// The default already lists total_points; a second entry is a duplicate that would
				// emit a duplicate RankingFields struct field.
				y.RankingTiebreakers = append(y.RankingTiebreakers, Tiebreaker{Metric: "total_points"})
			},
			expectedError: "duplicate metric 'total_points'",
		},
		{
			name: "duplicate playoff tiebreaker metric",
			modify: func(y *GameYAML) {
				y.PlayoffTiebreakers = append(y.PlayoffTiebreakers, Tiebreaker{Metric: "total_points"})
			},
			expectedError: "playoff_tiebreakers",
		},
		{
			name: "scoring count ids that CamelCase to the same identifier",
			modify: func(y *GameYAML) {
				// "rackLow" -> "RackLow", same as the fixture's "rack_low".
				y.ScoringCounts = append(y.ScoringCounts, ScoringCount{ID: "rackLow", Phases: []PhasePoints{{Phase: "auto", Points: 1}}})
			},
			expectedError: "colliding with scoring count",
		},
		{
			name: "status ids that CamelCase to the same identifier",
			modify: func(y *GameYAML) {
				y.Statuses = append(y.Statuses, Status{ID: "Park", Phases: []PhasePoints{{Phase: "endgame", Points: 2}}})
			},
			expectedError: "colliding with status",
		},
		{
			name: "enum status first value scores points",
			modify: func(y *GameYAML) {
				y.Statuses = append(y.Statuses, Status{ID: "gizmo", Phases: []PhasePoints{{Phase: "endgame", Points: 1}}, Values: []StatusValue{
					{ID: "low", Points: 2}, {ID: "high", Points: 5},
				}})
			},
			expectedError: "first enum value 'low' must have points: 0",
		},
		{
			name: "id resolves to the built-in RankingPoints field",
			modify: func(y *GameYAML) {
				y.Statuses = append(y.Statuses, Status{ID: "ranking", Phases: []PhasePoints{{Phase: "endgame", Points: 1}}})
			},
			expectedError: "RankingPoints collides with the built-in RankingFields field",
		},
		{
			name: "PointsVal consts collide across a count and a status",
			modify: func(y *GameYAML) {
				// count "foo" in auto -> FooAutoPointsVal; bool status "foo_auto" -> FooAutoPointsVal.
				y.ScoringCounts = append(y.ScoringCounts, ScoringCount{ID: "foo", GamePiece: y.GamePieces[0].ID, Phases: []PhasePoints{{Phase: "auto", Points: 1}}})
				y.Statuses = append(y.Statuses, Status{ID: "foo_auto", Phases: []PhasePoints{{Phase: "endgame", Points: 2}}})
			},
			expectedError: "generated const FooAutoPointsVal collides",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start from a fresh copy of the self-contained fixture (a new one per case, so appends
			// in one case don't leak into another).
			yamlData := *testGameYAML()

			tt.modify(&yamlData)

			validationErrors := validateGameYAML(&yamlData)
			assert.NotEmpty(t, validationErrors)

			found := false
			for _, errStr := range validationErrors {
				if assert.Contains(t, errStr, tt.expectedError) {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected error containing: %q, got: %v", tt.expectedError, validationErrors)
		})
	}
}

func TestValidateCustomScoringLogic(t *testing.T) {
	// Self-contained fixture: one ranking_point (ComputeAutoRp), a Score exposing AutoRackLowCount /
	// TeleopRackHighCount / ParkStatuses / ClimbStatuses, and the usual summary point fields.
	base := testGameYAML()

	writeLogic := func(t *testing.T, content string) string {
		path := filepath.Join(t.TempDir(), "custom_scoring_logic.go")
		assert.Nil(t, os.WriteFile(path, []byte(content), 0644))
		return path
	}

	// A logic file defining the fixture's one logic func with the given body.
	logicWith := func(body string) string {
		return "package game\n" +
			"func ComputeAutoRp(score, opponentScore Score, summary ScoreSummary) bool {\n" + body + "\n}\n"
	}

	t.Run("valid logic matching the config", func(t *testing.T) {
		path := writeLogic(t, logicWith(
			"\tparked := 0\n\tfor _, p := range score.ParkStatuses {\n\t\tif p {\n\t\t\tparked++\n\t\t}\n\t}\n"+
				"\treturn parked >= 2 && score.AutoRackLowCount > 0 && summary.AutoPoints >= 9"))
		assert.Empty(t, validateCustomScoringLogic(base, path))
	})

	t.Run("no false positives on method calls and non-param selectors", func(t *testing.T) {
		path := writeLogic(t, logicWith(
			"\tfor _, foul := range score.Fouls {\n\t\t_ = foul.PointValue()\n\t}\n\treturn summary.AutoPoints >= 9"))
		assert.Empty(t, validateCustomScoringLogic(base, path))
	})

	t.Run("obsolete Score field with suggestion", func(t *testing.T) {
		// A near-miss typo of a real field should be flagged and suggest the real field.
		path := writeLogic(t, logicWith("\treturn score.AutoRackLowKount > 2"))
		errs := validateCustomScoringLogic(base, path)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0], "AutoRackLowKount")
		assert.Contains(t, errs[0], "did you mean 'AutoRackLowCount'")
	})

	t.Run("unknown ScoreSummary field", func(t *testing.T) {
		path := writeLogic(t, logicWith("\treturn summary.BogusPoints > 0"))
		errs := validateCustomScoringLogic(base, path)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0], "summary.BogusPoints")
		assert.Contains(t, errs[0], "is not a field generated")
	})

	t.Run("missing logic func emits a stub with field/helper reference", func(t *testing.T) {
		content := "package game\nfunc SomethingElse(score, opponentScore Score, summary ScoreSummary) bool { return false }\n"
		errs := validateCustomScoringLogic(base, writeLogic(t, content))
		assert.Len(t, errs, 1)
		// The copy-pasteable stub for the missing func.
		assert.Contains(t, errs[0], "func ComputeAutoRp(score, opponentScore Score, summary ScoreSummary) bool")
		// The data reference: a count field, a status helper (bool + enum with values), a summary total.
		assert.Contains(t, errs[0], "AutoRackLowCount")
		assert.Contains(t, errs[0], "score.AnyParkStatus()")
		assert.Contains(t, errs[0], "Any"+"ClimbStatus(atLeast ClimbStatus)")
		assert.Contains(t, errs[0], "ClimbNone")
		assert.Contains(t, errs[0], "RackPoints")
		assert.Contains(t, errs[0], "HasRankingPointFoul")
	})

	t.Run("missing file with no ranking points is fine", func(t *testing.T) {
		noRPs := *base
		noRPs.RPs = nil
		assert.Empty(t, validateCustomScoringLogic(&noRPs, filepath.Join(t.TempDir(), "does_not_exist.go")))
	})
}

// TestGeneratedFieldSetsMatchTemplates guards the hand-maintained base-field lists in
// generatedFieldSets (validate_logic.go) against drifting from what score.go.tmpl /
// score_summary.go.tmpl actually emit. If a base field is added to a template but not to
// generatedFieldSets, the validator would falsely reject a valid custom_scoring_logic.go and halt
// generation; this catches that. Runs against the generated structs for the default config; skipped
// on a fresh checkout where `go generate` hasn't produced them yet.
func TestGeneratedFieldSetsMatchTemplates(t *testing.T) {
	data, err := os.ReadFile("../../game/custom_game.yaml")
	assert.Nil(t, err)
	var y GameYAML
	assert.Nil(t, yaml.Unmarshal(data, &y))
	scoreFields, summaryFields := generatedFieldSets(&y)

	check := func(genPath, structName string, allowed map[string]bool) {
		src, err := os.ReadFile(genPath)
		if err != nil {
			t.Skipf("%s not present; run `go generate ./...` first (%v)", genPath, err)
		}
		for _, field := range structFieldNames(t, src, structName) {
			assert.Truef(t, allowed[field],
				"generated %s.%s is missing from generatedFieldSets in validate_logic.go — the validator will falsely reject it",
				structName, field)
		}
	}
	check("../../game/generated_score.go", "Score", scoreFields)
	check("../../game/generated_score_summary.go", "ScoreSummary", summaryFields)
}

// structFieldNames returns the declared field names of the named struct in Go source src.
func structFieldNames(t *testing.T, src []byte, structName string) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	assert.Nil(t, err)
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != structName {
			return true
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			for _, f := range st.Fields.List {
				for _, nm := range f.Names {
					names = append(names, nm.Name)
				}
			}
		}
		return false
	})
	return names
}

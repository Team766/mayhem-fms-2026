// Run via `go generate ./...` from the repo root, which also picks up the unrelated stringer
// directives in plc/plc.go and model/match.go. The directive below runs with this directory
// (cmd/generate/) as its working directory, hence the ../../ prefix on the default custom_game.yaml path.
//
//go:generate go run . -f ../../game/custom_game.yaml -out ../..
package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var goIdentRegexp = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

var validElementPhases = map[string]bool{"auto": true, "teleop": true, "endgame": true}
var validStatusPhases = map[string]bool{"auto": true, "endgame": true}

// cleanPatterns lists every generated-file glob, matching the patterns in .gitignore.
var cleanPatterns = []string{
	"game/generated_*.go",
	"web/generated_*.go",
	"tournament/generated_*.go",
	"templates/generated_*.html",
	"static/js/generated_*.js",
	"cmd/generate/generated_*_test.go",
}

// runClean removes every file matching cleanPatterns, run from the repo root.
func runClean() error {
	removed := 0
	for _, pattern := range cleanPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if err := os.Remove(match); err != nil {
				return err
			}
			fmt.Println("Removed", match)
			removed++
		}
	}
	if removed == 0 {
		fmt.Println("Nothing to clean.")
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "clean" {
		if err := runClean(); err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning generated files: %v\n", err)
			os.Exit(1)
		}
		return
	}

	yamlPath := flag.String("f", "game/custom_game.yaml", "path to the game definition YAML to read")
	// outRoot decouples the generated-file destinations from the input YAML's location: output always
	// lands in the standard repo layout (out/game, out/templates, out/static/js, out/cmd/generate),
	// so a config under game/examples/ generates into the same place game/custom_game.yaml would.
	// Default "." works when run from the repo root; the go:generate directive passes "../.." since it
	// runs in cmd/generate/.
	outRoot := flag.String("out", ".", "repo root the generated files are written under")
	flag.Parse()

	data, err := os.ReadFile(*yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading custom_game.yaml: %v\n", err)
		os.Exit(1)
	}

	var yamlData GameYAML
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing custom_game.yaml: %v\n", err)
		os.Exit(1)
	}

	// Validation
	validationErrors := validateGameYAML(&yamlData)

	// Only check the hand-written scoring logic once the config itself is valid — otherwise the
	// generated field set it's checked against may be malformed, producing misleading errors.
	if len(validationErrors) == 0 {
		logicPath := filepath.Join(*outRoot, "game", "custom_scoring_logic.go")
		validationErrors = append(validationErrors, validateCustomScoringLogic(&yamlData, logicPath)...)
	}

	if len(validationErrors) > 0 {
		fmt.Fprintln(os.Stderr, "Validation errors in custom_game.yaml:")
		for _, errStr := range validationErrors {
			fmt.Fprintf(os.Stderr, "  - %s\n", errStr)
		}
		os.Exit(1)
	}

	// Codegen target dirs — always the standard repo layout under outRoot, independent of where the
	// input YAML lives.
	gameDir := filepath.Join(*outRoot, "game")

	if err := generateConstants(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating constants: %v\n", err)
		os.Exit(1)
	}

	if err := generateScore(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating Score struct: %v\n", err)
		os.Exit(1)
	}

	if err := generateScoreSummary(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating ScoreSummary: %v\n", err)
		os.Exit(1)
	}

	if err := generateRankingFields(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating RankingFields: %v\n", err)
		os.Exit(1)
	}

	if err := generateScoreTest(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating score test: %v\n", err)
		os.Exit(1)
	}

	if err := generateScoreSummaryTest(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating score summary test: %v\n", err)
		os.Exit(1)
	}

	if err := generateRankingFieldsTest(&yamlData, gameDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating ranking fields test: %v\n", err)
		os.Exit(1)
	}

	webDir := filepath.Join(*outRoot, "web")
	tournamentDir := filepath.Join(*outRoot, "tournament")

	if err := generateReportsRankings(&yamlData, webDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating rankings report handler: %v\n", err)
		os.Exit(1)
	}

	if err := generateReportsRankingsTest(&yamlData, webDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating rankings report test: %v\n", err)
		os.Exit(1)
	}

	if err := generateQualificationRankingsTest(&yamlData, tournamentDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating qualification rankings test: %v\n", err)
		os.Exit(1)
	}

	templatesDir := filepath.Join(*outRoot, "templates")
	staticJsDir := filepath.Join(*outRoot, "static/js")
	cmdGenerateDir := filepath.Join(*outRoot, "cmd/generate")

	if err := generateScoringPanelTemplate(&yamlData, templatesDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating scoring panel template: %v\n", err)
		os.Exit(1)
	}

	if err := generateScoringPanelJS(&yamlData, staticJsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating scoring panel JS: %v\n", err)
		os.Exit(1)
	}

	if err := generateAudienceDisplayTemplate(&yamlData, templatesDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating audience display template: %v\n", err)
		os.Exit(1)
	}

	if err := generateAudienceDisplayJS(&yamlData, staticJsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating audience display JS: %v\n", err)
		os.Exit(1)
	}

	if err := generateRefereePanelTemplate(&yamlData, templatesDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating referee panel template: %v\n", err)
		os.Exit(1)
	}

	if err := generateRefereePanelJS(&yamlData, staticJsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating referee panel JS: %v\n", err)
		os.Exit(1)
	}

	if err := generateTemplateTest(&yamlData, cmdGenerateDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating template test: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Code generation complete successfully.")
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func validateGameYAML(yamlData *GameYAML) []string {
	var validationErrors []string

	if yamlData.Game.Name == "" {
		validationErrors = append(validationErrors, "game.name is required")
	}
	if yamlData.Fouls.MinorFoulPoints <= 0 {
		validationErrors = append(validationErrors, "fouls.minor_foul_points must be > 0")
	}
	if yamlData.Fouls.MajorFoulPoints <= 0 {
		validationErrors = append(validationErrors, "fouls.major_foul_points must be > 0")
	}

	seenIDs := make(map[string]bool)
	checkDup := func(id string, context string) {
		if id == "" {
			return
		}
		if seenIDs[id] {
			validationErrors = append(validationErrors, fmt.Sprintf("duplicate id: '%s' in %s", id, context))
		}
		seenIDs[id] = true
	}

	// Game pieces
	gamePieces := make(map[string]bool)
	for i, gp := range yamlData.GamePieces {
		if gp.ID == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("game_pieces[%d]: id is required", i))
		} else if !goIdentRegexp.MatchString(gp.ID) {
			validationErrors = append(validationErrors, fmt.Sprintf("game_pieces[%d]: id '%s' must be a valid Go identifier (letters, digits, underscores; cannot start with a digit)", i, gp.ID))
		} else {
			checkDup(gp.ID, "game_pieces")
			gamePieces[gp.ID] = true
		}
	}

	// Scoring groups
	scoringGroups := make(map[string]bool)
	for i, dg := range yamlData.ScoringGroups {
		if dg.ID == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_groups[%d]: id is required", i))
		} else if !goIdentRegexp.MatchString(dg.ID) {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_groups[%d]: id '%s' must be a valid Go identifier (letters, digits, underscores; cannot start with a digit)", i, dg.ID))
		} else {
			checkDup(dg.ID, "scoring_groups")
			scoringGroups[dg.ID] = true
		}
	}

	// scoring_counts. countCamel guards against two distinct ids that CamelCase to the same Go
	// identifier (e.g. "deck2" and "deck_2" -> "Deck2"), which would emit a duplicate Score field,
	// Adjust<Camel>Count method, and PointsVal const.
	countCamel := make(map[string]string)
	for i, sc := range yamlData.ScoringCounts {
		if sc.ID == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d]: id is required", i))
			continue
		}
		if !goIdentRegexp.MatchString(sc.ID) {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d]: id '%s' must be a valid Go identifier (letters, digits, underscores; cannot start with a digit)", i, sc.ID))
			continue
		}
		checkDup(sc.ID, "scoring_counts")
		if camel := toCamelCase(sc.ID); countCamel[camel] != "" && countCamel[camel] != sc.ID {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: id CamelCases to '%s', colliding with scoring count '%s' (both would generate the same Score field/method)", i, sc.ID, camel, countCamel[camel]))
		} else {
			countCamel[camel] = sc.ID
		}

		if sc.GamePiece == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: game_piece is required", i, sc.ID))
		} else if !gamePieces[sc.GamePiece] {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: unknown game_piece '%s'", i, sc.ID, sc.GamePiece))
		}
		if sc.ScoringGroup != "" && !scoringGroups[sc.ScoringGroup] {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: unknown scoring_group '%s'", i, sc.ID, sc.ScoringGroup))
		}

		if len(sc.Phases) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: at least one phase is required", i, sc.ID))
			continue
		}
		seenPhases := make(map[string]bool)
		for j, ep := range sc.Phases {
			if !validElementPhases[ep.Phase] {
				validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s.phases[%d]: unknown phase '%s'", i, sc.ID, j, ep.Phase))
				continue
			}
			if seenPhases[ep.Phase] {
				validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: duplicate phase '%s'", i, sc.ID, ep.Phase))
			}
			seenPhases[ep.Phase] = true
			if ep.Points <= 0 {
				validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s.phases[%d]: points must be > 0", i, sc.ID, j))
			}
		}
		if seenPhases["teleop"] && seenPhases["endgame"] {
			validationErrors = append(validationErrors, fmt.Sprintf("scoring_counts[%d].%s: cannot be scored in both teleop and endgame (teleop play continues through endgame)", i, sc.ID))
		}
	}

	// statuses. statusCamel guards CamelCase collisions among status ids (which would produce a
	// duplicate <Camel>Statuses field, Set<Camel>Status method, and enum type), analogous to counts.
	statusCamel := make(map[string]string)
	for i, status := range yamlData.Statuses {
		if status.ID == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d]: id is required", i))
			continue
		}
		if !goIdentRegexp.MatchString(status.ID) {
			validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d]: id '%s' must be a valid Go identifier (letters, digits, underscores; cannot start with a digit)", i, status.ID))
			continue
		}
		checkDup(status.ID, "statuses")
		if camel := toCamelCase(status.ID); statusCamel[camel] != "" && statusCamel[camel] != status.ID {
			validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: id CamelCases to '%s', colliding with status '%s' (both would generate the same Score field/method)", i, status.ID, camel, statusCamel[camel]))
		} else {
			statusCamel[camel] = status.ID
		}

		if len(status.Phases) != 1 {
			validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: exactly one phase is required (got %d)", i, status.ID, len(status.Phases)))
		} else if !validStatusPhases[status.Phases[0].Phase] {
			validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s.phases[0]: unknown phase '%s' (only auto and endgame are supported for statuses)", i, status.ID, status.Phases[0].Phase))
		}

		if len(status.Values) > 0 {
			if len(status.Values) < 2 {
				validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: enum status requires at least 2 values", i, status.ID))
			}
			// The generated [3]<Camel>Status array zero-values to the first declared value, so every
			// un-scored robot holds it. If it scored points, they'd be awarded silently (and the
			// generated tests, which assume a zero Score contributes 0, would fail). Require the
			// baseline value to be worth 0.
			if len(status.Values) > 0 && status.Values[0].Points != 0 {
				validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: the first enum value '%s' must have points: 0 (it is the default state of an un-scored robot)", i, status.ID, status.Values[0].ID))
			}
			statusVals := make(map[string]bool)
			valCamel := make(map[string]string)
			for j, val := range status.Values {
				if val.ID == "" {
					validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s values[%d]: id is required", i, status.ID, j))
				} else {
					if statusVals[val.ID] {
						validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: duplicate value id '%s'", i, status.ID, val.ID))
					}
					statusVals[val.ID] = true
					if camel := toCamelCase(val.ID); valCamel[camel] != "" && valCamel[camel] != val.ID {
						validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: value id '%s' CamelCases to '%s', colliding with value '%s'", i, status.ID, val.ID, camel, valCamel[camel]))
					} else {
						valCamel[camel] = val.ID
					}
				}
			}
		} else if len(status.Phases) == 1 {
			if status.Phases[0].Points <= 0 {
				validationErrors = append(validationErrors, fmt.Sprintf("statuses[%d].%s: phases[0].points must be > 0 for bool status", i, status.ID))
			}
		}
	}

	// RPs
	for i, rp := range yamlData.RPs {
		if rp.ID == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("ranking_points[%d]: id is required", i))
		} else if !goIdentRegexp.MatchString(rp.ID) {
			validationErrors = append(validationErrors, fmt.Sprintf("ranking_points[%d]: id '%s' must be a valid Go identifier (letters, digits, underscores; cannot start with a digit)", i, rp.ID))
		} else {
			checkDup(rp.ID, "ranking_points")
		}

		if rp.LogicFunc == "" || !goIdentRegexp.MatchString(rp.LogicFunc) {
			validationErrors = append(validationErrors, fmt.Sprintf("ranking_points[%d].logic_func: '%s' is not a valid Go identifier", i, rp.LogicFunc))
		}
	}

	buckets := buildScoringGroups(yamlData)

	// Reject ids whose generated point field (CamelCase(id)+"Points") collides with a built-in field
	// or with another generated field. This catches e.g. a scoring_group/status id of "auto_points"
	// (-> AutoPoints) or "match" (-> MatchPoints), and two ids that CamelCase to the same name (e.g.
	// "auto_points" and "Auto_points"), either of which would emit uncompilable Go. "RankingPoints"
	// is reserved too: it isn't a ScoreSummary built-in, but a tiebreaker on such an id would emit a
	// RankingFields field colliding with the built-in RankingPoints.
	summaryFields := map[string]string{
		"AutoPoints": "a built-in field", "TeleopPoints": "a built-in field",
		"EndgamePoints": "a built-in field", "MatchPoints": "a built-in field",
		"FoulPoints": "a built-in field", "BonusRankingPoints": "a built-in field",
		"RankingPoints": "the built-in RankingFields field",
	}
	checkSummaryField := func(id, context string) {
		field := toCamelCase(id) + "Points"
		if existing, ok := summaryFields[field]; ok {
			validationErrors = append(validationErrors, fmt.Sprintf("%s '%s': generated point field %s collides with %s", context, id, field, existing))
			return
		}
		summaryFields[field] = context + " '" + id + "'"
	}
	for _, bucket := range buckets {
		checkSummaryField(bucket.ID, "scoring group") // bucket id = scoring_group id, or ungrouped count's own id
	}
	for _, status := range yamlData.Statuses {
		checkSummaryField(status.ID, "status")
	}

	// Reject collisions among the generated *PointsVal package consts. These live in one package
	// scope, and a count scored in a phase, a bool status, and an enum value each emit one — so a
	// count "foo" in auto (FooAutoPointsVal) and a bool status "foo_auto" (FooAutoPointsVal) would
	// clash even though neither the raw ids nor the CamelCase checks above catch it.
	pointsValConsts := make(map[string]string)
	checkPointsVal := func(name, source string) {
		if existing, ok := pointsValConsts[name]; ok {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: generated const %s collides with %s", source, name, existing))
			return
		}
		pointsValConsts[name] = source
	}
	for _, sc := range yamlData.ScoringCounts {
		for _, ep := range sc.Phases {
			if validElementPhases[ep.Phase] {
				checkPointsVal(toCamelCase(sc.ID)+phaseFieldPrefix[ep.Phase]+"PointsVal", "scoring count '"+sc.ID+"'")
			}
		}
	}
	for _, status := range yamlData.Statuses {
		if len(status.Values) == 0 {
			checkPointsVal(toCamelCase(status.ID)+"PointsVal", "status '"+status.ID+"'")
		} else {
			for _, val := range status.Values {
				checkPointsVal(toCamelCase(status.ID)+toCamelCase(val.ID)+"PointsVal", "status '"+status.ID+"' value '"+val.ID+"'")
			}
		}
	}

	// Build the set of valid tiebreaker metrics: the built-in phase/total points, plus every
	// ScoreSummary point field — one per scoring-group bucket (a scoring_group id, or an ungrouped
	// element's own id) and one per status. A raw element that is grouped is not valid on its own;
	// tiebreak on its group instead.
	validElements := map[string]bool{
		"auto_points":    true,
		"teleop_points":  true,
		"endgame_points": true,
		"total_points":   true,
	}
	for _, bucket := range buckets {
		validElements[bucket.ID] = true
	}
	for _, status := range yamlData.Statuses {
		validElements[status.ID] = true
	}

	// ranking_tiebreakers — each metric must be known and must not repeat: two entries resolving to
	// the same RankingFields field would emit a duplicate struct field (and a duplicate composite-
	// literal key in the generated tests), which fails to compile.
	seenRankingTiebreaker := make(map[string]bool)
	for i, tb := range yamlData.RankingTiebreakers {
		if !validElements[tb.Metric] {
			validationErrors = append(validationErrors, fmt.Sprintf("ranking_tiebreakers[%d]: unknown metric '%s'", i, tb.Metric))
		} else if seenRankingTiebreaker[tb.Metric] {
			validationErrors = append(validationErrors, fmt.Sprintf("ranking_tiebreakers[%d]: duplicate metric '%s'", i, tb.Metric))
		}
		seenRankingTiebreaker[tb.Metric] = true
	}

	// playoff_tiebreakers — likewise reject repeats: a duplicate metric emits a duplicate
	// DetermineMatchStatus tiebreak test function, which fails the generated test build.
	seenPlayoffTiebreaker := make(map[string]bool)
	for i, tb := range yamlData.PlayoffTiebreakers {
		if !validElements[tb.Metric] {
			validationErrors = append(validationErrors, fmt.Sprintf("playoff_tiebreakers[%d]: unknown metric '%s'", i, tb.Metric))
		} else if seenPlayoffTiebreaker[tb.Metric] {
			validationErrors = append(validationErrors, fmt.Sprintf("playoff_tiebreakers[%d]: duplicate metric '%s'", i, tb.Metric))
		}
		seenPlayoffTiebreaker[tb.Metric] = true
	}

	return validationErrors
}

// Code generators for the web UI surfaces — scoring panel, referee panel, and audience display
// (HTML + JS) — produced by executing the committed templates/static custom_*.tmpl sources. The
// test generator for these surfaces lives in codegen_web_tests.go; the server Go in codegen_go.go.

package main

import "path/filepath"

// ScoringBucket is one rolled-up scoring group — a ScoreSummary point field and an audience-display
// entry. ID/DisplayName are the resolved, presentable label; CountIDs are the scoring counts
// merged into this bucket.
type ScoringBucket struct {
	ID          string
	DisplayName string
	CountIDs    []string
}

// buildScoringGroups resolves every scoring count into the bucket it rolls up into, merging counts
// that land in the same bucket. A count with a scoring_group joins that group's bucket; a count with
// no scoring_group stands alone as its own bucket under its own id. (game_piece is piece identity,
// not a rollup — to group counts, give them a shared scoring_group.)
//
// The lookup key is tagged by source ("sg:"/"ct:") so a scoring_group and an ungrouped count that
// happen to share the same id string aren't merged into one bucket by accident.
func buildScoringGroups(yamlData *GameYAML) []ScoringBucket {
	var buckets []ScoringBucket
	seen := make(map[string]int) // lookup key -> index in buckets, so repeats merge into one bucket

	for _, sc := range yamlData.ScoringCounts {
		var lookupKey, groupID, displayName string
		if sc.ScoringGroup != "" { // grouped: roll up with the other counts in this scoring_group
			lookupKey = "sg:" + sc.ScoringGroup
			groupID = sc.ScoringGroup
			displayName = sc.ScoringGroup
			for _, group := range yamlData.ScoringGroups {
				if group.ID == sc.ScoringGroup {
					displayName = group.DisplayName // prefer the group's label over its raw id
				}
			}
		} else { // ungrouped: the count is its own bucket
			lookupKey = "ct:" + sc.ID
			groupID = sc.ID
			displayName = sc.DisplayName
		}

		if idx, ok := seen[lookupKey]; ok {
			buckets[idx].CountIDs = append(buckets[idx].CountIDs, sc.ID)
		} else {
			seen[lookupKey] = len(buckets)
			buckets = append(buckets, ScoringBucket{ID: groupID, DisplayName: displayName, CountIDs: []string{sc.ID}})
		}
	}

	return buckets
}

var phaseSectionTitle = map[string]string{"auto": "Auto", "teleop": "Teleop", "endgame": "Endgame"}

// generateReportsRankings emits web/generated_reports_rankings_custom.go — the CSV/PDF qualification
// rankings report handlers, whose middle columns track the configured ranking_tiebreakers so the
// report never references a RankingFields column the current custom_game.yaml doesn't generate.
func generateReportsRankings(yamlData *GameYAML, webDir string) error {
	return renderGoTemplate("reports_rankings_custom.go.tmpl", filepath.Join(webDir, "generated_reports_rankings_custom.go"), buildTemplateData(yamlData))
}

func generateScoringPanelTemplate(yamlData *GameYAML, templatesDir string) error {
	return renderWebTemplate(
		filepath.Join(templatesDir, "custom_scoring_panel.html.tmpl"),
		filepath.Join(templatesDir, "generated_scoring_panel.html"),
		buildTemplateData(yamlData),
	)
}

func generateScoringPanelJS(yamlData *GameYAML, staticJsDir string) error {
	return renderWebTemplate(
		filepath.Join(staticJsDir, "custom_scoring_panel.js.tmpl"),
		filepath.Join(staticJsDir, "generated_scoring_panel.js"),
		buildTemplateData(yamlData),
	)
}

func generateAudienceDisplayTemplate(yamlData *GameYAML, templatesDir string) error {
	return renderWebTemplate(
		filepath.Join(templatesDir, "custom_audience_display.html.tmpl"),
		filepath.Join(templatesDir, "generated_audience_display.html"),
		buildTemplateData(yamlData),
	)
}

func generateAudienceDisplayJS(yamlData *GameYAML, staticJsDir string) error {
	return renderWebTemplate(
		filepath.Join(staticJsDir, "custom_audience_display.js.tmpl"),
		filepath.Join(staticJsDir, "generated_audience_display.js"),
		buildTemplateData(yamlData),
	)
}

func generateRefereePanelTemplate(yamlData *GameYAML, templatesDir string) error {
	return renderWebTemplate(
		filepath.Join(templatesDir, "custom_referee_panel.html.tmpl"),
		filepath.Join(templatesDir, "generated_referee_panel.html"),
		buildTemplateData(yamlData),
	)
}

func generateRefereePanelJS(yamlData *GameYAML, staticJsDir string) error {
	return renderWebTemplate(
		filepath.Join(staticJsDir, "custom_referee_panel.js.tmpl"),
		filepath.Join(staticJsDir, "generated_referee_panel.js"),
		buildTemplateData(yamlData),
	)
}

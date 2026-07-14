// Generator for the test of the web UI surfaces (codegen_web.go): emits
// cmd/generate/generated_template_test.go, which asserts the rendered templates parse and contain
// the expected per-element markup.

package main

import "path/filepath"

func generateTemplateTest(yamlData *GameYAML, destDir string) error {
	return renderGoTemplate("template_test.go.tmpl", filepath.Join(destDir, "generated_template_test.go"), buildTemplateData(yamlData))
}

// generateReportsRankingsTest emits web/generated_reports_rankings_custom_test.go — the CSV/PDF
// rankings report tests, whose fixtures and expected CSV are built from the configured tiebreakers.
func generateReportsRankingsTest(yamlData *GameYAML, webDir string) error {
	return renderGoTemplate("reports_rankings_custom_test.go.tmpl", filepath.Join(webDir, "generated_reports_rankings_custom_test.go"), buildTemplateData(yamlData))
}

// generateQualificationRankingsTest emits tournament/generated_qualification_rankings_custom_test.go
// — the end-to-end CalculateRankings test, which grants points via the first scoring count the
// current custom_game.yaml declares rather than a hard-coded field name.
func generateQualificationRankingsTest(yamlData *GameYAML, tournamentDir string) error {
	return renderGoTemplate("qualification_rankings_custom_test.go.tmpl", filepath.Join(tournamentDir, "generated_qualification_rankings_custom_test.go"), buildTemplateData(yamlData))
}

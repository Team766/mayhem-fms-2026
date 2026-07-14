// View model for the template-based UI/code generation. buildTemplateData turns a validated
// GameYAML into TemplateData — the stable contract the .tmpl files consume. The model carries the
// irreducible facts (ids, display names, phases, points, group membership) plus a few cross-cutting
// resolutions that are NOT pure single-item transforms (scoring-group rollups, resolved tiebreaker
// fields). Pure name-derivations (CamelCase, field/const names, phase prefixes) are intentionally
// NOT stored here: the templates compose them from the `camel`/`phasePrefix` helpers (see
// template_funcs.go) plus a literal suffix, so the naming convention lives in one place and a new
// template rarely needs a new field.
//
// The model never carries runtime values: the actual numbers arrive over the websocket at match
// time, and the generated JS reads them off the live payload by name. Because the same toCamelCase
// produces both a Go struct field and the string a template emits, the two cannot drift.

package main

import (
	"fmt"
	"strings"
)

// CountView is one scoring count scored in one phase. The generated Score field is
// phasePrefix(Phase)+camel(ID)+"Count", composed in the templates.
type CountView struct {
	ID          string
	DisplayName string
	Phase       string // "auto" | "teleop" | "endgame"
	Points      int
}

// ScoringCountView is one scoring count with all of its phases. Group is the resolved rollup-bucket
// id (its scoring_group, else its own id); camel(Group)+"Points" is the ScoreSummary field its
// points feed.
type ScoringCountView struct {
	ID          string
	DisplayName string
	Group       string
	Phases      []CountView
}

// ValueView is one named state of an enum status. (A bool status carries no values; its single
// point value is StatusView.Points.)
type ValueView struct {
	ID          string
	DisplayName string
	Points      int
}

// StatusView is one per-robot status. IsBool drives both the storage ([3]bool vs [3]<Camel>Status)
// and the UI (toggle vs cycle). For a bool, Points is the point value and Values is empty; for an
// enum, Values holds the states (each with its own points) and Points is unused.
type StatusView struct {
	ID          string
	DisplayName string
	Phase       string
	IsBool      bool
	Points      int
	Values      []ValueView
}

// PhaseView groups the counts and statuses scored in one phase (phase-major layout, for the panels).
// Only non-empty phases are included. CountFields is this phase's Score count-field names, in
// declaration order — a precomputed join the referee panel renders as a JS accessor array.
type PhaseView struct {
	Name        string // "auto"
	Title       string // "Auto"
	Counts      []CountView
	Statuses    []StatusView
	CountFields []string
}

// GroupView is one scoring-group rollup bucket: a ScoreSummary point field (camel(ID)+"Points") and
// an audience-display entry. CountFields are the member counts' Score fields, phase-expanded — a
// precomputed join the live audience counter sums.
type GroupView struct {
	ID          string
	DisplayName string
	CountFields []string
}

// RPView is one ranking-point bonus. The generated ScoreSummary bool is camel(ID)+"RankingPoint";
// LogicFunc is the hand-written func in custom_scoring_logic.go that computes it.
type RPView struct {
	ID          string
	DisplayName string
	LogicFunc   string
}

// TiebreakerView is one playoff-tiebreaker comparison: the resolved ScoreSummary field to compare
// and the human label shown when it breaks the tie (e.g. Field "AutoPoints", Label "AUTO POINTS").
type TiebreakerView struct {
	Field string
	Label string
}

// ScoreStmt describes how a test can give an alliance a scoring lead in a generated Score: assign
// Value to score.Field. Field is a generated Score field (a scoring count, or a status array) and
// Value is the Go literal to assign it. Both empty when the config declares nothing to score.
type ScoreStmt struct {
	Field string
	Value string
}

// TemplateData is the complete, stable contract exposed to the .tmpl files.
type TemplateData struct {
	GameName        string
	MinorFoulPoints int
	MajorFoulPoints int
	Phases          []PhaseView        // phase-major: UI sections laid out top-to-bottom
	ScoringCounts   []ScoringCountView // declaration order: count accessors, summary accumulation
	ScoringGroups   []GroupView        // rollup buckets: summary point fields + audience entries
	Statuses        []StatusView
	RankingPoints   []RPView
	// RankingTiebreakerFields are the resolved RankingFields/ScoreSummary field names for each
	// ranking_tiebreakers metric, in order, e.g. ["MatchPoints", "AutoPoints"].
	RankingTiebreakerFields []string
	// RankingTiebreakers are the same tiebreakers with a human column Label alongside the Field, for
	// surfaces that show a readable header (the PDF rankings report) rather than the raw Go field name.
	RankingTiebreakers []TiebreakerView
	PlayoffTiebreakers []TiebreakerView // DetermineMatchStatus tiebreak cascade
	// RankingTestScore is how the generated qualification-rankings test gives an alliance a lead —
	// derived from the config so the test works for count-based, bool-status, and enum-status games
	// alike (and is empty for a game that declares nothing to score).
	RankingTestScore ScoreStmt
}

// metricFieldName maps a tiebreaker metric to its Go field name on RankingFields/ScoreSummary.
// Built-in point metrics have fixed names; any other metric (a scoring-group bucket id, or a status
// id) becomes "{Camel}Points". This is a resolution (a lookup), not a pure transform, so it stays
// here rather than in a template helper.
func metricFieldName(metric string) string {
	switch metric {
	case "auto_points":
		return "AutoPoints"
	case "teleop_points":
		return "TeleopPoints"
	case "endgame_points":
		return "EndgamePoints"
	case "total_points":
		return "MatchPoints"
	default:
		return toCamelCase(metric) + "Points"
	}
}

// playoffTiebreakerLabel returns the human label for a playoff-tiebreaker metric: fixed strings for
// the built-in point metrics, else the uppercased display name of the referenced group/status.
func playoffTiebreakerLabel(metric string, y *GameYAML) string {
	switch metric {
	case "auto_points":
		return "AUTO POINTS"
	case "teleop_points":
		return "TELEOP POINTS"
	case "endgame_points":
		return "ENDGAME POINTS"
	case "total_points":
		return "TOTAL POINTS"
	default:
		// A non-built-in metric is a scoring-group id (or, for an ungrouped count, its own id),
		// or a status id. Prefer the group/count/status display name for the label.
		name := metric
		for _, g := range y.ScoringGroups {
			if g.ID == metric {
				name = g.DisplayName
			}
		}
		for _, sc := range y.ScoringCounts {
			if sc.ID == metric {
				name = sc.DisplayName
			}
		}
		for _, s := range y.Statuses {
			if s.ID == metric {
				name = s.DisplayName
			}
		}
		return strings.ToUpper(name)
	}
}

// rankingTiebreakerLabel returns a short, readable column header for a ranking-tiebreaker metric
// (e.g. "Match Pts", "Auto Pts"), used for the PDF rankings report. Built-in point metrics have
// fixed labels; any other metric is a scoring-group/count/status id, labeled by its display name.
func rankingTiebreakerLabel(metric string, y *GameYAML) string {
	switch metric {
	case "auto_points":
		return "Auto Pts"
	case "teleop_points":
		return "Teleop Pts"
	case "endgame_points":
		return "Endgame Pts"
	case "total_points":
		return "Match Pts"
	default:
		name := metric
		for _, g := range y.ScoringGroups {
			if g.ID == metric {
				name = g.DisplayName
			}
		}
		for _, sc := range y.ScoringCounts {
			if sc.ID == metric {
				name = sc.DisplayName
			}
		}
		for _, s := range y.Statuses {
			if s.ID == metric {
				name = s.DisplayName
			}
		}
		return name
	}
}

// buildRankingTestScore derives a single scoring move that gives an alliance a lead, so the generated
// qualification-rankings test never has to name a hard-coded field. It prefers the first scoring
// count; failing that (a status-only game) the first status — a bool set true on all robots, or an
// enum set to its highest-scoring value. Empty when the config declares nothing to score.
func buildRankingTestScore(y *GameYAML) ScoreStmt {
	if len(y.ScoringCounts) > 0 {
		sc := y.ScoringCounts[0]
		ph := sc.Phases[0]
		return ScoreStmt{Field: phaseFieldPrefix[ph.Phase] + toCamelCase(sc.ID) + "Count", Value: "10"}
	}
	if len(y.Statuses) > 0 {
		st := y.Statuses[0]
		field := toCamelCase(st.ID) + "Statuses"
		if len(st.Values) == 0 { // bool status
			return ScoreStmt{Field: field, Value: "[3]bool{true, true, true}"}
		}
		best := st.Values[0] // enum status: pick the highest-scoring state
		for _, v := range st.Values[1:] {
			if v.Points > best.Points {
				best = v
			}
		}
		enum := "game." + toCamelCase(st.ID) + toCamelCase(best.ID)
		return ScoreStmt{Field: field, Value: fmt.Sprintf("[3]game.%sStatus{%s, %s, %s}", toCamelCase(st.ID), enum, enum, enum)}
	}
	return ScoreStmt{}
}

// phaseOrder is the canonical phase ordering used everywhere a UI is laid out top-to-bottom.
var phaseOrder = []string{"auto", "teleop", "endgame"}

// buildStatusView resolves a schema Status into its view form. IsBool is presence-based: a bool
// status omits the values list (len 0); an enum lists its states.
func buildStatusView(status Status) StatusView {
	sv := StatusView{
		ID:          status.ID,
		DisplayName: status.DisplayName,
		Phase:       status.Phases[0].Phase,
		IsBool:      len(status.Values) == 0,
	}
	if sv.IsBool {
		sv.Points = status.Phases[0].Points
	} else {
		for _, v := range status.Values {
			sv.Values = append(sv.Values, ValueView{ID: v.ID, DisplayName: v.DisplayName, Points: v.Points})
		}
	}
	return sv
}

// buildTemplateData turns a validated GameYAML into the view model the templates consume.
func buildTemplateData(yamlData *GameYAML) TemplateData {
	td := TemplateData{
		GameName:        yamlData.Game.Name,
		MinorFoulPoints: yamlData.Fouls.MinorFoulPoints,
		MajorFoulPoints: yamlData.Fouls.MajorFoulPoints,
	}

	// Per-status views, in declaration order.
	statusViews := make([]StatusView, len(yamlData.Statuses))
	for i, status := range yamlData.Statuses {
		statusViews[i] = buildStatusView(status)
	}
	td.Statuses = statusViews

	// Resolve every scoring count to its rollup bucket once (scoring_group, else itself), so the
	// count-major and group views agree on where each count's points land.
	buckets := buildScoringGroups(yamlData)
	countGroup := make(map[string]string) // scoring-count id -> its bucket id
	for _, bucket := range buckets {
		for _, countID := range bucket.CountIDs {
			countGroup[countID] = bucket.ID
		}
	}

	// Phase-major views, in canonical order, each pre-filtered to its counts and statuses. Empty
	// phases dropped so a template can range without an emptiness check.
	for _, phase := range phaseOrder {
		pv := PhaseView{Name: phase, Title: phaseSectionTitle[phase]}
		for _, sc := range yamlData.ScoringCounts {
			for _, ep := range sc.Phases {
				if ep.Phase == phase {
					pv.Counts = append(pv.Counts, CountView{ID: sc.ID, DisplayName: sc.DisplayName, Phase: phase, Points: ep.Points})
					pv.CountFields = append(pv.CountFields, phaseFieldPrefix[phase]+toCamelCase(sc.ID)+"Count")
				}
			}
		}
		for _, sv := range statusViews {
			if sv.Phase == phase {
				pv.Statuses = append(pv.Statuses, sv)
			}
		}
		if len(pv.Counts) > 0 || len(pv.Statuses) > 0 {
			td.Phases = append(td.Phases, pv)
		}
	}

	// Scoring-count-major views: each count with its phases, in declaration order, and its bucket.
	for _, sc := range yamlData.ScoringCounts {
		scv := ScoringCountView{ID: sc.ID, DisplayName: sc.DisplayName, Group: countGroup[sc.ID]}
		for _, ep := range sc.Phases {
			scv.Phases = append(scv.Phases, CountView{ID: sc.ID, DisplayName: sc.DisplayName, Phase: ep.Phase, Points: ep.Points})
		}
		td.ScoringCounts = append(td.ScoringCounts, scv)
	}

	// Scoring-group rollup buckets — one ScoreSummary point field and audience entry each. CountFields
	// joins each member count to its per-phase Score fields for the live audience counter.
	for _, bucket := range buckets {
		gv := GroupView{ID: bucket.ID, DisplayName: bucket.DisplayName}
		for _, countID := range bucket.CountIDs {
			for i := range yamlData.ScoringCounts {
				if yamlData.ScoringCounts[i].ID == countID {
					for _, ep := range yamlData.ScoringCounts[i].Phases {
						gv.CountFields = append(gv.CountFields, phaseFieldPrefix[ep.Phase]+toCamelCase(countID)+"Count")
					}
				}
			}
		}
		td.ScoringGroups = append(td.ScoringGroups, gv)
	}

	// Ranking tiebreaker field names (resolved), in order, plus a labeled form for readable headers.
	for _, tb := range yamlData.RankingTiebreakers {
		td.RankingTiebreakerFields = append(td.RankingTiebreakerFields, metricFieldName(tb.Metric))
		td.RankingTiebreakers = append(td.RankingTiebreakers, TiebreakerView{
			Field: metricFieldName(tb.Metric),
			Label: rankingTiebreakerLabel(tb.Metric, yamlData),
		})
	}

	// How the qualification-rankings test grants a scoring lead (count/status-aware).
	td.RankingTestScore = buildRankingTestScore(yamlData)

	// Playoff tiebreaker cascade.
	for _, tb := range yamlData.PlayoffTiebreakers {
		td.PlayoffTiebreakers = append(td.PlayoffTiebreakers, TiebreakerView{
			Field: metricFieldName(tb.Metric),
			Label: "TIEBREAK: " + playoffTiebreakerLabel(tb.Metric, yamlData),
		})
	}

	// Ranking points.
	for _, rp := range yamlData.RPs {
		td.RankingPoints = append(td.RankingPoints, RPView{ID: rp.ID, DisplayName: rp.DisplayName, LogicFunc: rp.LogicFunc})
	}

	return td
}

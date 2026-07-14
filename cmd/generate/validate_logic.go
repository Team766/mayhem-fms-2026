// Generate-time validation of the one hand-written, generated-adjacent file: game/custom_scoring_logic.go.
//
// The custom ranking-point functions there reference fields on the generated Score and ScoreSummary
// structs by name. When a scoring element is renamed or removed in custom_game.yaml, those references
// go stale and `go build -tags custom` fails with a bare "Score has no field or method X" — a message
// pitched in generated Go field names, not the yaml ids the author actually edited.
//
// This runs at `go generate` time, where we already know the exact field set the config produces, and
// turns that into a connected error: it names the offending field, points at the line, and suggests
// the closest current field. It is deliberately a best-effort message layer, not a type checker:
//   - It only tracks field accesses on parameters whose type is spelled Score/ScoreSummary in the
//     function signature (the documented logic_func shape). Accesses through a local alias or a helper
//     function are not followed.
//   - Anything it doesn't catch still fails the subsequent `go build`, just with the terse message.
// So a miss degrades to today's behavior; it never lets a genuinely broken reference through.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// validateCustomScoringLogic checks game/custom_scoring_logic.go against the field set the current
// config generates. It reports: (1) any ranking_points logic_func with no matching top-level function,
// and (2) any Score/ScoreSummary field access that isn't a generated field, with a "did you mean"
// suggestion. It returns human-readable error strings (empty if clean).
func validateCustomScoringLogic(yamlData *GameYAML, logicPath string) []string {
	// No ranking points means no logic_func references to satisfy; a missing file is then fine.
	src, err := os.ReadFile(logicPath)
	if err != nil {
		if os.IsNotExist(err) {
			if len(yamlData.RPs) == 0 {
				return nil
			}
			return []string{fmt.Sprintf("%s: file not found, but ranking_points declare logic funcs that must be defined there", logicPath)}
		}
		return []string{fmt.Sprintf("%s: %v", logicPath, err)}
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, logicPath, src, parser.AllErrors)
	if err != nil {
		return []string{fmt.Sprintf("%s: could not parse: %v", logicPath, err)}
	}

	scoreFields, summaryFields := generatedFieldSets(yamlData)

	var errs []string

	// (1) Every declared logic_func must exist as a top-level function.
	funcs := map[string]bool{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			funcs[fn.Name.Name] = true
		}
	}
	var missing []RankingPoint
	for _, rp := range yamlData.RPs {
		if rp.LogicFunc != "" && !funcs[rp.LogicFunc] {
			missing = append(missing, rp)
		}
	}
	if len(missing) > 0 {
		errs = append(errs, missingLogicFuncMessage(yamlData, logicPath, missing))
	}

	// (2) Field accesses on Score/ScoreSummary parameters must be generated fields.
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// Map each parameter of type Score/ScoreSummary to the field set it must satisfy.
		paramFields := map[string]map[string]bool{}
		if fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				switch typeIdentName(field.Type) {
				case "Score":
					for _, name := range field.Names {
						paramFields[name.Name] = scoreFields
					}
				case "ScoreSummary":
					for _, name := range field.Names {
						paramFields[name.Name] = summaryFields
					}
				}
			}
		}
		if len(paramFields) == 0 {
			continue
		}

		// One pre-order walk. A selector in call position (x.Method()) is a method call, not field
		// access; we record those as we descend so the field check can skip them — ast.Inspect visits
		// a CallExpr before its own .Fun selector child, so the marker is always set in time.
		methodCalls := map[*ast.SelectorExpr]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					methodCalls[sel] = true
				}
				return true
			}
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || methodCalls[sel] {
				return true
			}
			base, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			fields, tracked := paramFields[base.Name]
			if !tracked || fields[sel.Sel.Name] {
				return true
			}
			pos := fset.Position(sel.Sel.Pos())
			msg := fmt.Sprintf("%s:%d:%d: '%s.%s' is not a field generated from custom_game.yaml",
				logicPath, pos.Line, pos.Column, base.Name, sel.Sel.Name)
			if suggestion := closestField(sel.Sel.Name, fields); suggestion != "" {
				msg += fmt.Sprintf(" — did you mean '%s'?", suggestion)
			}
			msg += " (a scoring element was likely renamed or removed; update this reference to match)"
			errs = append(errs, msg)
			return true
		})
	}

	return errs
}

// missingLogicFuncMessage builds a copy-pasteable stub for each undefined logic_func, followed by a
// one-shot reference of the data available to scoring logic for the current config — so an author
// adding a ranking point sees the exact function shape and field/helper names without hunting through
// the generated Go.
func missingLogicFuncMessage(y *GameYAML, logicPath string, missing []RankingPoint) string {
	var b strings.Builder
	if len(missing) == 1 {
		fmt.Fprintf(&b, "%s: ranking_points '%s' needs logic_func '%s'. Add it and implement it:\n",
			logicPath, missing[0].ID, missing[0].LogicFunc)
	} else {
		fmt.Fprintf(&b, "%s: %d ranking-point logic functions are missing. Add them and implement:\n", logicPath, len(missing))
	}
	for _, rp := range missing {
		fmt.Fprintf(&b, "\nfunc %s(score, opponentScore Score, summary ScoreSummary) bool {\n\t// TODO: implement (ranking_points '%s')\n\treturn false\n}\n",
			rp.LogicFunc, rp.ID)
	}
	b.WriteString(dataReference(y))
	return b.String()
}

// dataReference renders the fields available to scoring logic — grouped by kind and aligned one
// element per line — so an author can scan the id -> generated-name mapping instead of parsing one
// long comma-separated list.
func dataReference(y *GameYAML) string {
	// Column width to left-align the id/label across every section.
	width := len("scoring groups")
	for _, sc := range y.ScoringCounts {
		if len(sc.ID) > width {
			width = len(sc.ID)
		}
	}
	for _, st := range y.Statuses {
		if len(st.ID) > width {
			width = len(st.ID)
		}
	}
	row := func(label, value string) string { return fmt.Sprintf("      %-*s  %s\n", width, label, value) }

	var b strings.Builder
	b.WriteString("\nData available to the logic (generated from the current custom_game.yaml):\n")

	if len(y.ScoringCounts) > 0 {
		b.WriteString("\n  score / opponentScore — raw per-element counts:\n")
		for _, sc := range y.ScoringCounts {
			fields := make([]string, len(sc.Phases))
			for i, ep := range sc.Phases {
				fields[i] = phaseFieldPrefix[ep.Phase] + toCamelCase(sc.ID) + "Count"
			}
			b.WriteString(row(sc.ID, strings.Join(fields, ", ")))
		}
	}

	if len(y.Statuses) > 0 {
		b.WriteString("\n  score / opponentScore — per-robot status helpers:\n")
		for _, st := range y.Statuses {
			name := toCamelCase(st.ID)
			var sig string
			if len(st.Values) == 0 {
				sig = fmt.Sprintf("score.Any%sStatus() / score.Count%sStatus()", name, name)
			} else {
				vals := make([]string, len(st.Values))
				for i, v := range st.Values {
					vals[i] = name + toCamelCase(v.ID)
				}
				sig = fmt.Sprintf("score.Any%sStatus(atLeast %sStatus) / score.Count%sStatus(...)  [%s]",
					name, name, name, strings.Join(vals, ", "))
			}
			b.WriteString(row(st.ID, sig))
		}
	}

	b.WriteString("\n  summary — computed point totals:\n")
	b.WriteString(row("phases/match", "AutoPoints, TeleopPoints, EndgamePoints, MatchPoints, FoulPoints, Score"))
	var groups []string
	for _, bucket := range buildScoringGroups(y) {
		groups = append(groups, toCamelCase(bucket.ID)+"Points")
	}
	if len(groups) > 0 {
		b.WriteString(row("scoring groups", strings.Join(groups, ", ")))
	}
	var statusPts []string
	for _, st := range y.Statuses {
		statusPts = append(statusPts, toCamelCase(st.ID)+"Points")
	}
	if len(statusPts) > 0 {
		b.WriteString(row("statuses", strings.Join(statusPts, ", ")))
	}

	b.WriteString("\n  opponent fouls (bonus RP):\n")
	b.WriteString("      opponentScore.HasRankingPointFoul(ruleNumbers ...string)\n")
	return b.String()
}

// generatedFieldSets returns the exact field names the score.go.tmpl and score_summary.go.tmpl
// templates emit for this config — kept in sync with those templates by construction.
func generatedFieldSets(yamlData *GameYAML) (scoreFields, summaryFields map[string]bool) {
	scoreFields = map[string]bool{
		// Hand-written base fields on the generated Score struct.
		"Fouls": true, "PlayoffDq": true, "Hub": true,
	}
	for _, sc := range yamlData.ScoringCounts {
		for _, ep := range sc.Phases {
			scoreFields[phaseFieldPrefix[ep.Phase]+toCamelCase(sc.ID)+"Count"] = true
		}
	}
	for _, st := range yamlData.Statuses {
		scoreFields[toCamelCase(st.ID)+"Statuses"] = true
	}

	summaryFields = map[string]bool{
		"AutoPoints": true, "TeleopPoints": true, "EndgamePoints": true, "MatchPoints": true,
		"FoulPoints": true, "Score": true, "PlayoffDq": true, "NumOpponentMajorFouls": true,
		"BonusRankingPoints": true,
	}
	for _, bucket := range buildScoringGroups(yamlData) {
		summaryFields[toCamelCase(bucket.ID)+"Points"] = true
	}
	for _, st := range yamlData.Statuses {
		summaryFields[toCamelCase(st.ID)+"Points"] = true
	}
	for _, rp := range yamlData.RPs {
		summaryFields[toCamelCase(rp.ID)+"RankingPoint"] = true
	}
	return scoreFields, summaryFields
}

// typeIdentName returns the base type name of a parameter type expression, unwrapping a pointer and a
// package qualifier so "Score", "*Score", and "game.Score" all yield "Score". Returns "" otherwise.
func typeIdentName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return typeIdentName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// closestField returns the valid field name closest to target by edit distance, if one is close
// enough to be a plausible typo/rename (within a third of the target's length), else "".
func closestField(target string, valid map[string]bool) string {
	best := ""
	bestDist := len(target)/3 + 1 // threshold: at most ~1/3 of the name may differ
	for field := range valid {
		if d := levenshtein(target, field); d < bestDist {
			bestDist = d
			best = field
		}
	}
	return best
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

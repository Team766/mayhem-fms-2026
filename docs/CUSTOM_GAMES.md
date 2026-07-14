# Custom Game Mode

## Overview

Cheesy Arena normally implements one specific FRC game per season, hardcoded into the binary. Custom game mode is an alternative compile target (Go build tag `custom`) that drives the same arena/match-control system from a YAML config instead — scoring rules, the referee/scoring/audience UI, and rankings all come from `game/custom_game.yaml` rather than hand-written game logic.

Use it for off-season events, custom or non-FRC competitions, demos, or any event running a game that isn't the current FRC season's.

## Quick Start

```bash
# 1. Edit the game definition
vim game/custom_game.yaml

# 2. Write the RP logic functions referenced by custom_game.yaml
vim game/custom_scoring_logic.go

# 3. Generate the Go structs, scoring logic, and UI surfaces
go generate ./...

# 4. Build and run
go build -tags custom
./cheesy-arena [-dev]
```

Open `http://<host>:8080`. `-dev` is required when testing the FMS locally rather than on a machine with IP `10.0.100.5`.

Re-run `go generate ./...` any time `custom_game.yaml` changes, then rebuild.

## How to define a custom game

### Schema (`custom_game.yaml`)

The authoritative schema is `cmd/generate/schema.go`. Schema walkthrough, with examples from the shipped `game/custom_game.yaml`:

**`game`** — top-level metadata.
```yaml
game:
  name: "My Custom Game"
```

**`fouls`** — points awarded to the opponent per foul.
```yaml
fouls:
  minor_foul_points: 5
  major_foul_points: 15
```

**`game_pieces`** — the game pieces being manipulated by alliance robots. Required on each `scoring_counts` entry.
```yaml
game_pieces:
  - id: game_piece_1
    display_name: "Game Piece 1"
  - id: game_piece_2
    display_name: "Game Piece 2"
```

**`scoring_groups`** — rollup buckets. A `scoring_counts` entry tagged with a `scoring_group` has its live count and points summed into that bucket — both the `ScoreSummary` point field (`summary.<Group>Points`, e.g. `summary.Structure1Points`) and the audience display. An entry with no `scoring_group` is its own bucket under its own id (so a lone element needs no wrapper group). Tiebreakers reference buckets, not raw elements.
```yaml
scoring_groups:
  - id: structure1
    display_name: "Struct 1"
  - id: structure2
    display_name: "Struct 2"
```

**`scoring_counts`** — countable scoring actions. Each lists the phases it can be scored in, each with its own point value.
```yaml
scoring_counts:
  - id: structure1_level1
    display_name: "S1L1"
    game_piece: game_piece_1
    scoring_group: structure1
    phases:
      - phase: auto
        points: 3
      - phase: teleop
        points: 1

  - id: structure1_level2
    display_name: "S1L2"
    game_piece: game_piece_1
    scoring_group: structure1
    phases:
      - phase: auto
        points: 5
      - phase: teleop
        points: 3
```
- `phase` is `auto`, `teleop`, or `endgame`. An entry can declare more than one if it's scorable in several, each with an independent count and point value — but **not both `teleop` and `endgame`**, since teleop play continues through endgame (a teleop count is already live during endgame).
- Every `scoring_counts` entry must set `game_piece`. `scoring_group` is optional — set it when you want several entries (e.g. `structure1_level1` and `structure1_level2`) rolled into one total (e.g. "Struct 1") in both the summary and the audience display; leave it off and the entry stands alone as its own bucket.
- `display_name` is shown on the scoring panel. The audience display shows the resolved `scoring_group` label (or, for an ungrouped entry, this `display_name`).
- Scoring panel and referee panel never group — every entry always shows as its own raw count, organized by phase.

**`statuses`** — tracks a per-robot status for an auto or endgame achievement (3 robots per alliance).
```yaml
statuses:
  - id: leave
    display_name: "Leave"
    phases:
      - phase: auto
        points: 3

  - id: park
    display_name: "Park"
    phases:
      - phase: endgame
        points: 2
```
A status's value can be a simple bool, or one of several named states:
```yaml
  - id: muster
    display_name: "Muster"
    phases:
      - phase: auto
    values:
      - id: none
        display_name: "None"
        points: 0
      - id: partial
        display_name: "Partial"
        points: 3
      - id: full
        display_name: "Full"
        points: 6
```
- `values` (2 or more entries) tracks the status as one of several named states, each with its own point value.
- Without `values`, the status is a simple true/false flag, and `points` is the value awarded when true.
- `phases` takes exactly one entry, `auto` or `endgame` (no `teleop`).

**`ranking_points`** — custom RP bonus conditions, backed by hand-written Go (see below).
```yaml
ranking_points:
  - id: auton_rp
    display_name: "Auto Bonus"
    logic_func: "ComputeAutonRP"
```

**`ranking_tiebreakers`** / **`playoff_tiebreakers`** — metric cascades for sorting rankings and breaking playoff ties. Each entry is one `metric`: a built-in (`auto_points`, `teleop_points`, `endgame_points`, `total_points`), a **scoring-group id** (a `scoring_groups` id, or an ungrouped entry's own id), or a `statuses` id. A *grouped* element can't be tiebroken on individually — tiebreak on its group. Opponent major-foul count is always the first playoff tiebreaker, implicitly.
```yaml
ranking_tiebreakers:
  - metric: total_points
  - metric: auto_points

playoff_tiebreakers:
  - metric: auto_points
  - metric: total_points
```

### Custom RP Scoring

`game/custom_scoring_logic.go` (`//go:build custom`) is the one generated-adjacent file you write by hand — the generator never overwrites it. Every `logic_func` named in `ranking_points` needs a matching function here:

```go
func ComputeXRP(score, opponentScore Score, summary ScoreSummary) bool
```

`summary` carries this alliance's point totals — prefer its generated totals (e.g. `summary.AutoPoints`, `summary.Structure1Points`) over re-deriving them from raw counts, so the RP logic can't drift from the generated point math. Note the ranking-point fields (`summary.<X>RankingPoint`) and `summary.BonusRankingPoints` are **not yet populated** when these funcs run — they're computed from the funcs' results — so don't read them here. `score`/`opponentScore` give the raw per-element counts for thresholds the summary doesn't expose (and for cross-alliance logic). The opponent's *summary* is intentionally not passed — it would recurse back through this same logic.

For the `custom_game.yaml` schema above, that would look like:
```go
func ComputeAutonRP(score, opponentScore Score, summary ScoreSummary) bool {
	return summary.AutoPoints >= 9   // reference the generated total, not raw counts
}

func ComputeEndgameRP(score, opponentScore Score, summary ScoreSummary) bool {
	return score.CountParkStatus() >= 2   // status helper (see below)
}
```

**Status helpers.** For every status the generator emits `Any<Status>Status` / `Count<Status>Status` methods on `Score`, so you don't hand-roll the three-robot loop. A bool status takes no argument (`score.AnyParkStatus()`, `score.CountParkStatus()`); an **enum** status takes an `atLeast` threshold and counts robots whose state is at least that value, compared by **declared value order** (list your enum values weakest-first): `score.CountMusterStatus(MusterPartial) >= 2` means "at least two robots mustered partially or better."

**Opponent-foul helper.** To award a bonus RP when the *opponent* commits specific fouls (the standard FRC pattern), call `HasRankingPointFoul` on the opponent's score with the rule numbers:
```go
func ComputeSafetyRP(score, opponentScore Score, summary ScoreSummary) bool {
	return opponentScore.HasRankingPointFoul("G418", "G428")
}
```
A foul only counts when its rule is flagged `is_ranking_point` in [`game/custom_rules.go`](#custom-rules), so keep that flag in sync with the numbers you pass. (This helper is committed library code in `game/custom_foul_helpers.go`, not generated.)

Field names (`AutoStructure1Level1Count`, `ParkStatuses`, etc.) are derived from each `scoring_counts`/`statuses` id — run the generator first so they exist. Renaming or removing an entry means updating this file by hand to match. To make that easy to get right, `go generate` validates this file against the current config: when a `logic_func` named in `ranking_points` has no matching function here, it prints a **copy-pasteable stub** for it followed by a reference of the data available for the current config (the `score`/`opponentScore` count fields, the status helpers with their enum values, and the `summary` point totals) — so you can start without hunting through the generated Go. It also flags any `score`/`opponentScore`/`summary` field access that isn't a generated field — pointing at the line and suggesting the closest current field (e.g. *"`score.AutoStructure1Level1Count` is not a field generated from custom_game.yaml — did you mean `AutoShelfL1Count`?"*). It's a best-effort aid over direct parameter accesses; anything it can't see (a local alias, a helper function) still surfaces at `go build -tags custom`, just as a terser compiler error.

### Custom Rules

`game/custom_rules.go` (`//go:build custom`) defines the rule list shown in the referee panel's foul-assignment dropdown — also hand-written, never generated. It ships with a placeholder ruleset; replace the `rules` slice with your own:

```go
var rules = []*Rule{
	{1, "G206", false, true, "Description of the rule..."},
	// IsMajor, IsRankingPoint, ...
}
```

Rule assignment in the referee panel is optional, so an unedited or empty ruleset doesn't block scoring.

### Custom UI Template and Game Assets

The scoring panel, referee panel, and audience display HTML/JS are produced by the generator from hand-edited **template sources** — `templates/custom_{scoring_panel,referee_panel,audience_display}.html.tmpl` and `static/js/custom_{scoring_panel,referee_panel,audience_display}.js.tmpl`. These are committed, hand-editable defaults that ship with a working layout, so for normal use there's nothing to touch. To customize a surface's markup or behavior, edit its `.tmpl` and re-run `go generate ./...`.

Inside a `.tmpl`, `[[ ]]` marks generate-time directives; `{{ }}` is passed through untouched for the FMS server's own template engine to handle at request time. The data a template can reference (phases, scoring counts, scoring groups, statuses, and their pre-resolved field names) is the `TemplateData` value defined in `cmd/generate/viewmodel.go` — read that struct for the available fields.

Styling is hand-editable: `static/css/scoring_panel.css`, `static/css/referee_panel.css`, and `static/css/audience_display.css` are plain, committed stylesheets shared between the stock and custom builds, not generated. Custom-panel-only rules live in `static/css/custom_scoring_panel.css`. Edit them directly to restyle any of the three surfaces.

For branding, swap `static/img/game-logo.png` (small in-match badge) and `static/img/blinds-logo.png` (large final-score/idle badge) for your game's artwork. Both render as circular badges (`object-fit: cover`, `border-radius: 50%`) — crop source images tightly to a square bounding the circle to avoid visible margins.

## Generated Files

Regenerate with **`go generate ./...`** (or `go run ./cmd/generate`). Use the `./...` form: a bare `go generate` run from the repo root matches only the root package, which has no directive, so it does nothing and prints nothing — edits to `custom_game.yaml` then silently don't take effect and the next `go build -tags custom` compiles against stale generated code. The command reads `custom_game.yaml` (and, for the UI surfaces, the `custom_*.tmpl` template sources above) and writes these files — all gitignored, all regenerated from scratch on every run, never hand-edited:

| File | Contents |
|---|---|
| `game/generated_constants.go` | `CustomGameMode = true`, foul points, enum constants for status `values` |
| `game/generated_score.go` | `Score` struct (one counter per declared phase, status arrays), `Phase` enum, `Adjust*`/`Set*Status` methods |
| `game/generated_score_summary.go` | `ScoreSummary`, point accumulation (per scoring group), `DetermineMatchStatus` tiebreak cascade |
| `game/generated_ranking_fields.go` | `RankingFields`, `Less()` tiebreaker comparison |
| `templates/generated_scoring_panel.html` + `static/js/generated_scoring_panel.js` | Scoring panel: counters and status toggles, organized by phase |
| `templates/generated_audience_display.html` + `static/js/generated_audience_display.js` | Audience display: live ticker and final breakdown, grouped per the scoring-group resolution chain. The HTML loads the shared `audience_display.js` plus the generated companion, which defines only the custom `handleRealtimeScoreGenerated`/`handleScorePostedGenerated` handlers the shared file dispatches to. |
| `templates/generated_referee_panel.html` + `static/js/generated_referee_panel.js` | Referee panel: raw per-element counts by phase, per-robot status badges |
| `web/generated_reports_rankings_custom.go` | Qualification-rankings CSV/PDF report handlers, whose columns track the configured `ranking_tiebreakers` |
| `game/generated_score_test.go`, `game/generated_score_summary_test.go`, `game/generated_ranking_fields_test.go` | Tests for `Score` mutators, `ScoreSummary` (point computation, group rollup, tiebreak cascade), and `RankingFields` (ranking sort cascade) |
| `web/generated_reports_rankings_custom_test.go`, `tournament/generated_qualification_rankings_custom_test.go` | Tests for the rankings report (CSV columns per configured tiebreakers) and end-to-end `CalculateRankings` (scores via the first declared scoring count, so no field name is hard-coded) |
| `cmd/generate/generated_template_test.go` | Tests that the rendered templates parse and contain the expected per-element markup |

`go run ./cmd/generate clean` removes all of the above. The `custom_*.tmpl` template sources are committed and hand-edited, so they are *not* removed by `clean`.

## What's disabled in custom mode

Custom mode keeps the full arena/match-control system — timing, driver-station networking, the PLC, field monitor, alliance selection, playoff brackets, qualification scheduling, and reports all work exactly as in the stock build. What it turns off are the integrations that assume the *current FRC season's* game and event infrastructure:

- **The Blue Alliance (TBA)** — both publishing match results/rankings and downloading team data. The TBA client is compiled out entirely (`partner/tba_custom.go` is a no-op stub), and `TbaPublishingEnabled`/`TbaDownloadEnabled` are force-cleared.
- **Nexus for FRC** — including Nexus AutoQueue. `NexusEnabled`/`NexusAutoQueueEnabled` are force-cleared.
- **Bitfocus Companion / Stream Deck** — the `CompanionAddress` is force-cleared, disabling Companion-driven display switching.

These are forced off whenever the settings form is saved under a custom build (`web/setup_settings.go`, gated on `game.CustomGameMode`), regardless of what the form submitted — the corresponding settings-page controls have no effect in a custom binary. There is no per-event toggle to re-enable them; they are unavailable by construction because a custom game has no FRC event key, no TBA event, and no season-specific Companion layout.

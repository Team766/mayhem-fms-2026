//go:build custom

package game

type Rule struct {
	Id             int
	RuleNumber     string
	IsMajor        bool
	IsRankingPoint bool
	Description    string
}

// All rules from the 2022 game that carry point penalties.
// @formatter:off
var rules = []*Rule{

	{1, "G210", true, false, "A strategy aimed at forcing an opponent to violate a rule is not allowed."},
	{2, "G401", true, false, "In AUTO, each DRIVE TEAM member must remain in their staged areas. A DRIVE TEAM member staged behind a HUMAN STARTING LINE may not contact anything in front of that HUMAN STARTING LINE, unless for personal or equipment safety, to press the E-Stop or A-Stop, or granted permission by a Head REFEREE or FTA."},
	{3, "G402", true, false, "In AUTO, a DRIVE TEAM member may not directly or indirectly interact with a ROBOT or an OPERATOR CONSOLE unless for personal safety, OPERATOR CONSOLE safety, or pressing an E-Stop or A-Stop."},
	{4, "G403", false, false, "A ROBOT may not cross the midfield line during auton."},
	{5, "G408", true, false, "Neither a ROBOT nor a HUMAN PLAYER may damage a treasure."},
	{6, "G415", true, false, "A ROBOT may not contact another robot inside of its perimeter."},
	{7, "G424", true, false, "A ROBOT may not deliberately attach to, tip, or entangle with an opponent ROBOT."},
	{8, "G425", false, false, "A ROBOT may not PIN an opponent's ROBOT for more than 3 seconds."},
	{9, "G429", true, false, "A DRIVE TEAM member must remain in their designated area as follows: A. DRIVERS and COACHES may not contact anything outside their ALLIANCE AREA, B. a DRIVER must use the OPERATOR CONSOLE in the DRIVER STATION to which they are assigned, as indicated on the team sign, C. a HUMAN PLAYER may not contact anything outside their ALLIANCE AREA, and D. a TECHNICIAN may not contact anything outside their designated area."},
	{10, "G430", true, false, "A ROBOT shall be operated only by the DRIVERS."},
	{11, "G434", true, false, "COACHES may not touch CANNONBALLS, unless for safety purposes."},
	{12, "MA2601", true, true, "Contacting the opposing teams balance beam at during the endgame [Ref Discretion, Can stack with MA2602]. This results in an automatic endgame RP for the other team, no matter the scoring."},
	{12, "MA2602", true, true, "Contacting the opposing team AT ALL while they’re in their own Balance Beam 5 seconds after endgame begins [Can stack with MA2601]. This results in an automatic endgame RP for the other team, no matter the scoring."},
	{13, "MA2603", true, true, "Going into another team's safe house within auton. This also results in an automatic auto RP for the other team, no matter the scoring. "},
	{14, "MA2604", true, true, "No Hoarding: More than 6 non-scoring treasures in your safe house. Major Foul is given at the beginning of infraction, given every 10 seconds 6 non-scoring treasures are in the safe house."},
	{15, "MA2606", false, false, "MA2606 Violation: Entering another alliance’s Human Player Loading zone, Safe House, or touching the balance beam before endgame. This can be reapplied every 10 seconds if contact does not end."},
	{16, "MA2607", false, false, "Placing/Throwing a treasure into the field not through the Human Player Loading holes [doesn’t apply during the “toss”, may be escalated by the head ref if flagrant]"},
	{17, "MA2608", false, false, "Placing/Throwing a treasure that bounces not first on the same alliance’s robot, or the Human Player Loading zone Floor."},
	{18, "MA2609", false, false, "Contacting scoring own shelf repeatably or in a unsafe manner"},
	{19, "MA2610", false, false, "Starting with over 1 piece during auton. The robot is also ineligible to gain points within auton by scoring in the shelf"},
	{20, "MA2611", false, false, "Descoring pieces from other teams [can stack with MA2620]"},
	{21, "MA2612", false, false, "Robot deliberately permanently destroying a treasure "},
	{22, "MA2613", false, false, "Adding treasures within endgame [treasure does not count for scoring]"},
	{23, "MA2614", false, false, "Placing/Shooting treasures within the opposing alliance zones"},
	{24, "MA2616", false, false, "Controlling more than 3 gamepieces at a non-momentarily time [additional treasures do not count for scoring][ref discretion]"},
	{25, "MA2617", false, false, "Throwing a treasure into/on the shelf not inside own Safe House[treasure does not count for scoring]"},
}

// @formatter:on
var ruleMap map[int]*Rule

// Returns the rule having the given ID, or nil if no such rule exists.
func GetRuleById(id int) *Rule {
	return GetAllRules()[id]
}

// Returns a slice of all defined rules that carry point penalties.
func GetAllRules() map[int]*Rule {
	if ruleMap == nil {
		ruleMap = make(map[int]*Rule, len(rules))
		for _, rule := range rules {
			ruleMap[rule.Id] = rule
		}
	}
	return ruleMap
}

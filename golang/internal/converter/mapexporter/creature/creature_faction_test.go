package creature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
)

func factionFixture() *factionCatalog {
	return &factionCatalog{factions: map[int]factionDefinition{
		100: {ID: 100, Name: "Same", Kind: "map_player"}, 200: {ID: 200, Name: "Other", Kind: "map_player"}, 300: {ID: 300, Name: "Generic", Kind: "neutral_by_reaction"},
	}, templates: map[int]factionTemplate{
		1: {ID: 1, Faction: 1, Group: 3, Friends: 2, Enemies: 4}, 2: {ID: 2, Faction: 2, Group: 5, Friends: 4, Enemies: 2},
		10: {ID: 10, Faction: 100, Group: 8}, 11: {ID: 11, Faction: 100, Group: 8}, 20: {ID: 20, Faction: 200, Group: 16},
		50: {ID: 50, Faction: 300, Enemies: 2},
	}}
}
func TestFactionMergePreservesBothDirectionsAndNames(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*factionCatalog)
		wantSame bool
	}{
		{"equivalent", func(c *factionCatalog) {}, true},
		{"outgoing differs", func(c *factionCatalog) { a := c.templates[10]; a.Enemies = 16; c.templates[10] = a }, false},
		{"incoming differs", func(c *factionCatalog) {
			a := c.templates[11]
			a.Group = 32
			c.templates[11] = a
			b := c.templates[20]
			b.Enemies = 32
			c.templates[20] = b
		}, false},
		{"different names", func(c *factionCatalog) { a := c.templates[11]; a.Faction = 200; c.templates[11] = a }, false},
		{"difference only toward generic neutral", func(c *factionCatalog) {
			a := c.templates[10]
			a.Enemies = 16
			c.templates[10] = a
			f := c.factions[200]
			f.Kind = "neutral_by_reaction"
			c.factions[200] = f
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := factionFixture()
			tc.change(c)
			p, err := MakeFactionPlan("Horde", map[int]int{10: 4, 11: 5, 20: 2}, c)
			if err != nil {
				t.Fatal(err)
			}
			if (p.owners[10] == p.owners[11]) != tc.wantSame {
				t.Fatalf("incorrect collapse: %+v", p.Players)
			}
			// Every retained original template pair must exactly match its merged edge.
			for a, as := range p.owners {
				if as >= 24 {
					continue
				}
				for b, bs := range p.owners {
					if bs >= 24 {
						continue
					}
					ai, bi := 0, 0
					for i, g := range p.Players {
						if g.Slot == as {
							ai = i
						}
						if g.Slot == bs {
							bi = i
						}
					}
					if p.Reactions[ai][bi] != reactionTo(c.templates[a], c.templates[b]) {
						t.Fatal("lost directed reaction")
					}
				}
			}
		})
	}
}
func TestFactionReactionPriorityAndSide(t *testing.T) {
	c := factionFixture()
	for _, tc := range []struct {
		side  string
		owner int
	}{{"Horde", 27}, {"Alliance", 24}} {
		p, err := MakeFactionPlan(tc.side, map[int]int{50: 1}, c)
		if err != nil || p.owners[50] != tc.owner {
			t.Fatalf("%s: %+v %v", tc.side, p, err)
		}
	}
	target := c.templates[1]
	for _, tc := range []struct {
		source factionTemplate
		want   FactionReaction
	}{
		{factionTemplate{Faction: 300, Enemies: 7, FriendIDs: []int{1}}, FactionFriendly},
		{factionTemplate{Faction: 300, Friends: 7, EnemyIDs: []int{1}}, factionHostile},
		{factionTemplate{Faction: 300, Flags: 0x2000}, factionHostile},
		{factionTemplate{Faction: 300}, factionNeutral},
	} {
		if got := reactionTo(tc.source, target); got != tc.want {
			t.Fatalf("%+v: %s want %s", tc.source, got, tc.want)
		}
	}
}
func TestFactionOverflowMovesSmallestGroupsAndRechecksMerges(t *testing.T) {
	c := factionFixture()
	counts := map[int]int{10: 10, 11: 10, 20: 1}
	a := c.templates[10]
	a.Enemies = 16
	c.templates[10] = a
	for i := 0; i < 20; i++ {
		id := 1000 + i
		c.factions[id] = factionDefinition{ID: id, Name: fmt.Sprintf("Group%02d", i), Kind: "map_player"}
		c.templates[id] = factionTemplate{ID: id, Faction: id}
		counts[id] = 50
	}
	p, err := MakeFactionPlan("Horde", counts, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Collapsed) != 1 || p.Collapsed[0].TemplateIDs[0] != 20 || p.owners[20] != 27 {
		t.Fatalf("wrong overflow choice: %+v", p.Collapsed)
	}
	if p.owners[10] != p.owners[11] || len(p.Players) != 22 {
		t.Fatal("did not merge after removing distinguishing participant")
	}
	first, _ := json.Marshal(p)
	for range 10 {
		again, err := MakeFactionPlan("Horde", counts, c)
		raw, _ := json.Marshal(again)
		if err != nil || !bytes.Equal(first, raw) {
			t.Fatal("nondeterministic faction plan")
		}
	}
	for _, g := range p.Players[1:] {
		if g.Slot < 2 || g.Slot > 23 {
			t.Fatalf("invalid NPC slot %d", g.Slot)
		}
	}
}
func TestFactionCatalogAndInvalidInput(t *testing.T) {
	c, err := loadCreatureFactions()
	if err != nil {
		t.Fatal(err)
	}
	if c.factions[72].Kind != "map_player" || c.factions[31].Kind != "neutral_by_reaction" {
		t.Fatal("curated policy missing")
	}
	for _, side := range []string{"Horde", "Alliance"} {
		p, err := MakeFactionPlan(side, map[int]int{12: 2, 14: 3, 32: 1, 38: 2, 35: 4}, c)
		if err != nil {
			t.Fatal(err)
		}
		if p.owners[12] < 2 || p.owners[12] >= 24 || p.owners[14] != 24 || p.owners[32] != 27 || p.owners[38] != 24 || p.owners[35] != 27 {
			t.Fatalf("wrong catalog ownership: %v", p.owners)
		}
	}
	if _, err := MakeFactionPlan("Scourge", nil, nil); err == nil {
		t.Fatal("invalid side accepted")
	}
	if _, err := MakeFactionPlan("Horde", map[int]int{999999: 1}, c); err == nil {
		t.Fatal("unknown template silently classified")
	}
}

func TestFactionLuaNamedVariablesAndConsecutiveDirections(t *testing.T) {
	p := &FactionPlan{
		UserSlot: 1,
		Players:  []FactionPlayer{{Slot: 1, Name: "Player (Alliance)", FactionName: "Alliance"}, {Slot: 2, Name: "Stormwind", FactionName: "Stormwind"}, {Slot: 3, Name: "Stormwind [123]", FactionName: "Stormwind"}},
		Reactions: [][]FactionReaction{
			{FactionFriendly, factionHostile, factionNeutral},
			{FactionFriendly, FactionFriendly, factionNeutral},
			{factionHostile, FactionFriendly, FactionFriendly},
		},
	}
	script := []byte("function Trig_LUA_map_init_Actions()\nend\n")
	got := InjectFactionAlliances(script, p)
	if !bytes.Equal(got, InjectFactionAlliances(got, p)) {
		t.Fatal("faction injection is not idempotent")
	}
	text := string(got)
	for _, declaration := range []string{
		"local player_Alliance_1 = Player(1)",
		"local player_Stormwind_2 = Player(2)",
		"local player_Stormwind_3 = Player(3)",
		"local neutralHostile = Player(24)",
		"local neutralPassive = Player(27)",
	} {
		if !strings.Contains(text, declaration) || strings.Index(text, declaration) > strings.Index(text, "    setAllianceState(") {
			t.Fatalf("declaration missing or after calls: %s", declaration)
		}
	}
	if strings.Count(text, "local function setAllianceState(") != 1 || strings.Count(text, "SetPlayerAllianceStateBJ(") != 8 || strings.Contains(text, "SetPlayerAlliance(") {
		t.Fatal("helper must use the eight common-library BJ states without manual flag overrides")
	}
	// Match every common-library state, including its p2-to-p1 convention.
	if !strings.Contains(text, `state == "enemy" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_UNALLIED)`) {
		t.Fatal("incorrect common-library mapping: enemy")
	}
	if !strings.Contains(text, `state == "enemy vision" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_UNALLIED_VISION)`) {
		t.Fatal("incorrect common-library mapping: enemy vision")
	}
	if !strings.Contains(text, `state == "allied" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED)`) {
		t.Fatal("incorrect common-library mapping: allied")
	}
	if !strings.Contains(text, `state == "allied vision" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED_VISION)`) {
		t.Fatal("incorrect common-library mapping: allied vision")
	}
	if !strings.Contains(text, `state == "allied share unit" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED_UNITS)`) {
		t.Fatal("incorrect common-library mapping: allied share unit")
	}
	if !strings.Contains(text, `state == "allied share advanced" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED_ADVUNITS)`) {
		t.Fatal("incorrect common-library mapping: allied share advanced")
	}
	if !strings.Contains(text, `state == "neutral" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_NEUTRAL)`) {
		t.Fatal("incorrect common-library mapping: neutral")
	}
	if !strings.Contains(text, `state == "neutral vision" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_NEUTRAL_VISION)`) {
		t.Fatal("incorrect common-library mapping: neutral vision")
	}
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "    setAllianceState(") {
			lines = append(lines, line)
		}
	}
	want := []string{
		`    setAllianceState(player_Stormwind_2, player_Alliance_1, "enemy")`,
		`    setAllianceState(player_Alliance_1, player_Stormwind_2, "allied share unit")`,
		`    setAllianceState(player_Stormwind_3, player_Alliance_1, "neutral")`,
		`    setAllianceState(player_Alliance_1, player_Stormwind_3, "enemy")`,
		`    setAllianceState(neutralHostile, player_Alliance_1, "enemy")`,
		`    setAllianceState(player_Alliance_1, neutralHostile, "enemy")`,
		`    setAllianceState(neutralPassive, player_Alliance_1, "neutral")`,
		`    setAllianceState(player_Alliance_1, neutralPassive, "neutral")`,
		`    setAllianceState(player_Stormwind_3, player_Stormwind_2, "neutral")`,
		`    setAllianceState(player_Stormwind_2, player_Stormwind_3, "allied")`,
	}
	if len(lines) != 18 {
		t.Fatalf("unexpected directed call count: %d", len(lines))
	}
	for i, line := range want {
		if lines[i] != line {
			t.Fatalf("call %d: %s want %s", i, lines[i], line)
		}
	}
	for i := 0; i < len(lines); i += 2 {
		a := strings.Split(strings.TrimPrefix(lines[i], "    setAllianceState("), ", ")
		b := strings.Split(strings.TrimPrefix(lines[i+1], "    setAllianceState("), ", ")
		if a[0] != b[1] || a[1] != b[0] || !strings.Contains(text, lines[i]+"\n"+lines[i+1]) {
			t.Fatalf("opposite directions are not consecutive: %s / %s", lines[i], lines[i+1])
		}
	}
	cfg := settings.DefaultMapExportConfig()
	plan, err := MakeFactionPlan("", nil, nil)
	if err != nil || cfg.PlayerFaction != "Alliance" || plan.UserSlot != 1 || plan.Players[0].Name != "Player (Alliance)" {
		t.Fatalf("inconsistent default: %+v %v", plan, err)
	}
}

func TestFriendlyNPCSharesControlOnlyWithUser(t *testing.T) {
	for _, userSlot := range []int{0, 1} {
		p := &FactionPlan{
			UserSlot:  userSlot,
			Players:   []FactionPlayer{{Slot: 0, FactionName: "First"}, {Slot: 1, FactionName: "Second"}},
			Reactions: [][]FactionReaction{{FactionFriendly, FactionFriendly}, {FactionFriendly, FactionFriendly}},
		}
		text := string(InjectFactionAlliances([]byte("function Trig_LUA_map_init_Actions()\nend\n"), p))
		user, npc := "player_First_0", "player_Second_1"
		if userSlot == 1 {
			user, npc = npc, user
		}
		grant := fmt.Sprintf("setAllianceState(%s, %s, \"allied share unit\")", user, npc)
		reverse := fmt.Sprintf("setAllianceState(%s, %s, \"allied\")", npc, user)
		if !strings.Contains(text, grant) || !strings.Contains(text, reverse) || strings.Count(text, `, "allied share unit")`) != 1 {
			t.Fatal("friendly NPC must grant unit control to the user only")
		}
	}
}

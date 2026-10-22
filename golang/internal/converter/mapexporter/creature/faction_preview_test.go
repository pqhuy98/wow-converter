package creature

import (
	"bytes"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wc3/translators"
)

func TestFactionSummaryUsesNPCReactionAndNeutralOwnership(t *testing.T) {
	p := &FactionPlan{
		UserSlot: 1,
		Players:  []FactionPlayer{{Slot: 2, Name: "Ally", Count: 4}, {Slot: 1}, {Slot: 3, Name: "Enemy", Count: 7}, {Slot: 4, Name: "Neutral", Count: 2}},
		Reactions: [][]FactionReaction{
			{FactionFriendly, FactionFriendly, factionNeutral, factionNeutral},
			{factionHostile, FactionFriendly, FactionFriendly, factionNeutral},
			{factionNeutral, factionHostile, FactionFriendly, factionNeutral},
			{factionNeutral, factionNeutral, factionNeutral, FactionFriendly},
		},
		Assignments: []factionAssignment{{Slot: 24, Count: 5}, {Slot: 24, Count: 3}, {Slot: 27, Count: 6}, {Slot: 2, Count: 4}},
	}
	s := summarizeFactions(p)
	if len(s.Allies) != 1 || s.Allies[0].Name != "Ally" || s.Allies[0].Count != 4 || len(s.Enemies) != 1 || s.Enemies[0].Name != "Enemy" || len(s.Neutral) != 1 || s.Neutral[0].Count != 2 || s.NeutralHostile != 8 || s.NeutralPassive != 6 {
		t.Fatalf("incorrect summary: %+v", s)
	}
	if _, err := PreviewFactionSummary("invalid", nil); err == nil {
		t.Fatal("invalid user faction accepted")
	}
}

func TestFactionSummaryGroupsSameNamesWithinEachReaction(t *testing.T) {
	p := &FactionPlan{
		UserSlot: 1,
		Players: []FactionPlayer{
			{Slot: 1}, {Slot: 2, Name: "Stormwind", Count: 4}, {Slot: 3, Name: "Stormwind", Count: 7},
			{Slot: 4, Name: "Stormwind", Count: 2}, {Slot: 5, Name: "Stormwind", Count: 3},
			{Slot: 6, Name: "Stormwind", Count: 5}, {Slot: 7, Name: "Stormwind", Count: 6},
		},
		Reactions: [][]FactionReaction{{FactionFriendly}, {FactionFriendly}, {FactionFriendly}, {factionHostile}, {factionHostile}, {factionNeutral}, {factionNeutral}},
	}
	s := summarizeFactions(p)
	if len(s.Allies) != 1 || s.Allies[0].Name != "Stormwind" || s.Allies[0].Count != 11 || len(s.Enemies) != 1 || s.Enemies[0].Count != 5 || len(s.Neutral) != 1 || s.Neutral[0].Count != 11 {
		t.Fatalf("same-name grouping lost reaction or count: %+v", s)
	}
	if p.Players[1].Count != 4 || p.Players[2].Count != 7 {
		t.Fatal("summary changed the export plan")
	}
}

func TestFactionPlanPlayerNamesHaveNoIDDiagnosticSuffix(t *testing.T) {
	c := factionFixture()
	a := c.templates[10]
	a.Enemies = 16
	c.templates[10] = a
	p, err := MakeFactionPlan("Horde", map[int]int{10: 4, 11: 5, 20: 2}, c)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, player := range p.Players {
		if player.FactionName == "Same" {
			count++
			if player.Name != "Same" {
				t.Fatalf("diagnostic suffix in player name %q", player.Name)
			}
		}
	}
	if count != 2 || p.owners[10] == p.owners[11] {
		t.Fatal("display names changed distinct NPC alliance ownership")
	}
	mm := extra.NewMapManager()
	mm.EnsureMapInfo("names")
	ConfigureFactionSlots(mm, p)
	info := translators.InfoTranslator{}.WarToJSON(translators.InfoTranslator{}.JSONToWar(mm.Info).Buffer).JSON
	for _, player := range info.Players[1:] {
		if bytes.ContainsAny([]byte(player.Name), "[]") {
			t.Fatalf("diagnostic suffix in serialized map player %q", player.Name)
		}
	}
	script, err := ConfigureFactionScript([]byte("function InitCustomPlayerSlots()\nend\nfunction InitCustomTeams()\nend\nfunction config()\nend\nfunction WowConverterInitFactions()\nend\n"), mm, p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.ContainsAny(script, "[]") {
		t.Fatal("diagnostic suffix in generated Lua player names")
	}
}

package creature

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func ConfigureFactionSlots(mm *extra.MapManager, p *FactionPlan) {
	previous := map[int]data.Player{}
	for _, v := range mm.Info.Players {
		previous[v.PlayerNum] = v
	}
	for _, v := range mm.Players {
		previous[v.PlayerNum] = v
	}
	mm.Info.Players = nil
	mm.Info.Forces = nil
	for _, g := range p.Players {
		player := previous[g.Slot]
		player.PlayerNum, player.Name, player.Type, player.Race = g.Slot, g.Name, 2, 1
		if g.Slot == p.UserSlot {
			player.Type = 1
			if p.PlayerFaction == "Horde" {
				player.Race = 2
			}
		}
		player.StartingPos.Fixed = true
		player.AllyLowPriorities, player.AllyHighPriorities, player.EnemyLowPriorities, player.EnemyHighPriorities = 0, 0, 0, 0
		mm.Info.Players = append(mm.Info.Players, player)
		mm.Info.Forces = append(mm.Info.Forces, data.Force{Name: g.Name, Players: 1 << g.Slot})
	}
	mm.Players = append([]data.Player(nil), mm.Info.Players...)
	mm.Info.Map.Flags.UseCustomForces = true
	mm.Info.Map.Flags.IsMeleeMap = false
	mm.Info.Map.Flags.FixedPlayerSetting = true
}

// The custom trigger copy survives World Editor regenerating config()/main().
// No dynamic alliances belong only in the generated map script.
func InjectFactionAlliances(script []byte, p *FactionPlan) []byte {
	anchor := regexp.MustCompile(`(?m)^function Trig_LUA_map_init_Actions\(\)\r?\n`)
	if !anchor.Match(script) {
		return script
	}
	block := regexp.MustCompile(`(?ms)^-- WOW_CONVERTER_FACTIONS_BEGIN\r?\n.*?^-- WOW_CONVERTER_FACTIONS_END\r?\n`)
	script = block.ReplaceAll(script, nil)
	script = regexp.MustCompile(`(?m)^\s*WowConverterInitFactions\(\)\r?\n`).ReplaceAll(script, nil)
	var b strings.Builder
	b.WriteString(`-- WOW_CONVERTER_FACTIONS_BEGIN
local function setAllianceState(p1, p2, state)
    if state == "enemy" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_UNALLIED)
    elseif state == "enemy vision" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_UNALLIED_VISION)
    elseif state == "allied" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED)
    elseif state == "allied vision" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED_VISION)
    elseif state == "allied share unit" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED_UNITS)
    elseif state == "allied share advanced" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_ALLIED_ADVUNITS)
    elseif state == "neutral" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_NEUTRAL)
    elseif state == "neutral vision" then
        SetPlayerAllianceStateBJ(p2, p1, bj_ALLIANCE_NEUTRAL_VISION)
    end
end

function WowConverterInitFactions()
`)
	variables := make([]string, len(p.Players))
	identifier := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	for i, a := range p.Players {
		variables[i] = fmt.Sprintf("player_%s_%d", strings.Trim(identifier.ReplaceAllString(a.FactionName, "_"), "_"), a.Slot)
		fmt.Fprintf(&b, "    local %s = Player(%d)\n", variables[i], a.Slot)
	}
	b.WriteString("    local neutralHostile = Player(24)\n    local neutralPassive = Player(27)\n\n")
	for i := range p.Players {
		for j := i + 1; j < len(p.Players); j++ {
			writeFactionAlliance(&b, variables[i], variables[j], p.Reactions[i][j], p.Players[j].Slot == p.UserSlot)
			writeFactionAlliance(&b, variables[j], variables[i], p.Reactions[j][i], p.Players[i].Slot == p.UserSlot)
		}
		// Generic neutrals retain stock relationships with each participant.
		for _, n := range []struct {
			variable string
			reaction FactionReaction
		}{{"neutralHostile", factionHostile}, {"neutralPassive", factionNeutral}} {
			writeFactionAlliance(&b, variables[i], n.variable, n.reaction, false)
			writeFactionAlliance(&b, n.variable, variables[i], n.reaction, false)
		}
	}
	b.WriteString("end\n-- WOW_CONVERTER_FACTIONS_END\n")
	script = anchor.ReplaceAllLiteral(script, []byte(b.String()+"function Trig_LUA_map_init_Actions()\n    WowConverterInitFactions()\n"))
	return script
}

func writeFactionAlliance(b *strings.Builder, source, target string, r FactionReaction, shareControl bool) {
	state := "neutral"
	if r == factionHostile {
		state = "enemy"
	} else if r == FactionFriendly {
		state = "allied"
		if shareControl {
			state = "allied share unit"
		}
	}
	// The common library helper takes the target first, then the source.
	fmt.Fprintf(b, "    setAllianceState(%s, %s, %q)\n", target, source, state)
}

func ConfigureFactionScript(script []byte, mm *extra.MapManager, p *FactionPlan) ([]byte, error) {
	var slots, teams, config strings.Builder
	slots.WriteString("function InitCustomPlayerSlots()\n")
	// Keep red Orc and blue Human reserved, with the unselected slot closed.
	for slot := 0; slot < 24; slot++ {
		fmt.Fprintf(&slots, "    SetPlayerController(Player(%d), MAP_CONTROL_NONE)\n", slot)
	}
	for _, s := range []struct {
		slot int
		race string
	}{{0, "ORC"}, {1, "HUMAN"}} {
		fmt.Fprintf(&slots, "    SetPlayerColor(Player(%d), ConvertPlayerColor(%d))\n    SetPlayerRacePreference(Player(%d), RACE_PREF_%s)\n    SetPlayerRaceSelectable(Player(%d), false)\n", s.slot, s.slot, s.slot, s.race, s.slot)
	}
	teams.WriteString("function InitCustomTeams()\n")
	config.WriteString("function config()\n")
	// Preserve the template's map-name/description expressions (WTS references).
	for _, pattern := range []string{`(?m)^SetMapName\([^\r\n]*\)`, `(?m)^SetMapDescription\([^\r\n]*\)`} {
		if call := regexp.MustCompile(pattern).Find(script); call != nil {
			config.Write(call)
			config.WriteByte('\n')
		}
	}
	fmt.Fprintf(&config, "SetPlayers(%d)\nSetTeams(%d)\nSetGamePlacement(MAP_PLACEMENT_USE_MAP_SETTINGS)\n", len(p.Players), len(p.Players))
	for index, g := range p.Players {
		player := mm.Info.Players[index]
		control := "COMPUTER"
		if g.Slot == p.UserSlot {
			control = "USER"
		}
		fmt.Fprintf(&slots, "    SetPlayerStartLocation(Player(%d), %d)\n    ForcePlayerStartLocation(Player(%d), %d)\n    SetPlayerName(Player(%d), %q)\n    SetPlayerColor(Player(%d), ConvertPlayerColor(%d))\n    SetPlayerRaceSelectable(Player(%d), false)\n    SetPlayerController(Player(%d), MAP_CONTROL_%s)\n    SetPlayerSlotAvailable(Player(%d), MAP_CONTROL_%s)\n", g.Slot, index, g.Slot, index, g.Slot, g.Name, g.Slot, g.Slot, g.Slot, g.Slot, control, g.Slot, control)
		race := "HUMAN"
		if player.Race == 2 {
			race = "ORC"
		}
		fmt.Fprintf(&slots, "    SetPlayerRacePreference(Player(%d), RACE_PREF_%s)\n", g.Slot, race)
		fmt.Fprintf(&teams, "    SetPlayerTeam(Player(%d), %d)\n", g.Slot, index)
		fmt.Fprintf(&config, "DefineStartLocation(%d, %.9g, %.9g)\n", index, player.StartingPos.X, player.StartingPos.Y)
	}
	slots.WriteString("end\n")
	teams.WriteString("end\n")
	config.WriteString("InitCustomPlayerSlots()\nInitCustomTeams()\nend\n")
	for _, r := range []struct {
		name string
		body string
	}{{"InitCustomPlayerSlots", slots.String()}, {"InitCustomTeams", teams.String()}, {"config", config.String()}} {
		re := regexp.MustCompile(`(?ms)^function ` + r.name + `\(\)\r?\n.*?^end\b\r?\n?`)
		if !re.Match(script) {
			return nil, fmt.Errorf("template has no %s function", r.name)
		}
		script = re.ReplaceAllLiteral(script, []byte(r.body))
	}
	// Map init must run the alliance hook even when the output is subsequently
	// saved by WE; injectFactionAlliances installs it in both Lua and WCT.
	if !bytes.Contains(script, []byte("WowConverterInitFactions()")) {
		return nil, fmt.Errorf("template has no faction initialization hook")
	}
	return script, nil
}

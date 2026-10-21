package mapexporter

import (
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

// World Editor needs a stock sloc placement, not just W3I player coordinates.
func ensurePlayerStartLocation(mm *extra.MapManager) {
	position := [3]float32{(mm.Info.Camera.Bounds[0] + mm.Info.Camera.Bounds[2]) / 2, (mm.Info.Camera.Bounds[1] + mm.Info.Camera.Bounds[3]) / 2, 0}
	found := false
	for _, u := range mm.Units {
		if u.Player == 0 && (u.Unit.Type == "sloc" || u.Type == "sloc") {
			position, found = u.Position, true
			break
		}
	}
	if !found {
		terrain := mm.GetTerrain()
		if len(terrain.GroundHeight) > 0 && len(terrain.GroundHeight[0]) > 0 {
			position[2] = float32(config.DataHeightToGameZ(GetTerrainHeight(terrain, .5, .5)))
		}
		mm.Units = append(mm.Units, extra.UnitInstance{Type: "sloc", Unit: data.Unit{
			Type: "sloc", Skin: "sloc", Player: 0, Position: position, Scale: [3]float32{1, 1, 1},
			Flags: 2, Hitpoints: 100, Mana: -1, RandomItemSetPtr: -1, TargetAcquisition: -1, Color: -1, Waygate: -1, ID: -1,
		}})
	}
	players := mm.Info.Players
	if mm.Players != nil {
		players = mm.Players
	}
	for i := range players {
		if players[i].PlayerNum == 0 {
			players[i].StartingPos = data.PlayerStartingPosition{X: position[0], Y: position[1], Fixed: true}
		}
	}
	mm.Info.Players = players
	if mm.Players != nil {
		mm.Players = players
	}
}

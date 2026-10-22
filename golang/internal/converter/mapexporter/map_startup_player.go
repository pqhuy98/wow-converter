package mapexporter

import (
	"github.com/pqhuy98/wow-converter/internal/config"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func ensureFactionStartLocations(mm *extra.MapManager, userPlayer int) {
	ensurePlayerStartLocation(mm, userPlayer)
	center := [3]float32{(mm.Info.Camera.Bounds[0] + mm.Info.Camera.Bounds[2]) / 2, (mm.Info.Camera.Bounds[1] + mm.Info.Camera.Bounds[3]) / 2, 0}
	// Reserve the center for the user; nearby grid cells keep editor markers
	// individually selectable, even with all 24 player slots in use.
	side := 1
	for side*side < len(mm.Info.Players) {
		side += 2
	}
	cell := 0
	for _, player := range mm.Info.Players {
		if player.PlayerNum == userPlayer {
			continue
		}
		if cell == side*side/2 {
			cell++
		}
		position := center
		position[0] += float32(cell%side-side/2) * 200
		position[1] += float32(cell/side-side/2) * 200
		ensurePlayerStartLocationAt(mm, player.PlayerNum, position)
		cell++
	}
}

// World Editor needs a stock sloc placement, not just W3I player coordinates.
func ensurePlayerStartLocation(mm *extra.MapManager, userPlayer int) {
	position := [3]float32{(mm.Info.Camera.Bounds[0] + mm.Info.Camera.Bounds[2]) / 2, (mm.Info.Camera.Bounds[1] + mm.Info.Camera.Bounds[3]) / 2, 0}
	ensurePlayerStartLocationAt(mm, userPlayer, position)
}

func ensurePlayerStartLocationAt(mm *extra.MapManager, userPlayer int, position [3]float32) {
	found := false
	for _, u := range mm.Units {
		if u.Player == userPlayer && (u.Unit.Type == "sloc" || u.Type == "sloc") {
			position, found = u.Position, true
			break
		}
	}
	if !found {
		terrain := mm.GetTerrain()
		if len(terrain.GroundHeight) > 0 && len(terrain.GroundHeight[0]) > 0 {
			percentX := float64(position[0]-terrain.Map.Offset.X) / float64(terrain.Map.Width*128)
			percentY := float64(position[1]-terrain.Map.Offset.Y) / float64(terrain.Map.Height*128)
			position[2] = float32(config.DataHeightToGameZ(mapterrain.GetTerrainHeight(terrain, percentX, percentY)))
		}
		mm.Units = append(mm.Units, extra.UnitInstance{Type: "sloc", Unit: data.Unit{
			Type: "sloc", Skin: "sloc", Player: userPlayer, Position: position, Scale: [3]float32{1, 1, 1},
			Flags: 2, Hitpoints: 100, Mana: -1, RandomItemSetPtr: -1, TargetAcquisition: -1, Color: -1, Waygate: -1, ID: -1,
		}})
	}
	players := mm.Info.Players
	if mm.Players != nil {
		players = mm.Players
	}
	for i := range players {
		if players[i].PlayerNum == userPlayer {
			players[i].StartingPos = data.PlayerStartingPosition{X: position[0], Y: position[1], Fixed: true}
		}
	}
	mm.Info.Players = players
	if mm.Players != nil {
		mm.Players = players
	}
}

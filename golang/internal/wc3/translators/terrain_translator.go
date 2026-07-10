package translators

import (
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

// TerrainTranslator handles war3map.w3e.
type TerrainTranslator struct{}

// JSONToWar serializes Terrain to binary.
func (TerrainTranslator) JSONToWar(terrain data.Terrain) wc3.WarResult {
	out := wc3.NewHexBufferWriter()
	out.AddChars("W3E!")
	out.AddInt(12)
	out.AddChar(terrain.Tileset)
	if terrain.CustomTileset {
		out.AddInt(1)
	} else {
		out.AddInt(0)
	}
	out.AddInt(len(terrain.TilePalette))
	for _, tile := range terrain.TilePalette {
		out.AddChars(tile)
	}
	out.AddInt(len(terrain.CliffTilePalette))
	for _, tile := range terrain.CliffTilePalette {
		out.AddChars(tile)
	}
	out.AddInt(terrain.Map.Width + 1)
	out.AddInt(terrain.Map.Height + 1)
	out.AddFloat(terrain.Map.Offset.X)
	out.AddFloat(terrain.Map.Offset.Y)

	for i := 0; i < len(terrain.GroundHeight); i++ {
		for j := 0; j < len(terrain.GroundHeight[i]); j++ {
			hasBoundary := int16(0)
			if terrain.BoundaryFlag[i][j] {
				hasBoundary = 0x4000
			}
			out.AddShort(int16(terrain.GroundHeight[i][j]))
			out.AddShort(int16(terrain.WaterHeight[i][j]) | hasBoundary)
			out.AddShort(int16(terrain.Flags[i][j]) | int16(terrain.GroundTexture[i][j]))
			out.AddByte(byte(terrain.GroundVariation[i][j] | terrain.CliffVariation[i][j]))
			out.AddByte(byte(terrain.CliffTexture[i][j] | terrain.LayerHeight[i][j]))
		}
	}
	return wc3.WarResult{Buffer: out.GetBuffer()}
}

// WarToJSON parses war3map.w3e bytes into Terrain.
func (TerrainTranslator) WarToJSON(buffer []byte) wc3.JsonResult[data.Terrain] {
	result := data.Terrain{
		Map:              data.MapSize{Width: 1, Height: 1},
		TilePalette:      []string{},
		CliffTilePalette: []string{},
	}
	buf := wc3.NewW3Buffer(buffer)

	_ = buf.ReadChars(4)
	version := buf.ReadInt()
	result.Tileset = buf.ReadChars(1)
	result.CustomTileset = buf.ReadInt() == 1

	numTilePalettes := int(buf.ReadInt())
	for i := 0; i < numTilePalettes; i++ {
		result.TilePalette = append(result.TilePalette, buf.ReadChars(4))
	}
	numCliffTilePalettes := int(buf.ReadInt())
	for i := 0; i < numCliffTilePalettes; i++ {
		result.CliffTilePalette = append(result.CliffTilePalette, buf.ReadChars(4))
	}

	width := int(buf.ReadInt()) - 1
	height := int(buf.ReadInt()) - 1
	result.Map.Width = width
	result.Map.Height = height
	result.Map.Offset.X = buf.ReadFloat()
	result.Map.Offset.Y = buf.ReadFloat()

	rowWidth := width + 1
	var (
		arrGroundHeight, arrWaterHeight, arrFlags, arrGroundTexture []int
		arrGroundVariation, arrCliffVariation, arrCliffTexture, arrLayerHeight []int
		arrBoundaryFlag []bool
	)
	for !buf.IsExhausted() {
		groundHeight := int(buf.ReadShort())
		waterHeightAndBoundary := buf.ReadShort()
		var flagsAndGroundTexture int32
		if version >= 12 {
			flagsAndGroundTexture = int32(buf.ReadShort())
		} else {
			flagsAndGroundTexture = int32(buf.ReadByte())
		}
		groundAndCliffVariation := buf.ReadByte()
		cliffTextureAndLayerHeight := buf.ReadByte()

		waterHeight := int(waterHeightAndBoundary & 32767)
		boundaryFlag := (waterHeightAndBoundary & 0x4000) == 0x4000
		var flagsMask, textureMask int32
		if version >= 12 {
			flagsMask = 0xffc0
			textureMask = 0x3f
		} else {
			flagsMask = 240
			textureMask = 15
		}

		arrGroundHeight = append(arrGroundHeight, groundHeight)
		arrWaterHeight = append(arrWaterHeight, waterHeight)
		arrBoundaryFlag = append(arrBoundaryFlag, boundaryFlag)
		arrFlags = append(arrFlags, int(flagsAndGroundTexture&flagsMask))
		arrGroundTexture = append(arrGroundTexture, int(flagsAndGroundTexture&textureMask))
		arrGroundVariation = append(arrGroundVariation, int(groundAndCliffVariation&248))
		arrCliffVariation = append(arrCliffVariation, int(groundAndCliffVariation&7))
		arrCliffTexture = append(arrCliffTexture, int(cliffTextureAndLayerHeight&240))
		arrLayerHeight = append(arrLayerHeight, int(cliffTextureAndLayerHeight&15))
	}

	result.GroundHeight = splitLargeArrayIntoWidthArrays(arrGroundHeight, rowWidth)
	result.WaterHeight = splitLargeArrayIntoWidthArrays(arrWaterHeight, rowWidth)
	result.BoundaryFlag = splitLargeArrayIntoWidthArraysBool(arrBoundaryFlag, rowWidth)
	result.Flags = splitLargeArrayIntoWidthArraysUint16(arrFlags, rowWidth)
	result.GroundTexture = splitLargeArrayIntoWidthArrays(arrGroundTexture, rowWidth)
	result.GroundVariation = splitLargeArrayIntoWidthArrays(arrGroundVariation, rowWidth)
	result.CliffVariation = splitLargeArrayIntoWidthArrays(arrCliffVariation, rowWidth)
	result.CliffTexture = splitLargeArrayIntoWidthArrays(arrCliffTexture, rowWidth)
	result.LayerHeight = splitLargeArrayIntoWidthArrays(arrLayerHeight, rowWidth)

	return wc3.JsonResult[data.Terrain]{JSON: result}
}

func splitLargeArrayIntoWidthArrays(arr []int, width int) [][]int {
	if width <= 0 {
		return nil
	}
	rows := make([][]int, 0, len(arr)/width)
	for i := 0; i < len(arr)/width; i++ {
		rows = append(rows, arr[i*width:(i+1)*width])
	}
	return rows
}

func splitLargeArrayIntoWidthArraysBool(arr []bool, width int) [][]bool {
	if width <= 0 {
		return nil
	}
	rows := make([][]bool, 0, len(arr)/width)
	for i := 0; i < len(arr)/width; i++ {
		rows = append(rows, arr[i*width:(i+1)*width])
	}
	return rows
}

func splitLargeArrayIntoWidthArraysUint16(arr []int, width int) [][]uint16 {
	if width <= 0 {
		return nil
	}
	rows := make([][]uint16, 0, len(arr)/width)
	for i := 0; i < len(arr)/width; i++ {
		row := make([]uint16, width)
		for j := 0; j < width; j++ {
			row[j] = uint16(arr[i*width+j])
		}
		rows = append(rows, row)
	}
	return rows
}

// TerrainJSONToWar is a package-level alias for TerrainTranslator.JSONToWar.
func TerrainJSONToWar(terrain data.Terrain) wc3.WarResult {
	return TerrainTranslator{}.JSONToWar(terrain)
}

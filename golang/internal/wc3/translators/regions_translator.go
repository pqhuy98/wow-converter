package translators

import (
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

var regionsTranslatorInstance = &RegionsTranslator{}

// RegionsTranslator handles war3map.w3r.
type RegionsTranslator struct{}

// GetRegionsTranslator returns the singleton RegionsTranslator.
func GetRegionsTranslator() *RegionsTranslator {
	return regionsTranslatorInstance
}

// JSONToWar serializes regions to binary.
func (RegionsTranslator) JSONToWar(regions []data.Region) wc3.WarResult {
	return regionsTranslatorInstance.jsonToWar(regions)
}

// WarToJSON parses war3map.w3r bytes.
func (RegionsTranslator) WarToJSON(buffer []byte) wc3.JsonResult[[]data.Region] {
	return regionsTranslatorInstance.warToJSON(buffer)
}

func (*RegionsTranslator) jsonToWar(regions []data.Region) wc3.WarResult {
	out := wc3.NewHexBufferWriter()
	out.AddInt(5)
	out.AddInt(len(regions))

	for _, region := range regions {
		out.AddFloat(region.Position.Left)
		out.AddFloat(region.Position.Bottom)
		out.AddFloat(region.Position.Right)
		out.AddFloat(region.Position.Top)
		out.AddString(region.Name)
		out.AddInt(int(region.ID))

		if region.WeatherEffect != "" {
			out.AddChars(region.WeatherEffect)
		} else {
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(0)
		}

		ambientSound := region.AmbientSound
		out.AddString(ambientSound)

		out.AddByte(region.Color[2])
		out.AddByte(region.Color[1])
		out.AddByte(region.Color[0])
		out.AddByte(0xff)
	}

	return wc3.WarResult{Buffer: out.GetBuffer()}
}

func (*RegionsTranslator) warToJSON(buffer []byte) wc3.JsonResult[[]data.Region] {
	result := []data.Region{}
	buf := wc3.NewW3Buffer(buffer)

	buf.ReadInt()
	numRegions := int(buf.ReadInt())
	for i := 0; i < numRegions; i++ {
		region := data.Region{}
		region.Position.Left = buf.ReadFloat()
		region.Position.Bottom = buf.ReadFloat()
		region.Position.Right = buf.ReadFloat()
		region.Position.Top = buf.ReadFloat()
		region.Name = buf.ReadString()
		region.ID = buf.ReadInt()
		region.WeatherEffect = buf.ReadChars(4)
		region.AmbientSound = buf.ReadString()
		region.Color = [3]byte{
			buf.ReadByte(),
			buf.ReadByte(),
			buf.ReadByte(),
		}
		region.Color = [3]byte{region.Color[2], region.Color[1], region.Color[0]}
		buf.ReadByte()
		result = append(result, region)
	}

	return wc3.JsonResult[[]data.Region]{JSON: result}
}

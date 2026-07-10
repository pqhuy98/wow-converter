package translators

import (
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

var camerasTranslatorInstance = &CamerasTranslator{}

// CamerasTranslator handles war3map.w3c.
type CamerasTranslator struct{}

// GetCamerasTranslator returns the singleton CamerasTranslator.
func GetCamerasTranslator() *CamerasTranslator {
	return camerasTranslatorInstance
}

// JSONToWar serializes cameras to binary.
func (CamerasTranslator) JSONToWar(cameras []data.Camera) wc3.WarResult {
	return camerasTranslatorInstance.jsonToWar(cameras)
}

// WarToJSON parses war3map.w3c bytes.
func (CamerasTranslator) WarToJSON(buffer []byte) wc3.JsonResult[[]data.Camera] {
	return camerasTranslatorInstance.warToJSON(buffer)
}

func (*CamerasTranslator) jsonToWar(cameras []data.Camera) wc3.WarResult {
	out := wc3.NewHexBufferWriter()
	out.AddInt(0)
	out.AddInt(len(cameras))

	for _, camera := range cameras {
		out.AddFloat(camera.Target.X)
		out.AddFloat(camera.Target.Y)
		out.AddFloat(camera.OffsetZ)
		out.AddFloat(float32(camera.Rotation))
		out.AddFloat(float32(camera.Aoa))
		out.AddFloat(camera.Distance)
		out.AddFloat(camera.Roll)
		out.AddFloat(float32(camera.Fov))
		out.AddFloat(camera.FarClipping)
		nearClipping := camera.NearClipping
		if nearClipping == 0 {
			nearClipping = 16
		}
		out.AddFloat(nearClipping)
		out.AddFloat(camera.LocalPitch)
		out.AddFloat(camera.LocalYaw)
		out.AddFloat(camera.LocalRoll)
		out.AddString(camera.Name)
	}

	return wc3.WarResult{Buffer: out.GetBuffer()}
}

func (*CamerasTranslator) warToJSON(buffer []byte) wc3.JsonResult[[]data.Camera] {
	result := []data.Camera{}
	buf := wc3.NewW3Buffer(buffer)

	buf.ReadInt()
	numCameras := int(buf.ReadInt())
	for i := 0; i < numCameras; i++ {
		camera := data.Camera{NearClipping: 16}
		camera.Target.X = buf.ReadFloat()
		camera.Target.Y = buf.ReadFloat()
		camera.OffsetZ = buf.ReadFloat()
		camera.Rotation = wc3.Angle(buf.ReadFloat())
		camera.Aoa = wc3.Angle(buf.ReadFloat())
		camera.Distance = buf.ReadFloat()
		camera.Roll = buf.ReadFloat()
		camera.Fov = wc3.Angle(buf.ReadFloat())
		camera.FarClipping = buf.ReadFloat()
		camera.NearClipping = buf.ReadFloat()
		camera.LocalPitch = buf.ReadFloat()
		camera.LocalYaw = buf.ReadFloat()
		camera.LocalRoll = buf.ReadFloat()
		camera.Name = buf.ReadString()
		result = append(result, camera)
	}

	return wc3.JsonResult[[]data.Camera]{JSON: result}
}

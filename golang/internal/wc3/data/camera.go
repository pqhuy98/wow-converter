package data

import "github.com/pqhuy98/wow-converter/internal/wc3"

// CameraTarget is the XY target of a camera.
type CameraTarget struct {
	X float32
	Y float32
}

// Camera is a war3map.w3c camera definition.
type Camera struct {
	Target       CameraTarget
	OffsetZ      float32
	Rotation     wc3.Angle
	Aoa          wc3.Angle
	Distance     float32
	Roll         float32
	Fov          wc3.Angle
	FarClipping  float32
	NearClipping float32
	LocalPitch   float32
	LocalYaw     float32
	LocalRoll    float32
	Name         string
}

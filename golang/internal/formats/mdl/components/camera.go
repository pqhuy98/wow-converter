package components

import (
	stdmath "math"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type CameraTarget struct {
	Position math.Vector3
}

type Camera struct {
	Name        string
	Position    math.Vector3
	FieldOfView float64
	FarClip     float64
	NearClip    float64
	Target      CameraTarget
	Translation *Animation
	Scaling     *Animation
	Rotation    *Animation
}

func CamerasToString(cameras []Camera) string {
	var blocks []string
	for _, cam := range cameras {
		var b strings.Builder
		b.WriteString("Camera \"")
		b.WriteString(cam.Name)
		b.WriteString("\" {\n")
		b.WriteString("Position { ")
		b.WriteString(FVector3(cam.Position))
		b.WriteString(" },\n")
		b.WriteString("FieldOfView ")
		b.WriteString(formatCameraFloat(cam.FieldOfView))
		b.WriteString(",\n")
		b.WriteString("FarClip ")
		b.WriteString(formatCameraClipFloat(cam.FarClip))
		b.WriteString(",\n")
		b.WriteString("NearClip ")
		b.WriteString(formatCameraClipFloat(cam.NearClip))
		b.WriteString(",\n")
		b.WriteString("Target {\n")
		b.WriteString("Position { ")
		b.WriteString(FVector3(cam.Target.Position))
		b.WriteString(" },\n")
		b.WriteString("}\n")
		b.WriteString(AnimationToString("Translation", cam.Translation))
		b.WriteString(AnimationToString("Rotation", cam.Rotation))
		b.WriteString(AnimationToString("Scaling", cam.Scaling))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func formatCameraFloat(v float64) string {
	return FVal(v)
}

func formatCameraClipFloat(v float64) string {
	return formatCameraFloat(stdmath.Nextafter(v, 0))
}

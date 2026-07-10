package mdl

import (
	"math"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func (mod *Modify) Scale(value float64) *Modify {
	mod.MDL.AccumScale *= value
	for _, geoset := range mod.MDL.Geosets {
		for _, vertex := range geoset.Vertices {
			vertex.Position[0] *= value
			vertex.Position[1] *= value
			vertex.Position[2] *= value
		}
	}
	for _, node := range mod.MDL.GetNodes() {
		if anim := node.NodeTranslation(); anim != nil {
			for ts, translation := range anim.KeyFrames {
				if v, ok := translation.(imath.Vector3); ok {
					anim.KeyFrames[ts] = imath.Vector3{v[0] * value, v[1] * value, v[2] * value}
				}
			}
		}
		if ap, ok := node.(*components.AttachmentPoint); ok {
			if anim := ap.Scaling; anim != nil {
				for ts, scaling := range anim.KeyFrames {
					if v, ok := scaling.(imath.Vector3); ok {
						anim.KeyFrames[ts] = imath.Vector3{v[0] * value, v[1] * value, v[2] * value}
					}
				}
			}
		}
		node.SetNodePivotPoint(imath.V3Scale(node.NodePivotPoint(), value))
	}
	for i := range mod.MDL.Cameras {
		mod.MDL.Cameras[i].Position = imath.V3Scale(mod.MDL.Cameras[i].Position, value)
		mod.MDL.Cameras[i].Target.Position = imath.V3Scale(mod.MDL.Cameras[i].Target.Position, value)
		mod.MDL.Cameras[i].FarClip *= value
		mod.MDL.Cameras[i].NearClip *= value
	}
	for _, shape := range mod.MDL.CollisionShapes {
		for vi := range shape.Vertices {
			shape.Vertices[vi][0] *= value
			shape.Vertices[vi][1] *= value
			shape.Vertices[vi][2] *= value
		}
		shape.BoundRadius *= value
		shape.PivotPoint = imath.V3Scale(shape.PivotPoint, value)
	}
	for i := range mod.MDL.Sequences {
		mod.MDL.Sequences[i].MoveSpeed *= value
	}

	scaleAnimOrStatic := func(a *components.AnimatedOrStatic[float64]) {
		if a == nil {
			return
		}
		if a.Static {
			a.Value *= value
		} else if a.Anim != nil {
			for k, v := range a.Anim.KeyFrames {
				if fv, ok := v.(float64); ok {
					a.Anim.KeyFrames[k] = fv * value
				}
			}
		}
	}

	for _, e := range mod.MDL.ParticleEmitter2s {
		scaleAnimOrStatic(&e.Width)
		scaleAnimOrStatic(&e.Length)
		scaleAnimOrStatic(&e.Speed)
		scaleAnimOrStatic(&e.Gravity)
		for i := range e.SegmentScaling {
			e.SegmentScaling[i] *= value
		}
	}
	for _, l := range mod.MDL.Lights {
		scaleAnimOrStatic(&l.AttenuationStart)
		scaleAnimOrStatic(&l.AttenuationEnd)
	}
	for _, r := range mod.MDL.RibbonEmitters {
		scaleAnimOrStatic(r.HeightAbove)
		scaleAnimOrStatic(r.HeightBelow)
		r.Gravity *= value
	}
	return mod
}

func (mod *Modify) FlipY() *Modify {
	for _, geoset := range mod.MDL.Geosets {
		for _, vertex := range geoset.Vertices {
			vertex.Position[1] *= -1
			vertex.Normal[1] *= -1
		}
		for fi := range geoset.Faces {
			face := &geoset.Faces[fi]
			tmp := face.Vertices[1]
			face.Vertices[1] = face.Vertices[2]
			face.Vertices[2] = tmp
		}
	}
	for _, node := range mod.MDL.GetNodes() {
		if anim := node.NodeTranslation(); anim != nil {
			for ts, translation := range anim.KeyFrames {
				if v, ok := translation.(imath.Vector3); ok {
					v[1] *= -1
					anim.KeyFrames[ts] = v
				}
			}
		}
		if anim := node.NodeRotation(); anim != nil {
			for ts, rotation := range anim.KeyFrames {
				if v, ok := rotation.(imath.QuaternionRotation); ok {
					v[0] *= -1
					v[2] *= -1
					anim.KeyFrames[ts] = v
				}
			}
		}
		p := node.NodePivotPoint()
		p[1] *= -1
		node.SetNodePivotPoint(p)
	}
	for _, shape := range mod.MDL.CollisionShapes {
		for vi := range shape.Vertices {
			shape.Vertices[vi][1] *= -1
		}
		shape.PivotPoint[1] *= -1
	}
	for i := range mod.MDL.Cameras {
		mod.MDL.Cameras[i].Position[1] *= -1
		mod.MDL.Cameras[i].Target.Position[1] *= -1
	}
	mod.MDL.Sync()
	return mod
}

func (mod *Modify) Translate(delta imath.Vector3) *Modify {
	for _, geoset := range mod.MDL.Geosets {
		for _, vertex := range geoset.Vertices {
			vertex.Position = imath.V3Sum(vertex.Position, delta)
		}
	}
	for _, node := range mod.MDL.GetNodes() {
		node.SetNodePivotPoint(imath.V3Sum(node.NodePivotPoint(), delta))
	}
	for i := range mod.MDL.Cameras {
		mod.MDL.Cameras[i].Position = imath.V3Sum(mod.MDL.Cameras[i].Position, delta)
		mod.MDL.Cameras[i].Target.Position = imath.V3Sum(mod.MDL.Cameras[i].Target.Position, delta)
	}
	mod.MDL.Sync()
	return mod
}

func (mod *Modify) ScaleSequenceDuration(sequence *components.Sequence, scalingFactor float64) *Modify {
	floorDuration := func(value float64) int {
		return int(math.Floor(value))
	}
	floorKey := func(value float64) int {
		// Match TS parity for scaled in-sequence timestamps that should land exactly
		// on an integer boundary but can arrive infinitesimally below it in Go.
		return int(math.Floor(value + 1e-9))
	}

	durationOffset := floorDuration(float64(sequence.Interval[1]-sequence.Interval[0]) * (scalingFactor - 1))

	updateKeyFrame := func(keyFrame map[int]any) map[int]any {
		newKeyFrame := map[int]any{}
		for _, timestamp := range components.SortedKeyInts(keyFrame) {
			value := keyFrame[timestamp]
			if timestamp <= sequence.Interval[0] {
				newKeyFrame[timestamp] = value
				continue
			}
			newTimestamp := timestamp
			if timestamp <= sequence.Interval[1] {
				newTimestamp = floorKey(float64(timestamp-sequence.Interval[0])*scalingFactor) + sequence.Interval[0]
			} else {
				newTimestamp = timestamp + durationOffset
			}
			newKeyFrame[newTimestamp] = value
		}
		return newKeyFrame
	}

	for _, anim := range mod.MDL.GetAnimated() {
		if anim.GlobalSeq != nil {
			continue
		}
		anim.KeyFrames = updateKeyFrame(anim.KeyFrames)
	}
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Interval[0] > sequence.Interval[1] {
			mod.MDL.Sequences[i].Interval[0] += durationOffset
			mod.MDL.Sequences[i].Interval[1] += durationOffset
		}
	}
	sequence.Interval[1] += durationOffset
	return mod
}

func (mod *Modify) Rotate(eulerRotation imath.EulerRotation) *Modify {
	for _, geoset := range mod.MDL.Geosets {
		for _, vertex := range geoset.Vertices {
			vertex.Position = imath.V3Rotate(vertex.Position, eulerRotation)
		}
	}
	for _, node := range mod.MDL.GetNodes() {
		node.SetNodePivotPoint(imath.V3Rotate(node.NodePivotPoint(), eulerRotation))
	}
	for i := range mod.MDL.Cameras {
		mod.MDL.Cameras[i].Position = imath.V3Rotate(mod.MDL.Cameras[i].Position, eulerRotation)
		mod.MDL.Cameras[i].Target.Position = imath.V3Rotate(mod.MDL.Cameras[i].Target.Position, eulerRotation)
	}
	mod.MDL.Sync()
	return mod
}

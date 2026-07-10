package mdl

import (
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type TransformValue struct {
	Position imath.Vector3
	Rotation imath.Vector3
	Scaling  imath.Vector3
}

func BuildChildrenLists(m *MDL) map[components.Node][]components.Node {
	childrenList := map[components.Node][]components.Node{}
	for _, node := range m.GetNodes() {
		if _, ok := childrenList[node]; !ok {
			childrenList[node] = nil
		}
		if parent := node.NodeParent(); parent != nil {
			childrenList[parent] = append(childrenList[parent], node)
		}
	}
	return childrenList
}

func IterateNodesAtTimestamp(m *MDL, sequence components.Sequence, timestamp int, callback func(node components.Node, value TransformValue)) {
	childrenList := BuildChildrenLists(m)

	var dfs func(current components.Node, currentValue TransformValue)
	dfs = func(current components.Node, currentValue TransformValue) {
		callback(current, currentValue)
		for _, child := range childrenList[current] {
			childValue := TransformValue{
				Position: currentValue.Position,
				Rotation: currentValue.Rotation,
				Scaling:  currentValue.Scaling,
			}

			transform := interpolateTransformEuler(child, sequence, timestamp)
			deltaPosition := imath.V3Sum(transform.Position, imath.V3Sub(child.NodePivotPoint(), current.NodePivotPoint()))
			deltaPosition = imath.V3Mul(deltaPosition, currentValue.Scaling)
			deltaPosition = imath.V3Rotate(deltaPosition, currentValue.Rotation)

			childValue.Position = imath.V3Sum(childValue.Position, deltaPosition)
			childValue.Rotation = imath.CalculateChildAbsoluteEulerRotation(currentValue.Rotation, transform.Rotation)
			childValue.Scaling = imath.V3Mul(currentValue.Scaling, transform.Scaling)

			dfs(child, childValue)
		}
	}

	for _, b := range m.Bones {
		if b.ParentBone == nil && b.Parent == nil {
			transform := interpolateTransformEuler(b, sequence, timestamp)
			dfs(b, TransformValue{
				Position: imath.V3Sum(transform.Position, b.PivotPoint),
				Rotation: transform.Rotation,
				Scaling:  transform.Scaling,
			})
		}
	}
}

func IterateNodesFromSubtree(m *MDL, node components.Node, callback func(node components.Node)) {
	childrenList := BuildChildrenLists(m)
	var dfs func(current components.Node)
	dfs = func(current components.Node) {
		callback(current)
		for _, child := range childrenList[current] {
			dfs(child)
		}
	}
	dfs(node)
}

func IterateVerticesAtTimestamp(m *MDL, sequence components.Sequence, timestamp int, callback func(v *components.GeosetVertex, vPos imath.Vector3, geoset *components.Geoset)) {
	nodeValues := map[components.Node]TransformValue{}
	IterateNodesAtTimestamp(m, sequence, timestamp, func(node components.Node, value TransformValue) {
		nodeValues[node] = value
	})

	vertices := map[*components.GeosetVertex]struct{}{}
	geosets := map[*components.GeosetVertex]*components.Geoset{}
	for _, geoset := range m.Geosets {
		var geosetAnim *components.GeosetAnim
		for i := range m.GeosetAnims {
			if m.GeosetAnims[i].Geoset == geoset {
				geosetAnim = &m.GeosetAnims[i]
				break
			}
		}
		alpha := 1.0
		if geosetAnim != nil && geosetAnim.Alpha != nil {
			if geosetAnim.Alpha.Static {
				alpha = geosetAnim.Alpha.Value
			} else if geosetAnim.Alpha.Anim != nil {
				alpha = interpolateKeyFramesFloat(sequence, geosetAnim.Alpha.Anim, timestamp, geosetAnim.Alpha.Anim.Interpolation == components.InterpLinear, 1)
			}
		}
		if alpha < 0.01 {
			continue
		}
		for _, f := range geoset.Faces {
			for _, v := range f.Vertices {
				vertices[v] = struct{}{}
				geosets[v] = geoset
			}
		}
	}

	for v := range vertices {
		translation := imath.Vector3{}
		for _, sw := range v.SkinWeights {
			boneValue := nodeValues[sw.Bone]
			vPosDeltaToBone := imath.V3Mul(imath.V3Rotate(imath.V3Sub(v.Position, sw.Bone.PivotPoint), boneValue.Rotation), boneValue.Scaling)
			vPos := imath.V3Sum(boneValue.Position, vPosDeltaToBone)
			vPosDelta := imath.V3Sub(vPos, v.Position)
			translation = imath.V3Sum(translation, imath.V3Scale(vPosDelta, float64(sw.Weight)/255))
		}
		vPos := imath.V3Sum(v.Position, translation)
		callback(v, vPos, geosets[v])
	}
}

func interpolateTransformEuler(node components.Node, sequence components.Sequence, timestamp int) TransformValue {
	value := InterpolateTransformQuat(node, sequence, timestamp)
	return TransformValue{
		Position: value.Position,
		Rotation: imath.QuaternionToEuler(value.Rotation),
		Scaling:  value.Scaling,
	}
}

type TransformQuatValue struct {
	Position imath.Vector3
	Rotation imath.QuaternionRotation
	Scaling  imath.Vector3
}

func InterpolateTransformQuat(node components.Node, sequence components.Sequence, timestamp int) TransformQuatValue {
	position := imath.Vector3{}
	if node.NodeTranslation() != nil {
		position = interpolateKeyFramesVector3(sequence, node.NodeTranslation(), timestamp, node.NodeTranslation().Interpolation == components.InterpLinear, imath.Vector3{})
	}
	rotation := imath.QuatNoRotation()
	if node.NodeRotation() != nil {
		rotation = interpolateKeyFramesQuat(sequence, node.NodeRotation(), timestamp, node.NodeRotation().Interpolation == components.InterpLinear, imath.QuatNoRotation())
	}
	scaling := imath.Vector3{1, 1, 1}
	if node.NodeScaling() != nil {
		scaling = interpolateKeyFramesVector3(sequence, node.NodeScaling(), timestamp, node.NodeScaling().Interpolation == components.InterpLinear, imath.Vector3{1, 1, 1})
	}
	return TransformQuatValue{Position: position, Rotation: rotation, Scaling: scaling}
}

func InterpolateKeyFramesFloat(sequence components.Sequence, keyFrames map[int]any, timestamp int, linear bool, defaultValue float64) float64 {
	lowTs, highTs := -1, -1
	var low, high float64
	hasLow, hasHigh := false, false
	for ts, value := range keyFrames {
		if ts < sequence.Interval[0] || sequence.Interval[1] < ts {
			continue
		}
		fv, ok := value.(float64)
		if !ok {
			if iv, ok := value.(int); ok {
				fv = float64(iv)
			} else {
				continue
			}
		}
		if ts <= timestamp && (lowTs == -1 || ts > lowTs) {
			lowTs, low, hasLow = ts, fv, true
		}
		if ts >= timestamp && (highTs == -1 || ts < highTs) {
			highTs, high, hasHigh = ts, fv, true
		}
	}
	t := 0.0
	if highTs != lowTs {
		t = float64(timestamp-lowTs) / float64(highTs-lowTs)
	}
	if hasLow && hasHigh {
		if linear {
			return imath.V3LerpScalar(low, high, t)
		}
		return imath.V3NoInterpScalar(low, high, t)
	}
	if hasLow {
		return low
	}
	if hasHigh {
		return high
	}
	return defaultValue
}

func interpolateKeyFramesVector3(sequence components.Sequence, anim *components.Animation, timestamp int, linear bool, defaultValue imath.Vector3) imath.Vector3 {
	lowTs, highTs := -1, -1
	var low, high imath.Vector3
	hasLow, hasHigh := false, false
	for ts, value := range anim.KeyFrames {
		if ts < sequence.Interval[0] || sequence.Interval[1] < ts {
			continue
		}
		v, ok := value.(imath.Vector3)
		if !ok {
			continue
		}
		if ts <= timestamp && (lowTs == -1 || ts > lowTs) {
			lowTs, low, hasLow = ts, v, true
		}
		if ts >= timestamp && (highTs == -1 || ts < highTs) {
			highTs, high, hasHigh = ts, v, true
		}
	}
	t := 0.0
	if highTs != lowTs {
		t = float64(timestamp-lowTs) / float64(highTs-lowTs)
	}
	if hasLow && hasHigh {
		if linear {
			return imath.V3Lerp(low, high, t)
		}
		return low
	}
	if hasLow {
		return low
	}
	if hasHigh {
		return high
	}
	return defaultValue
}

func interpolateKeyFramesQuat(sequence components.Sequence, anim *components.Animation, timestamp int, linear bool, defaultValue imath.QuaternionRotation) imath.QuaternionRotation {
	lowTs, highTs := -1, -1
	var low, high imath.QuaternionRotation
	hasLow, hasHigh := false, false
	for ts, value := range anim.KeyFrames {
		if ts < sequence.Interval[0] || sequence.Interval[1] < ts {
			continue
		}
		v, ok := value.(imath.QuaternionRotation)
		if !ok {
			continue
		}
		if ts <= timestamp && (lowTs == -1 || ts > lowTs) {
			lowTs, low, hasLow = ts, v, true
		}
		if ts >= timestamp && (highTs == -1 || ts < highTs) {
			highTs, high, hasHigh = ts, v, true
		}
	}
	t := 0.0
	if highTs != lowTs {
		t = float64(timestamp-lowTs) / float64(highTs-lowTs)
	}
	if hasLow && hasHigh {
		if linear {
			return imath.V3Slerp(low, high, t)
		}
		return low
	}
	if hasLow {
		return low
	}
	if hasHigh {
		return high
	}
	return defaultValue
}

func interpolateKeyFramesFloat(sequence components.Sequence, anim *components.Animation, timestamp int, linear bool, defaultValue float64) float64 {
	return InterpolateKeyFramesFloat(sequence, anim.KeyFrames, timestamp, linear, defaultValue)
}

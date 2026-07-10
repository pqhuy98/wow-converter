package components

import (
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type Interpolation string

const (
	InterpLinear     Interpolation = "Linear"
	InterpDontInterp Interpolation = "DontInterp"
	InterpHermite    Interpolation = "Hermite"
	InterpBezier     Interpolation = "Bezier"
)

type AnimationType string

const (
	AnimTypeTranslation AnimationType = "translation"
	AnimTypeRotation    AnimationType = "rotation"
	AnimTypeScaling     AnimationType = "scaling"
	AnimTypeAlpha       AnimationType = "alpha"
	AnimTypeColor       AnimationType = "color"
	AnimTypeTVertex     AnimationType = "tvertex"
	AnimTypeTVertexAnim AnimationType = "tvertexAnim"
	AnimTypeOthers      AnimationType = "others"
)

type InOutTan struct {
	InTan  math.Vector3
	OutTan math.Vector3
}

type Animation struct {
	GlobalSeq     *GlobalSequence
	Interpolation Interpolation
	KeyFrames     map[int]any
	InOutTans     map[int]InOutTan
	Type          AnimationType
}

type AnimatedOrStatic[T any] struct {
	Static bool
	Value  T
	Anim   *Animation
}

func AnimationToString(typeName string, animation *Animation) string {
	if animation == nil || len(animation.KeyFrames) == 0 {
		return ""
	}

	timestamps := sortedKeys(animation.KeyFrames)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(typeName)
	b.WriteString(" ")
	b.WriteString(FVal(float64(len(timestamps))))
	b.WriteString(" {\n\n")
	b.WriteString(string(animation.Interpolation))
	b.WriteString(",\n\n")
	if animation.GlobalSeq != nil {
		b.WriteString("GlobalSeqId ")
		b.WriteString(FVal(float64(animation.GlobalSeq.ID)))
		b.WriteString(",\n\n")
	}
	for _, timestamp := range timestamps {
		value := animation.KeyFrames[timestamp]
		b.WriteString(FVal(float64(timestamp)))
		b.WriteString(": ")
		switch v := value.(type) {
		case math.Vector3:
			b.WriteString("{ ")
			b.WriteString(FVector3(v))
			b.WriteString(" }")
		case math.Vector2:
			b.WriteString("{ ")
			b.WriteString(FVector([]float64{v[0], v[1], 0}))
			b.WriteString(" }")
		case math.QuaternionRotation:
			b.WriteString("{ ")
			b.WriteString(FVector(v[:]))
			b.WriteString(" }")
		case float64:
			b.WriteString(FVal(v))
		case int:
			b.WriteString(FVal(float64(v)))
		default:
			if arr, ok := value.([]float64); ok {
				b.WriteString("{ ")
				b.WriteString(FVector(arr))
				b.WriteString(" }")
			} else if arr, ok := value.([]any); ok {
				b.WriteString("{ ")
				b.WriteString(FVector(anySliceToFloat64(arr)))
				b.WriteString(" }")
			} else if value == nil && (typeName == "Translation" || typeName == "Scaling") {
				b.WriteString("{ 0, 0, 0 }")
			} else if value == nil && typeName == "Rotation" {
				b.WriteString("{ 0, 0, 0, 1 }")
			}
		}
		b.WriteString(",\n")
		if animation.InOutTans != nil {
			if tan, ok := animation.InOutTans[timestamp]; ok {
				b.WriteString("InTan { ")
				b.WriteString(FVector3(tan.InTan))
				b.WriteString(" },\n")
				b.WriteString("OutTan { ")
				b.WriteString(FVector3(tan.OutTan))
				b.WriteString(" },\n")
			}
		}
	}
	b.WriteString("\n}\n")
	return b.String()
}

func anySliceToFloat64(values []any) []float64 {
	out := make([]float64, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case float64:
			out = append(out, v)
		case int:
			out = append(out, float64(v))
		case int32:
			out = append(out, float64(v))
		case uint32:
			out = append(out, float64(v))
		}
	}
	return out
}

func AnimatedValueToString(typeName string, animatedValue any) string {
	if animatedValue == nil {
		return ""
	}
	switch v := animatedValue.(type) {
	case AnimatedOrStatic[float64]:
		if v.Static {
			return "static " + typeName + " " + FVal(v.Value) + ","
		}
		return AnimationToString(typeName, v.Anim)
	case *AnimatedOrStatic[float64]:
		if v == nil {
			return ""
		}
		if v.Static {
			return "static " + typeName + " " + FVal(v.Value) + ","
		}
		return AnimationToString(typeName, v.Anim)
	case AnimatedOrStatic[math.Vector3]:
		if v.Static {
			return "static " + typeName + " { " + FVector3(v.Value) + " },"
		}
		return AnimationToString(typeName, v.Anim)
	case *AnimatedOrStatic[math.Vector3]:
		if v == nil {
			return ""
		}
		if v.Static {
			return "static " + typeName + " { " + FVector3(v.Value) + " },"
		}
		return AnimationToString(typeName, v.Anim)
	default:
		if anim, ok := animatedValue.(*Animation); ok {
			return AnimationToString(typeName, anim)
		}
	}
	return ""
}

func StaticTranslation() *Animation {
	return &Animation{
		Interpolation: InterpDontInterp,
		KeyFrames:     map[int]any{0: math.Vector3{}},
		Type:          AnimTypeTranslation,
	}
}

func StaticRotation() *Animation {
	return &Animation{
		Interpolation: InterpDontInterp,
		KeyFrames:     map[int]any{0: math.QuatNoRotation()},
		Type:          AnimTypeRotation,
	}
}

func StaticScaling() *Animation {
	return &Animation{
		Interpolation: InterpDontInterp,
		KeyFrames:     map[int]any{0: math.Vector3{1, 1, 1}},
		Type:          AnimTypeScaling,
	}
}

func sortedKeys(m map[int]any) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func SortedKeyInts(m map[int]any) []int {
	return sortedKeys(m)
}

func HasKeyFrames(v any) bool {
	switch a := v.(type) {
	case *Animation:
		return a != nil && len(a.KeyFrames) > 0
	case AnimatedOrStatic[float64]:
		return !a.Static && a.Anim != nil && len(a.Anim.KeyFrames) > 0
	case *AnimatedOrStatic[float64]:
		return a != nil && !a.Static && a.Anim != nil && len(a.Anim.KeyFrames) > 0
	case AnimatedOrStatic[math.Vector3]:
		return !a.Static && a.Anim != nil && len(a.Anim.KeyFrames) > 0
	case *AnimatedOrStatic[math.Vector3]:
		return a != nil && !a.Static && a.Anim != nil && len(a.Anim.KeyFrames) > 0
	}
	return false
}

func GetAnim(v any) *Animation {
	switch a := v.(type) {
	case *Animation:
		return a
	case AnimatedOrStatic[float64]:
		if !a.Static {
			return a.Anim
		}
	case *AnimatedOrStatic[float64]:
		if a != nil && !a.Static {
			return a.Anim
		}
	case AnimatedOrStatic[math.Vector3]:
		if !a.Static {
			return a.Anim
		}
	case *AnimatedOrStatic[math.Vector3]:
		if a != nil && !a.Static {
			return a.Anim
		}
	}
	return nil
}

func CloneKeyFrames(src map[int]any) map[int]any {
	if src == nil {
		return nil
	}
	dst := make(map[int]any, len(src))
	for k, v := range src {
		dst[k] = cloneAny(v)
	}
	return dst
}

func cloneAny(v any) any {
	switch val := v.(type) {
	case math.Vector3:
		return math.Vector3{val[0], val[1], val[2]}
	case math.QuaternionRotation:
		return math.QuaternionRotation{val[0], val[1], val[2], val[3]}
	case float64:
		return val
	case int:
		return val
	case []float64:
		out := make([]float64, len(val))
		copy(out, val)
		return out
	default:
		return v
	}
}

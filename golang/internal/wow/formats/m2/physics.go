package m2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	imath "github.com/pqhuy98/wow-converter/internal/math"
)

var ErrUnsupportedPhysics = errors.New("unsupported physics rig")

// PhysicsData retains source PHYS coordinates (Z-up), unlike the M2 renderer data.
type PhysicsData struct {
	Bodies []PhysicsBody
	Joints []PhysicsJoint
}

type PhysicsBody struct {
	Bone, Type                                      int
	Position                                        imath.Vector3
	Gravity, LinearDamping, AngularDamping, Inertia float64
	Shapes                                          []PhysicsShape
}

type PhysicsShape struct {
	A, B     imath.Vector3
	Radius   float64
	Vertices []imath.Vector3
	Planes   [][4]float64
}

type PhysicsFrame struct {
	Axes   [3]imath.Vector3
	Origin imath.Vector3
}

type PhysicsJoint struct {
	A, B, Type         int
	FrameA, FrameB     PhysicsFrame
	Lower, Upper, Cone float64
}

// parsePhysics reads MoP–retail PHYS bodies, collision shapes and articulated joints.
// PHYS FourCCs are reversed on disk; the enclosing M2 FourCCs are not.
func parsePhysics(data []byte) (*PhysicsData, error) {
	chunks := map[string][]byte{}
	for p := 0; p < len(data); {
		if len(data)-p < 8 {
			for _, b := range data[p:] {
				if b != 0 {
					return nil, fmt.Errorf("truncated chunk")
				}
			}
			break
		}
		n := int(binary.LittleEndian.Uint32(data[p+4:]))
		if n > len(data)-p-8 {
			return nil, fmt.Errorf("chunk exceeds PHYS payload")
		}
		tag := string([]byte{data[p+3], data[p+2], data[p+1], data[p]})
		if tag == "\x00\x00\x00\x00" && n == 0 {
			break
		}
		if _, exists := chunks[tag]; exists {
			return nil, fmt.Errorf("duplicate %s", tag)
		}
		chunks[tag] = data[p+8 : p+8+n]
		p += 8 + n
	}
	if len(chunks["PHYS"]) != 2 || binary.LittleEndian.Uint16(chunks["PHYS"]) > 6 {
		return nil, fmt.Errorf("%w: PHYS version", ErrUnsupportedPhysics)
	}
	var invalid bool
	f := func(b []byte, p int) float64 {
		v := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[p:])))
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
			invalid = true
		}
		return v
	}
	v := func(b []byte, p int) imath.Vector3 { return imath.Vector3{f(b, p), f(b, p+4), f(b, p+8)} }
	frame := func(b []byte, p int) PhysicsFrame {
		return PhysicsFrame{Axes: [3]imath.Vector3{v(b, p), v(b, p+12), v(b, p+24)}, Origin: v(b, p+36)}
	}
	records := func(tag string, stride int) ([][]byte, error) {
		b := chunks[tag]
		if len(b)%stride != 0 {
			return nil, fmt.Errorf("invalid %s stride", tag)
		}
		out := make([][]byte, len(b)/stride)
		for i := range out {
			out[i] = b[i*stride : (i+1)*stride]
		}
		return out, nil
	}
	// Resolve convex hull payloads. Header pointers are runtime-only, not file offsets.
	var hulls []PhysicsShape
	if b := chunks["PLYT"]; len(b) > 0 {
		if len(b) < 4 {
			return nil, fmt.Errorf("truncated PLYT")
		}
		n := int(binary.LittleEndian.Uint32(b))
		if n > (len(b)-4)/80 {
			return nil, fmt.Errorf("invalid PLYT count")
		}
		p := 4 + n*80
		for i := 0; i < n; i++ {
			h := b[4+i*80:]
			nv, nf, ne := int(binary.LittleEndian.Uint32(h)), int(binary.LittleEndian.Uint32(h[16:])), int(binary.LittleEndian.Uint32(h[40:]))
			if nv > 65536 || nf > 65536 || ne > 262144 || nv*12+nf*17+ne*4 > len(b)-p {
				return nil, fmt.Errorf("invalid PLYT payload")
			}
			s := PhysicsShape{}
			for k := 0; k < nv; k++ {
				s.Vertices = append(s.Vertices, v(b, p))
				p += 12
			}
			for k := 0; k < nf; k++ {
				s.Planes = append(s.Planes, [4]float64{f(b, p), f(b, p+4), f(b, p+8), f(b, p+12)})
				p += 16
			}
			p += nf + ne*4
			hulls = append(hulls, s)
		}
	}
	var shapeRefs []PhysicsShape
	for _, spec := range []struct {
		tag    string
		stride int
	}{{"SHAP", 20}, {"SHP2", 32}} {
		r, err := records(spec.tag, spec.stride)
		if err != nil {
			return nil, err
		}
		for _, b := range r {
			typ, id := int(binary.LittleEndian.Uint16(b)), int(int16(binary.LittleEndian.Uint16(b[2:])))
			var s PhysicsShape
			if typ == 3 {
				if id < 0 || id >= len(hulls) {
					return nil, fmt.Errorf("invalid hull index")
				}
				s = hulls[id]
			} else {
				tag, stride := "", 0
				switch typ {
				case 0:
					tag, stride = "BOXS", 60
				case 1:
					tag, stride = "CAPS", 28
				case 2:
					tag, stride = "SPHS", 16
				default:
					return nil, fmt.Errorf("%w: shape %d", ErrUnsupportedPhysics, typ)
				}
				shapes, err := records(tag, stride)
				if err != nil {
					return nil, err
				}
				if id < 0 || id >= len(shapes) {
					return nil, fmt.Errorf("invalid %s index", tag)
				}
				c := shapes[id]
				switch typ {
				case 0:
					fr, half := frame(c, 0), v(c, 48)
					for axis := 0; axis < 3; axis++ {
						for _, sign := range []float64{-1, 1} {
							normal := imath.V3Scale(fr.Axes[axis], sign)
							s.Planes = append(s.Planes, [4]float64{normal[0], normal[1], normal[2], -imath.V3Dot(normal, fr.Origin) - half[axis]})
						}
					}
					for mask := 0; mask < 8; mask++ {
						p := fr.Origin
						for axis := 0; axis < 3; axis++ {
							sign := -1.0
							if mask&(1<<axis) != 0 {
								sign = 1
							}
							p = imath.V3Sum(p, imath.V3Scale(fr.Axes[axis], half[axis]*sign))
						}
						s.Vertices = append(s.Vertices, p)
					}
				case 1:
					s.A, s.B, s.Radius = v(c, 0), v(c, 12), f(c, 24)
				case 2:
					s.A, s.B, s.Radius = v(c, 0), v(c, 0), f(c, 12)
				}
			}
			if s.Radius < 0 {
				return nil, fmt.Errorf("negative shape radius")
			}
			shapeRefs = append(shapeRefs, s)
		}
	}
	result := &PhysicsData{}
	for _, spec := range []struct {
		tag    string
		stride int
	}{{"BODY", 28}, {"BDY2", 32}, {"BDY3", 44}, {"BDY4", 48}} {
		r, err := records(spec.tag, spec.stride)
		if err != nil {
			return nil, err
		}
		for _, b := range r {
			body := PhysicsBody{Type: int(binary.LittleEndian.Uint16(b)), Position: v(b, 4), Gravity: 1, Inertia: 1}
			shapeOffset := 16
			if spec.stride <= 32 {
				body.Bone = int(binary.LittleEndian.Uint32(b[16:]))
				shapeOffset = 20
				if spec.stride == 32 {
					body.Gravity = f(b, 28)
				}
			} else {
				body.Bone = int(binary.LittleEndian.Uint16(b[2:]))
				body.Gravity, body.Inertia, body.LinearDamping, body.AngularDamping = f(b, 24), f(b, 28), f(b, 32), f(b, 36)
			}
			start, n := int(binary.LittleEndian.Uint32(b[shapeOffset:])), int(binary.LittleEndian.Uint32(b[shapeOffset+4:]))
			if start > len(shapeRefs) || n > len(shapeRefs)-start {
				return nil, fmt.Errorf("body shapes out of range")
			}
			body.Shapes = append(body.Shapes, shapeRefs[start:start+n]...)
			result.Bodies = append(result.Bodies, body)
		}
	}
	if len(result.Bodies) > 512 {
		return nil, fmt.Errorf("too many physics bodies")
	}
	r, err := records("JOIN", 16)
	if err != nil {
		return nil, err
	}
	for _, b := range r {
		j := PhysicsJoint{A: int(binary.LittleEndian.Uint32(b)), B: int(binary.LittleEndian.Uint32(b[4:])), Type: int(binary.LittleEndian.Uint16(b[12:]))}
		if j.A >= len(result.Bodies) || j.B >= len(result.Bodies) || j.A == j.B {
			return nil, fmt.Errorf("joint bodies out of range")
		}
		id := int(int16(binary.LittleEndian.Uint16(b[14:])))
		var tags []struct {
			tag    string
			stride int
		}
		switch j.Type {
		case 0:
			tags = []struct {
				tag    string
				stride int
			}{{"SPHJ", 28}}
		case 1:
			tags = []struct {
				tag    string
				stride int
			}{{"SHOJ", 116}, {"SHJ2", 124}}
		case 2:
			tags = []struct {
				tag    string
				stride int
			}{{"WELJ", 104}, {"WLJ2", 112}, {"WLJ3", 116}}
		case 3:
			tags = []struct {
				tag    string
				stride int
			}{{"REVJ", 112}, {"REV2", 120}}
		default:
			return nil, fmt.Errorf("%w: joint %d", ErrUnsupportedPhysics, j.Type)
		}
		var record []byte
		for _, spec := range tags {
			rows, err := records(spec.tag, spec.stride)
			if err != nil {
				return nil, err
			}
			if len(rows) > 0 {
				if id < 0 || id >= len(rows) {
					return nil, fmt.Errorf("joint index out of range")
				}
				record = rows[id]
			}
		}
		if record == nil {
			return nil, fmt.Errorf("missing joint record")
		}
		if j.Type == 0 {
			j.FrameA.Origin, j.FrameB.Origin = v(record, 0), v(record, 12)
			j.FrameA.Axes = [3]imath.Vector3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
			j.FrameB.Axes = j.FrameA.Axes
			j.Cone = 180
		} else {
			j.FrameA, j.FrameB = frame(record, 0), frame(record, 48)
			if j.Type == 1 || j.Type == 3 {
				j.Lower, j.Upper = f(record, 96), f(record, 100)
				if j.Type == 1 {
					j.Cone = f(record, 104)
				}
			}
		}
		if j.Cone < 0 || j.Cone > 180 {
			return nil, fmt.Errorf("invalid joint cone angle")
		}
		result.Joints = append(result.Joints, j)
	}
	if invalid {
		return nil, fmt.Errorf("non-finite or excessive physics value")
	}
	return result, nil
}

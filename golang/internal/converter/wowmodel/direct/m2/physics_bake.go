package directm2

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

type physicsPose struct {
	p imath.Vector3
	q imath.QuaternionRotation
	s imath.Vector3
}
type physicsCurve struct {
	times            []int
	values           []imath.QuaternionRotation
	global           int
	rotation, linear bool
}
type physicsBone struct {
	pivot  imath.Vector3
	parent int
	curves [3]physicsCurve
}
type physicsState struct {
	p, point, velocity imath.Vector3
	q                  imath.QuaternionRotation
}

type physicsCollider struct {
	body                         int
	shape                        m2.PhysicsShape
	localCenter, center, a, axis imath.Vector3
	bound, radius, axisLength2   float64
	planes                       [][4]float64
}
type physicsLink struct {
	body           int
	joint          m2.PhysicsJoint
	frameA, frameB imath.QuaternionRotation
	center         imath.Vector3
}

func bakePhysics(ctx context.Context, model *mdl.MDL, data *m2.PhysicsData, scale float64) error {
	if data == nil || len(data.Joints) == 0 {
		return nil
	}
	if scale <= 0 {
		return fmt.Errorf("invalid physics model scale")
	}
	started := time.Now()
	indices := map[components.Node]int{}
	for i, b := range model.Bones {
		indices[b] = i
	}
	bones := make([]physicsBone, len(model.Bones))
	for i, b := range model.Bones {
		parent := -1
		if b.Parent != nil {
			var ok bool
			parent, ok = indices[b.Parent]
			if !ok {
				return fmt.Errorf("physics bone has non-bone parent")
			}
		}
		bones[i] = physicsBone{pivot: imath.V3Scale(b.PivotPoint, 1/scale), parent: parent, curves: [3]physicsCurve{compilePhysicsCurve(b.Translation, false), compilePhysicsCurve(b.Rotation, true), compilePhysicsCurve(b.Scaling, false)}}
	}
	active := make([]bool, len(bones))
	bodyForBone := make([]int, len(bones))
	for i := range bodyForBone {
		bodyForBone[i] = -1
	}
	for i, b := range data.Bodies {
		if b.Bone < 0 || b.Bone >= len(bones) {
			return fmt.Errorf("physics bone %d out of range", b.Bone)
		}
		if bodyForBone[b.Bone] >= 0 {
			return fmt.Errorf("%w: multiple bodies on bone %d", m2.ErrUnsupportedPhysics, b.Bone)
		}
		bodyForBone[b.Bone] = i
		for n, steps := b.Bone, 0; n >= 0; n = bones[n].parent {
			if steps >= len(bones) {
				return fmt.Errorf("cyclic physics skeleton")
			}
			active[n] = true
			steps++
		}
	}
	var order []int
	visited := make([]bool, len(bones))
	var visit func(int)
	visit = func(i int) {
		if visited[i] {
			return
		}
		visited[i] = true
		if bones[i].parent >= 0 {
			visit(bones[i].parent)
		}
		order = append(order, i)
	}
	for i := range bones {
		if active[i] {
			visit(i)
		}
	}
	byBody := map[int]m2.PhysicsJoint{}
	for _, j := range data.Joints {
		if data.Bodies[j.B].Type != 1 {
			continue
		}
		if _, exists := byBody[j.B]; exists {
			return fmt.Errorf("%w: multiple constraints on body %d", m2.ErrUnsupportedPhysics, j.B)
		}
		byBody[j.B] = j
	}
	var links []physicsLink
	states := make([]uint8, len(data.Bodies))
	var add func(int) error
	add = func(i int) error {
		if states[i] == 2 || data.Bodies[i].Type != 1 {
			return nil
		}
		if states[i] == 1 {
			return fmt.Errorf("%w: cyclic joints", m2.ErrUnsupportedPhysics)
		}
		states[i] = 1
		j, ok := byBody[i]
		if !ok {
			return fmt.Errorf("%w: unanchored body %d", m2.ErrUnsupportedPhysics, i)
		}
		if err := add(j.A); err != nil {
			return err
		}
		center := physicsShapeCenter(data.Bodies[i].Shapes)
		if imath.V3Dot(center, center) < 1e-6 {
			return fmt.Errorf("%w: body %d has no lever arm", m2.ErrUnsupportedPhysics, i)
		}
		links = append(links, physicsLink{body: i, joint: j, frameA: physicsFrameQuat(j.FrameA), frameB: physicsFrameQuat(j.FrameB), center: center})
		states[i] = 2
		return nil
	}
	for i := range data.Bodies {
		if err := add(i); err != nil {
			return err
		}
	}
	if len(links) == 0 {
		return nil
	}
	// Compile the authored curves before adding any baked keys. Only ancestors of
	// PHYS bodies are sampled; face/finger tracks never enter the simulation loop.
	poses := make([]physicsPose, len(bones))
	state := make([]physicsState, len(data.Bodies))
	colliders := makePhysicsColliders(data)
	rotations, translations := make([]*components.Animation, len(bones)), make([]*components.Animation, len(bones))
	for _, link := range links {
		i := data.Bodies[link.body].Bone
		// Secondary motion tolerates more error than authored body animation.
		rotations[i] = &components.Animation{ReductionTolerance: 2 * math.Pi / 180, Type: components.AnimTypeRotation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{}}
		translations[i] = &components.Animation{ReductionTolerance: 0.5, Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{}}
		// Preserve the animated anchors too: freezing a parent invalidates the bake.
		for n := i; n >= 0 && model.Bones[n].Name != "Head"; n = bones[n].parent {
			model.Bones[n].PhysicsDriven = true
		}
	}
	samples := 0
	for _, seq := range model.Sequences {
		if err := ctx.Err(); err != nil {
			return err
		}
		duration := seq.Interval[1] - seq.Interval[0]
		if duration < 0 {
			return fmt.Errorf("negative physics sequence duration")
		}
		curves := physicsSequenceCurves(bones, seq)
		evaluate := func(t int, simulated bool) {
			for _, i := range order {
				b := bones[i]
				c := curves[i]
				tr := samplePhysicsCurve(c[0], t, imath.QuaternionRotation{})
				rot := samplePhysicsCurve(c[1], t, imath.QuatNoRotation())
				sc := samplePhysicsCurve(c[2], t, imath.QuaternionRotation{1, 1, 1, 0})
				parent := physicsPose{q: imath.QuatNoRotation(), s: imath.Vector3{1, 1, 1}}
				delta := b.pivot
				if b.parent >= 0 {
					parent = poses[b.parent]
					delta = imath.V3Sub(delta, bones[b.parent].pivot)
				}
				p := physicsPose{p: imath.V3Sum(parent.p, physicsRotate(parent.q, imath.V3Mul(parent.s, imath.V3Sum(delta, imath.Vector3{tr[0] / scale, tr[1] / scale, tr[2] / scale})))), q: physicsMul(parent.q, rot), s: imath.V3Mul(parent.s, imath.Vector3{sc[0], sc[1], sc[2]})}
				if id := bodyForBone[i]; simulated && id >= 0 && data.Bodies[id].Type == 1 {
					p.q = state[id].q
					p.p = imath.V3Sub(state[id].p, physicsRotate(p.q, imath.V3Sub(data.Bodies[id].Position, b.pivot)))
				}
				poses[i] = p
			}
		}
		kinematics := func() {
			for i, b := range data.Bodies {
				if b.Type == 1 {
					continue
				}
				pose := poses[b.Bone]
				state[i].q = pose.q
				state[i].p = imath.V3Sum(pose.p, physicsRotate(pose.q, imath.V3Mul(pose.s, imath.V3Sub(b.Position, bones[b.Bone].pivot))))
			}
			updatePhysicsColliders(colliders, state)
		}
		evaluate(seq.Interval[0], false)
		kinematics()
		for _, l := range links {
			p := poses[data.Bodies[l.body].Bone]
			state[l.body] = physicsState{q: p.q, p: p.p, point: imath.V3Sum(p.p, physicsRotate(p.q, l.center))}
		}
		// Settle at the starting pose, then warm a full loop. This avoids the
		// initial upward rest-pose collapse appearing in Stand/Portrait animations.
		for k := 0; k < 120; k++ {
			stepPhysics(data, links, state, colliders, 1.0/60)
		}
		steps := int(math.Ceil(float64(duration) * 60 / 1000))
		dt := 1.0 / 60
		if steps > 0 {
			dt = float64(duration) / 1000 / float64(steps)
		}
		if !seq.NonLooping {
			for k := 1; k <= steps; k++ {
				if k%128 == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				evaluate(seq.Interval[0]+int(math.Round(float64(k*duration)/float64(steps))), false)
				kinematics()
				stepPhysics(data, links, state, colliders, dt)
			}
		}
		// Keep the solver at 60 Hz for stable contacts; export at 15 Hz.
		times := make([]int, 0, steps/4+2)
		for k := 0; k <= steps; k++ {
			if k%128 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			t := seq.Interval[0]
			if steps > 0 {
				t += int(math.Round(float64(k*duration) / float64(steps)))
			}
			evaluate(t, false)
			kinematics()
			if k > 0 {
				stepPhysics(data, links, state, colliders, dt)
			}
			if k%4 != 0 && k != steps {
				continue
			}
			evaluate(t, true)
			times = append(times, t)
			samples++
			for _, l := range links {
				i := data.Bodies[l.body].Bone
				parent := physicsPose{q: imath.QuatNoRotation(), s: imath.Vector3{1, 1, 1}}
				delta := bones[i].pivot
				if p := bones[i].parent; p >= 0 {
					parent = poses[p]
					delta = imath.V3Sub(delta, bones[p].pivot)
				}
				q := physicsMul(physicsConjugate(parent.q), poses[i].q)
				local := physicsRotate(physicsConjugate(parent.q), imath.V3Sub(poses[i].p, parent.p))
				for axis := range local {
					if math.Abs(parent.s[axis]) > 1e-8 {
						local[axis] /= parent.s[axis]
					}
				}
				rotations[i].KeyFrames[t] = q
				translations[i].KeyFrames[t] = imath.V3Scale(imath.V3Sub(local, delta), scale)
			}
		}
		if !seq.NonLooping {
			for _, l := range links {
				i := data.Bodies[l.body].Bone
				closePhysicsLoop(rotations[i], translations[i], times)
			}
		}
	}
	for _, l := range links {
		i := data.Bodies[l.body].Bone
		model.Bones[i].Rotation = rotations[i]
		model.Bones[i].Translation = translations[i]
	}
	log.Printf("Baked physics: %d bodies, %d sequences, %d samples in %s", len(links), len(model.Sequences), samples, time.Since(started).Round(time.Millisecond))
	return nil
}

func stepPhysics(data *m2.PhysicsData, links []physicsLink, state []physicsState, colliders []physicsCollider, dt float64) {
	// ponytail: an articulated pendulum bake, not a full Domino rigid-body port.
	// Exact anchors, source angular limits and kinematic collisions give stable
	// secondary motion in O(bodies * colliders); joint loops need a coupled solver.
	for _, l := range links {
		body := data.Bodies[l.body]
		s := &state[l.body]
		parent := state[l.joint.A]
		anchor := imath.V3Sum(parent.p, physicsRotate(parent.q, l.joint.FrameA.Origin))
		previous := s.point
		damping := math.Exp(-math.Max(0, body.LinearDamping+body.AngularDamping) * dt)
		predicted := imath.V3Sum(previous, imath.V3Sum(imath.V3Scale(s.velocity, dt*damping), imath.Vector3{0, 0, -9.81 * body.Gravity * dt * dt / math.Max(0.1, body.Inertia)}))
		lever := imath.V3Sub(l.center, l.joint.FrameB.Origin)
		orient := func(target imath.Vector3) {
			base := parent.q
			q := physicsMul(physicsBetween(physicsRotate(base, lever), imath.V3Sub(target, anchor)), base)
			q = limitPhysicsJoint(q, parent.q, l)
			s.q = q
			s.p = imath.V3Sub(anchor, physicsRotate(q, l.joint.FrameB.Origin))
			s.point = imath.V3Sum(s.p, physicsRotate(q, l.center))
		}
		orient(predicted)
		for iteration := 0; iteration < 2; iteration++ {
			correction := imath.Vector3{}
			for _, shape := range body.Shapes {
				if len(shape.Vertices) > 0 {
					for _, point := range shape.Vertices {
						correction = imath.V3Sum(correction, physicsContact(imath.V3Sum(s.p, physicsRotate(s.q, point)), 0.002, l.joint.A, colliders))
					}
				} else {
					for _, point := range [3]imath.Vector3{shape.A, imath.V3Mean(shape.A, shape.B), shape.B} {
						correction = imath.V3Sum(correction, physicsContact(imath.V3Sum(s.p, physicsRotate(s.q, point)), shape.Radius, l.joint.A, colliders))
					}
				}
			}
			if imath.V3Dot(correction, correction) < 1e-12 {
				break
			}
			// Move the center less than a shape's extremity to avoid overcorrection.
			orient(imath.V3Sum(s.point, imath.V3Scale(correction, 0.5)))
		}
		s.velocity = imath.V3Scale(imath.V3Sub(s.point, previous), 1/dt)
		// Bound impulse spikes from discontinuous source poses, without changing
		// the stable fixed step used for ordinary animation-driven motion.
		maxSpeed := 30 * math.Sqrt(imath.V3Dot(lever, lever))
		speed := math.Sqrt(imath.V3Dot(s.velocity, s.velocity))
		if speed > maxSpeed {
			s.velocity = imath.V3Scale(s.velocity, maxSpeed/speed)
		}
	}
}

func makePhysicsColliders(data *m2.PhysicsData) []physicsCollider {
	var out []physicsCollider
	for i, b := range data.Bodies {
		if b.Type == 1 {
			continue
		}
		for _, shape := range b.Shapes {
			c := physicsCollider{body: i, shape: shape, radius: shape.Radius, localCenter: imath.V3Mean(shape.A, shape.B), planes: make([][4]float64, len(shape.Planes))}
			points := shape.Vertices
			if len(points) > 0 {
				c.localCenter = imath.Vector3{}
				for _, p := range points {
					c.localCenter = imath.V3Sum(c.localCenter, p)
				}
				c.localCenter = imath.V3Scale(c.localCenter, 1/float64(len(points)))
			} else {
				points = []imath.Vector3{shape.A, shape.B}
			}
			for _, p := range points {
				c.bound = math.Max(c.bound, imath.V3Distance(p, c.localCenter)+shape.Radius)
			}
			out = append(out, c)
		}
	}
	return out
}

func updatePhysicsColliders(colliders []physicsCollider, state []physicsState) {
	for i := range colliders {
		c := &colliders[i]
		s := state[c.body]
		c.center = imath.V3Sum(s.p, physicsRotate(s.q, c.localCenter))
		c.a = imath.V3Sum(s.p, physicsRotate(s.q, c.shape.A))
		c.axis = physicsRotate(s.q, imath.V3Sub(c.shape.B, c.shape.A))
		c.axisLength2 = imath.V3Dot(c.axis, c.axis)
		for k, p := range c.shape.Planes {
			n := physicsRotate(s.q, imath.Vector3{p[0], p[1], p[2]})
			c.planes[k] = [4]float64{n[0], n[1], n[2], p[3] - imath.V3Dot(n, s.p)}
		}
	}
}

func physicsContact(point imath.Vector3, radius float64, parent int, colliders []physicsCollider) imath.Vector3 {
	result := imath.Vector3{}
	if point[2] < radius {
		result[2] = radius - point[2]
	}
	for i := range colliders {
		c := &colliders[i]
		if c.body == parent {
			continue
		}
		delta := imath.V3Sub(point, c.center)
		bound := c.bound + radius
		if imath.V3Dot(delta, delta) > bound*bound {
			continue
		}
		if len(c.planes) > 0 {
			depth, normal := math.Inf(1), imath.Vector3{}
			for _, plane := range c.planes {
				n := imath.Vector3{plane[0], plane[1], plane[2]}
				dist := imath.V3Dot(n, point) + plane[3] - radius
				if dist >= 0 {
					depth = -1
					break
				}
				if -dist < depth {
					depth = -dist
					normal = n
				}
			}
			if depth > 0 && !math.IsInf(depth, 0) {
				result = imath.V3Sum(result, imath.V3Scale(normal, depth))
			}
		} else {
			t := 0.0
			if c.axisLength2 > 1e-12 {
				t = math.Max(0, math.Min(1, imath.V3Dot(imath.V3Sub(point, c.a), c.axis)/c.axisLength2))
			}
			delta = imath.V3Sub(point, imath.V3Sum(c.a, imath.V3Scale(c.axis, t)))
			length2 := imath.V3Dot(delta, delta)
			r := radius + c.radius
			if length2 >= r*r {
				continue
			}
			distance := math.Sqrt(length2)
			if distance < 1e-8 {
				delta = imath.Vector3{-1, 0, 0}
				distance = 1
			}
			result = imath.V3Sum(result, imath.V3Scale(delta, (r-math.Sqrt(length2))/distance))
		}
	}
	return result
}

func limitPhysicsJoint(q, parent imath.QuaternionRotation, l physicsLink) imath.QuaternionRotation {
	a := physicsMul(parent, l.frameA)
	rel := physicsMul(physicsConjugate(a), physicsMul(q, l.frameB))
	if rel[3] < 0 {
		for k := range rel {
			rel[k] = -rel[k]
		}
	}
	twist := physicsNormalize(imath.QuaternionRotation{0, 0, rel[2], rel[3]})
	swing := physicsMul(rel, physicsConjugate(twist))
	cone := l.joint.Cone * math.Pi / 180
	if l.joint.Type == 3 || l.joint.Type == 2 {
		cone = 0
	}
	angle := 2 * math.Acos(math.Min(1, math.Abs(swing[3])))
	if angle > cone && angle > 1e-8 {
		swing = imath.V3Slerp(imath.QuatNoRotation(), swing, cone/angle)
	}
	twistAngle := 2 * math.Atan2(twist[2], twist[3])
	if l.joint.Upper > l.joint.Lower {
		twistAngle = math.Max(l.joint.Lower*math.Pi/180, math.Min(l.joint.Upper*math.Pi/180, twistAngle))
	}
	if l.joint.Type == 2 {
		twistAngle = 0
	}
	twist = imath.QuaternionRotation{0, 0, math.Sin(twistAngle / 2), math.Cos(twistAngle / 2)}
	return physicsNormalize(physicsMul(physicsMul(a, physicsMul(swing, twist)), physicsConjugate(l.frameB)))
}

func physicsShapeCenter(shapes []m2.PhysicsShape) imath.Vector3 {
	center := imath.Vector3{}
	weight := 0.0
	for _, s := range shapes {
		p := imath.V3Mean(s.A, s.B)
		w := math.Max(1e-6, s.Radius*s.Radius*(imath.V3Distance(s.A, s.B)+s.Radius*4/3))
		if len(s.Vertices) > 0 {
			p = imath.Vector3{}
			for _, v := range s.Vertices {
				p = imath.V3Sum(p, v)
			}
			p = imath.V3Scale(p, 1/float64(len(s.Vertices)))
			w = 1
		}
		center = imath.V3Sum(center, imath.V3Scale(p, w))
		weight += w
	}
	if weight > 0 {
		return imath.V3Scale(center, 1/weight)
	}
	return center
}

func compilePhysicsCurve(a *components.Animation, rotation bool) physicsCurve {
	c := physicsCurve{rotation: rotation, global: -1}
	if a == nil {
		return c
	}
	c.linear = a.Interpolation == components.InterpLinear
	if a.GlobalSeq != nil {
		c.global = a.GlobalSeq.Duration
	}
	for t := range a.KeyFrames {
		c.times = append(c.times, t)
	}
	sort.Ints(c.times)
	for _, t := range c.times {
		switch v := a.KeyFrames[t].(type) {
		case imath.Vector3:
			c.values = append(c.values, imath.QuaternionRotation{v[0], v[1], v[2], 0})
		case imath.QuaternionRotation:
			c.values = append(c.values, physicsNormalize(v))
		default:
			c.values = append(c.values, imath.QuaternionRotation{})
		}
	}
	return c
}

func physicsSequenceCurves(bones []physicsBone, seq components.Sequence) [][3]physicsCurve {
	out := make([][3]physicsCurve, len(bones))
	for i, b := range bones {
		for k, c := range b.curves {
			if c.global < 0 {
				lo := sort.SearchInts(c.times, seq.Interval[0])
				hi := sort.Search(len(c.times), func(j int) bool { return c.times[j] > seq.Interval[1] })
				c.times, c.values = c.times[lo:hi], c.values[lo:hi]
			}
			out[i][k] = c
		}
	}
	return out
}

func samplePhysicsCurve(c physicsCurve, t int, def imath.QuaternionRotation) imath.QuaternionRotation {
	if len(c.times) == 0 {
		return def
	}
	if c.global > 0 {
		t %= c.global
	}
	hi := sort.Search(len(c.times), func(i int) bool { return c.times[i] >= t })
	if hi == 0 {
		return c.values[0]
	}
	if hi == len(c.times) {
		return c.values[hi-1]
	}
	if c.times[hi] == t {
		return c.values[hi]
	}
	if !c.linear {
		return c.values[hi-1]
	}
	alpha := float64(t-c.times[hi-1]) / float64(c.times[hi]-c.times[hi-1])
	if c.rotation {
		return imath.V3Slerp(c.values[hi-1], c.values[hi], alpha)
	}
	v := c.values[hi-1]
	for k := range v {
		v[k] += (c.values[hi][k] - v[k]) * alpha
	}
	return v
}

func closePhysicsLoop(rotation, translation *components.Animation, times []int) {
	if len(times) < 2 {
		return
	}
	start, end := times[0], times[len(times)-1]
	first, last := rotation.KeyFrames[start].(imath.QuaternionRotation), rotation.KeyFrames[end].(imath.QuaternionRotation)
	correction := physicsMul(first, physicsConjugate(last))
	delta := imath.V3Sub(translation.KeyFrames[start].(imath.Vector3), translation.KeyFrames[end].(imath.Vector3))
	for _, t := range times {
		x := float64(t-start) / float64(end-start)
		x = x * x * (3 - 2*x)
		rotation.KeyFrames[t] = physicsNormalize(physicsMul(imath.V3Slerp(imath.QuatNoRotation(), correction, x), rotation.KeyFrames[t].(imath.QuaternionRotation)))
		translation.KeyFrames[t] = imath.V3Sum(translation.KeyFrames[t].(imath.Vector3), imath.V3Scale(delta, x))
	}
	rotation.KeyFrames[end] = first
	translation.KeyFrames[end] = translation.KeyFrames[start]
}

func physicsNormalize(q imath.QuaternionRotation) imath.QuaternionRotation {
	length := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	if length < 1e-12 {
		return imath.QuatNoRotation()
	}
	for k := range q {
		q[k] /= length
	}
	return q
}
func physicsConjugate(q imath.QuaternionRotation) imath.QuaternionRotation {
	return imath.QuaternionRotation{-q[0], -q[1], -q[2], q[3]}
}
func physicsMul(a, b imath.QuaternionRotation) imath.QuaternionRotation {
	return imath.QuaternionRotation{a[3]*b[0] + a[0]*b[3] + a[1]*b[2] - a[2]*b[1], a[3]*b[1] - a[0]*b[2] + a[1]*b[3] + a[2]*b[0], a[3]*b[2] + a[0]*b[1] - a[1]*b[0] + a[2]*b[3], a[3]*b[3] - a[0]*b[0] - a[1]*b[1] - a[2]*b[2]}
}
func physicsCross(a, b imath.Vector3) imath.Vector3 {
	return imath.Vector3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func physicsRotate(q imath.QuaternionRotation, v imath.Vector3) imath.Vector3 {
	u := imath.Vector3{q[0], q[1], q[2]}
	c := physicsCross(u, v)
	return imath.V3Sum(v, imath.V3Scale(imath.V3Sum(imath.V3Scale(c, q[3]), physicsCross(u, c)), 2))
}
func physicsBetween(a, b imath.Vector3) imath.QuaternionRotation {
	a, b = imath.V3Normalize(a), imath.V3Normalize(b)
	d := imath.V3Dot(a, b)
	if d < -0.999999 {
		axis := physicsCross(a, imath.Vector3{1, 0, 0})
		if imath.V3Dot(axis, axis) < 1e-6 {
			axis = physicsCross(a, imath.Vector3{0, 1, 0})
		}
		axis = imath.V3Normalize(axis)
		return imath.QuaternionRotation{axis[0], axis[1], axis[2], 0}
	}
	c := physicsCross(a, b)
	return physicsNormalize(imath.QuaternionRotation{c[0], c[1], c[2], 1 + d})
}
func physicsFrameQuat(f m2.PhysicsFrame) imath.QuaternionRotation {
	x, y, z := f.Axes[0], f.Axes[1], f.Axes[2]
	tr := x[0] + y[1] + z[2]
	if tr > 0 {
		s := math.Sqrt(tr+1) * 2
		return physicsNormalize(imath.QuaternionRotation{(y[2] - z[1]) / s, (z[0] - x[2]) / s, (x[1] - y[0]) / s, s / 4})
	}
	i := 0
	if y[1] > x[0] {
		i = 1
	}
	if z[2] > f.Axes[i][i] {
		i = 2
	}
	j, k := (i+1)%3, (i+2)%3
	s := math.Sqrt(math.Max(0, 1+f.Axes[i][i]-f.Axes[j][j]-f.Axes[k][k])) * 2
	if s < 1e-8 {
		return imath.QuatNoRotation()
	}
	q := imath.QuaternionRotation{}
	q[i] = s / 4
	q[j] = (f.Axes[j][i] + f.Axes[i][j]) / s
	q[k] = (f.Axes[k][i] + f.Axes[i][k]) / s
	q[3] = (f.Axes[j][k] - f.Axes[k][j]) / s
	return physicsNormalize(q)
}

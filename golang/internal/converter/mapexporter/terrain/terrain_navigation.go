package terrain

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

type navigationTriangle struct {
	v     [3]imath.Vector3
	slope float64
}
type navigationSpan struct {
	z                     float64
	cell, owner, distance int
	triangle              int // -1 for a synthetic jump ramp.
}
type navigationJump struct{ from, to int }
type navigationGrid struct {
	width, height       int
	ox, oy              float64
	triangles           []navigationTriangle
	bins                [][]int
	cells               [][]int
	spans               []navigationSpan
	parents, seedCounts []int
	jumps               []navigationJump
	limits              settings.NavigationConfig
	minZ, maxZ          float64
	water               *MapWater
}

type TerrainNavigation struct {
	Width                 int     `json:"width"`
	Height                int     `json:"height"`
	Triangles             int     `json:"collisionTriangles"`
	GroundNPCs            int     `json:"groundNPCs"`
	RejectedNPCs          int     `json:"unsupportedNPCs"`
	Reachable             int     `json:"reachableCells"`
	TraversalReachable    int     `json:"traversalReachableCells"`
	ProjectionBlocked     int     `json:"projectionBlockedCells"`
	GroundBlocked         int     `json:"groundBlockedCells"`
	AirBlocked            int     `json:"airBlockedCells"`
	WaterBlocked          int     `json:"nativeWaterBlockedCells"`
	Conflicts             int     `json:"overlappingCells"`
	JumpLinks             int     `json:"jumpLinks"`
	BlockerInstances      int     `json:"blockerInstances"`
	BlockerTypes          int     `json:"blockerTypes"`
	BlockerPadding        int     `json:"nativeGridBlockedCells"`
	NPCHeightMedianBefore float64 `json:"npcHeightMedianErrorBefore"`
	NPCHeightMedianAfter  float64 `json:"npcHeightMedianErrorAfter"`
	Pathing               []byte  `json:"-"`
	waterBlocked          []bool
	seedPositions         []imath.Vector3
}

func GenerateReachableTerrain(ctx context.Context, m *common.WowObjectManager, terrain data.Terrain, cfg *settings.MapExportConfig, rawScale float64, water ...*MapWater) (*TerrainNavigation, error) {
	limits := cfg.Navigation
	if limits == (settings.NavigationConfig{}) {
		limits = settings.DefaultNavigationConfig()
	}
	if limits.Disabled {
		return nil, nil
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	ext := common.ComputeAbsoluteMinMaxExtents(m.Roots)
	size := imath.V3Sub(ext.Max, ext.Min)
	if size[0] <= 0 || size[1] <= 0 || size[2] <= 0 {
		return nil, nil
	}
	sx := float64(terrain.Map.Width) * 128 / size[0]
	sy := float64(terrain.Map.Height) * 128 / size[1]
	sz := config.MaxGameHeightDiff() / (size[2] * (cfg.Terrain.ClampPercent.Upper - cfg.Terrain.ClampPercent.Lower))
	unitScale := rawScale * sz
	if unitScale <= 0 || math.IsInf(unitScale, 0) || math.IsNaN(unitScale) {
		return nil, fmt.Errorf("invalid navigation export scale")
	}
	limits.Radius *= unitScale
	limits.Height *= unitScale
	limits.StepHeight *= unitScale
	limits.JumpHeight *= unitScale
	limits.JumpDistance *= unitScale
	limits.MaxDrop *= unitScale
	limits.SeedSnap *= unitScale
	limits.AirBlockHeight *= unitScale
	g := &navigationGrid{width: terrain.Map.Width * 4, height: terrain.Map.Height * 4,
		ox: float64(terrain.Map.Offset.X), oy: float64(terrain.Map.Offset.Y), limits: limits,
		minZ: config.DataHeightToGameZ(float64(config.DataHeightMin())), maxZ: config.DataHeightToGameZ(float64(config.DataHeightMax()))}
	if len(water) > 0 {
		g.water = water[0]
	}
	g.bins = make([][]int, g.width*g.height)
	g.cells = make([][]int, len(g.bins))
	transform := func(p imath.Vector3, abs common.ObjectAbsolute) imath.Vector3 {
		p = imath.V3Sum(abs.Position, imath.V3Rotate(imath.V3Scale(p, abs.ScaleFactor), abs.Rotation))
		return imath.Vector3{g.ox + (p[0]-ext.Min[0])*sx, g.oy + (p[1]-ext.Min[1])*sy, g.minZ + (p[2]-ext.Min[2]-size[2]*cfg.Terrain.ClampPercent.Lower)*sz}
	}
	var seeds []imath.Vector3
	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		if common.IsWowUnit(obj) {
			// Grounded status is physical: airborne/swimming spawns without
			// supporting collision near their feet cannot seed the search.
			seeds = append(seeds, transform(imath.Vector3{}, abs))
			return
		}
		if obj.Model == nil {
			return
		}
		if obj.Type == common.WowObjectADT {
			for _, geo := range obj.Model.MDL.Geosets {
				for _, f := range geo.Faces {
					var tri [3]imath.Vector3
					for j := range tri {
						tri[j] = transform(f.Vertices[j].Position, abs)
					}
					g.addTriangle(tri)
				}
			}
		} else {
			for _, src := range obj.Model.Collision {
				var tri [3]imath.Vector3
				for j := range tri {
					tri[j] = transform(src[j], abs)
				}
				g.addTriangle(tri)
			}
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(seeds) == 0 {
		log.Printf("Terrain navigation: no NPC seeds; retaining ADT terrain")
		return nil, nil
	}
	log.Printf("Terrain navigation: rasterizing %d collision triangles into %dx%d cells", len(g.triangles), g.width, g.height)
	stageStarted := time.Now()
	if err := g.rasterize(ctx); err != nil {
		return nil, err
	}
	log.Printf("Terrain navigation: rasterized %d walkable spans in %s", len(g.spans), time.Since(stageStarted).Round(time.Millisecond))
	report := &TerrainNavigation{Width: g.width, Height: g.height, Triangles: len(g.triangles)}
	log.Printf("Terrain navigation: grounding %d NPC seeds", len(seeds))
	stageStarted = time.Now()
	queue := g.seed(seeds, report)
	log.Printf("Terrain navigation: grounded %d NPCs, %d unsupported, %d starting spans in %s", report.GroundNPCs, report.RejectedNPCs, len(queue), time.Since(stageStarted).Round(time.Millisecond))
	if len(queue) == 0 {
		log.Printf("Terrain navigation: no grounded NPCs (%d unsupported); retaining ADT terrain", report.RejectedNPCs)
		return nil, nil
	}
	report.NPCHeightMedianBefore = navigationNPCError(terrain, report.seedPositions)
	log.Printf("Terrain navigation: searching walking and jumping routes")
	stageStarted = time.Now()
	if err := g.search(ctx, queue); err != nil {
		return nil, err
	}
	log.Printf("Terrain navigation: route search finished in %s; projecting %d jump links", time.Since(stageStarted).Round(time.Millisecond), len(g.jumps))
	stageStarted = time.Now()
	g.projectJumps()
	log.Printf("Terrain navigation: jump links projected in %s; fitting terrain and classifying pathing", time.Since(stageStarted).Round(time.Millisecond))
	report.JumpLinks = len(g.jumps)
	stageStarted = time.Now()
	g.project(terrain, report)
	log.Printf("Terrain navigation: terrain and pathing projected in %s", time.Since(stageStarted).Round(time.Millisecond))
	report.NPCHeightMedianAfter = navigationNPCError(terrain, report.seedPositions)
	log.Printf("Terrain navigation: %d ground NPCs, %d reachable cells, %d ground blockers, %d air blockers, %d overlaps, %d jump links", report.GroundNPCs, report.Reachable, report.GroundBlocked, report.AirBlocked, report.Conflicts, report.JumpLinks)
	return report, ctx.Err()
}

func (g *navigationGrid) addTriangle(v [3]imath.Vector3) {
	for _, p := range v {
		for _, n := range p {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return
			}
		}
	}
	loX, hiX := min(v[0][0], v[1][0], v[2][0]), max(v[0][0], v[1][0], v[2][0])
	loY, hiY := min(v[0][1], v[1][1], v[2][1]), max(v[0][1], v[1][1], v[2][1])
	x0, x1 := int(math.Floor((loX-g.ox)/32)), int(math.Floor((hiX-g.ox)/32))
	y0, y1 := int(math.Floor((loY-g.oy)/32)), int(math.Floor((hiY-g.oy)/32))
	if x1 < 0 || y1 < 0 || x0 >= g.width || y0 >= g.height {
		return
	}
	n := NavigationCross(imath.V3Sub(v[1], v[0]), imath.V3Sub(v[2], v[0]))
	slope := math.Inf(1)
	if n[2] > 1e-9 {
		slope = math.Hypot(n[0], n[1]) / n[2]
	}
	id := len(g.triangles)
	g.triangles = append(g.triangles, navigationTriangle{v: v, slope: slope})
	for y := max(0, y0); y <= min(g.height-1, y1); y++ {
		for x := max(0, x0); x <= min(g.width-1, x1); x++ {
			g.bins[y*g.width+x] = append(g.bins[y*g.width+x], id)
		}
	}
}

func (g *navigationGrid) point(cell int, z float64) imath.Vector3 {
	return imath.Vector3{g.ox + (float64(cell%g.width)+.5)*32, g.oy + (float64(cell/g.width)+.5)*32, z}
}

func (g *navigationGrid) rasterize(ctx context.Context) error {
	tanSlope := math.Tan(g.limits.MaxSlopeDegrees * math.Pi / 180)
	started := time.Now()
	nextProgress := started.Add(5 * time.Second)
	for cell, ids := range g.bins {
		if cell%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if now := time.Now(); !now.Before(nextProgress) {
				log.Printf("Terrain navigation: rasterizing %d/%d cells (%.1f%%), %d walkable spans, elapsed %s", cell, len(g.bins), float64(cell)*100/float64(len(g.bins)), len(g.spans), now.Sub(started).Round(time.Millisecond))
				nextProgress = now.Add(5 * time.Second)
			}
		}
		p := g.point(cell, 0)
		var surfaces []navigationSpan
		for _, id := range ids {
			t := g.triangles[id]
			if t.slope > tanSlope {
				continue
			}
			z, ok := NavigationHeight(t.v, p[0], p[1])
			if !ok || z < g.minZ || z > g.maxZ {
				continue
			}
			if g.water != nil && g.water.blocksGround(p[0], p[1], z) {
				continue
			}
			surfaces = append(surfaces, navigationSpan{z: z, triangle: id})
		}
		sort.SliceStable(surfaces, func(i, j int) bool { return surfaces[i].z < surfaces[j].z })
		last := math.Inf(-1)
		for _, surface := range surfaces {
			z := surface.z
			if z-last < .5 {
				continue
			}
			last = z
			p[2] = z + .5
			top := p
			top[2] = z + g.limits.Height
			if g.segmentBlocked(p, top) {
				continue
			}
			// A thin wall can separate two NPC-seeded rooms even when both cell
			// centers are reachable. Reserve its whole pathing-cell footprint;
			// otherwise the projected grid would open a route through the wall.
			blocked := false
			for _, id := range ids {
				t := g.triangles[id]
				if t.slope <= tanSlope {
					continue
				}
				bottom := z + g.limits.StepHeight + .5
				if navigationTriangleBox(t.v, imath.Vector3{p[0], p[1], (bottom + top[2]) / 2}, imath.Vector3{16.01, 16.01, max(.01, (top[2]-bottom)/2)}) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			id := len(g.spans)
			g.spans = append(g.spans, navigationSpan{z: z, cell: cell, owner: -1, triangle: surface.triangle})
			g.cells[cell] = append(g.cells[cell], id)
		}
	}
	return nil
}

func (g *navigationGrid) seed(seeds []imath.Vector3, r *TerrainNavigation) []int {
	var queue []int
	for _, p := range seeds {
		x, y := int(math.Floor((p[0]-g.ox)/32)), int(math.Floor((p[1]-g.oy)/32))
		best := -1
		cost := math.Inf(1)
		if x >= 0 && x < g.width && y >= 0 && y < g.height {
			// Match the NPC's actual foot position first. Cell centers next to
			// walls may be blocked although the NPC can walk to the next cell.
			support := p
			supportFound := false
			diff := g.limits.SeedSnap
			for _, tri := range g.bins[y*g.width+x] {
				t := g.triangles[tri]
				if t.slope > math.Tan(g.limits.MaxSlopeDegrees*math.Pi/180) {
					continue
				}
				z, ok := NavigationHeight(t.v, p[0], p[1])
				if !ok || z < g.minZ || z > g.maxZ {
					continue
				}
				if d := math.Abs(z - p[2]); d <= diff {
					support[2] = z
					diff = d
					supportFound = true
				}
			}
			if supportFound {
				for yy := max(0, y-1); yy <= min(g.height-1, y+1); yy++ {
					for xx := max(0, x-1); xx <= min(g.width-1, x+1); xx++ {
						for _, id := range g.cells[yy*g.width+xx] {
							b := g.point(g.spans[id].cell, g.spans[id].z)
							delta := imath.V3Sub(b, support)
							distance := math.Hypot(delta[0], delta[1])
							score := imath.V3Dot(delta, delta)
							if score >= cost || distance > 48 || math.Abs(delta[2]) > g.limits.StepHeight+distance*math.Tan(g.limits.MaxSlopeDegrees*math.Pi/180) {
								continue
							}
							if g.traverse(support, b, 0) {
								best = id
								cost = score
							}
						}
					}
				}
			}
		}
		if best < 0 {
			r.RejectedNPCs++
			continue
		}
		r.GroundNPCs++
		r.seedPositions = append(r.seedPositions, p)
		if g.spans[best].owner >= 0 {
			g.seedCounts[g.spans[best].owner]++
			continue
		}
		owner := len(g.parents)
		g.parents = append(g.parents, owner)
		g.seedCounts = append(g.seedCounts, 1)
		g.spans[best].owner = owner
		queue = append(queue, best)
	}
	return queue
}

func (g *navigationGrid) root(i int) int {
	for g.parents[i] != i {
		g.parents[i] = g.parents[g.parents[i]]
		i = g.parents[i]
	}
	return i
}
func (g *navigationGrid) join(a, b int) {
	a, b = g.root(a), g.root(b)
	if a == b {
		return
	}
	if g.seedCounts[a] < g.seedCounts[b] {
		a, b = b, a
	}
	g.parents[b] = a
	g.seedCounts[a] += g.seedCounts[b]
}

func (g *navigationGrid) search(ctx context.Context, queue []int) error {
	started := time.Now()
	nextProgress := started.Add(5 * time.Second)
	directions := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}}
	for head := 0; head < len(queue); head++ {
		if head%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if now := time.Now(); !now.Before(nextProgress) {
				log.Printf("Terrain navigation: route search processed %d spans, %d pending, %d jump links, elapsed %s", head, len(queue)-head, len(g.jumps), now.Sub(started).Round(time.Millisecond))
				nextProgress = now.Add(5 * time.Second)
			}
		}
		from := queue[head]
		a := g.spans[from]
		x, y := a.cell%g.width, a.cell/g.width
		for _, d := range directions {
			maxSteps := max(1, int(g.limits.JumpDistance/(32*math.Hypot(float64(d[0]), float64(d[1])))))
			for step := 1; step <= maxSteps; step++ {
				nx, ny := x+d[0]*step, y+d[1]*step
				if nx < 0 || ny < 0 || nx >= g.width || ny >= g.height {
					break
				}
				connected := false
				for _, to := range g.cells[ny*g.width+nx] {
					b := g.spans[to]
					delta := b.z - a.z
					distance := 32 * float64(step) * math.Hypot(float64(d[0]), float64(d[1]))
					walk := step == 1 && math.Abs(delta) <= g.limits.StepHeight+distance*math.Tan(g.limits.MaxSlopeDegrees*math.Pi/180)
					jump := g.limits.JumpHeight > 0 && distance <= g.limits.JumpDistance && delta <= g.limits.JumpHeight && delta >= -g.limits.MaxDrop
					if !walk && !jump {
						continue
					}
					// Avoid repeated sweeps inside a component already visited from both sides.
					if b.owner >= 0 && g.root(a.owner) == g.root(b.owner) {
						connected = true
						continue
					}
					pa, pb := g.point(a.cell, a.z), g.point(b.cell, b.z)
					if !walk || !g.traverse(pa, pb, 0) {
						if !jump || !g.traverse(pa, pb, g.limits.JumpHeight) {
							continue
						}
					}
					connected = true
					if step > 1 {
						g.jumps = append(g.jumps, navigationJump{from: from, to: to})
					}
					if b.owner >= 0 {
						g.join(a.owner, b.owner)
					} else {
						g.spans[to].owner = a.owner
						g.spans[to].distance = a.distance + step
						queue = append(queue, to)
					}
				}
				// Longer jumps are needed at gaps/obstructions, not across ordinary ground.
				if step == 1 && connected {
					break
				}
			}
		}
	}
	return nil
}

func (g *navigationGrid) traverse(a, b imath.Vector3, jump float64) bool {
	d := imath.V3Sub(b, a)
	steps := max(2, int(math.Ceil(math.Hypot(d[0], d[1])/8)))
	offsets := [][2]float64{{0, 0}, {g.limits.Radius, 0}, {-g.limits.Radius, 0}, {0, g.limits.Radius}, {0, -g.limits.Radius}}
	points := make([]imath.Vector3, steps+1)
	for i := range points {
		t := float64(i) / float64(steps)
		p := imath.V3Sum(a, imath.V3Scale(d, t))
		if jump == 0 || i == 0 || i == steps {
			// Walking follows the actual floor, rather than a chord through
			// hills. Raise the feet to the highest support under the body radius
			// so uphill clearance probes cannot start inside a sloping road.
			base := math.Inf(-1)
			allowance := g.limits.StepHeight + (g.limits.Radius+8)*math.Tan(g.limits.MaxSlopeDegrees*math.Pi/180)
			for _, offset := range offsets {
				probe := p
				probe[0] += offset[0]
				probe[1] += offset[1]
				z, ok := g.supportHeight(probe, allowance)
				if !ok {
					return false
				}
				base = max(base, z)
			}
			p[2] = base
		}
		points[i] = p
		if jump == 0 && i > 0 {
			prev := points[i-1]
			if math.Abs(p[2]-prev[2]) > g.limits.StepHeight+math.Hypot(p[0]-prev[0], p[1]-prev[1])*math.Tan(g.limits.MaxSlopeDegrees*math.Pi/180) {
				return false
			}
		}
	}
	if jump > 0 {
		a, b = points[0], points[steps]
		d = imath.V3Sub(b, a)
		if d[2] > jump {
			return false
		}
		// Keep the apex within JumpHeight above takeoff at unequal landings.
		arc := (jump - d[2]/2 + math.Sqrt(jump*(jump-d[2]))) / 2
		for i := 1; i < steps; i++ {
			t := float64(i) / float64(steps)
			points[i] = imath.V3Sum(a, imath.V3Scale(d, t))
			points[i][2] += 4 * arc * t * (1 - t)
		}
	}
	for _, offset := range offsets {
		for i := 0; i <= steps; i++ {
			p := points[i]
			p[0] += offset[0]
			p[1] += offset[1]
			p[2] += .5
			top := p
			top[2] += g.limits.Height - .5
			if g.segmentBlocked(p, top) {
				return false
			}
		}
		for _, h := range []float64{.75, g.limits.Height * .5, g.limits.Height} {
			prev := points[0]
			prev[0] += offset[0]
			prev[1] += offset[1]
			prev[2] += h
			for i := 1; i <= steps; i++ {
				p := points[i]
				p[0] += offset[0]
				p[1] += offset[1]
				p[2] += h
				if g.segmentBlocked(prev, p) {
					return false
				}
				prev = p
			}
		}
	}
	return true
}

func (g *navigationGrid) supportHeight(p imath.Vector3, allowance float64) (float64, bool) {
	x, y := int(math.Floor((p[0]-g.ox)/32)), int(math.Floor((p[1]-g.oy)/32))
	if x < 0 || y < 0 || x >= g.width || y >= g.height {
		return 0, false
	}
	best, diff := 0.0, allowance
	found := false
	for _, id := range g.bins[y*g.width+x] {
		t := g.triangles[id]
		if t.slope > math.Tan(g.limits.MaxSlopeDegrees*math.Pi/180) {
			continue
		}
		z, ok := NavigationHeight(t.v, p[0], p[1])
		if !ok || z < g.minZ || z > g.maxZ {
			continue
		}
		if d := math.Abs(z - p[2]); d <= diff {
			best = z
			diff = d
			found = true
		}
	}
	return best, found
}

func (g *navigationGrid) segmentBlocked(a, b imath.Vector3) bool {
	x0, x1 := int(math.Floor((min(a[0], b[0])-g.ox)/32)), int(math.Floor((max(a[0], b[0])-g.ox)/32))
	y0, y1 := int(math.Floor((min(a[1], b[1])-g.oy)/32)), int(math.Floor((max(a[1], b[1])-g.oy)/32))
	for y := max(0, y0); y <= min(g.height-1, y1); y++ {
		for x := max(0, x0); x <= min(g.width-1, x1); x++ {
			for _, id := range g.bins[y*g.width+x] {
				if navigationIntersection(g.triangles[id].v, a, b) {
					return true
				}
			}
		}
	}
	return false
}

func (g *navigationGrid) projectJumps() {
	for _, link := range g.jumps {
		a, b := g.spans[link.from], g.spans[link.to]
		x0, y0, x1, y1 := a.cell%g.width, a.cell/g.width, b.cell%g.width, b.cell/g.width
		n := int(max(math.Abs(float64(x1-x0)), math.Abs(float64(y1-y0))))
		for i := 1; i < n; i++ {
			t := float64(i) / float64(n)
			cell := (y0+(y1-y0)*i/n)*g.width + x0 + (x1-x0)*i/n
			id := len(g.spans)
			g.spans = append(g.spans, navigationSpan{z: a.z + (b.z-a.z)*t, cell: cell, owner: a.owner, distance: a.distance + i, triangle: -1})
			g.cells[cell] = append(g.cells[cell], id)
		}
	}
}

func (g *navigationGrid) better(a, b int) bool {
	if b < 0 {
		return true
	}
	sa, sb := g.spans[a], g.spans[b]
	ca, cb := g.seedCounts[g.root(sa.owner)], g.seedCounts[g.root(sb.owner)]
	if ca != cb {
		return ca > cb
	}
	if sa.distance != sb.distance {
		return sa.distance < sb.distance
	}
	return sa.z < sb.z
}

func (g *navigationGrid) project(terrain data.Terrain, r *TerrainNavigation) {
	selected := make([]int, len(g.cells))
	for i := range selected {
		selected[i] = -1
	}
	for cell, ids := range g.cells {
		n := 0
		for _, id := range ids {
			if g.spans[id].owner < 0 {
				continue
			}
			n++
			if g.better(id, selected[cell]) {
				selected[cell] = id
			}
		}
		if n > 1 {
			r.Conflicts++
		}
	}
	// Fit corners to the nearest selected surface in the winning NPC component.
	// Evaluate its plane at the corner, rather than copying a cell-center height:
	// the latter makes even a planar road into a staircase with blocked stripes.
	for y := 0; y <= terrain.Map.Height; y++ {
		for x := 0; x <= terrain.Map.Width; x++ {
			best := -1
			distance := math.Inf(1)
			px, py := g.ox+float64(x)*128, g.oy+float64(y)*128
			for yy := max(0, y*4-4); yy <= min(g.height-1, y*4+3); yy++ {
				for xx := max(0, x*4-4); xx <= min(g.width-1, x*4+3); xx++ {
					if id := selected[yy*g.width+xx]; id >= 0 {
						p := g.point(g.spans[id].cell, g.spans[id].z)
						d := math.Hypot(p[0]-px, p[1]-py)
						count := g.seedCounts[g.root(g.spans[id].owner)]
						bestCount := 0
						if best >= 0 {
							bestCount = g.seedCounts[g.root(g.spans[best].owner)]
						}
						if best < 0 || count > bestCount || count == bestCount && (d < distance || d == distance && g.better(id, best)) {
							best, distance = id, d
						}
					}
				}
			}
			if best >= 0 {
				s := g.spans[best]
				z := s.z
				if s.triangle >= 0 {
					z = navigationPlaneHeight(g.triangles[s.triangle].v, px, py)
				}
				// Do not extrapolate a shore's selected plane across a lake.
				// Real collision floors (including bridges/pool beds) still fit.
				old := config.DataHeightToGameZ(float64(terrain.GroundHeight[y][x]))
				if _, _, wet := g.water.sample(px, py, old, false); wet && s.triangle >= 0 {
					if _, inside := NavigationHeight(g.triangles[s.triangle].v, px, py); !inside {
						continue
					}
				}
				terrain.GroundHeight[y][x] = config.GameZToDataHeight(max(g.minZ, min(g.maxZ, z)))
			}
		}
	}
	r.Pathing = make([]byte, len(g.cells))
	// Propagate the nearest reachable surface into blocked areas for air-height
	// classification, including the interiors of wide towers and mountain tops.
	nearest := make([]int, len(selected))
	copy(nearest, selected)
	queue := make([]int, 0, len(selected))
	for cell, id := range selected {
		if id >= 0 {
			r.TraversalReachable++
			queue = append(queue, cell)
		}
	}
	for head := 0; head < len(queue); head++ {
		cell := queue[head]
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			x, y := cell%g.width+d[0], cell/g.width+d[1]
			if x < 0 || y < 0 || x >= g.width || y >= g.height {
				continue
			}
			c := y*g.width + x
			if nearest[c] < 0 {
				nearest[c] = nearest[cell]
				queue = append(queue, c)
			}
		}
	}
	for cell, id := range selected {
		p := g.point(cell, 0)
		u, v := (p[0]-g.ox)/(float64(g.width)*32), (p[1]-g.oy)/(float64(g.height)*32)
		ground := config.DataHeightToGameZ(GetTerrainHeight(terrain, u, v))
		// A chosen surface must survive the 128-unit terrain projection. Otherwise
		// block it instead of opening an accidental connection between floors.
		if id >= 0 && math.Abs(g.spans[id].z-ground) <= max(g.limits.StepHeight, 24) {
			r.Reachable++
			continue
		}
		flags := byte(2 | 8) // ground and building pathing disabled; air stays open.
		if id >= 0 {
			r.ProjectionBlocked++
		}
		if nearest[cell] >= 0 {
			z := g.spans[nearest[cell]].z
			for _, tri := range g.bins[cell] {
				v := g.triangles[tri].v
				bottom, top := z+g.limits.AirBlockHeight, max(v[0][2], v[1][2], v[2][2])
				// Include vertical wall faces, which have no downward-ray hit.
				if top >= bottom && navigationTriangleBox(v, imath.Vector3{p[0], p[1], (bottom + top) / 2}, imath.Vector3{16.01, 16.01, max(.01, (top-bottom)/2)}) {
					flags |= 4
					break
				}
			}
		}
		r.Pathing[cell] = flags
		// Deep water gets its native swimming mask at final serialization,
		// rather than thousands of ground-blocker doodads over the lake bed.
		if h, bank, wet := g.water.sample(p[0], p[1], ground, false); wet && bank >= 0 && bank <= 1 && h-ground > waterDeepDepth && flags&4 == 0 {
			r.Pathing[cell] = 0
			if r.waterBlocked == nil {
				r.waterBlocked = make([]bool, len(selected))
			}
			r.waterBlocked[cell] = true
			r.WaterBlocked++
			continue
		}
		if flags&4 != 0 {
			r.AirBlocked++
		} else {
			r.GroundBlocked++
		}
	}
}

func (r *TerrainNavigation) Write(dir string, includePathing bool) error {
	if includePathing {
		b := make([]byte, 16+len(r.Pathing))
		copy(b, "MP3W")
		binary.LittleEndian.PutUint32(b[4:], 0)
		binary.LittleEndian.PutUint32(b[8:], uint32(r.Width))
		binary.LittleEndian.PutUint32(b[12:], uint32(r.Height))
		copy(b[16:], r.Pathing)
		if err := os.WriteFile(filepath.Join(dir, "war3map.wpm"), b, 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(filepath.Join(dir, "war3map.wpm")); err != nil && !os.IsNotExist(err) {
		return err
	}
	preview := image.NewNRGBA(image.Rect(0, 0, r.Width, r.Height))
	for i, flags := range r.Pathing {
		c := color.NRGBA{R: 52, G: 180, B: 90, A: 255}
		if flags&2 != 0 {
			c = color.NRGBA{R: 190, G: 90, B: 40, A: 255}
		}
		if flags&4 != 0 {
			c = color.NRGBA{R: 130, G: 60, B: 180, A: 255}
		}
		preview.SetNRGBA(i%r.Width, r.Height-1-i/r.Width, c)
	}
	f, err := os.Create(filepath.Join(dir, "terrain-navigation.png"))
	if err != nil {
		return err
	}
	encodeErr := png.Encode(f, preview)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "terrain-navigation.json"), encoded, 0o644)
}

func navigationNPCError(terrain data.Terrain, seeds []imath.Vector3) float64 {
	errors := make([]float64, 0, len(seeds))
	for _, p := range seeds {
		u := (p[0] - float64(terrain.Map.Offset.X)) / (float64(terrain.Map.Width) * 128)
		v := (p[1] - float64(terrain.Map.Offset.Y)) / (float64(terrain.Map.Height) * 128)
		errors = append(errors, math.Abs(p[2]-config.DataHeightToGameZ(GetTerrainHeight(terrain, u, v))))
	}
	if len(errors) == 0 {
		return 0
	}
	sort.Float64s(errors)
	return errors[len(errors)/2]
}

func NavigationCross(a, b imath.Vector3) imath.Vector3 {
	return imath.Vector3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func NavigationHeight(v [3]imath.Vector3, x, y float64) (float64, bool) {
	d := (v[1][1]-v[2][1])*(v[0][0]-v[2][0]) + (v[2][0]-v[1][0])*(v[0][1]-v[2][1])
	if math.Abs(d) < 1e-9 {
		return 0, false
	}
	a := ((v[1][1]-v[2][1])*(x-v[2][0]) + (v[2][0]-v[1][0])*(y-v[2][1])) / d
	b := ((v[2][1]-v[0][1])*(x-v[2][0]) + (v[0][0]-v[2][0])*(y-v[2][1])) / d
	return a*v[0][2] + b*v[1][2] + (1-a-b)*v[2][2], a >= -1e-7 && b >= -1e-7 && a+b <= 1+1e-7
}

func navigationPlaneHeight(v [3]imath.Vector3, x, y float64) float64 {
	n := NavigationCross(imath.V3Sub(v[1], v[0]), imath.V3Sub(v[2], v[0]))
	return v[0][2] - (n[0]*(x-v[0][0])+n[1]*(y-v[0][1]))/n[2]
}
func navigationIntersection(v [3]imath.Vector3, a, b imath.Vector3) bool {
	d := imath.V3Sub(b, a)
	e1, e2 := imath.V3Sub(v[1], v[0]), imath.V3Sub(v[2], v[0])
	h := NavigationCross(d, e2)
	det := imath.V3Dot(e1, h)
	if math.Abs(det) < 1e-9 {
		return false
	}
	s := imath.V3Sub(a, v[0])
	u := imath.V3Dot(s, h) / det
	if u < 0 || u > 1 {
		return false
	}
	q := NavigationCross(s, e1)
	vv := imath.V3Dot(d, q) / det
	if vv < 0 || u+vv > 1 {
		return false
	}
	t := imath.V3Dot(e2, q) / det
	return t > 1e-6 && t < 1-1e-6
}

func navigationTriangleBox(v [3]imath.Vector3, center, half imath.Vector3) bool {
	for i := range v {
		v[i] = imath.V3Sub(v[i], center)
	}
	edges := [3]imath.Vector3{imath.V3Sub(v[1], v[0]), imath.V3Sub(v[2], v[1]), imath.V3Sub(v[0], v[2])}
	axes := []imath.Vector3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, NavigationCross(edges[0], edges[1])}
	for _, edge := range edges {
		axes = append(axes, NavigationCross(edge, imath.Vector3{1, 0, 0}), NavigationCross(edge, imath.Vector3{0, 1, 0}), NavigationCross(edge, imath.Vector3{0, 0, 1}))
	}
	for _, axis := range axes {
		a, b, c := imath.V3Dot(v[0], axis), imath.V3Dot(v[1], axis), imath.V3Dot(v[2], axis)
		r := half[0]*math.Abs(axis[0]) + half[1]*math.Abs(axis[1]) + half[2]*math.Abs(axis[2])
		if min(a, b, c) > r || max(a, b, c) < -r {
			return false
		}
	}
	return true
}

func (wc3 *Wc3Converter) BlockModelCells(navigation *TerrainNavigation) {
	if navigation != nil {
		for y, row := range wc3.modelCells {
			for x, model := range row {
				if model {
					for j := 0; j < 4; j++ {
						for i := 0; i < 4; i++ {
							navigation.Pathing[(y*4+j)*navigation.Width+x*4+i] |= terrainFlagUnwalkable | terrainFlagUnbuildable
						}
					}
				}
			}
		}
	}
}

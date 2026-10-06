package directm2

const bakeChartOccupancyHeadChunk = 4096

// bakeChartOccupancy indexes the charts that already own each raster sample.
// Head chunks are allocated only for occupied raster regions, avoiding a
// full-grid allocation for sparse chart layouts.
type bakeChartOccupancy struct {
	sampleCount         int
	heads               []*[bakeChartOccupancyHeadChunk]uint32
	nodes               []bakeChartOccupant
	candidateHeads      []uint32
	candidateTails      []uint32
	candidateWriteNext  []bakeChartCandidate
	candidateGeneration []uint32
	generation          uint32
}

type bakeChartOccupant struct {
	chart int
	next  int // one-based node index; zero terminates the list
}

type bakeChartCandidate struct {
	write uint32
	next  uint32 // one-based node index; zero terminates the list
}

func newBakeChartOccupancy(width, height int) *bakeChartOccupancy {
	sampleCount := max(0, width*height)
	return &bakeChartOccupancy{
		sampleCount: sampleCount,
		heads:       make([]*[bakeChartOccupancyHeadChunk]uint32, (sampleCount+bakeChartOccupancyHeadChunk-1)/bakeChartOccupancyHeadChunk),
	}
}

func (o *bakeChartOccupancy) headAt(sample int) int {
	if sample < 0 || sample >= o.sampleCount {
		return 0
	}
	chunk := o.heads[sample/bakeChartOccupancyHeadChunk]
	if chunk == nil {
		return 0
	}
	return int(chunk[sample%bakeChartOccupancyHeadChunk])
}

func (o *bakeChartOccupancy) setHead(sample int, node uint32) {
	chunkIndex := sample / bakeChartOccupancyHeadChunk
	if o.heads[chunkIndex] == nil {
		o.heads[chunkIndex] = &[bakeChartOccupancyHeadChunk]uint32{}
	}
	o.heads[chunkIndex][sample%bakeChartOccupancyHeadChunk] = node
}

// beginFace starts a fresh candidate pass and grows the per-chart head/tail
// tables as charts are added. Generation tags avoid clearing them per face.
func (o *bakeChartOccupancy) beginFace(chartCount int) uint32 {
	if len(o.candidateHeads) < chartCount {
		missing := chartCount - len(o.candidateHeads)
		o.candidateHeads = append(o.candidateHeads, make([]uint32, missing)...)
		o.candidateTails = append(o.candidateTails, make([]uint32, missing)...)
		o.candidateGeneration = append(o.candidateGeneration, make([]uint32, missing)...)
	}
	if o.generation == ^uint32(0) {
		clear(o.candidateGeneration)
		o.generation = 0
	}
	o.generation++
	o.candidateWriteNext = o.candidateWriteNext[:0]
	return o.generation
}

// addCandidate groups one colliding face write under its chart. All linked
// records use one reusable contiguous per-face slice.
func (o *bakeChartOccupancy) addCandidate(chart, write int, generation uint32) {
	if chart < 0 || chart >= len(o.candidateHeads) {
		return
	}
	if o.candidateGeneration[chart] != generation {
		o.candidateGeneration[chart] = generation
		o.candidateHeads[chart], o.candidateTails[chart] = 0, 0
	}
	node := uint32(len(o.candidateWriteNext) + 1)
	o.candidateWriteNext = append(o.candidateWriteNext, bakeChartCandidate{write: uint32(write)})
	if tail := o.candidateTails[chart]; tail == 0 {
		o.candidateHeads[chart] = node
	} else {
		o.candidateWriteNext[tail-1].next = node
	}
	o.candidateTails[chart] = node
}

func (o *bakeChartOccupancy) candidateHead(chart int, generation uint32) uint32 {
	if chart < 0 || chart >= len(o.candidateHeads) || o.candidateGeneration[chart] != generation {
		return 0
	}
	return o.candidateHeads[chart]
}

// add records a chart only when it first claims a raster sample. Callers keep
// one occupancy node per (sample, chart) pair, even when later faces replace
// the sample's stored bakePixel.
func (o *bakeChartOccupancy) add(sample, chart int) {
	if sample < 0 || sample >= o.sampleCount {
		return
	}
	o.nodes = append(o.nodes, bakeChartOccupant{chart: chart, next: o.headAt(sample)})
	o.setHead(sample, uint32(len(o.nodes)))
}

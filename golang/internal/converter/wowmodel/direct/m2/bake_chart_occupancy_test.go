package directm2

import "testing"

func TestBakeChartOccupancyGroupsCollidingWritesDeterministically(t *testing.T) {
	occupancy := newBakeChartOccupancy(4, 2)
	occupancy.add(3, 1)
	occupancy.add(3, 0)
	occupancy.add(6, 2)

	generation := occupancy.beginFace(4)
	for _, pair := range [][2]int{{1, 0}, {0, 0}, {2, 1}, {0, 2}} {
		occupancy.addCandidate(pair[0], pair[1], generation)
	}

	if occupancy.candidateHead(3, generation) != 0 {
		t.Fatal("chart without an occupied write was given a candidate")
	}
	for chart, want := range map[int][]int{0: {0, 2}, 1: {0}, 2: {1}} {
		var got []int
		for node := occupancy.candidateHead(chart, generation); node != 0; node = occupancy.candidateWriteNext[node-1].next {
			got = append(got, int(occupancy.candidateWriteNext[node-1].write))
		}
		if len(got) != len(want) {
			t.Fatalf("chart %d has candidate writes %v, want %v", chart, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("chart %d candidate order is %v, want %v", chart, got, want)
			}
		}
	}
}

func TestBakeChartOccupancyResetsCandidatesAndGrowsChartTables(t *testing.T) {
	occupancy := newBakeChartOccupancy(1, 1)
	occupancy.generation = ^uint32(0)
	first := occupancy.beginFace(1)
	occupancy.addCandidate(0, 4, first)
	if got := occupancy.candidateHead(0, first); got != 1 {
		t.Fatalf("first candidate node should use one-based index 1, got %d", got)
	}

	second := occupancy.beginFace(3)
	occupancy.addCandidate(2, 7, second)
	if occupancy.candidateHead(0, second) != 0 || occupancy.candidateHead(2, second) != 1 {
		t.Fatal("candidate generation reset or chart-table growth failed")
	}
	if got := occupancy.candidateWriteNext[0].write; got != 7 {
		t.Fatalf("new face retained stale candidate records: got write %d", got)
	}
}

func TestBakeChartOccupancyAllocatesOnlyTouchedHeadChunks(t *testing.T) {
	occupancy := newBakeChartOccupancy(8192, 128)
	if len(occupancy.heads) != (8192*128+bakeChartOccupancyHeadChunk-1)/bakeChartOccupancyHeadChunk {
		t.Fatal("head index does not span the full sample grid")
	}
	occupancy.add(12, 0)
	occupancy.add(4095, 1)
	if occupancy.heads[0] == nil || occupancy.heads[1] != nil {
		t.Fatal("occupancy allocated a head chunk with no samples")
	}
	occupancy.add(4096, 2)
	if occupancy.heads[1] == nil || occupancy.headAt(8192) != 0 {
		t.Fatal("occupancy did not allocate the next chunk or rejected an out-of-range sample")
	}
}

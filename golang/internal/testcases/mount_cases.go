package testcases

// MountCase describes a rider + mount export parity case.
type MountCase struct {
	RiderBase  string
	MountPath  string
	MountScale float64
	SeatOffset []float64
	Size       string
}

var mountCases = []MountCase{
	{
		RiderBase: "https://www.wowhead.com/mop-classic/dressing-room?orc-warrior#fM80m0zN89c8u8VkZ8G8VRs8I8VRh8N8VRp8A8VRe877gyxZ808yx2808zRZm808ytM87VtVQ808ytz808tmw808yx1808tVb808tMM87MtLs87o",
		MountPath: "https://www.wowhead.com/mop-classic/npc=64986/heavenly-onyx-cloud-serpent",
		Size:      "hero",
	},
	{
		RiderBase:  "https://www.wowhead.com/dressing-room?orc-warrior#fM80m0zN89c8G8ol8u8oV8I8o28N8kq8A8kI8fo8M1r8rb8MKS8zYh8dAn8Mx4808rr8MPI8fk8M1A8rL8MQg8fV8M1o8fm8M1M8rf8MPj8rw8MPt8ri8MPE8L8kg877oyxZ808yx287cytM87VtVQ808ytz808tmw808yx1808tVb808tMM87MtLs87o",
		MountPath:  "https://www.wowhead.com/mop-classic/npc=64986/heavenly-onyx-cloud-serpent",
		MountScale: 1,
		SeatOffset: []float64{0, 0, 2.5},
		Size:       "hero",
	},
}

// MountCases returns rider+mount export regression cases.
func MountCases() []MountCase { return mountCases }

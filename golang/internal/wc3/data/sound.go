package data

// Sound is a war3map.w3s sound definition.
type Sound struct {
	Name       string
	File       string
	Effect     string
	Volume     int
	Pitch      int
	PitchVariance int
	Channel    int
	MinDistance float32
	MaxDistance float32
	DistanceCutoff float32
	ConeInside  float32
	ConeOutside float32
	ConeOutsideVolume int
	Priority    int
	Loop        bool
}

package data

// Rect is a region bounding rectangle.
type Rect struct {
	Left   float32
	Bottom float32
	Right  float32
	Top    float32
}

// Region is a war3map.w3r trigger region.
type Region struct {
	Position      Rect
	Name          string
	ID            int32
	WeatherEffect string
	AmbientSound  string
	Color         [3]byte
}

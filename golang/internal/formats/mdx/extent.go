package mdx

// Extent holds bounding sphere data.
type Extent struct {
	BoundsRadius float32
	Min          []float32
	Max          []float32
}

func NewExtent() *Extent {
	return &Extent{
		Min: make([]float32, 3),
		Max: make([]float32, 3),
	}
}

func (e *Extent) ReadMdx(stream *BinaryStream) error {
	r, err := stream.ReadFloat32()
	if err != nil {
		return err
	}
	e.BoundsRadius = r
	e.Min, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	e.Max, err = stream.ReadFloat32Array(3)
	return err
}

func (e *Extent) WriteMdx(stream *BinaryStream) {
	stream.WriteFloat32(e.BoundsRadius)
	stream.WriteFloat32Array(e.Min)
	stream.WriteFloat32Array(e.Max)
}

func (e *Extent) WriteMdl(stream *TokenStream) {
	if e.Min[0] != 0 || e.Min[1] != 0 || e.Min[2] != 0 {
		stream.WriteVectorAttrib("MinimumExtent", e.Min)
	}
	if e.Max[0] != 0 || e.Max[1] != 0 || e.Max[2] != 0 {
		stream.WriteVectorAttrib("MaximumExtent", e.Max)
	}
	if e.BoundsRadius != 0 {
		stream.WriteNumberAttrib("BoundsRadius", float64(e.BoundsRadius))
	}
}

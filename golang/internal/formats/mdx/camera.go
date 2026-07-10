package mdx

// Camera is a model camera.
type Camera struct {
	AnimatedObject
	Name              string
	Position          []float32
	FieldOfView       float32
	FarClippingPlane  float32
	NearClippingPlane float32
	TargetPosition    []float32
}

func NewCamera() *Camera {
	return &Camera{
		Position:       make([]float32, 3),
		TargetPosition: make([]float32, 3),
	}
}

func (c *Camera) ReadMdx(stream *BinaryStream, _ int) error {
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	c.Name, err = stream.Read(80)
	if err != nil {
		return err
	}
	c.Position, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	c.FieldOfView, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	c.FarClippingPlane, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	c.NearClippingPlane, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	c.TargetPosition, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	return c.ReadAnimations(stream, int(size)-120)
}

func (c *Camera) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(c.GetByteLength(0)))
	written := stream.Write(c.Name)
	stream.Skip(80 - written)
	stream.WriteFloat32Array(c.Position)
	stream.WriteFloat32(c.FieldOfView)
	stream.WriteFloat32(c.FarClippingPlane)
	stream.WriteFloat32(c.NearClippingPlane)
	stream.WriteFloat32Array(c.TargetPosition)
	c.WriteAnimations(stream)
}

func (c *Camera) ReadMdl(stream *TokenStream) error {
	name, err := stream.Read()
	if err != nil {
		return err
	}
	c.Name = name
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "Position":
			_, err := stream.ReadVector(c.Position)
			return err
		case "Translation":
			return c.ReadAnimation(stream, "KCTR")
		case "Rotation":
			return c.ReadAnimation(stream, "KCRL")
		case "FieldOfView":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			c.FieldOfView = float32(v)
			return nil
		case "FarClip":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			c.FarClippingPlane = float32(v)
			return nil
		case "NearClip":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			c.NearClippingPlane = float32(v)
			return nil
		case "Target":
			return stream.ReadBlockIter(func(t string) error {
				switch t {
				case "Position":
					_, err := stream.ReadVector(c.TargetPosition)
					return err
				case "Translation":
					return c.ReadAnimation(stream, "KTTR")
				default:
					return unknownToken("Camera "+c.Name+"'s Target", t)
				}
			})
		default:
			return unknownToken("Camera "+c.Name, token)
		}
	})
}

func (c *Camera) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("Camera", c.Name)
	stream.WriteVectorAttrib("Position", c.Position)
	c.WriteAnimation(stream, "KCTR")
	c.WriteAnimation(stream, "KCRL")
	stream.WriteNumberAttrib("FieldOfView", float64(c.FieldOfView))
	stream.WriteNumberAttrib("FarClip", float64(c.FarClippingPlane))
	stream.WriteNumberAttrib("NearClip", float64(c.NearClippingPlane))
	stream.StartBlock("Target")
	stream.WriteVectorAttrib("Position", c.TargetPosition)
	c.WriteAnimation(stream, "KTTR")
	stream.EndBlock()
	stream.EndBlock()
}

func (c *Camera) GetByteLength(_ int) int {
	return 120 + c.AnimatedObject.GetByteLength(0)
}

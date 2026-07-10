package mdx

// CollisionShapeType for collision shape types.
type CollisionShapeType uint32

const (
	CollisionShapeBox      CollisionShapeType = 0
	CollisionShapePlane    CollisionShapeType = 1
	CollisionShapeSphere   CollisionShapeType = 2
	CollisionShapeCylinder CollisionShapeType = 3
)

// CollisionShape is a collision volume.
type CollisionShape struct {
	*GenericObject
	Type         CollisionShapeType
	Vertices     [2][]float32
	BoundsRadius float32
}

func NewCollisionShape() *CollisionShape {
	return &CollisionShape{
		GenericObject: NewGenericObject(0x2000),
		Vertices:      [2][]float32{make([]float32, 3), make([]float32, 3)},
	}
}

func (c *CollisionShape) ReadMdx(stream *BinaryStream, _ int) error {
	if err := c.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	typ, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	c.Type = CollisionShapeType(typ)
	c.Vertices[0], err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	if c.Type != CollisionShapeSphere {
		c.Vertices[1], err = stream.ReadFloat32Array(3)
		if err != nil {
			return err
		}
	}
	if c.Type == CollisionShapeSphere || c.Type == CollisionShapeCylinder {
		c.BoundsRadius, err = stream.ReadFloat32()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CollisionShape) WriteMdx(stream *BinaryStream, _ int) {
	c.GenericObject.WriteMdx(stream)
	stream.WriteUint32(uint32(c.Type))
	stream.WriteFloat32Array(c.Vertices[0])
	if c.Type != CollisionShapeSphere {
		stream.WriteFloat32Array(c.Vertices[1])
	}
	if c.Type == CollisionShapeSphere || c.Type == CollisionShapeCylinder {
		stream.WriteFloat32(c.BoundsRadius)
	}
}

func (c *CollisionShape) ReadMdl(stream *TokenStream) error {
	return c.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "Box":
			c.Type = CollisionShapeBox
		case "Plane":
			c.Type = CollisionShapePlane
		case "Sphere":
			c.Type = CollisionShapeSphere
		case "Cylinder":
			c.Type = CollisionShapeCylinder
		case "Vertices":
			count, err := stream.ReadInt()
			if err != nil {
				return err
			}
			if _, err := stream.Read(); err != nil { // {
				return err
			}
			if _, err := stream.ReadVector(c.Vertices[0]); err != nil {
				return err
			}
			if count == 2 {
				if _, err := stream.ReadVector(c.Vertices[1]); err != nil {
					return err
				}
			}
			_, err = stream.Read() // }
			return err
		case "BoundsRadius":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			c.BoundsRadius = float32(v)
		default:
			return unknownToken("CollisionShape", token)
		}
		return nil
	})
}

func (c *CollisionShape) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("CollisionShape", c.Name)
	c.WriteGenericHeader(stream)

	typeName := ""
	vertices := 2
	boundsRadius := false
	switch c.Type {
	case CollisionShapeBox:
		typeName = "Box"
	case CollisionShapePlane:
		typeName = "Plane"
	case CollisionShapeSphere:
		typeName = "Sphere"
		vertices = 1
		boundsRadius = true
	case CollisionShapeCylinder:
		typeName = "Cylinder"
		boundsRadius = true
	}

	stream.WriteFlag(typeName)
	stream.StartBlock("Vertices", vertices)
	stream.WriteVector(c.Vertices[0])
	if vertices == 2 {
		stream.WriteVector(c.Vertices[1])
	}
	stream.EndBlock()
	if boundsRadius {
		stream.WriteNumberAttrib("BoundsRadius", float64(c.BoundsRadius))
	}
	c.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (c *CollisionShape) GetByteLength(_ int) int {
	size := 16 + c.GenericObject.GetByteLength()
	if c.Type != CollisionShapeSphere {
		size += 12
	}
	if c.Type == CollisionShapeSphere || c.Type == CollisionShapeCylinder {
		size += 4
	}
	return size
}

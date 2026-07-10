package mdx

import "fmt"

// AnimatedObject is the parent for objects with animated data.
type AnimatedObject struct {
	Animations []*Animation
}

func (o *AnimatedObject) ReadAnimations(stream *BinaryStream, size int) error {
	end := stream.Index() + size
	for stream.Index() < end {
		name, err := stream.ReadBinary(4)
		if err != nil {
			return err
		}
		anim, _, err := newAnimationForTag(name)
		if err != nil {
			return err
		}
		if err := anim.ReadMdx(stream, name); err != nil {
			return err
		}
		o.Animations = append(o.Animations, anim)
	}
	return nil
}

func (o *AnimatedObject) WriteAnimations(stream *BinaryStream) {
	for _, anim := range o.Animations {
		anim.WriteMdx(stream)
	}
}

// ReadAnimatedBlock reads block keys, merging "static" prefix tokens.
func (o *AnimatedObject) ReadAnimatedBlock(stream *TokenStream, fn func(token string) error) error {
	return stream.ReadBlockIter(func(token string) error {
		if token == "static" {
			next, err := stream.Read()
			if err != nil {
				return err
			}
			return fn("static " + next)
		}
		return fn(token)
	})
}

func (o *AnimatedObject) ReadAnimation(stream *TokenStream, name string) error {
	anim, _, err := newAnimationForTag(name)
	if err != nil {
		return err
	}
	if err := anim.ReadMdl(stream, name); err != nil {
		return err
	}
	o.Animations = append(o.Animations, anim)
	return nil
}

func (o *AnimatedObject) WriteAnimation(stream *TokenStream, name string) bool {
	for _, anim := range o.Animations {
		if anim.Name == name {
			entry := animationMap[name]
			anim.WriteMdl(stream, entry.mdlName)
			return true
		}
	}
	return false
}

func (o *AnimatedObject) GetByteLength(_ int) int {
	size := 0
	for _, anim := range o.Animations {
		size += anim.GetByteLength()
	}
	return size
}

func isGenericAnimation(name string) bool {
	return name == "KGTR" || name == "KGRT" || name == "KGSC"
}

func (o *AnimatedObject) eachAnimation(wantGeneric bool) []*Animation {
	var out []*Animation
	for _, anim := range o.Animations {
		isGeneric := isGenericAnimation(anim.Name)
		if (wantGeneric && isGeneric) || (!wantGeneric && !isGeneric) {
			out = append(out, anim)
		}
	}
	return out
}

// GenericObjectFlags for generic object flags.
type GenericObjectFlags uint32

const (
	GenericFlagNone                    GenericObjectFlags = 0x0
	GenericFlagDontInheritTranslation  GenericObjectFlags = 0x1
	GenericFlagDontInheritScaling      GenericObjectFlags = 0x2
	GenericFlagDontInheritRotation       GenericObjectFlags = 0x4
	GenericFlagBillboarded             GenericObjectFlags = 0x8
	GenericFlagBillboardedLockX        GenericObjectFlags = 0x10
	GenericFlagBillboardedLockY        GenericObjectFlags = 0x20
	GenericFlagBillboardedLockZ        GenericObjectFlags = 0x40
	GenericFlagCameraAnchored          GenericObjectFlags = 0x80
)

// GenericObject is the parent for world objects with spatial animations.
type GenericObject struct {
	AnimatedObject
	Name     string
	ObjectID int32
	ParentID int32
	Flags    GenericObjectFlags
}

func NewGenericObject(flags GenericObjectFlags) *GenericObject {
	return &GenericObject{Flags: flags, ObjectID: -1, ParentID: -1}
}

func (o *GenericObject) ReadMdx(stream *BinaryStream) error {
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	name, err := stream.Read(80)
	if err != nil {
		return err
	}
	o.Name = name
	objectID, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	o.ObjectID = objectID
	parentID, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	o.ParentID = parentID
	flags, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	o.Flags = GenericObjectFlags(flags)
	return o.ReadAnimations(stream, int(size)-96)
}

func (o *GenericObject) WriteMdx(stream *BinaryStream) {
	stream.WriteUint32(uint32(o.GetGenericByteLength()))
	written := stream.Write(o.Name)
	stream.Skip(80 - written)
	stream.WriteInt32(o.ObjectID)
	stream.WriteInt32(o.ParentID)
	stream.WriteUint32(uint32(o.Flags))
	for _, anim := range o.eachAnimation(true) {
		anim.WriteMdx(stream)
	}
}

func (o *GenericObject) WriteNonGenericAnimationChunks(stream *BinaryStream) {
	for _, anim := range o.eachAnimation(false) {
		anim.WriteMdx(stream)
	}
}

func (o *GenericObject) ReadGenericBlock(stream *TokenStream, fn func(token string) error) error {
	name, err := stream.Read()
	if err != nil {
		return err
	}
	o.Name = name
	return o.ReadAnimatedBlock(stream, func(token string) error {
		switch token {
		case "ObjectId":
			id, err := stream.ReadInt()
			if err != nil {
				return err
			}
			o.ObjectID = int32(id)
		case "Parent":
			id, err := stream.ReadInt()
			if err != nil {
				return err
			}
			o.ParentID = int32(id)
		case "BillboardedLockZ":
			o.Flags |= GenericFlagBillboardedLockZ
		case "BillboardedLockY":
			o.Flags |= GenericFlagBillboardedLockY
		case "BillboardedLockX":
			o.Flags |= GenericFlagBillboardedLockX
		case "Billboarded":
			o.Flags |= GenericFlagBillboarded
		case "CameraAnchored":
			o.Flags |= GenericFlagCameraAnchored
		case "DontInherit":
			return stream.ReadBlockIter(func(t string) error {
				switch t {
				case "Rotation":
					o.Flags |= GenericFlagDontInheritRotation
				case "Translation":
					o.Flags |= GenericFlagDontInheritTranslation
				case "Scaling":
					o.Flags |= GenericFlagDontInheritScaling
				}
				return nil
			})
		case "Translation":
			return o.ReadAnimation(stream, "KGTR")
		case "Rotation":
			return o.ReadAnimation(stream, "KGRT")
		case "Scaling":
			return o.ReadAnimation(stream, "KGSC")
		default:
			return fn(token)
		}
		return nil
	})
}

func (o *GenericObject) WriteGenericHeader(stream *TokenStream) {
	stream.WriteNumberAttrib("ObjectId", float64(o.ObjectID))
	if o.ParentID != -1 {
		stream.WriteNumberAttrib("Parent", float64(o.ParentID))
	}
	if o.Flags&GenericFlagBillboardedLockZ != 0 {
		stream.WriteFlag("BillboardedLockZ")
	}
	if o.Flags&GenericFlagBillboardedLockY != 0 {
		stream.WriteFlag("BillboardedLockY")
	}
	if o.Flags&GenericFlagBillboardedLockX != 0 {
		stream.WriteFlag("BillboardedLockX")
	}
	if o.Flags&GenericFlagBillboarded != 0 {
		stream.WriteFlag("Billboarded")
	}
	if o.Flags&GenericFlagCameraAnchored != 0 {
		stream.WriteFlag("CameraAnchored")
	}
	if o.Flags&GenericFlagDontInheritRotation != 0 {
		stream.WriteFlag("DontInherit { Rotation }")
	}
	if o.Flags&GenericFlagDontInheritTranslation != 0 {
		stream.WriteFlag("DontInherit { Translation }")
	}
	if o.Flags&GenericFlagDontInheritScaling != 0 {
		stream.WriteFlag("DontInherit { Scaling }")
	}
}

func (o *GenericObject) WriteGenericAnimations(stream *TokenStream) {
	o.WriteAnimation(stream, "KGTR")
	o.WriteAnimation(stream, "KGRT")
	o.WriteAnimation(stream, "KGSC")
}

func (o *GenericObject) GetGenericByteLength() int {
	size := 96
	for _, anim := range o.eachAnimation(true) {
		size += anim.GetByteLength()
	}
	return size
}

func (o *GenericObject) GetByteLength() int {
	return 96 + o.AnimatedObject.GetByteLength(0)
}

func unknownToken(typ, token string) error {
	return fmt.Errorf("Unknown token in %s: %q", typ, token)
}

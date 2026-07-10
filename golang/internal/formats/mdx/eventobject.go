package mdx

import "strconv"

// EventObject is an animation event track object.
type EventObject struct {
	*GenericObject
	GlobalSequenceID int32
	Tracks           []uint32
}

func NewEventObject() *EventObject {
	return &EventObject{
		GenericObject:    NewGenericObject(0x400),
		GlobalSequenceID: -1,
	}
}

func (e *EventObject) ReadMdx(stream *BinaryStream, _ int) error {
	if err := e.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // KEVT
		return err
	}
	count, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	gs, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	e.GlobalSequenceID = gs
	e.Tracks, err = stream.ReadUint32Array(int(count))
	return err
}

func (e *EventObject) WriteMdx(stream *BinaryStream, _ int) {
	e.GenericObject.WriteMdx(stream)
	stream.WriteBinary("KEVT")
	stream.WriteUint32(uint32(len(e.Tracks)))
	stream.WriteInt32(e.GlobalSequenceID)
	stream.WriteUint32Array(e.Tracks)
}

func (e *EventObject) ReadMdl(stream *TokenStream) error {
	return e.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "EventTrack":
			count, err := stream.ReadInt()
			if err != nil {
				return err
			}
			e.Tracks = make([]uint32, count)
			if _, err := stream.Read(); err != nil { // {
				return err
			}
			if peek, _ := stream.Peek(); peek == "GlobalSeqId" {
				if _, err := stream.Read(); err != nil {
					return err
				}
				gs, err := stream.ReadInt()
				if err != nil {
					return err
				}
				e.GlobalSequenceID = int32(gs)
			}
			for i := 0; i < count; i++ {
				v, err := stream.ReadInt()
				if err != nil {
					return err
				}
				e.Tracks[i] = uint32(v)
			}
			_, err = stream.Read() // }
			return err
		default:
			return unknownToken("EventObject", token)
		}
	})
}

func (e *EventObject) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("EventObject", e.Name)
	e.WriteGenericHeader(stream)
	stream.StartBlock("EventTrack", len(e.Tracks))
	if e.GlobalSequenceID != -1 {
		stream.WriteNumberAttrib("GlobalSeqId", float64(e.GlobalSequenceID))
	}
	for _, track := range e.Tracks {
		stream.WriteFlag(strconv.Itoa(int(track)))
	}
	stream.EndBlock()
	e.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (e *EventObject) GetByteLength(_ int) int {
	return 12 + len(e.Tracks)*4 + e.GenericObject.GetByteLength()
}

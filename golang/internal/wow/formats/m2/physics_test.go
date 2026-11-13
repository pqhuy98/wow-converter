package m2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestParsePhysicsRigAndRejectInvalidReferences(t *testing.T) {
	makeData := func(bodyBone uint16, shapeCount uint32, gravity float32) []byte {
		var out bytes.Buffer
		chunk := func(tag string, data []byte) {
			out.WriteString(string([]byte{tag[3], tag[2], tag[1], tag[0]}))
			_ = binary.Write(&out, binary.LittleEndian, uint32(len(data)))
			out.Write(data)
		}
		chunk("PHYS", []byte{6, 0})
		caps := make([]byte, 28)
		binary.LittleEndian.PutUint32(caps[24:], math.Float32bits(.02))
		chunk("CAPS", caps)
		shape := make([]byte, 32)
		binary.LittleEndian.PutUint16(shape, 1)
		chunk("SHP2", shape)
		bodies := make([]byte, 96)
		binary.LittleEndian.PutUint16(bodies[48:], 1)
		binary.LittleEndian.PutUint16(bodies[50:], bodyBone)
		binary.LittleEndian.PutUint32(bodies[68:], shapeCount)
		binary.LittleEndian.PutUint32(bodies[72:], math.Float32bits(gravity))
		chunk("BDY4", bodies)
		joint := make([]byte, 124)
		for _, o := range []int{0, 16, 32, 48, 64, 80} {
			binary.LittleEndian.PutUint32(joint[o:], math.Float32bits(1))
		}
		binary.LittleEndian.PutUint32(joint[104:], math.Float32bits(70))
		chunk("SHJ2", joint)
		join := make([]byte, 16)
		binary.LittleEndian.PutUint32(join[4:], 1)
		binary.LittleEndian.PutUint16(join[12:], 1)
		chunk("JOIN", join)
		return out.Bytes()
	}
	data := makeData(87, 1, 1.5)
	p, err := parsePhysics(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Bodies) != 2 || p.Bodies[1].Bone != 87 || p.Bodies[1].Gravity != 1.5 || len(p.Bodies[1].Shapes) != 1 || len(p.Joints) != 1 || p.Joints[0].Cone != 70 {
		t.Fatalf("lost source physics: %+v", p)
	}
	for _, bad := range [][]byte{data[:len(data)-1], makeData(87, 999, 1), makeData(87, 1, float32(math.NaN()))} {
		if _, err := parsePhysics(bad); err == nil {
			t.Fatal("invalid physics accepted")
		}
	}
	future := append([]byte(nil), data...)
	future[8] = 7
	if _, err := parsePhysics(future); !errors.Is(err, ErrUnsupportedPhysics) {
		t.Fatalf("future format not identified: %v", err)
	}
}

func TestPhysicsExternalFileAndMetadataOnlyLoader(t *testing.T) {
	// Metadata readers intentionally have no CASC callback. PFID must not make
	// those readers fetch dependencies or dereference a nil callback.
	payload := make([]byte, 512)
	copy(payload, "MD20")
	binary.LittleEndian.PutUint32(payload[4:], 274)
	var file bytes.Buffer
	file.WriteString("MD21")
	_ = binary.Write(&file, binary.LittleEndian, uint32(len(payload)))
	file.Write(payload)
	file.WriteString("PFID")
	_ = binary.Write(&file, binary.LittleEndian, uint32(4))
	_ = binary.Write(&file, binary.LittleEndian, uint32(17))
	if err := NewLoader(buffer.From(file.Bytes()), nil).Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	called := false
	l := NewLoader(buffer.From(file.Bytes()), func(ctx context.Context, id uint32) ([]byte, error) {
		called = true
		if id != 17 {
			t.Fatalf("wrong PFID %d", id)
		}
		return []byte{'S', 'Y', 'H', 'P', 2, 0, 0, 0, 6, 0}, nil
	})
	if err := l.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called || l.Physics == nil {
		t.Fatal("external physics not loaded")
	}
}

func TestPhysicsLoaderRejectsTruncatedChunks(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("PFI"),
		{'P', 'F', 'I', 'D', 0, 0, 0, 0},
		{'P', 'F', 'I', 'D', 1, 0, 0, 0, 17},
		{'P', 'F', 'I', 'D', 4, 0, 0, 0, 17},
		{'P', 'F', 'D', 'C', 255, 255, 255, 255},
	} {
		if err := NewLoader(buffer.From(data), nil).Load(context.Background()); err == nil {
			t.Fatalf("invalid chunk accepted: %v", data)
		}
	}
}

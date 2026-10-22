package creature

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

func TestCreatureDeathSoundLookup(t *testing.T) {
	ctx := context.Background()
	first, err := firstDeathSoundFiles(ctx, map[uint32]db.DB2Row{
		9: {"SoundKitID": int64(5), "FileDataID": int64(10)},
		2: {"SoundKitID": int64(5), "FileDataID": int64(90)},
		1: {"SoundKitID": int64(5), "FileDataID": int64(0)},
		3: {"SoundKitID": int64(6), "FileDataID": int64(80)},
	})
	if err != nil || first[5] != 90 {
		t.Fatalf("first entry, not smallest file: %v %v", first, err)
	}
	models := map[uint32]db.DB2Row{7: {"SoundID": int64(12)}}
	sounds := map[uint32]db.DB2Row{11: {"SoundDeathID": int64(5)}, 12: {"SoundDeathID": int64(6)}, 13: {"SoundDeathID": int64(0)}}
	model := func(id uint32) db.DB2Row { return models[id] }
	sound := func(id uint32) db.DB2Row { return sounds[id] }
	for _, tc := range []struct {
		name    string
		display db.DB2Row
		want    int
	}{
		{"display override", db.DB2Row{"ModelID": int64(7), "SoundID": int64(11)}, 90},
		{"model fallback", db.DB2Row{"ModelID": int64(7)}, 80},
		{"absent override record", db.DB2Row{"ModelID": int64(7), "SoundID": int64(99)}, 80},
		{"explicit silence", db.DB2Row{"ModelID": int64(7), "SoundID": int64(13)}, 0},
		{"missing display", nil, 0},
	} {
		if got := creatureDeathSoundFile(tc.display, model, sound, first); got != tc.want {
			t.Fatalf("%s: %d want %d", tc.name, got, tc.want)
		}
	}
}

func deathTestWAV(channels int, samples []int16) []byte {
	out := append([]byte("RIFF"), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(out[4:], uint32(36+2*len(samples)))
	out = append(out, []byte("WAVEfmt ")...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	for _, v := range []uint16{1, uint16(channels)} {
		out = binary.LittleEndian.AppendUint16(out, v)
	}
	out = binary.LittleEndian.AppendUint32(out, 22050)
	out = binary.LittleEndian.AppendUint32(out, uint32(22050*channels*2))
	for _, v := range []uint16{uint16(channels * 2), 16} {
		out = binary.LittleEndian.AppendUint16(out, v)
	}
	out = append(out, []byte("data")...)
	out = binary.LittleEndian.AppendUint32(out, uint32(2*len(samples)))
	for _, v := range samples {
		out = binary.LittleEndian.AppendUint16(out, uint16(v))
	}
	return out
}

func TestDeathSoundMonoCodecs(t *testing.T) {
	wav, err := deathSoundWAV(context.Background(), deathTestWAV(2, []int16{1000, 3000, -4000, 2000}))
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(wav[22:]) != 1 || int16(binary.LittleEndian.Uint16(wav[44:])) != 2000 || int16(binary.LittleEndian.Uint16(wav[46:])) != -1000 {
		t.Fatal("incorrect mono downmix")
	}
	for _, extension := range []string{"ogg", "mp3"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "death-tone."+extension))
		if err != nil {
			t.Fatal(err)
		}
		wav, err := deathSoundWAV(context.Background(), raw)
		if err != nil {
			t.Fatalf("%s: %v", extension, err)
		}
		if len(wav) <= 44 || string(wav[8:12]) != "WAVE" || binary.LittleEndian.Uint16(wav[22:]) != 1 || binary.LittleEndian.Uint16(wav[34:]) != 16 {
			t.Fatalf("invalid converted %s", extension)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := deathSoundWAV(ctx, deathTestWAV(1, []int16{10})); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if _, err := deathSoundWAV(context.Background(), []byte("RIFFbroken")); err == nil {
		t.Fatal("truncated audio accepted")
	}
}

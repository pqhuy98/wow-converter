package common

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type unavailableGameObjectClient struct{ client.Client }

func (unavailableGameObjectClient) DownloadCascFile(context.Context, int) ([]byte, error) {
	return nil, errors.New("model file unavailable")
}

func TestGameObjectModelSkipsMissingDataAndPreservesCancellation(t *testing.T) {
	a := NewAssetManager(config.DefaultConfig(), unavailableGameObjectClient{}, nil)
	g := azerothcore.GameObject{GUID: 268620, Name: "Runeforge", DisplayID: 1287}
	for _, fid := range []int{0, 123} {
		model, err := a.gameObjectModel(context.Background(), g, fid)
		if err != nil || model != nil {
			t.Fatalf("unavailable model should skip: %v %v", model, err)
		}
	}
	valid := &Model{RelativePath: "valid.mdx"}
	a.models["gameobjects/456"] = valid
	if model, err := a.gameObjectModel(context.Background(), g, 456); err != nil || model != valid {
		t.Fatal("valid model was skipped")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.gameObjectModel(ctx, g, 0); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation was swallowed")
	}
}

func TestGameObjectTransformUsesServerBasisAndQuaternion(t *testing.T) {
	parent := &WowObject{Position: [3]float64{-500, -800, 120}, Rotation: [3]float64{0, 0, -math.Pi / 2}}
	g := azerothcore.GameObject{Position: [3]float64{4357.06, 3071.33, 354.362}, Orientation: 1.57, Rotation: [4]float64{0, 0, 2, 0}, UseQuaternion: true}
	pos, rot := gameObjectTransform(g, parent, 2)
	absolute := imath.V3Sum(parent.Position, imath.V3Rotate(pos, parent.Rotation))
	want := [3]float64{-8714.12, -6142.66, 708.724}
	for i := range want {
		if math.Abs(absolute[i]-want[i]) > 1e-8 {
			t.Fatalf("position = %v, want %v", absolute, want)
		}
	}
	absoluteRotation := imath.CalculateChildAbsoluteEulerRotation(parent.Rotation, rot)
	if math.Abs(absoluteRotation[2]) > 1e-8 {
		t.Fatalf("quaternion ignored: rotation = %v", absoluteRotation)
	}
	g.UseQuaternion = false
	_, rot = gameObjectTransform(g, parent, 2)
	absoluteRotation = imath.CalculateChildAbsoluteEulerRotation(parent.Rotation, rot)
	if math.Abs(math.Sin(absoluteRotation[2])-math.Sin(math.Pi+g.Orientation)) > 1e-8 {
		t.Fatal("AzerothCore orientation override lost")
	}
}

func TestGameObjectDBCModelsAndInvalidOffsets(t *testing.T) {
	names := []byte("\x00World\\IceCrown\\OrangeTubes.mdx\x00")
	raw := make([]byte, 28+len(names))
	copy(raw, "WDBC")
	for offset, value := range map[int]uint32{4: 1, 8: 2, 12: 8, 16: uint32(len(names)), 20: 9203, 24: 1} {
		binary.LittleEndian.PutUint32(raw[offset:], value)
	}
	copy(raw[28:], names)
	models, err := gameObjectDBCModels(raw)
	if err != nil {
		t.Fatal(err)
	}
	if models[9203] != "World\\IceCrown\\OrangeTubes.mdx" {
		t.Fatalf("wrong mapping: %v", models)
	}
	binary.LittleEndian.PutUint32(raw[24:], uint32(len(names)))
	if _, err := gameObjectDBCModels(raw); err == nil {
		t.Fatal("accepted out-of-range string offset")
	}
	if _, err := gameObjectDBCModels(raw[:19]); err == nil {
		t.Fatal("accepted truncated header")
	}
}

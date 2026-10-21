package character

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestExportHeadFreezeOptionAndAlwaysOnKeyReduction(t *testing.T) {
	for _, freeze := range []bool{false, true} {
		m := mdl.New(mdl.NewMDLOptions{})
		m.Sequences = []components.Sequence{{Name: "Mount", Interval: [2]int{0, 100}}, {Name: "Sleep", Interval: [2]int{200, 300}}}
		head, jaw := components.NewBone("Head"), components.NewBone("Jaw")
		jaw.Parent = head
		for _, b := range []*components.Bone{head, jaw} {
			b.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{
				0: imath.Vector3{}, 50: imath.Vector3{1, 0, 0}, 100: imath.Vector3{2, 0, 0},
				200: imath.Vector3{}, 250: imath.Vector3{1, 0, 0}, 300: imath.Vector3{2, 0, 0},
			}}
		}
		m.Bones = []*components.Bone{head, jaw}
		e := &CharacterExporter{Models: [][2]interface{}{{m, "fixture"}}}
		e.OptimizeModels(ExportOptimization{FreezeHead: freeze})
		if len(head.Translation.KeyFrames) != 4 {
			t.Fatal("conservative reduction should run with all optional flags off")
		}
		want := imath.Vector3{2, 0, 0}
		if freeze {
			want = imath.Vector3{}
		}
		if jaw.Translation.KeyFrames[100] != want {
			t.Fatalf("freezeHead=%v was not honored", freeze)
		}
		if len(m.Sequences) != 2 {
			t.Fatal("ordinary export must preserve Mount/Sleep")
		}
	}
	if !DefaultExportOptimization().FreezeHead {
		t.Fatal("map defaults must enable head freeze")
	}
}

func TestCreatureCacheRejectsUnversionedLegacyModel(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "npcs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "creature-604.mdx"), []byte("MDLX"), 0644); err != nil {
		t.Fatal(err)
	}
	job := &creatureExport{asset: &CreatureAsset{ConfigKey: "new-animation-policy", ModelPath: "npcs/wolf.mdx"}}
	cfg := config.DefaultConfig()
	cfg.OverrideModels = false
	cleanup, err := stageCreatureCache(context.Background(), root, []*creatureExport{job}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if job.cachedPath != "" {
		t.Fatal("legacy models without a policy fingerprint must be rebuilt")
	}
}

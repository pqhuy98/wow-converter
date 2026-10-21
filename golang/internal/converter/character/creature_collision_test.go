package character

import (
	"context"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestCreatureCollisionBoxUsesHorizontalEnclosingCircle(t *testing.T) {
	skeleton := m2.CAABox{Min: [3]float32{-.30555555, -.30555555, 0}, Max: [3]float32{.30555555, .30555555, 2.0312779}}
	radius := creatureCollisionBoxRadius(skeleton)
	if math.Abs(radius*56*1.011428-24.4753) > .0001 {
		t.Fatalf("skeleton radius: %g", radius*56*1.011428)
	}
	// Height cannot enlarge ground collision, and off-center boxes must still fit
	// a circle centered at the unit's placement origin.
	skeleton.Max[2] = 10000
	if got := creatureCollisionBoxRadius(skeleton); got != radius {
		t.Fatalf("height changed radius: %g", got)
	}
	offset := m2.CAABox{Min: [3]float32{-1, -2, 0}, Max: [3]float32{3, 4, 5}}
	if got := creatureCollisionBoxRadius(offset); got != 5 {
		t.Fatalf("off-center corner not enclosed: %g", got)
	}
	for _, box := range []m2.CAABox{{}, {Min: [3]float32{1, -1, 0}, Max: [3]float32{-1, 1, 2}}, {Min: [3]float32{-1, -1, 0}, Max: [3]float32{1, 1, 0}}, {Min: [3]float32{-1, -1, 0}, Max: [3]float32{float32(math.Inf(1)), 1, 2}}, {Min: [3]float32{-1, -1, 0}, Max: [3]float32{1, float32(math.NaN()), 2}}} {
		if got := creatureCollisionBoxRadius(box); got != 0 {
			t.Fatalf("invalid box did not request fallback: %g", got)
		}
	}
}

func TestSharedCreatureModelsRetainDifferentSourceCollision(t *testing.T) {
	shared := &CreatureAsset{ModelPath: "npcs/shared.mdx", ContentKey: "same", CollisionRadius: 12}
	other := shareCreatureAsset(shared, 24)
	if other == shared || other.ModelPath != shared.ModelPath || other.CollisionRadius != 24 || shared.CollisionRadius != 12 {
		t.Fatal("content dedup lost source-specific collision")
	}
	if shareCreatureAsset(shared, 12) != shared {
		t.Fatal("equal source metadata should share the asset")
	}
	if len(uniqueCreatureAssets(CreatureAssets{"a": shared, "b": other})) != 1 {
		t.Fatal("collision differences duplicated model assets")
	}
	root := t.TempDir()
	// A collision field is metadata only: old models remain usable, and refreshed
	// values survive the persisted cache without changing the appearance key.
	if err := os.MkdirAll(filepath.Join(root, "npcs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeCreatureAssetCache(root, map[string]*CreatureAsset{"key": other}); err != nil {
		t.Fatal(err)
	}
	if got := readCreatureAssetCache(root)["key"].CollisionRadius; got != 24 {
		t.Fatalf("collision cache lost: %g", got)
	}
}

func TestCachedCreatureModelRefreshesCollisionWithoutFreshExport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "npcs"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "npcs", "same.mdx")
	raw := mdx.NewModel().SaveMdx()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	old := map[string]*CreatureAsset{"key": {ConfigKey: "key", ModelPath: "npcs/same.mdx", CollisionRadius: 999}}
	job := &creatureExport{asset: &CreatureAsset{ConfigKey: "key", ModelPath: "npcs/same.mdx", CollisionRadius: 24}}
	cfg := config.DefaultConfig()
	cfg.OverrideModels = false
	cleanup, err := stageCreatureCache(context.Background(), root, []*creatureExport{job}, cfg, old)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if job.cachedPath == "" {
		t.Fatal("cached model was not reused")
	}
	if err := exportCreatureAppearance(context.Background(), job, root, cfg, nil); err != nil {
		t.Fatal(err)
	}
	if job.asset.CollisionRadius != 24 {
		t.Fatal("stale collision metadata overwrote refreshed source box")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(raw) {
		t.Fatal("collision refresh changed the cached model")
	}
}

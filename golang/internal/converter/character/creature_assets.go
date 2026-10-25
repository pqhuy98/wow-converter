package character

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

// CreatureAsset is shared by NPCs with the same resolved export appearance.
type CreatureAsset struct {
	CharacterModel bool `json:"characterModel,omitempty"`
	WowRace        int  `json:"wowRace,omitempty"`
	// CollisionRadius is in exported model units, before WE model/unit scaling.
	// Zero means the source has no usable M2 collision box.
	CollisionRadius float64  `json:"collisionRadius,omitempty"`
	ModelPath       string   `json:"model"`
	ConfigKey       string   `json:"configKey"`
	ContentKey      string   `json:"contentKey,omitempty"`
	Textures        []string `json:"textures,omitempty"`
}

// CreatureAssets maps source export inputs to shared output assets.
type CreatureAssets map[string]*CreatureAsset

func (a CreatureAssets) For(c azerothcore.Creature) *CreatureAsset { return a[creatureInputKey(c)] }

type creatureExport struct {
	creature   azerothcore.Creature
	character  Character
	metadata   CharacterData
	asset      *CreatureAsset
	cachedPath string
	standReady bool
}

// CreatureExportCallbacks select map NPC idles and observe completed files.
// ModelsDone drains readers before deduplication/cache cleanup removes files.
type CreatureExportCallbacks struct {
	Progress   func(completed, total int)
	ModelReady func(model string)
	ModelsDone func()
	// UseStandReady selects the idle experiment before appearance deduplication.
	// It is applied only to resolved character models.
	UseStandReady func(azerothcore.Creature) bool
}

// ExportCreatureModels resolves appearances before conversion, and exports each once.
func ExportCreatureModels(ctx context.Context, creatures []azerothcore.Creature, outputPath string, cfg config.Config, wowClient client.Client, workers int, callbacks CreatureExportCallbacks) (CreatureAssets, error) {
	done := sync.OnceFunc(func() {
		if callbacks.ModelsDone != nil {
			callbacks.ModelsDone()
		}
	})
	defer done()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if wowClient != nil {
		if err := wowClient.InitModelCaches(ctx); err != nil {
			return nil, fmt.Errorf("initialize creature model caches: %w", err)
		}
	}
	build := ""
	if wowClient != nil {
		info, err := wowClient.GetCASCInfo(ctx)
		if err != nil {
			return nil, err
		}
		build = info.BuildKey
	}
	resolver := NewCharacterExporter(cfg, wowClient)
	resolver.HTTP = resolver.HTTP.WithContext(ctx)
	type modelInfo struct {
		particles       bool
		collisionRadius float64
	}
	models := map[int]modelInfo{}
	jobs, inputs, err := planCreatureExports(ctx, creatures, cfg, build, callbacks.UseStandReady, func(c azerothcore.Creature) (CharacterData, error) {
		ch := creatureCharacter(c)
		ec := resolver.newExportContext("", ch, "")
		ec.ctx = ctx
		meta, err := resolver.resolveBaseMetadata(&ec, wowhead.ZamURL{Expansion: wowhead.ExpansionLatestAvailable, Type: wowhead.ZamTypeNPC, DisplayID: c.Model.CreatureDisplayID})
		if err != nil {
			return meta, err
		}
		if hasResolvedModel(meta.Model) {
			info, known := models[*meta.Model]
			if !known {
				raw, err := wowClient.DownloadCascFile(ctx, *meta.Model)
				if err != nil {
					return meta, err
				}
				loader := m2.NewLoader(buffer.From(raw), nil)
				if err := loader.Load(ctx); err != nil {
					return meta, err
				}
				info = modelInfo{particles: len(loader.ParticleEmitters) > 0, collisionRadius: creatureCollisionBoxRadius(loader.CollisionBox) * cfg.RawModelScaleUp}
				models[*meta.Model] = info
			}
			// Display scale affects only particle sizes in creature conversion.
			if meta.Character == nil && !info.particles {
				meta.Scale = 1
				if meta.Creature != nil {
					copy := *meta.Creature
					copy.ParticleColor = nil
					meta.Creature = &copy
				}
			}
		}
		return meta, nil
	})
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if hasResolvedModel(job.metadata.Model) {
			job.asset.CollisionRadius = models[*job.metadata.Model].collisionRadius
		}
	}
	old := readCreatureAssetCache(outputPath)
	if err = os.MkdirAll(filepath.Join(outputPath, "npcs"), 0755); err != nil {
		return nil, err
	}
	cleanup, err := stageCreatureCache(ctx, outputPath, jobs, cfg, old)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	var completed atomic.Int32
	var progressMu sync.Mutex
	tasks := make([]func() error, 0, len(jobs))
	for _, job := range jobs {
		job := job
		tasks = append(tasks, func() error {
			if err := exportCreatureAppearance(ctx, job, outputPath, cfg, wowClient); err != nil {
				return err
			}
			if callbacks.ModelReady != nil {
				callbacks.ModelReady(job.asset.ModelPath)
			}
			if callbacks.Progress != nil {
				progressMu.Lock()
				callbacks.Progress(int(completed.Add(1)), len(jobs))
				progressMu.Unlock()
			}
			return nil
		})
	}
	err = common.WorkerPoolContext(ctx, workers, tasks)
	done()
	if err != nil {
		return nil, err
	}
	// Distinct configurations can still produce identical models (unused overrides).
	byContent := map[string]*CreatureAsset{}
	aliases := map[*CreatureAsset]*CreatureAsset{}
	for _, job := range jobs {
		asset := job.asset
		if asset.ContentKey != "" {
			if shared := byContent[asset.ContentKey]; shared != nil {
				aliases[asset] = shareCreatureAsset(shared, asset.CollisionRadius, asset.CharacterModel, asset.WowRace)
				if err = os.Remove(filepath.Join(outputPath, filepath.FromSlash(asset.ModelPath))); err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				continue
			}
			byContent[asset.ContentKey] = asset
		}
	}
	for key, asset := range inputs {
		if shared := aliases[asset]; shared != nil {
			inputs[key] = shared
		}
	}
	cache := map[string]*CreatureAsset{}
	for _, job := range jobs {
		asset := job.asset
		if shared := aliases[asset]; shared != nil {
			asset = shared
		}
		cache[job.asset.ConfigKey] = asset
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = writeCreatureAssetCache(outputPath, cache); err != nil {
		return nil, err
	}
	if err = cleanCreatureAssets(outputPath, old, cache); err != nil {
		return nil, err
	}
	log.Printf("NPC appearances: %d source configurations, %d planned groups, %d unique models", len(inputs), len(jobs), len(uniqueCreatureAssets(inputs)))
	return inputs, nil
}

func planCreatureExports(ctx context.Context, creatures []azerothcore.Creature, cfg config.Config, build string, useStandReady func(azerothcore.Creature) bool, resolve func(azerothcore.Creature) (CharacterData, error)) ([]*creatureExport, CreatureAssets, error) {
	ordered := append([]azerothcore.Creature(nil), creatures...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		an, bn := creatureSlug(a.Template.Name), creatureSlug(b.Template.Name)
		if an != bn {
			return an < bn
		}
		if a.Model.CreatureDisplayID != b.Model.CreatureDisplayID {
			return a.Model.CreatureDisplayID < b.Model.CreatureDisplayID
		}
		return creatureInputKey(a) < creatureInputKey(b)
	})
	inputs := CreatureAssets{}
	byAppearance := map[string]*creatureExport{}
	byDisplay := map[int]CharacterData{}
	var jobs []*creatureExport
	for _, c := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if c.Model.CreatureDisplayID <= 0 {
			return nil, nil, fmt.Errorf("no display id found for creature template %d", c.Template.Entry)
		}
		input := creatureInputKey(c)
		if inputs[input] != nil {
			continue
		}
		meta, ok := byDisplay[c.Model.CreatureDisplayID]
		if !ok {
			var err error
			meta, err = resolve(c)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve creature display %d: %w", c.Model.CreatureDisplayID, err)
			}
			byDisplay[c.Model.CreatureDisplayID] = meta
		}
		ch := creatureCharacter(c)
		key, err := creatureAppearanceKey(meta, ch, cfg, build)
		if err != nil {
			return nil, nil, err
		}
		standReady := meta.Character != nil && useStandReady != nil && useStandReady(c)
		if standReady {
			// Keep ordinary and experimental idles separate, including cached exports.
			key += "-stand-ready-v1"
		}
		job := byAppearance[key]
		if job == nil {
			job = &creatureExport{creature: c, character: ch, metadata: meta, standReady: standReady, asset: &CreatureAsset{ConfigKey: key, CharacterModel: meta.Character != nil}}
			if meta.Character != nil {
				job.asset.WowRace = meta.Character.Race
			}
			byAppearance[key] = job
			jobs = append(jobs, job)
		}
		inputs[input] = job.asset
	}
	names := map[string]int{}
	for _, job := range jobs {
		names[creatureSlug(job.creature.Template.Name)]++
	}
	used := map[string]bool{}
	for _, job := range jobs {
		name := creatureSlug(job.creature.Template.Name)
		if names[name] > 1 {
			name += "_" + strconv.Itoa(job.creature.Model.CreatureDisplayID)
		}
		stem := name
		for suffix := 2; used[name]; suffix++ {
			name = fmt.Sprintf("%s-%d", stem, suffix)
		}
		used[name] = true
		job.asset.ModelPath = "npcs/" + name + ".mdx"
	}
	return jobs, inputs, nil
}

func creatureAppearanceKey(meta CharacterData, ch Character, cfg config.Config, build string) (string, error) {
	ch.Base = Ref{}
	cfg.ExportAssetDir = ""
	cfg.OverrideModels = false
	cfg.OverrideTextures = false
	cfg.IsBulkExport = false
	if meta.Scale == 0 {
		meta.Scale = 1
	}
	if meta.Character == nil {
		meta.Equipment = nil
		meta.TextureFiles = nil
		meta.ItemEffects = nil
		meta.SeparateShoulders = false
		meta.Textures = positiveTextureMap(meta.Textures)
		if len(meta.Textures) == 0 {
			meta.Textures = nil
		}
		if meta.Creature != nil {
			copy := *meta.Creature
			copy.CreatureCustomizations = nil
			if copy.ParticleColor == nil && len(copy.CreatureGeosetData) == 0 {
				meta.Creature = nil
			} else {
				meta.Creature = &copy
			}
		}
	}
	raw, err := json.Marshal(struct {
		Version   int
		Build     string
		Metadata  CharacterData
		Character Character
		Config    config.Config
	}{4, build, meta, ch, cfg})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func creatureInputKey(c azerothcore.Creature) string {
	// Identical displays/equipment can have different combat capability. Retain
	// the source policy inputs here; appearance keys still share identical output.
	raw, _ := json.Marshal(struct {
		Character
		Gameplay *azerothcore.CreatureGameplay `json:"gameplay,omitempty"`
	}{creatureCharacter(c), c.Gameplay})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func creatureCharacter(c azerothcore.Creature) Character {
	ch := Character{Base: DisplayRef(c.Model.CreatureDisplayID), AttackTag: GuessAttackTag(0, 0), InGameMovespeed: 270}
	if c.Equipment == nil {
		return ch
	}
	ch.AttachItems = map[string]AttachItem{}
	invR, invL := 0, 0
	if item := c.Equipment.Item1; item != nil {
		invR = item.InventoryType
		if attachment, ok := InventoryTypeToEquipmentSlot(invR, 0); ok {
			ch.AttachItems[strconv.Itoa(int(attachment))] = AttachItem{Path: WowheadRef(fmt.Sprintf("https://www.wowhead.com/wotlk/item=%d", item.Entry))}
		}
	}
	if item := c.Equipment.Item2; item != nil {
		invL = item.InventoryType
		if attachment, ok := InventoryTypeToEquipmentSlot(invL, 1); ok {
			ch.AttachItems[strconv.Itoa(int(attachment))] = AttachItem{Path: WowheadRef(fmt.Sprintf("https://www.wowhead.com/wotlk/item=%d", item.Entry))}
		}
	}
	ch.AttackTag = GuessAttackTag(invR, invL)
	return ch
}

func creatureSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	result := b.String()
	if result == "" {
		return "npc"
	}
	if len(result) > 100 {
		result = strings.TrimRight(result[:100], "-")
	}
	return result
}

func uniqueCreatureAssets(assets CreatureAssets) []*CreatureAsset {
	seen := map[string]bool{}
	var out []*CreatureAsset
	for _, asset := range assets {
		if !seen[asset.ModelPath] {
			seen[asset.ModelPath] = true
			out = append(out, asset)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModelPath < out[j].ModelPath })
	return out
}

// The enclosing circle stays centered on the unit origin, including off-center boxes.
func creatureCollisionBoxRadius(box m2.CAABox) float64 {
	for i := 0; i < 3; i++ {
		lo, hi := float64(box.Min[i]), float64(box.Max[i])
		if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || hi <= lo {
			return 0
		}
	}
	x := math.Max(math.Abs(float64(box.Min[0])), math.Abs(float64(box.Max[0])))
	y := math.Max(math.Abs(float64(box.Min[1])), math.Abs(float64(box.Max[1])))
	return math.Hypot(x, y)
}

func shareCreatureAsset(shared *CreatureAsset, collisionRadius float64, characterModel bool, wowRace int) *CreatureAsset {
	if shared.CollisionRadius == collisionRadius && shared.CharacterModel == characterModel && shared.WowRace == wowRace {
		return shared
	}
	// Identical rendered models can have different source collision/classification.
	copy := *shared
	copy.CollisionRadius = collisionRadius
	copy.CharacterModel = characterModel
	copy.WowRace = wowRace
	return &copy
}

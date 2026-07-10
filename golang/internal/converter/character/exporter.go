package character

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/server/pathsafe"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

var wantedZPerSize = map[string]float64{
	"small": 60, "medium": 104, "large": 150, "hero": 175, "semi-giant": 262, "giant": 350,
}

// CharacterExporter orchestrates character model export.
type CharacterExporter struct {
	Config       config.Config
	AssetManager *common.AssetManager
	WowClient    client.Client
	Wowhead      WowheadClient
	HTTP         *wowhead.HTTPClient
	Models       [][2]interface{} // [*mdl.MDL, output path]
}

// NewCharacterExporter creates an exporter with the given config and wow client.
func NewCharacterExporter(cfg config.Config, wowClient client.Client) *CharacterExporter {
	httpClient := wowhead.NewHTTPClient()
	return &CharacterExporter{
		Config:       cfg,
		AssetManager: common.NewAssetManager(cfg, wowClient),
		WowClient:    wowClient,
		Wowhead:      DefaultWowheadClient{HTTP: httpClient},
		HTTP:         httpClient,
	}
}

// ExportOptions configures optional export behavior.
type ExportOptions struct {
	LocalModelSkinID string
}

// ExportCharacter exports a character or creature to MDL.
func (e *CharacterExporter) ExportCharacter(ctx context.Context, char Character, outputFile string, opts ...ExportOptions) (*mdl.MDL, error) {
	start := time.Now()
	var skinID string
	if len(opts) > 0 {
		skinID = opts[0].LocalModelSkinID
	}
	if e.WowClient != nil {
		if err := e.WowClient.InitModelCaches(ctx); err != nil {
			log.Printf("InitModelCaches: %v", err)
		}
	}
	log.Printf("Exporting character %s", char.Base.Value)

	if char.Mount != nil && char.Mount.Path.Value != "" {
		result, err := e.exportCharacterWithMount(ctx, char, outputFile)
		if err != nil {
			return nil, err
		}
		log.Printf("Total character export took %s", ansi.Yellowf("%.2fs", time.Since(start).Seconds()))
		return result.MountMdl, nil
	}

	exportCtx := e.newExportContext(outputFile, char, skinID)
	model, err := e.exportBaseMdl(ctx, &exportCtx, char)
	if err != nil {
		return nil, err
	}
	model = e.postProcessModel(model, char, &exportCtx)
	model.Model.Name = outputFile
	e.IncludeMdlToOutput(model, outputFile)
	log.Printf("Total character export took %s", ansi.Yellowf("%.2fs", time.Since(start).Seconds()))
	return model, nil
}

func (e *CharacterExporter) newExportContext(outputFile string, char Character, localModelSkinID string) ExportContext {
	return ExportContext{
		AssetManager:     e.AssetManager,
		Config:           e.Config,
		OutputFile:       outputFile,
		WowClient:        e.WowClient,
		Wowhead:          e.Wowhead,
		HTTP:             e.HTTP,
		ForceSheathed:    char.ForceSheathed,
		WithCollision:    char.WithCollision,
		LocalModelSkinID: localModelSkinID,
	}
}

func (e *CharacterExporter) postProcessModel(model *mdl.MDL, char Character, exportCtx *ExportContext) *mdl.MDL {
	if !char.KeepCinematic {
		filtered := model.Sequences[:0]
		for _, seq := range model.Sequences {
			if !strings.Contains(seq.Name, "Cinematic") || seq.Keep {
				filtered = append(filtered, seq)
			}
		}
		model.Sequences = filtered
	}

	if len(model.Cameras) == 0 {
		model.Modify.AddPortraitCamera(char.PortraitCameraSequenceName)
	}

	if char.AttachItems != nil {
		for wowAttachmentID, itemPath := range char.AttachItems {
			attID := 0
			fmt.Sscanf(wowAttachmentID, "%d", &attID)
			var attachment *components.WowAttachment
			for i := range model.WowAttachments {
				if model.WowAttachments[i].WowAttachmentID == attID {
					attachment = &model.WowAttachments[i]
					break
				}
			}
			if attachment == nil {
				log.Printf("%s", ansi.Redf("Cannot find bone for wow attachment %d (%s)", attID, animmap.GetWoWAttachmentName(attID)))
				if len(model.WowAttachments) == 0 {
					log.Printf("%s", ansi.Redf(`No WoW attachments data found in this model "%s"`, char.Base.Value))
				}
				continue
			}
			itemResult, invType, err := e.exportItem(exportCtx, itemPath.Path)
			if err != nil {
				log.Printf("attachItems %s: %v", itemPath.Path.Value, err)
				continue
			}
			itemMdl := itemResult.MDL
			if mdl.CanAddMdlCollectionItemToModel(model, itemMdl) {
				model.Modify.AddMdlCollectionItemToModel(itemMdl)
				continue
			}
			if invType != nil {
				if attID == int(animmap.WoWAttachmentHandRight) && exportCtx.WeaponInventoryTypes[0] == nil {
					exportCtx.WeaponInventoryTypes[0] = invType
				}
				if (attID == int(animmap.WoWAttachmentHandLeft) || attID == int(animmap.WoWAttachmentShield)) && exportCtx.WeaponInventoryTypes[1] == nil {
					exportCtx.WeaponInventoryTypes[1] = invType
				}
			}
			if itemPath.Scale > 0 {
				itemMdl.Modify.Scale(itemPath.Scale)
			}
			if len(itemMdl.Bones) > 0 {
				itemMdl.Bones[0].Name += fmt.Sprintf("_atm_%d", attID)
			}
			model.Modify.AddMdlItemToBone(itemMdl, attachment.Bone)
		}
	}

	if char.AttackTag != "" {
		tag := char.AttackTag
		if tag == animmap.AttackTagAuto {
			r, l := 0, 0
			if exportCtx.WeaponInventoryTypes[0] != nil {
				r = *exportCtx.WeaponInventoryTypes[0]
			}
			if exportCtx.WeaponInventoryTypes[1] != nil {
				l = *exportCtx.WeaponInventoryTypes[1]
			}
			tag = GuessAttackTag(r, l)
		}
		log.Printf("Chosen attack tag: %s", tag)
		filtered := model.Sequences[:0]
		for _, seq := range model.Sequences {
			if tag == "" || seq.Data.AttackTag == "" || seq.Data.AttackTag == string(tag) {
				filtered = append(filtered, seq)
			}
		}
		model.Sequences = filtered
	}

	if char.Size != "" {
		if wantedZ, ok := wantedZPerSize[char.Size]; ok {
			standIdx := -1
			for i, seq := range model.Sequences {
				if seq.Name == "Stand" {
					standIdx = i
					break
				}
			}
			if standIdx < 0 && len(model.Sequences) > 0 {
				standIdx = 0
			}
			scale := char.Scale
			if scale == 0 {
				scale = 1
			}
			if standIdx >= 0 && len(model.Sequences) > 0 {
				maxZ := model.Modify.GetMaxZAtTimestamp(model.Sequences[standIdx], 0)
				if maxZ > 0 {
					model.Modify.Scale(scale * wantedZ / maxZ)
				} else {
					log.Printf("%s %v", ansi.Redf("Cannot scale model %s because the model is not above the ground:", model.Model.Name), map[string]float64{"maxStandZ": maxZ})
				}
			} else if model.Model.MaximumExtent[2] > 0 {
				model.Modify.Scale(scale * wantedZ / model.Model.MaximumExtent[2])
			} else {
				log.Printf("%s %v", ansi.Redf("Cannot scale model %s because it has non-positive maximum extent Z:", model.Model.Name), model.Model.MaximumExtent)
			}
		}
	} else if char.Scale > 0 {
		model.Modify.Scale(char.Scale)
	}

	hasWalk, hasWalkFast := false, false
	walkFastIdx := -1
	for i, seq := range model.Sequences {
		if seq.Name == "Walk" {
			hasWalk = true
		}
		if seq.Name == "Walk Fast" {
			hasWalkFast = true
			walkFastIdx = i
		}
	}
	if !hasWalk && hasWalkFast && walkFastIdx >= 0 {
		model.Sequences[walkFastIdx].Name = "Walk"
	}

	if char.InGameMovespeed > 0 {
		for i := range model.Sequences {
			seq := &model.Sequences[i]
			if seq.MoveSpeed > 0 && strings.Contains(seq.Name, "Walk") &&
				!strings.Contains(seq.Name, "Spin") && !strings.Contains(seq.Name, "Swim") &&
				!strings.Contains(seq.Name, "Alternate") {
				old := seq.MoveSpeed
				if old == 0 {
					old = 450
				}
				model.Modify.ScaleSequenceDuration(seq, old/char.InGameMovespeed)
				seq.MoveSpeed = char.InGameMovespeed
			}
		}
	}

	if !char.NoDecay {
		model.Modify.AddDecayAnimation()
	}

	for _, seq := range model.Sequences {
		if strings.HasPrefix(seq.Name, "Portrait Talk") {
			for i := range model.Sequences {
				if model.Sequences[i].Name == "Stand" {
					model.Modify.CloneSequence(&model.Sequences[i], "Portrait")
				}
			}
			break
		}
	}

	concatenateAttackSequences(model)

	model.Modify.RecomputeNormals()
	model.Modify.OptimizeKeyFrames()
	return model
}

func concatenateAttackSequences(model *mdl.MDL) {
	for _, attackTag := range []string{"Bow", "Rifle", "Thrown"} {
		var attacks []*components.Sequence
		for i := range model.Sequences {
			s := &model.Sequences[i]
			if s.Data.AttackTag == attackTag && s.Name == "Attack" {
				attacks = append(attacks, s)
			}
		}
		if len(attacks) == 0 {
			continue
		}
		sort.Slice(attacks, func(i, j int) bool {
			score := func(name string) int {
				if strings.HasPrefix(name, "Load") {
					return 1
				}
				if strings.HasPrefix(name, "Attack") {
					return 2
				}
				return 3
			}
			return score(attacks[i].Data.WowName) < score(attacks[j].Data.WowName)
		})
		names := make([]string, len(attacks))
		for i, s := range attacks {
			names[i] = s.Data.WowName
		}
		log.Printf("concatenate animations Attack %s from %v", attackTag, names)
		model.Modify.ConcatenateSequences(attacks, "Attack")
		remove := map[*components.Sequence]struct{}{}
		for _, s := range attacks {
			remove[s] = struct{}{}
		}
		filtered := model.Sequences[:0]
		for i := range model.Sequences {
			if _, skip := remove[&model.Sequences[i]]; !skip {
				filtered = append(filtered, model.Sequences[i])
			}
		}
		model.Sequences = filtered
	}
}

type mountExportResult struct {
	CharMdl  *mdl.MDL
	MountMdl *mdl.MDL
}

func (e *CharacterExporter) exportCharacterWithMount(ctx context.Context, char Character, outputFile string) (mountExportResult, error) {
	if char.Mount == nil {
		return mountExportResult{}, fmt.Errorf("mount is required")
	}
	mountName := outputFile + "_mount"
	mountChar := Character{
		Base: char.Mount.Path, InGameMovespeed: char.InGameMovespeed,
		AttackTag: char.AttackTag, KeepCinematic: char.KeepCinematic,
		NoDecay: char.NoDecay, ParticlesDensity: char.ParticlesDensity,
		PortraitCameraSequenceName: char.PortraitCameraSequenceName,
	}
	mountMdl, err := e.ExportCharacter(ctx, mountChar, mountName)
	if err != nil {
		return mountExportResult{}, err
	}
	var mountBone *components.Bone
	for _, a := range mountMdl.WowAttachments {
		if a.WowAttachmentID == int(animmap.WoWAttachmentShield) {
			mountBone = a.Bone
			break
		}
	}
	if mountBone == nil {
		return mountExportResult{}, fmt.Errorf("mount model doesn't have shield attachment for rider")
	}
	riderChar := char
	riderChar.Mount = nil
	riderChar.ForceSheathed = true
	charMdl, err := e.ExportCharacter(ctx, riderChar, outputFile)
	if err != nil {
		return mountExportResult{}, err
	}
	filtered := charMdl.Sequences[:0]
	for _, s := range charMdl.Sequences {
		if strings.Contains(s.Name, "Mount") || strings.Contains(s.Name, "Death") {
			filtered = append(filtered, s)
		}
	}
	charMdl.Sequences = filtered
	mountAnim := "Mount"
	if char.Mount.Animation != "" {
		mountAnim = char.Mount.Animation
	}
	hasMountAnim := false
	for i := range charMdl.Sequences {
		if charMdl.Sequences[i].Data.WowName == mountAnim {
			hasMountAnim = true
			charMdl.Sequences[i].Name = "Stand"
		} else if strings.Contains(charMdl.Sequences[i].Name, "Mount") {
			charMdl.Sequences[i].Name = "Mount " + charMdl.Sequences[i].Data.WowName
		}
	}
	if !hasMountAnim {
		return mountExportResult{}, fmt.Errorf("character model doesn't have any %s animation", mountAnim)
	}

	var overheadMounted, overheadPlayer *components.AttachmentPoint
	for i := range charMdl.Attachments {
		att := charMdl.Attachments[i]
		if att.Data == nil || att.Data.WowAttachment == nil {
			continue
		}
		switch att.Data.WowAttachment.WowAttachmentID {
		case int(animmap.WoWAttachmentPlayerNameMounted):
			overheadMounted = att
		case int(animmap.WoWAttachmentPlayerName):
			overheadPlayer = att
		}
	}
	if overheadMounted != nil {
		overheadMounted.Name = "Overhead"
		if overheadPlayer != nil {
			overheadPlayer.Name = fmt.Sprintf("Wow:%d:%s", animmap.WoWAttachmentPlayerName, animmap.GetWoWAttachmentName(int(animmap.WoWAttachmentPlayerName)))
		}
	}

	scale := charMdl.AccumScale
	if char.Mount.Scale > 0 {
		scale *= char.Mount.Scale
	}
	mountMdl.Modify.Scale(scale)
	atm := mountMdl.Modify.AddItemPathToBone(outputFile+".mdx", mountBone, false)
	applyMountRiderAttachment(mountMdl, atm, char.Mount.SeatOffset, charMdl.AccumScale)
	return mountExportResult{CharMdl: charMdl, MountMdl: mountMdl}, nil
}

func applyMountRiderAttachment(mountMdl *mdl.MDL, atm *components.AttachmentPoint, seatOffset []float64, riderScale float64) {
	if atm == nil {
		return
	}
	gs := lastOrAddGlobalSequence(mountMdl)
	if len(seatOffset) == 3 {
		atm.Translation = &components.Animation{
			GlobalSeq:     gs,
			Interpolation: components.InterpDontInterp,
			KeyFrames: map[int]any{
				0: imath.Vector3{
					seatOffset[0] * riderScale,
					seatOffset[1] * riderScale,
					seatOffset[2] * riderScale,
				},
			},
			Type: components.AnimTypeTranslation,
		}
	}
	for _, seq := range mountMdl.Sequences {
		if !strings.Contains(seq.Name, "Death") && !strings.Contains(seq.Name, "Decay") {
			continue
		}
		if atm.Scaling == nil {
			atm.Scaling = &components.Animation{
				GlobalSeq:     gs,
				Interpolation: components.InterpDontInterp,
				KeyFrames:     map[int]any{},
				Type:          components.AnimTypeScaling,
			}
		}
		zero := imath.Vector3{0, 0, 0}
		atm.Scaling.KeyFrames[seq.Interval[0]] = zero
		atm.Scaling.KeyFrames[seq.Interval[1]] = zero
	}
}

func lastOrAddGlobalSequence(m *mdl.MDL) *components.GlobalSequence {
	if len(m.GlobalSequences) > 0 {
		return m.GlobalSequences[len(m.GlobalSequences)-1]
	}
	duration := 1000
	for _, seq := range m.Sequences {
		if seq.Interval[1] > duration {
			duration = seq.Interval[1]
		}
	}
	created := components.NewGlobalSequence(0, duration)
	gs := &created
	m.GlobalSequences = append(m.GlobalSequences, gs)
	return gs
}

func (e *CharacterExporter) exportBaseMdl(ctx context.Context, exportCtx *ExportContext, char Character) (*mdl.MDL, error) {
	switch char.Base.Type {
	case "local":
		model, collision, err := ExportLocalModelAsMdl(e.AssetManager, e.Config, e.WowClient, char.Base.Value, LocalModelOptions{
			WithCollision: exportCtx.WithCollision, SkinIDOverride: exportCtx.LocalModelSkinID,
			KeepCinematic: char.KeepCinematic, AttackTag: char.AttackTag,
		})
		if err != nil {
			return nil, err
		}
		base := model.MDL
		if collision != nil {
			base.Geosets = append(base.Geosets, collision.Geosets...)
			base.Textures = append(base.Textures, collision.Textures...)
			base.Materials = append(base.Materials, collision.Materials...)
			base.Bones = append(base.Bones, collision.Bones...)
		}
		return base, nil
	case "wowhead", "displayID":
		var zam wowhead.ZamURL
		var err error
		if char.Base.Type == "wowhead" {
			zam, err = wowhead.GetZamURLFromWowheadURL(e.HTTP, char.Base.Value)
			if err != nil {
				return nil, err
			}
		} else {
			id := 0
			fmt.Sscanf(char.Base.Value, "%d", &id)
			zam = wowhead.ZamURL{Expansion: wowhead.ExpansionLatestAvailable, Type: wowhead.ZamTypeNPC, DisplayID: id}
		}
		if !e.Config.IsBulkExport {
			log.Printf("baseZam: type=%s display=%d", zam.Type, zam.DisplayID)
		}

		var npcMeta wowhead.CharacterData
		metaExpansion := exportCtx.ResolveMetaExpansion(zam.Expansion)
		if !e.Config.IsBulkExport && metaExpansion != zam.Expansion {
			log.Printf("Using wowhead meta expansion %s (URL had %s)", metaExpansion, zam.Expansion)
		}
		switch zam.Type {
		case wowhead.ZamTypeItem:
			if zam.SlotID != nil && *zam.SlotID > 0 {
				log.Printf("Exporting base model as equipment item: %s", GetEquipmentSlotName(*zam.SlotID))
				item, _, err := e.exportItem(exportCtx, char.Base)
				if err != nil {
					return nil, err
				}
				return item.MDL, nil
			}
			log.Printf("Exporting base model as creature NPC because slotId=0")
			npcMeta, err = wowhead.FetchNpcMeta(e.HTTP, wowhead.ExpansionLatestAvailable, zam.DisplayID)
		case wowhead.ZamTypeNPC:
			npcMeta, err = wowhead.FetchNpcMeta(e.HTTP, metaExpansion, zam.DisplayID)
		case wowhead.ZamTypeObject:
			npcMeta, err = wowhead.FetchObjectMeta(e.HTTP, metaExpansion, zam.DisplayID)
		case wowhead.ZamTypeDressingRoom:
			npcMeta, err = wowhead.DecodeDressingRoom(e.HTTP, zam.Expansion, zam.Hash)
		default:
			return nil, fmt.Errorf("unallowed base zam type: %s", zam.Type)
		}
		if err != nil {
			return nil, err
		}
		charMeta := convertWowheadCharacterData(npcMeta)
		preferClassicDB := exportCtx.IsClassicCASC(ctx)
		if zam.Type == wowhead.ZamTypeNPC {
			if dbMeta, ok := resolveNpcMetaFromDB(ctx, exportCtx, zam.DisplayID); ok {
				charMeta = mergeNpcMeta(charMeta, dbMeta, preferClassicDB)
			}
		}
		if hasResolvedModel(charMeta.Model) && charMeta.Character == nil {
			return ExportCreatureNpcAsMdl(exportCtx, charMeta)
		}
		if zam.Type == wowhead.ZamTypeNPC && charMeta.Character == nil {
			return nil, fmt.Errorf("creature display %d: no model from wowhead or CASC DB2", zam.DisplayID)
		}
		exportExpansion := metaExpansion
		if zam.Type == wowhead.ZamTypeDressingRoom {
			exportExpansion = zam.Expansion
		}
		return ExportCharacterAsMdl(exportCtx, charMeta, exportExpansion, char.KeepCinematic, char.AttackTag)
	default:
		return nil, fmt.Errorf("unknown base type: %s", char.Base.Type)
	}
}

func (e *CharacterExporter) exportItem(ctx *ExportContext, ref Ref) (*commonModel, *int, error) {
	switch ref.Type {
	case "local":
		model, _, err := ExportLocalModelAsMdl(e.AssetManager, e.Config, e.WowClient, ref.Value, LocalModelOptions{})
		return model, nil, err
	case "wowhead", "displayID":
		var zam wowhead.ZamURL
		var err error
		if ref.Type == "wowhead" {
			zam, err = wowhead.GetZamURLFromWowheadURL(e.HTTP, ref.Value)
		} else {
			id := 0
			fmt.Sscanf(ref.Value, "%d", &id)
			zam = wowhead.ZamURL{Expansion: wowhead.ExpansionLive, Type: wowhead.ZamTypeItem, DisplayID: id}
		}
		if err != nil {
			return nil, nil, err
		}
		if zam.Type != wowhead.ZamTypeItem {
			return nil, nil, fmt.Errorf("expected item zam url")
		}
		model, itemData, err := ExportZamItemAsMdl(ctx, zam, 0, 2, 0)
		if err != nil {
			return nil, nil, err
		}
		inv := itemData.InventoryType
		return model, &inv, nil
	default:
		return nil, nil, fmt.Errorf("unknown item type: %s", ref.Type)
	}
}

func convertWowheadCharacterData(meta wowhead.CharacterData) CharacterData {
	out := CharacterData{
		Model: meta.Model, Textures: meta.Textures, Character: meta.Character,
		Equipment: meta.Equipment, TextureFiles: meta.TextureFiles, ItemEffects: meta.ItemEffects,
	}
	if meta.Creature != nil {
		out.Creature = &CreatureMeta{
			CreatureGeosetData:     make([]GeosetEntry, len(meta.Creature.CreatureGeosetData)),
			CreatureCustomizations: meta.Creature.CreatureCustomizations,
		}
		for i, g := range meta.Creature.CreatureGeosetData {
			out.Creature.CreatureGeosetData[i] = GeosetEntry{GeosetIndex: g.GeosetIndex, GeosetValue: g.GeosetValue}
		}
	}
	return out
}

// IncludeMdlToOutput registers an MDL for final asset write.
func (e *CharacterExporter) IncludeMdlToOutput(m *mdl.MDL, outputFile string) {
	e.Models = append(e.Models, [2]interface{}{m, outputFile})
}

// ExportOptimization mirrors export request optimization flags.
type ExportOptimization struct {
	SortSequences                 bool
	RemoveUnusedVertices          bool
	RemoveUnusedNodes             bool
	RemoveUnusedMaterialsTextures bool
	FormatVersion                 string
	AllMaterialsUnshaded          bool
	ParticlesDensity              *float64
}

// DefaultExportOptimization returns the standard export optimization flags.
func DefaultExportOptimization() ExportOptimization {
	return ExportOptimization{
		SortSequences:                 true,
		RemoveUnusedVertices:          true,
		RemoveUnusedNodes:             true,
		RemoveUnusedMaterialsTextures: true,
		FormatVersion:                 "1000",
	}
}

// OptimizeModels applies per-request optimizations like the TypeScript server.
func (e *CharacterExporter) OptimizeModels(opt ExportOptimization) {
	for _, pair := range e.Models {
		m, ok := pair[0].(*mdl.MDL)
		if !ok {
			continue
		}
		if opt.SortSequences {
			m.Modify.SortSequences()
		}
		if opt.FormatVersion == "800" {
			m.Modify.ConvertToSd800()
		}
		if opt.RemoveUnusedVertices {
			m.Modify.RemoveUnusedVertices()
		}
		if opt.ParticlesDensity != nil {
			pd := *opt.ParticlesDensity
			if pd > 0 && pd != 1 {
				m.Modify.ScaleParticlesDensity(pd)
			} else if pd == 0 {
				m.ParticleEmitter2s = nil
			}
		}
		if opt.RemoveUnusedNodes {
			m.Modify.RemoveUnusedNodes()
		}
		if opt.RemoveUnusedMaterialsTextures {
			m.Modify.RemoveUnusedMaterialsTextures()
		}
		if opt.AllMaterialsUnshaded {
			for i := range m.Materials {
				for li := range m.Materials[i].Layers {
					m.Materials[i].Layers[li].Unshaded = true
				}
			}
		}
		m.Modify.OptimizeKeyFrames()
		m.Sync()
	}
}

// OptimizeModelsTextures optimizes models and purges unused textures.
func (e *CharacterExporter) OptimizeModelsTextures(opt ExportOptimization) {
	e.OptimizeModels(opt)
	e.registerMdlTexturesFromModels()
	var used []string
	for _, pair := range e.Models {
		if m, ok := pair[0].(*mdl.MDL); ok {
			for _, t := range m.Textures {
				used = append(used, t.Image)
			}
		}
	}
	e.AssetManager.PurgeTextures(used)
}

func (e *CharacterExporter) registerMdlTexturesFromModels() {
	prefix := e.Config.AssetPrefix + "/"
	for _, pair := range e.Models {
		m, ok := pair[0].(*mdl.MDL)
		if !ok {
			continue
		}
		for _, tex := range m.Textures {
			if png := strings.TrimSpace(tex.WowData.PngPath); png != "" {
				e.AssetManager.AddPngTexture(png, false)
				continue
			}
			if tex.Image == "" {
				continue
			}
			png := strings.TrimPrefix(strings.ReplaceAll(tex.Image, ".blp", ".png"), prefix)
			png = strings.TrimPrefix(png, e.Config.AssetPrefix+"\\")
			e.AssetManager.AddPngTexture(png, false)
		}
	}
}

// WriteAllModels writes registered models to disk.
func (e *CharacterExporter) WriteAllModels(outputDir, format string) ([]string, error) {
	var paths []string
	for _, pair := range e.Models {
		m := pair[0].(*mdl.MDL)
		rel := pair[1].(string)
		full, err := safeExportPath(outputDir, rel)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, err
		}
		var data []byte
		if format == "mdx" {
			b, err := m.ToMdx()
			if err != nil {
				return nil, err
			}
			data = b
			full += ".mdx"
		} else {
			data = []byte(m.ToMdl())
			full += ".mdl"
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return nil, err
		}
		log.Printf("Wrote model %s", ansi.Green(full))
		paths = append(paths, full)
	}
	return paths, nil
}

// WriteAllTextures exports textures to disk.
func (e *CharacterExporter) WriteAllTextures(outputDir string) ([]string, error) {
	return e.AssetManager.ExportTextures(outputDir)
}

// AggregateModelStats sums stats across all registered models.
func (e *CharacterExporter) AggregateModelStats(formatVersion string) map[string]int {
	fv := 1000
	if formatVersion == "800" {
		fv = 800
	}
	stats := map[string]int{"formatVersion": fv}
	for _, pair := range e.Models {
		m, ok := pair[0].(*mdl.MDL)
		if !ok {
			continue
		}
		stats["vertices"] += countMdlVertices(m)
		stats["faces"] += countMdlFaces(m)
		stats["globalSequences"] += len(m.GlobalSequences)
		stats["sequences"] += len(m.Sequences)
		stats["geosets"] += len(m.Geosets)
		stats["geosetAnims"] += len(m.GeosetAnims)
		stats["materials"] += len(m.Materials)
		stats["textures"] += len(m.Textures)
		stats["textureAnims"] += len(m.TextureAnims)
		stats["bones"] += len(m.Bones)
		stats["lights"] += len(m.Lights)
		stats["ribbonEmitters"] += len(m.RibbonEmitters)
		stats["particles"] += len(m.ParticleEmitter2s)
		stats["attachments"] += len(m.Attachments)
		stats["eventObjects"] += len(m.EventObjects)
		stats["helpers"] += len(m.Helpers)
		stats["collisionShapes"] += len(m.CollisionShapes)
		stats["cameras"] += len(m.Cameras)
	}
	return stats
}

func countMdlVertices(m *mdl.MDL) int {
	n := 0
	for _, g := range m.Geosets {
		n += len(g.Vertices)
	}
	return n
}

func countMdlFaces(m *mdl.MDL) int {
	n := 0
	for _, g := range m.Geosets {
		n += len(g.Faces) / 3
	}
	return n
}

func safeExportPath(outputDir, rel string) (string, error) {
	if err := pathsafe.ValidateRelativeRef(rel); err != nil {
		return "", fmt.Errorf("invalid output path")
	}
	return pathsafe.ResolveUnderBase(outputDir, rel)
}

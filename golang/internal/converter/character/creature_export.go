package character

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

// ExportCreatureModels exports unique creature display models.
func ExportCreatureModels(creatures []azerothcore.Creature, outputPath string, cfg config.Config, wowClient client.Client, workers int, onProgress func(completed, total int)) error {
	seen := map[int]struct{}{}
	var unique []azerothcore.Creature
	for _, c := range creatures {
		displayID := c.Model.CreatureDisplayID
		if displayID == 0 {
			return fmt.Errorf("no display id found for creature template %d", c.Template.Entry)
		}
		if _, ok := seen[displayID]; ok {
			continue
		}
		seen[displayID] = struct{}{}
		unique = append(unique, c)
	}
	rand.Shuffle(len(unique), func(i, j int) { unique[i], unique[j] = unique[j], unique[i] })

	total := len(unique)
	var completed atomic.Int32
	reportProgress := func() {
		if onProgress != nil {
			onProgress(int(completed.Load()), total)
		}
	}

	ctx := context.Background()
	if wowClient != nil {
		if err := wowClient.InitModelCaches(ctx); err != nil {
			return fmt.Errorf("initialize creature model caches: %w", err)
		}
	}

	tasks := make([]func() error, 0, len(unique))
	for i, c := range unique {
		i, c := i, c
		tasks = append(tasks, func() error {
			if err := exportOneCreature(ctx, cfg, wowClient, outputPath, c, i+1, len(unique)); err != nil {
				return err
			}
			completed.Add(1)
			reportProgress()
			return nil
		})
	}
	return common.WorkerPool(workers, tasks)
}

func exportOneCreature(ctx context.Context, cfg config.Config, wowClient client.Client, outputPath string, c azerothcore.Creature, index, total int) error {
	displayID := c.Model.CreatureDisplayID
	fileName := fmt.Sprintf("creature-%d", displayID)
	ext := ".mdl"
	if cfg.MDX {
		ext = ".mdx"
	}
	full := filepath.Join(outputPath, fileName+ext)
	if _, err := os.Stat(full); err == nil && !cfg.OverrideModels {
		return nil
	}

	log.Printf("")
	log.Printf("==== Exporting creature %s (%d/%d, guid=%d, displayId=%d)",
		ansi.Blue(c.Template.Name), index, total, c.Creature.GUID, displayID)

	start0 := time.Now()
	optStart := time.Now()

	attachItems := map[string]AttachItem{}
	if c.Equipment != nil {
		if c.Equipment.Item1 != nil {
			if attachmentID, ok := InventoryTypeToEquipmentSlot(c.Equipment.Item1.InventoryType, 0); ok {
				attachItems[strconv.Itoa(int(attachmentID))] = AttachItem{
					Path: WowheadRef(fmt.Sprintf("https://www.wowhead.com/wotlk/item=%d", c.Equipment.Item1.Entry)),
				}
			} else {
				log.Printf("Unmapped item 1: %d %d", c.Equipment.Item1.Entry, c.Equipment.Item1.Entry)
			}
		}
		if c.Equipment.Item2 != nil {
			if attachmentID, ok := InventoryTypeToEquipmentSlot(c.Equipment.Item2.InventoryType, 1); ok {
				attachItems[strconv.Itoa(int(attachmentID))] = AttachItem{
					Path: WowheadRef(fmt.Sprintf("https://www.wowhead.com/wotlk/item=%d", c.Equipment.Item2.Entry)),
				}
			} else {
				log.Printf("Unmapped item 2: %d %d", c.Equipment.Item2.Entry, c.Equipment.Item2.Entry)
			}
		}
	}

	ex := NewCharacterExporter(cfg, wowClient)
	ch := Character{
		Base:            DisplayRef(displayID),
		InGameMovespeed: 270,
		AttachItems:     attachItems,
	}
	if c.Equipment != nil {
		invR, invL := 0, 0
		if c.Equipment.Item1 != nil {
			invR = c.Equipment.Item1.InventoryType
		}
		if c.Equipment.Item2 != nil {
			invL = c.Equipment.Item2.InventoryType
		}
		ch.AttackTag = GuessAttackTag(invR, invL)
	}
	log.Printf("Attack tag: %s", ch.AttackTag)
	if _, err := ex.ExportCharacter(ctx, ch, fileName); err != nil {
		return fmt.Errorf("export creature %s: %w", fileName, err)
	}
	ex.OptimizeModelsTextures(DefaultExportOptimization())
	log.Printf("optimize models and textures took %s", ansi.Yellowf("%.2fs", time.Since(optStart).Seconds()))

	writeStart := time.Now()
	format := "mdl"
	if cfg.MDX {
		format = "mdx"
	}
	if _, err := ex.WriteAllTextures(outputPath); err != nil {
		return fmt.Errorf("write creature textures %s: %w", fileName, err)
	}
	if _, err := ex.WriteAllModels(outputPath, format); err != nil {
		return fmt.Errorf("write creature models %s: %w", fileName, err)
	}
	log.Printf("write models and textures took %s", ansi.Yellowf("%.2fs", time.Since(writeStart).Seconds()))
	log.Printf("%s %s",
		ansi.Greenf("=> Exported creature %s in", c.Template.Name),
		ansi.Yellowf("%.2fs", time.Since(start0).Seconds()))
	return nil
}

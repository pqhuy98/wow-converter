package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	"github.com/pqhuy98/wow-converter/internal/testcases"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

func main() {
	fresh := flag.Bool("fresh", false, "delete existing mdx/blp before export")
	limit := flag.Int("limit", 0, "run first N test cases only")
	offset := flag.Int("offset", 0, "skip first N test cases")
	classic := flag.Bool("classic", false, "use classic test cases")
	mount := flag.Bool("mount", false, "use mount rider+mount test cases")
	casesFile := flag.String("cases-file", "", "JSON file with random local M2/WMO parity cases")
	outDir := flag.String("out", "", "output directory (defaults to regression map)")
	format := flag.String("format", "mdx", "model output format: mdx or mdl")
	compare := flag.Bool("compare", false, "compare exported assets to baseline manifest")
	snapshotOnly := flag.Bool("snapshot", false, "write manifest.json only (no export)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()

	wowClient := client.NewHTTPClient(os.Getenv("WOW_DATA_SERVER_URL"))
	if err := wowClient.WaitUntilReady(ctx); err != nil {
		log.Fatalf("wow-data-server not ready: %v", err)
	}

	cases := testcases.RetailCases()
	caseCount := len(cases)
	var randomCases []testcases.RandomM2Case
	if *casesFile != "" {
		payload, err := testcases.LoadRandomM2Cases(*casesFile)
		if err != nil {
			log.Fatalf("load random cases: %v", err)
		}
		randomCases = payload.Cases
		caseCount = len(randomCases)
	}
	mapDir := workspace.ResolveRepoPath("maps/test-regression-retail.w3x")
	manifestPath := filepath.Join(mapDir, "manifest.json")
	if *mount {
		caseCount = len(testcases.MountCases())
		mapDir = workspace.ResolveRepoPath("maps/test-regression-mount.w3x")
		manifestPath = filepath.Join(mapDir, "manifest.json")
	} else if *classic && *casesFile == "" {
		cases = testcases.ClassicCases()
		caseCount = len(cases)
		mapDir = workspace.ResolveRepoPath("maps/test-regression-classic.w3x")
		manifestPath = filepath.Join(mapDir, "manifest.json")
	}
	if *outDir != "" {
		mapDir = *outDir
		manifestPath = filepath.Join(mapDir, "manifest.json")
	}
	startIdx := *offset
	endIdx := caseCount
	if *limit > 0 {
		endIdx = min(caseCount, startIdx+*limit)
	}
	if startIdx >= endIdx {
		cases = nil
	}

	if *fresh {
		_ = filepath.Walk(mapDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".mdx" || ext == ".mdl" || ext == ".blp" {
				return os.Remove(path)
			}
			return nil
		})
	}

	if *snapshotOnly {
		manifest, err := snapshot.Create(mapDir)
		if err != nil {
			log.Fatalf("snapshot: %v", err)
		}
		if err := snapshot.WriteManifest(manifestPath, manifest); err != nil {
			log.Fatalf("write manifest: %v", err)
		}
		fmt.Printf("Wrote snapshot %s (%d files)\n", manifestPath, len(manifest.Files))
		return
	}

	cfg := config.DefaultConfig()
	cfg.MaxTextureSize = 512
	cfg.OverrideModels = *casesFile != ""

	var mapUnits []mapUnitEntry
	for i := startIdx; i < endIdx; i++ {
		var ch character.Character
		var name string
		var particles *float64
		var exportOpts []character.ExportOptions
		if *mount {
			mc := testcases.MountCases()[i]
			ch = buildMountCharacter(mc)
			name = fmt.Sprintf("mount-case-%d", i)
		} else if len(randomCases) > 0 {
			rc := randomCases[i]
			ch = buildRandomM2Character(rc)
			name = rc.OutputName
			if rc.SkinID != "" && rc.ModelType != "wmo" {
				exportOpts = []character.ExportOptions{{LocalModelSkinID: rc.SkinID}}
			}
		} else {
			tc := cases[i]
			ch = buildCharacter(tc)
			name = deriveName(tc.Base)
			particles = tc.ParticlesDensity
		}
		mapUnits = append(mapUnits, mapUnitEntry{Name: name})

		outPath := filepath.Join(mapDir, name+"."+*format)
		if _, err := os.Stat(outPath); err == nil && !cfg.OverrideModels {
			log.Printf("Skipping existing %s", name)
			continue
		}

		log.Printf("Exporting %d/%d: %s", i-startIdx+1, endIdx-startIdx, name)
		start := time.Now()
		caseExporter := character.NewCharacterExporter(cfg, wowClient)
		exportedMdl, err := caseExporter.ExportCharacter(ctx, ch, name, exportOpts...)
		if err != nil {
			log.Fatalf("export %s: %v", name, err)
		}
		caseExporter.OptimizeModelsTextures(character.ExportOptimization{
			ParticlesDensity: particles,
			SortSequences: true, RemoveUnusedVertices: true, RemoveUnusedNodes: true,
			RemoveUnusedMaterialsTextures: true, FormatVersion: "1000",
		})
		if _, err := caseExporter.WriteAllTextures(mapDir); err != nil {
			log.Fatalf("textures %s: %v", name, err)
		}
		if _, err := caseExporter.WriteAllModels(mapDir, *format); err != nil {
			log.Fatalf("models %s: %v", name, err)
		}
		mapUnits[len(mapUnits)-1].MDL = exportedMdl
		log.Printf("Done %s in %.2fs", name, time.Since(start).Seconds())
	}

	if err := populateRegressionMapUnits(mapDir, *format, mapUnits); err != nil {
		log.Fatalf("populate map units: %v", err)
	}

	fmt.Printf("Test export complete: %d cases -> %s\n", endIdx-startIdx, mapDir)

	if *compare {
		baseline, err := snapshot.LoadManifest(manifestPath)
		if err != nil {
			log.Fatalf("load baseline manifest %s: %v (run with -snapshot first)", manifestPath, err)
		}
		tol := regexp.MustCompile(`\.(blp|png)$`)
		summary, err := snapshot.CompareManifestToDir(baseline, mapDir, snapshot.CompareOptions{
			ToleranceRegex: tol,
			MaxDelta:       2,
			BaselineDir:    mapDir,
		})
		if err != nil {
			log.Fatalf("compare: %v", err)
		}
		pass := snapshot.PrintSummary(os.Stdout, summary)
		if !pass {
			os.Exit(1)
		}
	}
}

func buildCharacter(tc testcases.TestCase) character.Character {
	pd := 0.5
	char := character.Character{
		Base:             character.WowheadRef(tc.Base),
		InGameMovespeed:  270,
		Scale:            1.5,
		ParticlesDensity: &pd,
		AttackTag:        animmap.AttackTagAuto,
	}
	if tc.Size != "" {
		char.Size = tc.Size
	}
	if tc.WeaponR != "" && tc.WeaponL == "" || tc.WeaponL != "" && tc.WeaponR == "" {
		char.AttackTag = animmap.AttackTag2H
	}
	if tc.WeaponR != "" && tc.WeaponL != "" {
		char.AttackTag = animmap.AttackTag1H
	}
	if tc.WeaponR != "" || tc.WeaponL != "" {
		char.AttachItems = map[string]character.AttachItem{}
		if tc.WeaponR != "" {
			char.AttachItems["1"] = character.AttachItem{Path: character.WowheadRef(tc.WeaponR), Scale: 1}
		}
		if tc.WeaponL != "" {
			char.AttachItems["2"] = character.AttachItem{Path: character.WowheadRef(tc.WeaponL), Scale: 1}
		}
	}
	if strings.HasPrefix(tc.Base, "local::") {
		localPath := strings.TrimPrefix(tc.Base, "local::")
		localPath = strings.ReplaceAll(localPath, `\\`, `\`)
		char.Base = character.LocalRef(localPath)
	}
	return char
}

func buildRandomM2Character(rc testcases.RandomM2Case) character.Character {
	pd := 0.5
	return character.Character{
		Base:             character.LocalRef(rc.LocalRef),
		InGameMovespeed:  270,
		Scale:            1.5,
		ParticlesDensity: &pd,
		AttackTag:        animmap.AttackTagAuto,
	}
}

func buildMountCharacter(mc testcases.MountCase) character.Character {
	char := character.Character{
		Base:            character.WowheadRef(mc.RiderBase),
		InGameMovespeed: 270,
		AttackTag:       animmap.AttackTagAuto,
		Mount:           &character.Mount{Path: character.WowheadRef(mc.MountPath)},
	}
	if mc.Size != "" {
		char.Size = mc.Size
	}
	if mc.MountScale > 0 {
		char.Mount.Scale = mc.MountScale
	}
	if len(mc.SeatOffset) == 3 {
		char.Mount.SeatOffset = mc.SeatOffset
	}
	return char
}

func deriveName(base string) string {
	if strings.HasPrefix(base, "local::") {
		name := filepath.Base(strings.TrimPrefix(base, "local::"))
		name = strings.TrimSuffix(name, filepath.Ext(name))
		return "local-" + name
	}
	if strings.Contains(base, "npc=") {
		parts := strings.Split(base, "npc=")
		id := strings.Split(parts[1], "/")[0]
		segs := strings.Split(base, "/")
		name := strings.Split(segs[len(segs)-1], "#")[0]
		return fmt.Sprintf("npc-%s-%s", name, id)
	}
	if strings.Contains(base, "object=") {
		parts := strings.Split(base, "object=")
		id := strings.Split(parts[1], "/")[0]
		segs := strings.Split(base, "/")
		name := strings.Split(segs[len(segs)-1], "#")[0]
		return fmt.Sprintf("object-%s-%s", name, id)
	}
	if strings.Contains(base, "item=") {
		parts := strings.Split(base, "item=")
		id := strings.Split(parts[1], "/")[0]
		segs := strings.Split(base, "/")
		name := strings.Split(segs[len(segs)-1], "#")[0]
		return fmt.Sprintf("item-%s-%s", name, id)
	}
	if strings.Contains(base, "dressing-room") {
		q := strings.Split(base, "?")
		if len(q) > 1 {
			return "dressing-room-" + strings.Split(q[1], "#")[0]
		}
	}
	return "export-" + strings.ReplaceAll(base, "/", "_")
}

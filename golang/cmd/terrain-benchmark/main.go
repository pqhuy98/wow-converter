// terrain-benchmark captures production WoW bakes and evaluates experimental
// native terrain fits. It never modifies existing exported maps.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/converter/terrainfit"
	"github.com/pqhuy98/wow-converter/internal/server/reportshot"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

func main() {
	outFlag := flag.String("out", "exported-assets/terrain-benchmark", "output directory relative to repository")
	sourceFlag := flag.String("source", "", "reuse captures from this directory (default OUT/source)")
	mode := flag.String("mode", "all", "capture, materials, material-fit, fit, refine, views, or all")
	materialRegion := flag.String("material-region", "", "limit material-fit to one named benchmark region")
	viewRegion := flag.String("view-region", "", "limit browser captures to one named benchmark region")
	flag.Parse()
	if _, err := workspace.ChdirRepoRoot(); err != nil {
		log.Fatal(err)
	}
	if err := workspace.LoadEnvFile(".env"); err != nil {
		log.Fatal(err)
	}
	out, err := filepath.Abs(*outFlag)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		log.Fatal(err)
	}
	sourceRoot := filepath.Join(out, "source")
	if *sourceFlag != "" {
		sourceRoot, err = filepath.Abs(*sourceFlag)
		if err != nil {
			log.Fatal(err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	regions := []terrainfit.Region{
		{Name: "Azeroth-34_28-36_29", Map: "Azeroth", MapID: 0, X0: 34, Y0: 28, X1: 36, Y1: 29},
		{Name: "Azeroth-32_48-32_49", Map: "Azeroth", MapID: 0, X0: 32, Y0: 48, X1: 32, Y1: 49},
		{Name: "Northrend-27_17-29_18", Map: "Northrend", MapID: 571, X0: 27, Y0: 17, X1: 29, Y1: 18},
		{Name: "Northrend-29_15-31_16", Map: "Northrend", MapID: 571, X0: 29, Y0: 15, X1: 31, Y1: 16},
	}
	if *mode != "all" && *mode != "capture" && *mode != "materials" && *mode != "material-fit" && *mode != "fit" && *mode != "refine" && *mode != "views" {
		log.Fatal("unknown mode")
	}
	if *mode == "all" || *mode == "capture" || *mode == "materials" {
		if _, err = bootstrap.StartWowDataServer(ctx); err != nil {
			log.Fatal(err)
		}
		source := server.GlobalRuntime.GetCascOptional()
		if source == nil {
			log.Fatal("CASC not loaded; configure .env")
		}
		server.WowConfig.SplitLargeTerrainBakes = false
		// Capture holes through mesh coverage, not black pixels or minimap imagery.
		server.WowConfig.MapsIncludeHoles = true
		for _, region := range regions {
			for y := region.Y0; y <= region.Y1; y++ {
				for x := region.X0; x <= region.X1; x++ {
					if *mode == "materials" {
						if err = terrainfit.CaptureMaterials(ctx, source, sourceRoot, region, x, y); err != nil {
							log.Fatal(err)
						}
						adt.ClearBakeTextureCache()
						continue
					}
					if err = ctx.Err(); err != nil {
						log.Fatal(err)
					}
					name := fmt.Sprintf("%s-%d_%d", region.Map, x, y)
					dir := filepath.Join(sourceRoot, name)
					if *mode == "all" {
						if err = terrainfit.CaptureMaterials(ctx, source, sourceRoot, region, x, y); err != nil {
							log.Fatal(err)
						}
						adt.ClearBakeTextureCache()
					}
					if terrainfit.CaptureValid(dir, source.GetBuildName()) {
						fmt.Println("CACHED", name)
						continue
					}
					fmt.Println("CAPTURE", name, source.GetBuildName())
					conv, e := adt.NewExporter(region.MapID, region.Map, x*64+y).ExportForConversion(ctx, source, dir, 8192, export.ADTExportOptions{OverwriteFiles: true, MapsIncludeHoles: true}, nil, nil)
					if e != nil {
						log.Fatal(name, ": ", e)
					}
					if err = terrainfit.SaveCapture(dir, source.GetBuildName(), conv); err != nil {
						log.Fatal(name, ": ", err)
					}
					adt.ClearBakeTextureCache()
				}
			}
		}
	}
	if *mode == "fit" || *mode == "all" {
		if err = terrainfit.Benchmark(ctx, out, sourceRoot, regions); err != nil {
			log.Fatal(err)
		}
	}
	if *mode == "all" || *mode == "fit" || *mode == "refine" {
		if err = terrainfit.Refine(ctx, out); err != nil {
			log.Fatal(err)
		}
	}
	if *mode == "material-fit" || *mode == "all" {
		if err = terrainfit.MaterialBenchmark(ctx, out, sourceRoot, *materialRegion); err != nil {
			log.Fatal(err)
		}
	}
	if *mode == "views" || *mode == "all" {
		if err = reportshot.CaptureTerrainBenchmark(ctx, out, *viewRegion); err != nil {
			log.Fatal(err)
		}
	}
	data, err := json.MarshalIndent(regions, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "regions.json"), data, 0644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("DONE", out)
}

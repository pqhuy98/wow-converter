package wmoexport

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	exportpkg "github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

// DoodadSetMaskEntry selects which WMO doodad sets to export.
type DoodadSetMaskEntry struct {
	Checked bool
}

// Exporter writes WMO interior doodad placement CSV for ADT map export.
type Exporter struct {
	Loader        *wmo.Loader
	doodadSetMask []DoodadSetMaskEntry
}

// NewExporter creates an exporter from raw WMO bytes.
func NewExporter(data []byte, fileDataID int) *Exporter {
	return &Exporter{
		Loader: wmo.NewLoader(buffer.From(data), fileDataID, "", false),
	}
}

// SetDoodadSetMask sets which doodad sets are included in the placement CSV.
func (e *Exporter) SetDoodadSetMask(mask []DoodadSetMaskEntry) {
	e.doodadSetMask = mask
}

// ExportDoodadPlacementCsv writes interior M2 placements for enabled doodad sets.
func (e *Exporter) ExportDoodadPlacementCsv(
	out string,
	options exportpkg.ADTExportOptions,
	progress *exportpkg.ProgressReporter,
) error {
	if err := e.Loader.Load(); err != nil {
		return err
	}

	csvPath := exportpkg.PlacementCsvPath(out)
	if !options.OverwriteFiles && writers.OutputFileExists(csvPath) {
		log.Write("Skipping model placement export %s (file exists, overwrite disabled)", csvPath)
		return nil
	}

	useAbsolute := options.EnableAbsoluteCSVPaths
	usePosix := options.PathFormat == "posix"
	outDir := filepath.Dir(out)
	csv := &writers.CSVWriter{Out: csvPath}
	csv.AddField(
		"ModelFile", "PositionX", "PositionY", "PositionZ",
		"RotationW", "RotationX", "RotationY", "RotationZ",
		"ScaleFactor", "DoodadSet", "FileDataID",
	)

	wmoLabel := strings.TrimSuffix(filepath.Base(out), filepath.Ext(out))
	doodadSets := e.Loader.DoodadSets
	for i, set := range doodadSets {
		if len(e.doodadSetMask) > i && !e.doodadSetMask[i].Checked {
			continue
		}

		count := int(set.DoodadCount)
		log.Write("Writing interior doodad placements for set %s (%d entries)...", set.Name, count)
		if progress != nil {
			progress.SetLabel(fmt.Sprintf("%s, %s", wmoLabel, set.Name), 0, count)
		}

		for j := uint32(0); j < set.DoodadCount; j++ {
			if progress != nil && j > 0 && j%50 == 0 {
				progress.SetLabel(fmt.Sprintf("%s, %s", wmoLabel, set.Name), int(j), count)
			}
			idx := set.FirstInstanceIndex + j
			if int(idx) >= len(e.Loader.Doodads) {
				continue
			}
			doodad := e.Loader.Doodads[idx]

			var fileDataID uint32
			if len(e.Loader.FileDataIDs) > 0 {
				if int(doodad.Offset) < len(e.Loader.FileDataIDs) {
					fileDataID = e.Loader.FileDataIDs[doodad.Offset]
				}
			} else if e.Loader.DoodadNames != nil {
				fileName := e.Loader.DoodadNames[int(doodad.Offset)]
				if id, ok := archivecasc.GetByFilename(fileName); ok {
					fileDataID = uint32(id)
				}
			}
			if fileDataID == 0 {
				continue
			}

			refName := exportpkg.ModelReferencePath(fileDataID, "m2", nil)
			var m2Path string
			if options.EnableSharedChildren {
				m2Path = writers.GetExportPath(refName)
			} else {
				m2Path = writers.ReplaceFile(out, refName)
			}

			modelPath, err := filepath.Rel(outDir, m2Path)
			if err != nil {
				modelPath = m2Path
			}
			if useAbsolute {
				modelPath, _ = filepath.Abs(filepath.Join(outDir, modelPath))
			}
			if usePosix {
				modelPath = writers.Win32ToPosix(modelPath)
			}

			rot := doodad.Rotation
			rotW, rotX, rotY, rotZ := float64(0), float64(0), float64(0), float64(0)
			if len(rot) > 0 {
				rotX = float64(rot[0])
			}
			if len(rot) > 1 {
				rotY = float64(rot[1])
			}
			if len(rot) > 2 {
				rotZ = float64(rot[2])
			}
			if len(rot) > 3 {
				rotW = float64(rot[3])
			}

			pos := doodad.Position
			csv.AddRow(map[string]any{
				"ModelFile":   modelPath,
				"PositionX":   posAt(pos, 0),
				"PositionY":   posAt(pos, 1),
				"PositionZ":   posAt(pos, 2),
				"RotationW":   rotW,
				"RotationX":   rotX,
				"RotationY":   rotY,
				"RotationZ":   rotZ,
				"ScaleFactor": doodad.Scale,
				"DoodadSet":   set.Name,
				"FileDataID":  fileDataID,
			})
		}
	}

	return csv.Write(options.OverwriteFiles)
}

func posAt(v []float32, i int) float64 {
	if i < len(v) {
		return float64(v[i])
	}
	return 0
}

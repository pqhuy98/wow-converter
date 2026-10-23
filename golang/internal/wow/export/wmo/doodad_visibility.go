package wmoexport

import (
	"context"
	"fmt"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

// ExcludeInteriorDoodads omits only placements referenced exclusively by indoor
// groups. Unreferenced, exterior and ambiguous placements remain included.
// On incomplete group data the exclusion mask stays empty, keeping all props.
func (e *Exporter) ExcludeInteriorDoodads(ctx context.Context, getFile func(context.Context, uint32) ([]byte, error)) error {
	e.excludeDoodads = nil
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.Loader.Load(); err != nil {
		return err
	}
	root := e.Loader
	if len(root.Doodads) == 0 {
		return nil
	}
	inside := make([]bool, len(root.Doodads))
	other := make([]bool, len(root.Doodads))
	fileName, _ := archivecasc.GetByID(root.FileDataID)
	for i := 0; i < int(root.GroupCount); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var id uint32
		if len(root.GroupIDs) > 0 {
			if i < len(root.GroupIDs) {
				id = root.GroupIDs[i]
			}
		} else if strings.HasSuffix(strings.ToLower(fileName), ".wmo") {
			groupName := fileName[:len(fileName)-4] + fmt.Sprintf("_%03d.wmo", i)
			if groupID, ok := archivecasc.GetByFilename(groupName); ok {
				id = uint32(groupID)
			}
		}
		if id == 0 {
			return fmt.Errorf("unable to resolve WMO group %d", i)
		}
		raw, err := getFile(ctx, id)
		if err != nil {
			return fmt.Errorf("WMO group %d: %w", i, err)
		}
		group := wmo.NewLoader(buffer.From(raw), int(id), "", false)
		if err := group.Load(); err != nil {
			return err
		}
		// Both flags, or neither flag, are ambiguous rather than interior-only.
		interiorOnly := group.Flags&0x2000 != 0 && group.Flags&0x8 == 0
		for _, index := range group.DoodadRefs {
			if int(index) >= len(inside) {
				continue
			}
			if interiorOnly {
				inside[index] = true
			} else {
				other[index] = true
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := range inside {
		inside[i] = inside[i] && !other[i]
	}
	e.excludeDoodads = inside
	return nil
}
